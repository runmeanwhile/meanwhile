package engine

import (
	"context"
	"io"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/telemetry"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Mock types for testing
type mockProvider struct{}

func (m *mockProvider) ID() string { return "mock" }
func (m *mockProvider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	return &mockStream{}, nil
}

type mockStream struct {
	called bool
}

func (m *mockStream) Recv() (provider.Event, error) {
	if m.called {
		return provider.Event{}, io.EOF
	}
	m.called = true
	return provider.Event{
		Type: provider.EventMessageCompleted,
		Message: agent.Message{
			Role:  agent.RoleAssistant,
			Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Mock response"}},
		},
	}, nil
}
func (m *mockStream) Close() error { return nil }

type mockTelemetry struct{}

func (m *mockTelemetry) StartTrace(ctx context.Context, input telemetry.SpanInput) (telemetry.Span, context.Context) {
	return &mockSpan{}, ctx
}
func (m *mockTelemetry) StartSpan(ctx context.Context, input telemetry.SpanInput) (telemetry.Span, context.Context) {
	return &mockSpan{}, ctx
}
func (m *mockTelemetry) Close(ctx context.Context) error { return nil }

type mockSpan struct{}

func (m *mockSpan) End(err error)                              {}
func (m *mockSpan) AddEvent(name string, attrs map[string]any) {}
func (m *mockSpan) SetAttribute(key string, value any)         {}

type mockLogger struct{}

func (m *mockLogger) Log(ev event.Event) error { return nil }

type mockMemory struct{}

func (m *mockMemory) Append(ctx context.Context, sessionID string, ev event.Event) error {
	return nil
}
func (m *mockMemory) Query(ctx context.Context, query memory.Query) ([]memory.Item, error) {
	return nil, nil
}
func (m *mockMemory) Summarize(ctx context.Context, sessionID string, policy memory.Policy) (memory.Summary, error) {
	return memory.Summary{}, nil
}
func (m *mockMemory) Stats(ctx context.Context, sessionID string, policy memory.Policy) (memory.EventStats, error) {
	return memory.EventStats{}, nil
}

type mockAutomator struct{}

func (m *mockAutomator) Capture(ctx context.Context, sess *Session) error {
	return nil
}

func TestWithProviderRegistry(t *testing.T) {
	e := &Engine{}
	reg := provider.NewRegistry()

	opt := WithProviderRegistry(reg)
	if err := opt(e); err != nil {
		t.Fatalf("WithProviderRegistry failed: %v", err)
	}

	if e.providers != reg {
		t.Error("Expected provider registry to be set")
	}
}

func TestWithProtocolRegistry(t *testing.T) {
	e := &Engine{}
	reg := protocol.NewRegistry()

	opt := WithProtocolRegistry(reg)
	if err := opt(e); err != nil {
		t.Fatalf("WithProtocolRegistry failed: %v", err)
	}

	if e.protocols != reg {
		t.Error("Expected protocol registry to be set")
	}
}

func TestWithHookRegistry(t *testing.T) {
	e := &Engine{}
	reg := hook.NewRegistry()

	opt := WithHookRegistry(reg)
	if err := opt(e); err != nil {
		t.Fatalf("WithHookRegistry failed: %v", err)
	}

	if e.hooks != reg {
		t.Error("Expected hook registry to be set")
	}
}

func TestWithToolRegistry(t *testing.T) {
	e := &Engine{}
	reg := tool.NewRegistry()

	opt := WithToolRegistry(reg)
	if err := opt(e); err != nil {
		t.Fatalf("WithToolRegistry failed: %v", err)
	}

	if e.tools != reg {
		t.Error("Expected tool registry to be set")
	}
}

func TestWithMemoryStore(t *testing.T) {
	e := &Engine{}
	store := &mockMemory{}

	opt := WithMemoryStore(store)
	if err := opt(e); err != nil {
		t.Fatalf("WithMemoryStore failed: %v", err)
	}

	if e.memory == nil {
		t.Error("Expected memory store to be set")
	}
}

func TestWithMemoryAutomator(t *testing.T) {
	e := &Engine{}
	automator := &mockAutomator{}

	opt := WithMemoryAutomator(automator)
	if err := opt(e); err != nil {
		t.Fatalf("WithMemoryAutomator failed: %v", err)
	}

	if e.memoryAutomator == nil {
		t.Error("Expected memory automator to be set")
	}
}

func TestWithProfileRegistry(t *testing.T) {
	e := &Engine{}
	reg := agent.NewRegistry()

	opt := WithProfileRegistry(reg)
	if err := opt(e); err != nil {
		t.Fatalf("WithProfileRegistry failed: %v", err)
	}

	if e.profiles != reg {
		t.Error("Expected profile registry to be set")
	}
}

func TestWithGlobalConfig(t *testing.T) {
	e := &Engine{}
	cfg := config.GlobalConfig{
		Defaults: config.AgentConfig{
			Name: "test",
		},
	}

	opt := WithGlobalConfig(cfg)
	if err := opt(e); err != nil {
		t.Fatalf("WithGlobalConfig failed: %v", err)
	}

	if e.cfg.Defaults.Name != "test" {
		t.Error("Expected global config to be set")
	}
}

func TestWithTelemetryClient(t *testing.T) {
	e := &Engine{}
	client := &mockTelemetry{}

	opt := WithTelemetryClient(client)
	if err := opt(e); err != nil {
		t.Fatalf("WithTelemetryClient failed: %v", err)
	}

	if e.telemetry == nil {
		t.Error("Expected telemetry client to be set")
	}
}

func TestWithLogger(t *testing.T) {
	e := &Engine{}
	log := &mockLogger{}

	opt := WithLogger(log)
	if err := opt(e); err != nil {
		t.Fatalf("WithLogger failed: %v", err)
	}

	if e.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestWithProvider(t *testing.T) {
	e := &Engine{
		providers: provider.NewRegistry(),
	}
	p := &mockProvider{}

	opt := WithProvider(p)
	if err := opt(e); err != nil {
		t.Fatalf("WithProvider failed: %v", err)
	}

	// Provider should be registered
	_, ok := e.providers.Get("mock")
	if !ok {
		t.Error("Expected provider to be registered")
	}

	// First provider should be default
	if e.defaultProvider != "mock" {
		t.Errorf("Expected default provider to be 'mock', got %s", e.defaultProvider)
	}
}

func TestWithProviderNil(t *testing.T) {
	e := &Engine{
		providers: provider.NewRegistry(),
	}

	opt := WithProvider(nil)
	err := opt(e)
	if err == nil {
		t.Error("Expected error for nil provider")
	}
}

func TestWithDefaultProvider(t *testing.T) {
	e := &Engine{}

	opt := WithDefaultProvider("custom")
	if err := opt(e); err != nil {
		t.Fatalf("WithDefaultProvider failed: %v", err)
	}

	if e.defaultProvider != "custom" {
		t.Errorf("Expected default provider to be 'custom', got %s", e.defaultProvider)
	}
}

func TestWithProviders(t *testing.T) {
	e := &Engine{
		providers: provider.NewRegistry(),
	}
	p1 := &mockProvider{}
	p2 := &mockProvider{}

	opt := WithProviders(p1, p2)
	if err := opt(e); err != nil {
		t.Fatalf("WithProviders failed: %v", err)
	}

	_, ok := e.providers.Get("mock")
	if !ok {
		t.Error("Expected providers to be registered")
	}
}

func TestWithProvidersNil(t *testing.T) {
	e := &Engine{
		providers: provider.NewRegistry(),
	}

	opt := WithProviders(&mockProvider{}, nil)
	err := opt(e)
	if err == nil {
		t.Error("Expected error for nil provider in list")
	}
}

func TestWithDefaultModel(t *testing.T) {
	e := &Engine{}

	opt := WithDefaultModel("gpt-4")
	if err := opt(e); err != nil {
		t.Fatalf("WithDefaultModel failed: %v", err)
	}

	model, ok := e.cfg.Defaults.Params["model"]
	if !ok || model != "gpt-4" {
		t.Errorf("Expected model to be 'gpt-4', got %v", model)
	}
}

func TestWithDefaultModelEmpty(t *testing.T) {
	e := &Engine{}

	opt := WithDefaultModel("")
	err := opt(e)
	if err == nil {
		t.Error("Expected error for empty model")
	}
}

func TestWithDefaultParam(t *testing.T) {
	e := &Engine{}

	opt := WithDefaultParam("temperature", 0.7)
	if err := opt(e); err != nil {
		t.Fatalf("WithDefaultParam failed: %v", err)
	}

	temp, ok := e.cfg.Defaults.Params["temperature"]
	if !ok || temp != 0.7 {
		t.Errorf("Expected temperature to be 0.7, got %v", temp)
	}
}

func TestWithDefaultParamEmptyKey(t *testing.T) {
	e := &Engine{}

	opt := WithDefaultParam("", "value")
	err := opt(e)
	if err == nil {
		t.Error("Expected error for empty param key")
	}
}

func TestNew(t *testing.T) {
	e, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if e == nil {
		t.Fatal("Expected non-nil engine")
	}

	// Check defaults are initialized
	if e.providers == nil {
		t.Error("Expected providers registry to be initialized")
	}
	if e.protocols == nil {
		t.Error("Expected protocols registry to be initialized")
	}
	if e.hooks == nil {
		t.Error("Expected hooks registry to be initialized")
	}
	if e.tools == nil {
		t.Error("Expected tools registry to be initialized")
	}
	// Skills registry is only initialized if cfg.Skills.Root is set
	if e.profiles == nil {
		t.Error("Expected profiles registry to be initialized")
	}
	if e.sessions == nil {
		t.Error("Expected sessions map to be initialized")
	}
}

func TestNewWithOptions(t *testing.T) {
	p := &mockProvider{}

	e, err := New(
		WithProvider(p),
		WithDefaultModel("gpt-4"),
	)
	if err != nil {
		t.Fatalf("New with options failed: %v", err)
	}

	if e.defaultProvider != "mock" {
		t.Error("Expected default provider to be set")
	}

	model, ok := e.cfg.Defaults.Params["model"]
	if !ok || model != "gpt-4" {
		t.Error("Expected model to be set")
	}
}

func TestNewWithOptionError(t *testing.T) {
	_, err := New(
		WithProvider(nil), // Should fail
	)
	if err == nil {
		t.Error("Expected error from invalid option")
	}
}
