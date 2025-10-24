package toxic

import (
	"context"
	"fmt"
	"log/slog"
	"nicemaxxingbot/app/client/openai"
	"nicemaxxingbot/app/client/whisper"
	"time"

	"github.com/avast/retry-go"
	"github.com/samber/do"
)

type Service struct {
	client        *openai.Client
	whisperClient *whisper.Client
}

func New(di *do.Injector) (*Service, error) {
	return &Service{
		client:        do.MustInvoke[*openai.Client](di),
		whisperClient: do.MustInvoke[*whisper.Client](di),
	}, nil
}

func (s *Service) checkToxicity(ctx context.Context, text string, useFreeClient bool) (*openai.AnalyzeResult, error) {
	var result *openai.AnalyzeResult

	attempts := 3

	err := retry.Do(func() error {
		res, err := s.client.Analyze(ctx, text, useFreeClient)
		if err != nil {
			return fmt.Errorf("CheckToxicity: %w", err)
		}

		result = res

		return nil
	}, retry.Context(ctx), retry.Attempts(uint(attempts)), retry.Delay(time.Second*5))
	if err != nil {
		return nil, fmt.Errorf("retry.Do: %w", err)
	}

	return result, nil
}

func (s *Service) ProcessTranscription(ctx context.Context, text string) (*openai.AnalyzeResult, error) {
	slogger := slog.With(slog.String("text", text))
	slogger.Debug("Processing text...")

	start := time.Now()
	slogger.Debug("Using free ai to check for toxicity...")

	toxicResult, err := s.checkToxicity(ctx, text, true)
	if err != nil {
		return nil, fmt.Errorf("checkToxicity(free): %w", err)
	}

	slogger.Debug("Checked toxicity (free)",
		slog.Bool("result", toxicResult.Toxic),
		slog.Duration("duration", time.Since(start)),
	)
	if !toxicResult.Toxic {
		return toxicResult, nil
	}

	// only use paid client to confirm
	start = time.Now()
	slogger.Debug("Confirming with paid ai..")

	toxicResult, err = s.checkToxicity(ctx, text, false)
	if err != nil {
		return nil, fmt.Errorf("checkToxicity(paid): %w", err)
	}

	slogger.Debug("Checked toxicity (paid)",
		slog.Bool("result", toxicResult.Toxic),
		slog.Duration("duration", time.Since(start)),
	)

	return toxicResult, nil
}
