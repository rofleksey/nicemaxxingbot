package mylog

import (
	"context"
	"log/slog"
	"nicemaxxingbot/app/config"
	"nicemaxxingbot/app/util/telemetry"
	"os"

	"github.com/phsym/console-slog"
	slogmulti "github.com/samber/slog-multi"
	slogtelegram "github.com/samber/slog-telegram/v2"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.szostok.io/version"
)

func Preinit() {
	slog.SetDefault(slog.New(console.NewHandler(os.Stderr, &console.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})))
}

func Init(cfg *config.Config, tel *telemetry.Telemetry) error {
	importantAttrs := []slog.Attr{
		slog.String(string(semconv.ServiceNameKey), cfg.ServiceName),
		slog.String(string(semconv.ServiceVersionKey), version.Get().Version),
	}
	router := slogmulti.Router()

	router = router.Add(console.NewHandler(os.Stderr, &console.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))

	if cfg.Telemetry.Enabled {
		router = router.Add(otelslog.NewHandler(cfg.ServiceName,
			otelslog.WithSource(true),
			otelslog.WithLoggerProvider(tel.LogProvider),
		).WithAttrs(importantAttrs))
	}

	if cfg.Log.Telegram.Token != "" {
		router = router.Add(
			slogtelegram.Option{
				Level:     slog.LevelDebug,
				Token:     cfg.Log.Telegram.Token,
				Username:  cfg.Log.Telegram.ChatID,
				AddSource: true,
			}.NewTelegramHandler().WithAttrs(importantAttrs),

			func(_ context.Context, r slog.Record) bool {
				hasTelegram := false

				r.Attrs(func(attr slog.Attr) bool {
					if attr.Key == "telegram" {
						hasTelegram = true
						return false
					}

					return true
				})

				return r.Level == slog.LevelError || hasTelegram
			},
		)
	}

	ctxHandler := &contextHandler{router.Handler()}

	logger := slog.New(ctxHandler)
	slog.SetDefault(logger)

	return nil
}
