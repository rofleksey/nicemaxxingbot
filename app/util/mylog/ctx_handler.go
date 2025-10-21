package mylog

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

var _ slog.Handler = (*contextHandler)(nil)

type contextHandler struct {
	handler slog.Handler
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{
		handler: h.handler.WithAttrs(attrs),
	}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{
		handler: h.handler.WithGroup(name),
	}
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(h.extractTelemetry(ctx)...)

	return h.handler.Handle(ctx, r) //nolint: wrapcheck
}

func (h *contextHandler) extractTelemetry(ctx context.Context) []slog.Attr {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return []slog.Attr{}
	}

	var attrs []slog.Attr
	spanCtx := span.SpanContext()

	if spanCtx.HasTraceID() {
		traceID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()
		attrs = append(attrs, slog.String("trace_id", traceID))
	}

	if spanCtx.HasSpanID() {
		spanID := spanCtx.SpanID().String()
		attrs = append(attrs, slog.String("span_id", spanID))
	}

	return attrs
}
