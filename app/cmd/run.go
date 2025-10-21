package cmd

import (
	"context"
	"log/slog"
	"nicemaxxingbot/app/client/openai"
	"nicemaxxingbot/app/client/twitch"
	"nicemaxxingbot/app/client/twitch_live"
	"nicemaxxingbot/app/client/whisper"
	"nicemaxxingbot/app/config"
	"nicemaxxingbot/app/service/stream"
	"nicemaxxingbot/app/service/toxic"
	"nicemaxxingbot/app/util/mylog"
	"nicemaxxingbot/app/util/telemetry"
	"os"
	"os/signal"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/samber/do"
	"github.com/spf13/cobra"
)

var configPath string

var Run = &cobra.Command{
	Use:   "run",
	Short: "Run notifier",
	Run:   runNotifier,
}

func init() {
	Run.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "Path to config yaml file (required)")
}

func runNotifier(_ *cobra.Command, _ []string) {
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	di := do.New()
	do.ProvideValue(di, appCtx)

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("Failed to load config",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:gocritic
		return
	}
	do.ProvideValue(di, cfg)

	if err = telemetry.InitSentry(cfg); err != nil {
		slog.Error("Failed to init sentry",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	defer sentry.Flush(3 * time.Second)

	tel, err := telemetry.Init(cfg)
	if err != nil {
		slog.Error("Failed to init telemetry",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	defer tel.Shutdown(appCtx)
	do.ProvideValue(di, tel)

	if err = mylog.Init(cfg, tel); err != nil {
		slog.Error("Failed to init logging",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	slog.InfoContext(appCtx, "Starting service...",
		slog.Bool("telegram", true),
	)

	metrics, err := telemetry.NewMetrics(cfg, tel.Meter)
	if err != nil {
		slog.Error("Failed to init metrics",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}
	do.ProvideValue(di, metrics)

	tracing := telemetry.NewTracing(cfg, tel.Tracer)
	do.ProvideValue(di, tracing)

	do.Provide(di, twitch.NewClient)
	do.Provide(di, twitch_live.NewClient)
	do.Provide(di, openai.NewClient)
	do.Provide(di, whisper.NewClient)
	do.Provide(di, toxic.New)
	do.Provide(di, stream.New)

	if err = do.MustInvoke[*openai.Client](di).Ping(appCtx); err != nil {
		slog.Error("Failed to init openai client",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}

	if err = do.MustInvoke[*whisper.Client](di).Ping(appCtx); err != nil {
		slog.Error("Failed to init whisper client",
			slog.Any("error", err),
		)
		os.Exit(1)
		return
	}

	go do.MustInvoke[*twitch.Client](di).RunRefreshLoop(appCtx)

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		slog.Info("Shutting down server...")

		cancel()
	}()

	do.MustInvoke[*stream.Service](di).Run(appCtx)

	slog.Info("Waiting for services to finish...")
	_ = di.Shutdown()
}
