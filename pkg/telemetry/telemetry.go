package telemetry

import "context"

// SpanInput configures a span or trace.
type SpanInput struct {
	Name       string
	Attributes map[string]any
}

// Span records trace information.
type Span interface {
	End(err error)
	AddEvent(name string, attrs map[string]any)
	SetAttribute(key string, value any)
}

// Client starts traces and spans.
type Client interface {
	StartTrace(ctx context.Context, input SpanInput) (Span, context.Context)
	StartSpan(ctx context.Context, input SpanInput) (Span, context.Context)
	Close(ctx context.Context) error
}
