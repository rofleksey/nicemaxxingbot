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

func (s *Service) checkToxicity(ctx context.Context, text string, useFreeClient bool) (string, bool, error) {
	resultStr := ""
	result := false

	attempts := 3

	err := retry.Do(func() error {
		phrase, isToxic, err := s.client.CheckToxicity(ctx, text, useFreeClient)
		if err != nil {
			return fmt.Errorf("CheckToxicity: %w", err)
		}

		resultStr = phrase
		result = isToxic

		return nil
	}, retry.Context(ctx), retry.Attempts(uint(attempts)), retry.Delay(time.Second*5))
	if err != nil {
		return "", false, fmt.Errorf("retry.Do: %w", err)
	}

	return resultStr, result, nil
}

func (s *Service) ProcessTranscription(ctx context.Context, text string) (string, bool, error) {
	slogger := slog.With(slog.String("text", text))
	slogger.Debug("Processing text...")

	start := time.Now()
	slogger.Debug("Using free ai to check for toxicity...")
	_, isToxic, err := s.checkToxicity(ctx, text, true)
	if err != nil {
		return "", false, fmt.Errorf("checkToxicity(free): %w", err)
	}

	slogger.Debug("Checked toxicity (free)",
		slog.Bool("result", isToxic),
		slog.Duration("duration", time.Since(start)),
	)

	if !isToxic {
		return "", false, nil
	}

	// only use paid client to confirm
	start = time.Now()
	slogger.Debug("Confirming with paid ai..")
	phrase, isToxic, err := s.checkToxicity(ctx, text, false)
	if err != nil {
		return "", false, fmt.Errorf("checkToxicity(paid): %w", err)
	}

	slogger.Debug("Checked toxicity (paid)",
		slog.Bool("result", isToxic),
		slog.Duration("duration", time.Since(start)),
	)
	if !isToxic {
		return "", false, nil
	}

	return phrase, true, nil
}
