package langfuse

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

const defaultEndpoint = "https://cloud.langfuse.com/api/public/otel"

var (
	// ErrMissingKeys indicates missing auth keys.
	ErrMissingKeys = errors.New("missing langfuse public/secret key")
)

// Config configures the Langfuse client.
type Config struct {
	PublicKey   string
	SecretKey   string
	Endpoint    string
	ServiceName string
	Timeout     time.Duration
}

// Client implements telemetry.Client using Langfuse OTLP.
type Client struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// NewClient creates a Langfuse telemetry client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.PublicKey == "" || cfg.SecretKey == "" {
		return nil, ErrMissingKeys
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "meanwhile"
	}

	otlpendpoint, urlPath, insecure, err := parseEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	auth := base64.StdEncoding.EncodeToString([]byte(cfg.PublicKey + ":" + cfg.SecretKey))
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(otlpendpoint),
		otlptracehttp.WithURLPath(urlPath),
		otlptracehttp.WithHeaders(map[string]string{"Authorization": "Basic " + auth}),
	}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)

	return &Client{
		provider: provider,
		tracer:   provider.Tracer("meanwhile"),
	}, nil
}

// StartTrace starts a new trace.
func (c *Client) StartTrace(ctx context.Context, input telemetry.SpanInput) (telemetry.Span, context.Context) {
	attrs := toAttributes(input.Attributes)
	ctx, span := c.tracer.Start(ctx, input.Name, trace.WithAttributes(attrs...), trace.WithNewRoot())
	return &otelSpan{span: span}, ctx
}

// StartSpan starts a child span.
func (c *Client) StartSpan(ctx context.Context, input telemetry.SpanInput) (telemetry.Span, context.Context) {
	attrs := toAttributes(input.Attributes)
	ctx, span := c.tracer.Start(ctx, input.Name, trace.WithAttributes(attrs...))
	return &otelSpan{span: span}, ctx
}

// Close flushes spans.
func (c *Client) Close(ctx context.Context) error {
	return c.provider.Shutdown(ctx)
}

type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) End(err error) {
	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()
}

func (s *otelSpan) AddEvent(name string, attrs map[string]any) {
	s.span.AddEvent(name, trace.WithAttributes(toAttributes(attrs)...))
}

func (s *otelSpan) SetAttribute(key string, value any) {
	attrs := toAttributes(map[string]any{key: value})
	if len(attrs) > 0 {
		s.span.SetAttributes(attrs[0])
	}
}

func toAttributes(attrs map[string]any) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, 0, len(attrs))
	for key, value := range attrs {
		switch v := value.(type) {
		case string:
			out = append(out, attribute.String(key, v))
		case bool:
			out = append(out, attribute.Bool(key, v))
		case int:
			out = append(out, attribute.Int(key, v))
		case int64:
			out = append(out, attribute.Int64(key, v))
		case float64:
			out = append(out, attribute.Float64(key, v))
		case fmt.Stringer:
			out = append(out, attribute.String(key, v.String()))
		default:
			out = append(out, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}
	return out
}

func parseEndpoint(raw string) (endpoint string, urlPath string, insecure bool, err error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", false, fmt.Errorf("parse endpoint: %w", err)
	}
	if parsed.Host == "" {
		return raw, "/v1/traces", false, nil
	}

	endpoint = parsed.Host
	path := strings.TrimSuffix(parsed.Path, "/")
	if !strings.HasSuffix(path, "/v1/traces") {
		path += "/v1/traces"
	}
	if path == "" {
		path = "/v1/traces"
	}
	return endpoint, path, parsed.Scheme == "http", nil
}
