package stream

import (
	"context"
	"fmt"
	"log/slog"
	"nicemaxxingbot/app/client/twitch"
	"nicemaxxingbot/app/client/twitch_live"
	"nicemaxxingbot/app/client/whisper"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"nicemaxxingbot/app/config"
	"nicemaxxingbot/app/service/toxic"

	"github.com/elliotchance/pie/v2"
	"github.com/ozgio/strutil"
	"github.com/samber/do"
)

const maxMessageLength = 400
const dataDir = "data"
const notificationFormat = "Nicemaxxing streak is over pingus It lasted for ~%d minutes pingus Toxic phrase: %s"
const turnOffText = "pingus Bot is muted for 12 hours pingus"
const turnOnText = "pingus Bot is back in action pingus"

type Service struct {
	cfg              *config.Config
	twitchClient     *twitch.Client
	whisperClient    *whisper.Client
	twitchLiveClient *twitch_live.Client
	toxicService     *toxic.Service

	m           sync.Mutex
	savedTime   time.Time
	turnOffTime time.Time

	textChan chan string
	wg       sync.WaitGroup
}

func New(di *do.Injector) (*Service, error) {
	return &Service{
		cfg:              do.MustInvoke[*config.Config](di),
		twitchClient:     do.MustInvoke[*twitch.Client](di),
		whisperClient:    do.MustInvoke[*whisper.Client](di),
		twitchLiveClient: do.MustInvoke[*twitch_live.Client](di),
		toxicService:     do.MustInvoke[*toxic.Service](di),
		textChan:         make(chan string, 1),
	}, nil
}

func (s *Service) Run(ctx context.Context) {
	defer close(s.textChan)
	defer s.wg.Wait()

	s.wg.Go(func() {
		s.runWorker(ctx)
	})
}

func (s *Service) runWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.processStream(ctx)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				continue
			}
		}
	}
}

func (s *Service) processStream(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	streamQualityArr, err := s.twitchLiveClient.GetM3U8(ctx, s.cfg.Streamer)
	if err != nil {
		slog.Warn("Failed to get stream URL",
			slog.String("error", err.Error()),
		)
		return
	}

	if len(streamQualityArr) == 0 {
		slog.Warn("No stream URL found")
		return
	}

	processingTimeout := time.Duration(s.cfg.Processing.BatchTimeout) * time.Second

	textProcessor := NewStringAccumulator(s.textChan, s.cfg.Processing.BatchSize, processingTimeout, s.processText)
	textProcessor.Start(ctx)
	defer textProcessor.Shutdown()

	s.m.Lock()
	s.savedTime = time.Now()
	s.m.Unlock()

	streamQualityIndex := pie.FindFirstUsing(streamQualityArr, func(q twitch_live.StreamQuality) bool {
		return q.Quality == "audio_only"
	})
	if streamQualityIndex < 0 {
		streamQualityIndex = 0
	}
	streamQuality := streamQualityArr[streamQualityIndex]

	slog.Info("Got stream URL",
		slog.String("quality", streamQuality.Quality),
		slog.String("resolution", streamQuality.Resolution),
		slog.String("url", streamQuality.URL),
	)

	m3u8URL := streamQuality.URL

	_ = os.RemoveAll(dataDir)

	if err = os.MkdirAll(dataDir, 0755); err != nil {
		slog.Error("Failed to create data dir",
			slog.Any("error", err),
		)
		return
	}

	args := []string{
		"-timeout", "10000000", // 10s
		"-reconnect", "0",
		"-reconnect_at_eof", "0",
		"-reconnect_streamed", "0",
		"-reconnect_delay_max", "0",
		"-i", m3u8URL,
		"-f", "segment",
		"-segment_time", "30",
		"-reset_timestamps", "1",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		filepath.Join(dataDir, "chunk_%04d.wav"),
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	defer cmd.Cancel()

	go func() {
		defer cancel()

		if err := cmd.Run(); err != nil {
			slog.Error("FFMpeg failed",
				slog.Any("error", err),
			)
			return
		}
	}()

	if err = s.processChunks(ctx, dataDir); err != nil {
		slog.Error("Failed to process chunks",
			slog.Any("error", err),
		)
	}

	cancel()
}

func (s *Service) processText(ctx context.Context, text string) {
	slogger := slog.With(
		slog.String("text", text),
	)

	slogger.Info("Processing text...")
	toxicResult, err := s.toxicService.ProcessTranscription(ctx, text)
	if err != nil {
		slogger.Error("Failed to process transcription",
			slog.Any("error", err),
		)
		return
	}

	if toxicResult.TurnOff {
		slogger.Info("Requested to turn the bot OFF",
			slog.Bool("telegram", true),
		)

		if !s.cfg.Twitch.DisableNotifications {
			if err = s.twitchClient.SendMessage(s.cfg.Streamer, turnOffText); err != nil {
				slogger.Error("Failed to send turn off notification",
					slog.String("phrase", toxicResult.Phrase),
				)
				return
			}
		}

		s.m.Lock()
		s.turnOffTime = time.Now().Add(12 * time.Hour)
		s.m.Unlock()

		return
	}

	if toxicResult.TurnOn {
		slogger.Info("Requested to turn the bot ON",
			slog.Bool("telegram", true),
		)

		if !s.cfg.Twitch.DisableNotifications {
			if err = s.twitchClient.SendMessage(s.cfg.Streamer, turnOnText); err != nil {
				slogger.Error("Failed to send turn on notification",
					slog.String("phrase", toxicResult.Phrase),
				)
				return
			}
		}

		s.m.Lock()
		s.turnOffTime = time.Time{}
		s.m.Unlock()

		return
	}

	if !toxicResult.Toxic {
		return
	}

	s.m.Lock()
	turnOffTime := s.turnOffTime
	s.m.Unlock()

	if s.cfg.Twitch.DisableNotifications {
		slogger.Info("Found toxic phrase, but notifications are disabled",
			slog.String("phrase", toxicResult.Phrase),
			slog.Bool("telegram", true),
		)
		return
	}

	if time.Now().Before(turnOffTime) {
		slogger.Info("Found toxic phrase, but bot is temporarily disabled",
			slog.String("phrase", toxicResult.Phrase),
			slog.Bool("telegram", true),
		)
		return
	}

	s.m.Lock()
	savedTime := s.savedTime
	s.savedTime = time.Now()
	s.m.Unlock()

	if savedTime.IsZero() {
		slogger.Error("No saved time found")
		return
	}

	streakDuration := time.Since(savedTime)
	streakDurationMinutes := int(streakDuration.Minutes())

	if streakDurationMinutes < s.cfg.Twitch.MinStreakLength {
		slogger.Info("Found toxic phrase, but streak is too low",
			slog.String("phrase", toxicResult.Phrase),
			slog.Bool("telegram", true),
		)
		return
	}

	notificationText := fmt.Sprintf(notificationFormat, streakDurationMinutes, toxicResult.Phrase)
	notificationText = strutil.Summary(notificationText, maxMessageLength, "...")

	if err = s.twitchClient.SendMessage(s.cfg.Streamer, notificationText); err != nil {
		slogger.Error("Failed to send notification",
			slog.String("phrase", toxicResult.Phrase),
		)
		return
	}

	slogger.Info("Found toxic phrase",
		slog.String("phrase", toxicResult.Phrase),
		slog.Bool("telegram", true),
	)
}

func (s *Service) processChunks(ctx context.Context, dataDir string) error {
	processedMap := make(map[string]struct{})
	lastNewChunkTime := time.Now()
	checkInterval := 5 * time.Second
	timeoutDuration := 2 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			files, err := filepath.Glob(filepath.Join(dataDir, "*.wav"))
			if err != nil {
				return fmt.Errorf("filepath.Glob: %w", err)
			}

			slices.Sort(files)
			fileCount := len(files)

			newChunkFound := false
			processedCount := 0

			for i, file := range files {
				file := file

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				// skip last file, as it might be incomplete
				if i == fileCount-1 {
					continue
				}

				if _, ok := processedMap[file]; ok {
					continue
				}

				newChunkFound = true
				processedMap[file] = struct{}{}
				processedCount++

				slog.Info("Processing chunk...",
					slog.String("file", file),
				)

				go func() {
					if err := s.processChunk(ctx, file); err != nil {
						slog.Error("Failed to process chunk",
							slog.String("file", file),
							slog.Any("error", err),
						)
						return
					}

					slog.Info("Chunk processed",
						slog.String("file", file),
					)
				}()
			}

			if newChunkFound {
				lastNewChunkTime = time.Now()
				slog.Debug("Got new chunks",
					slog.Int("count", processedCount),
					slog.Time("lastNewChunkTime", lastNewChunkTime),
				)
			} else {
				if time.Since(lastNewChunkTime) > timeoutDuration {
					return fmt.Errorf("no new chunks found for %v", timeoutDuration)
				}

				slog.Debug("No new chunks found",
					slog.Duration("timeSinceLastChunk", time.Since(lastNewChunkTime)),
					slog.Duration("timeoutIn", timeoutDuration-time.Since(lastNewChunkTime)),
				)
			}

			time.Sleep(checkInterval)
		}
	}
}

func (s *Service) transcribe(ctx context.Context, filePath string) (string, error) {
	slogger := slog.With(slog.String("filePath", filePath))

	start := time.Now()
	slogger.Debug("Transcribing file...")

	text, err := s.whisperClient.TranscribeFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("TranscribeFile: %w", err)
	}

	slogger = slog.With(slog.String("text", text))
	slogger.Debug("Got text",
		slog.Duration("duration", time.Since(start)),
	)

	return text, nil
}

func (s *Service) processChunk(ctx context.Context, filePath string) error {
	defer os.Remove(filePath)

	text, err := s.transcribe(ctx, filePath)
	if err != nil {
		return fmt.Errorf("transcribe: %w", err)
	}

	s.textChan <- text

	return nil
}
