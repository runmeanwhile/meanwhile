package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

func TestEngineProviderRegistry(t *testing.T) {
	e, _ := New()
	if e.ProviderRegistry() == nil {
		t.Error("Expected non-nil provider registry")
	}
}

func TestEngineProtocolRegistry(t *testing.T) {
	e, _ := New()
	if e.ProtocolRegistry() == nil {
		t.Error("Expected non-nil protocol registry")
	}
}

func TestEngineHookRegistry(t *testing.T) {
	e, _ := New()
	if e.HookRegistry() == nil {
		t.Error("Expected non-nil hook registry")
	}
}

func TestEngineToolRegistry(t *testing.T) {
	e, _ := New()
	if e.ToolRegistry() == nil {
		t.Error("Expected non-nil tool registry")
	}
}

func TestEngineProfileRegistry(t *testing.T) {
	e, _ := New()
	if e.ProfileRegistry() == nil {
		t.Error("Expected non-nil profile registry")
	}
}

func TestEngineCloseSession(t *testing.T) {
	e, _ := New(WithProvider(&mockProvider{}))
	ctx := context.Background()
	a := agent.Agent{Name: "test-agent", Model: "gpt-4"}

	sess, err := e.NewSession(ctx, SessionConfig{
		Name:         "test-session",
		Participants: []protocol.Participant{a},
		Protocol:     protocol.Solo(),
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	sessionID := sess.ID()

	if err := e.CloseSession(ctx, sessionID); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	if _, err := e.session(context.Background(), sessionID); err != ErrSessionNotFound {
		t.Error("Expected ErrSessionNotFound after close")
	}
}

func TestEngineCloseSessionNotFound(t *testing.T) {
	e, _ := New()
	ctx := context.Background()
	err := e.CloseSession(ctx, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound, got: %v", err)
	}
}

type failingAutomator struct{}

func (f failingAutomator) Capture(ctx context.Context, sess *Session) error {
	_ = ctx
	_ = sess
	return errors.New("capture failed")
}

func TestEngineCloseSessionReturnsMemoryAutomationError(t *testing.T) {
	e, _ := New(WithMemoryAutomator(failingAutomator{}))
	ctx := context.Background()

	sess, err := e.NewSession(ctx, SessionConfig{
		Name:     "test-session",
		Protocol: protocol.Solo(),
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	if err := e.CloseSession(ctx, sess.ID()); err == nil {
		t.Fatalf("expected memory automation error")
	}

	err = e.CloseSession(ctx, sess.ID())
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected session to be closed, got: %v", err)
	}
}

func TestEngineRegisterProfile(t *testing.T) {
	e, _ := New()
	profile := agent.Profile{
		ID:     "test-profile",
		Name:   "Test Profile",
		Prompt: "You are helpful",
	}

	e.RegisterProfile(profile)

	retrieved, ok := e.ProfileRegistry().Get("test-profile")
	if !ok {
		t.Error("Expected profile to be registered")
	}
	if retrieved.Name != "Test Profile" {
		t.Errorf("Expected 'Test Profile', got %s", retrieved.Name)
	}
}

func TestEngineAgentBuilder(t *testing.T) {
	e, _ := New()

	builder := e.Agent("test")
	if builder == nil {
		t.Fatal("Expected non-nil builder")
	}

	builderWithPrompt := e.Agent("test2", "prompt")
	if builderWithPrompt == nil {
		t.Fatal("Expected non-nil builder with prompt")
	}
}

func TestProtocolAsToolBasic(t *testing.T) {
	e, _ := New(WithProvider(&mockProvider{}))
	agent1 := agent.Agent{Name: "agent1", Model: "gpt-4"}

	tool := e.AsTool(
		protocol.Solo(),
		WithToolName("solo_tool"),
		WithToolDescription("A solo protocol tool"),
		WithToolParticipants(agent1),
	)

	if tool.ID() != "solo_tool" {
		t.Errorf("Expected tool ID 'solo_tool', got %s", tool.ID())
	}

	schema := tool.Schema()
	if len(schema.JSONSchema) == 0 {
		t.Error("Expected non-empty schema")
	}
}

func TestProtocolAsToolWithMultipleParticipants(t *testing.T) {
	e, _ := New(WithProvider(&mockProvider{}))
	agent1 := agent.Agent{Name: "agent1", Model: "gpt-4"}
	agent2 := agent.Agent{Name: "agent2", Model: "gpt-4"}

	tool := e.AsTool(
		protocol.Solo(),
		WithToolParticipants(agent1, agent2),
	)

	if tool == nil {
		t.Fatal("Expected non-nil tool")
	}
}

func TestProtocolAsToolWithFacilitator(t *testing.T) {
	e, _ := New(WithProvider(&mockProvider{}))
	agent1 := agent.Agent{Name: "agent1", Model: "gpt-4"}
	facilitator := agent.Agent{Name: "facilitator", Model: "gpt-4"}

	tool := e.AsTool(
		protocol.Solo(),
		WithToolParticipants(agent1),
		WithToolFacilitator(facilitator),
	)

	if tool == nil {
		t.Fatal("Expected non-nil tool")
	}
}

func TestProtocolAsToolWithTags(t *testing.T) {
	e, _ := New(WithProvider(&mockProvider{}))
	agent1 := agent.Agent{Name: "agent1", Model: "gpt-4"}

	tool := e.AsTool(
		protocol.Solo(),
		WithToolParticipants(agent1),
		WithToolTags("tag1", "tag2"),
	)

	if tool == nil {
		t.Fatal("Expected non-nil tool")
	}
}
