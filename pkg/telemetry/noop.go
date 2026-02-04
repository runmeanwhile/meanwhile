package telemetry

import "context"

// NoopClient is a no-op telemetry client.
type NoopClient struct{}

// NewNoop returns a no-op client.
func NewNoop() *NoopClient { return &NoopClient{} }

// StartTrace starts a no-op trace.
func (n *NoopClient) StartTrace(ctx context.Context, _ SpanInput) (Span, context.Context) {
	return noopSpan{}, ctx
}

// StartSpan starts a no-op span.
func (n *NoopClient) StartSpan(ctx context.Context, _ SpanInput) (Span, context.Context) {
	return noopSpan{}, ctx
}

// Close is a no-op.
func (n *NoopClient) Close(ctx context.Context) error {
	_ = ctx
	return nil
}

type noopSpan struct{}

func (noopSpan) End(err error) { _ = err }
func (noopSpan) AddEvent(name string, attrs map[string]any) {
	_ = name
	_ = attrs
}
func (noopSpan) SetAttribute(key string, value any) {
	_ = key
	_ = value
}
