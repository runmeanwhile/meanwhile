package engine

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

type staticProvider struct {
	id       string
	response string
}

func (p *staticProvider) ID() string { return p.id }

func (p *staticProvider) Stream(_ context.Context, req provider.Request) (provider.Stream, error) {
	if req.Model == "" {
		return nil, errors.New("model required")
	}
	return &staticStream{events: []provider.Event{{
		Type:    provider.EventMessageCompleted,
		Message: runtimeTextMessage(modelruntime.RoleAssistant, p.response),
	}}}, nil
}

type staticStream struct {
	idx    int
	events []provider.Event
}

func (s *staticStream) Recv() (provider.Event, error) {
	if s.idx >= len(s.events) {
		return provider.Event{}, io.EOF
	}
	ev := s.events[s.idx]
	s.idx++
	return ev, nil
}

func (s *staticStream) Close() error { return nil }

func TestMemoryAutomationOnClose(t *testing.T) {
	ctx := context.Background()
	store := memory.NewInMemoryStore()
	mockProvider := &staticProvider{
		id:       "mock",
		response: "User reported flaky wifi; preference: weekly status updates.",
	}

	eng, err := New(
		WithProvider(mockProvider),
		WithMemoryStore(store),
		WithMemoryAutomation(config.MemoryAutomationConfig{
			Enabled:    true,
			ProviderID: "mock",
			Model:      "mock-model",
			Context: config.MemoryAutomationContext{
				RecentMessages: 10,
				TokenLimit:     2000,
			},
		}),
	)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}

	sess, err := eng.NewSession(ctx, SessionConfig{
		ID:       "sess-1",
		Name:     "Session",
		Protocol: protocol.Solo(),
		Participants: []protocol.Participant{agent.Agent{
			Name: "agent",
		}},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	_ = sess.Emit(event.New(event.AgentMessageComplete, sess.ID(), textMessage(agent.RoleUser, "My wifi drops every hour.")))
	_ = sess.Emit(event.New(event.AgentMessageComplete, sess.ID(), textMessage(agent.RoleAssistant, "Let's check your router logs.")))

	if err := eng.CloseSession(ctx, sess.ID()); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	items, err := store.Query(ctx, memory.Query{SessionID: sess.ID(), Types: []event.Type{event.MemorySummary}})
	if err != nil {
		t.Fatalf("Query summary: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 summary event, got %d", len(items))
	}

	payload, ok := items[0].Event.Payload.(string)
	if !ok {
		t.Fatalf("expected string payload, got %T", items[0].Event.Payload)
	}
	if payload == "" {
		t.Fatal("summary payload missing")
	}
}

func TestMemoryAutomationMultipleSummaries(t *testing.T) {
	ctx := context.Background()
	store := memory.NewInMemoryStore()
	mockProvider := &staticProvider{
		id:       "mock",
		response: "Memory line",
	}

	eng, err := New(
		WithProvider(mockProvider),
		WithMemoryStore(store),
		WithMemoryAutomation(config.MemoryAutomationConfig{
			Enabled:    true,
			ProviderID: "mock",
			Model:      "mock-model",
		}),
	)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}

	sess, err := eng.NewSession(ctx, SessionConfig{
		ID:       "sess-2",
		Name:     "Session",
		Protocol: protocol.Solo(),
		Participants: []protocol.Participant{agent.Agent{
			Name: "agent",
		}},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	_ = sess.Emit(event.New(event.AgentMessageComplete, sess.ID(), textMessage(agent.RoleUser, "First note")))
	if err := eng.CloseSession(ctx, sess.ID()); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	sess2, err := eng.NewSession(ctx, SessionConfig{
		ID:       "sess-2",
		Name:     "Session",
		Protocol: protocol.Solo(),
		Participants: []protocol.Participant{agent.Agent{
			Name: "agent",
		}},
	})
	if err != nil {
		t.Fatalf("NewSession (2): %v", err)
	}

	_ = sess2.Emit(event.New(event.AgentMessageComplete, sess2.ID(), textMessage(agent.RoleUser, "Second note")))
	if err := eng.CloseSession(ctx, sess2.ID()); err != nil {
		t.Fatalf("CloseSession (2): %v", err)
	}

	items, err := store.Query(ctx, memory.Query{SessionID: sess.ID(), Types: []event.Type{event.MemorySummary}})
	if err != nil {
		t.Fatalf("Query summaries: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 summary events, got %d", len(items))
	}
}

func TestMemoryAutomationSubjectAgent(t *testing.T) {
	ctx := context.Background()
	store := memory.NewInMemoryStore()
	mockProvider := &staticProvider{
		id:       "mock",
		response: "Memory about agent",
	}

	eng, err := New(
		WithProvider(mockProvider),
		WithMemoryStore(store),
		WithMemoryAutomation(config.MemoryAutomationConfig{
			Enabled:    true,
			ProviderID: "mock",
			Model:      "mock-model",
		}),
	)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}

	sess, err := eng.NewSession(ctx, SessionConfig{
		ID:       "sess-3",
		Name:     "Session",
		Protocol: protocol.Solo(),
		Metadata: map[string]any{MemoryAutomationSubjectKey: "agent-123"},
		Participants: []protocol.Participant{agent.Agent{
			Name: "agent",
		}},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	_ = sess.Emit(event.New(event.AgentMessageComplete, sess.ID(), textMessage(agent.RoleUser, "Note")))
	if err := eng.CloseSession(ctx, sess.ID()); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	items, err := store.Query(ctx, memory.Query{SessionID: sess.ID(), Types: []event.Type{event.MemorySummary}})
	if err != nil {
		t.Fatalf("Query summary: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 summary event, got %d", len(items))
	}
	if items[0].Event.AgentID != "agent-123" {
		t.Fatalf("expected AgentID to be set, got %q", items[0].Event.AgentID)
	}
}

func TestMemoryAutomationNotAutoEnabledWithMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := memory.NewInMemoryStore()
	mockProvider := &staticProvider{
		id:       "mock",
		response: "Auto memory entry",
	}

	eng, err := New(
		WithProvider(mockProvider),
		WithMemoryStore(store),
		WithDefaultModel("mock-model"),
	)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}

	sess, err := eng.NewSession(ctx, SessionConfig{
		ID:       "sess-4",
		Name:     "Session",
		Protocol: protocol.Solo(),
		Participants: []protocol.Participant{agent.Agent{
			Name: "agent",
		}},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	_ = sess.Emit(event.New(event.AgentMessageComplete, sess.ID(), textMessage(agent.RoleUser, "Auto memory test")))
	if err := eng.CloseSession(ctx, sess.ID()); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	items, err := store.Query(ctx, memory.Query{SessionID: sess.ID(), Types: []event.Type{event.MemorySummary}})
	if err != nil {
		t.Fatalf("Query summary: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 summary events, got %d", len(items))
	}
}
