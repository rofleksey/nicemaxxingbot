package telemetry

import (
	"context"
	"fmt"
	"nicemaxxingbot/app/config"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Tracing struct {
	tracer oteltrace.Tracer
	cfg    *config.Config
}

func NewTracing(cfg *config.Config, tracer oteltrace.Tracer) *Tracing {
	return &Tracing{
		tracer: tracer,
		cfg:    cfg,
	}
}

func (t *Tracing) StartSpan(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	return t.tracer.Start(ctx, name, opts...) //nolint:spancheck
}

func (t *Tracing) StartServiceSpan(ctx context.Context, service, operation string) (context.Context, oteltrace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", service),
		attribute.String("service.operation", operation),
	}

	return t.tracer.Start(ctx, fmt.Sprintf("%s.%s", service, operation), //nolint:spancheck
		oteltrace.WithAttributes(attrs...),
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal),
	)
}

func (t *Tracing) Error(span oteltrace.Span, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())

	return err
}

func (t *Tracing) Success(span oteltrace.Span) {
	span.SetStatus(codes.Ok, "")
}

func (t *Tracing) AddAttributes(span oteltrace.Span, attrs map[string]interface{}) {
	for key, value := range attrs {
		switch v := value.(type) {
		case string:
			span.SetAttributes(attribute.String(key, v))
		case int:
			span.SetAttributes(attribute.Int(key, v))
		case int64:
			span.SetAttributes(attribute.Int64(key, v))
		case float64:
			span.SetAttributes(attribute.Float64(key, v))
		case bool:
			span.SetAttributes(attribute.Bool(key, v))
		default:
			span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}
}
