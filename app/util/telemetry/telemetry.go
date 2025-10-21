package telemetry

import (
	"context"
	"log/slog"
	"nicemaxxingbot/app/config"
	"time"

	"github.com/getsentry/sentry-go"
	sentryotel "github.com/getsentry/sentry-go/otel"
	"github.com/samber/oops"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	log2 "go.opentelemetry.io/otel/log"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.szostok.io/version"
)

type Telemetry struct {
	TracerProvider oteltrace.TracerProvider
	MeterProvider  otelmetric.MeterProvider
	LogProvider    *log.LoggerProvider
	Tracer         oteltrace.Tracer
	Meter          otelmetric.Meter
	Logger         log2.Logger
	Shutdown       func(context.Context) error
}

func Init(cfg *config.Config) (*Telemetry, error) {
	if !cfg.Telemetry.Enabled {
		noopTracerProvider := noop.NewTracerProvider()
		noopMeterProvider := sdkmetric.NewMeterProvider()
		noopLoggerProvider := log.NewLoggerProvider()

		return &Telemetry{
			TracerProvider: noopTracerProvider,
			MeterProvider:  noopMeterProvider,
			LogProvider:    noopLoggerProvider,
			Tracer:         noopTracerProvider.Tracer(cfg.ServiceName),
			Meter:          noopMeterProvider.Meter(cfg.ServiceName),
			Logger:         noopLoggerProvider.Logger(cfg.ServiceName),
			Shutdown:       func(context.Context) error { return nil },
		}, nil
	}

	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(version.Get().Version),
			semconv.DeploymentEnvironmentKey.String("production"),
		),
	)
	if err != nil {
		return nil, oops.Errorf("failed to create resource: %w", err)
	}

	var shutdownFuncs []func(context.Context) error

	tracerProvider, err := initTracerProvider(ctx, res)
	if err != nil {
		return nil, oops.Errorf("failed to initialize tracer provider: %w", err)
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)

	meterProvider, err := initMeterProvider(ctx, res)
	if err != nil {
		return nil, oops.Errorf("failed to initialize meter provider: %w", err)
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)

	loggerProvider, err := initLogProvider(ctx, res)
	if err != nil {
		return nil, oops.Errorf("failed to initialize logger provider: %w", err)
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		sentryotel.NewSentryPropagator(),
	))

	tracer := tracerProvider.Tracer(cfg.ServiceName)
	meter := meterProvider.Meter(cfg.ServiceName)
	logger := loggerProvider.Logger(cfg.ServiceName)

	shutdown := func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFuncs {
			if err := fn(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return oops.Errorf("shutdown errors: %v", errs)
		}
		return nil
	}

	slog.InfoContext(ctx, "Telemetry initialized")

	return &Telemetry{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		LogProvider:    loggerProvider,
		Tracer:         tracer,
		Meter:          meter,
		Logger:         logger,
		Shutdown:       shutdown,
	}, nil
}

func initTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, oops.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sentryotel.NewSentrySpanProcessor()),
	), nil
}

func initMeterProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, oops.Errorf("failed to create OTLP meter exporter: %w", err)
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(time.Minute),
		)),
		sdkmetric.WithResource(res),
	), nil
}

func initLogProvider(ctx context.Context, res *resource.Resource) (*log.LoggerProvider, error) {
	exporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, oops.Errorf("failed to create OTLP logs exporter: %w", err)
	}

	return log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(exporter)),
		log.WithResource(res),
	), nil
}

func InitSentry(cfg *config.Config) error {
	if cfg.Sentry.DSN == "" {
		return nil
	}

	return sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.Sentry.DSN,
		Environment:      "production",
		AttachStacktrace: true,
	})
}
