package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

func TestSessionID(t *testing.T) {
	s := &Session{id: "test-session"}
	if s.ID() != "test-session" {
		t.Errorf("Expected session ID 'test-session', got %s", s.ID())
	}
}

func TestSessionName(t *testing.T) {
	s := &Session{name: "Test Session"}
	if s.Name() != "Test Session" {
		t.Errorf("Expected session name 'Test Session', got %s", s.Name())
	}
}

func TestSessionTags(t *testing.T) {
	s := &Session{tags: []string{"tag1", "tag2"}}
	tags := s.Tags()

	if len(tags) != 2 {
		t.Fatalf("Expected 2 tags, got %d", len(tags))
	}

	if tags[0] != "tag1" || tags[1] != "tag2" {
		t.Errorf("Expected tags [tag1 tag2], got %v", tags)
	}

	// Verify it's a copy - modifying returned slice shouldn't affect original
	tags[0] = "modified"
	if s.tags[0] == "modified" {
		t.Error("Tags should be a copy, not a reference")
	}
}

func TestSessionMetadata(t *testing.T) {
	meta := map[string]any{"key": "value", "count": 42}
	s := &Session{metadata: meta}

	retrieved := s.Metadata()
	if len(retrieved) != 2 {
		t.Fatalf("Expected 2 metadata entries, got %d", len(retrieved))
	}

	if retrieved["key"] != "value" {
		t.Errorf("Expected metadata key='value', got %v", retrieved["key"])
	}

	if retrieved["count"] != 42 {
		t.Errorf("Expected metadata count=42, got %v", retrieved["count"])
	}
}

func TestSessionProtocolID(t *testing.T) {
	p := protocol.Solo()
	s := &Session{protocol: p}

	if s.ProtocolID() != "protocol.solo" {
		t.Errorf("Expected protocol ID 'protocol.solo', got %s", s.ProtocolID())
	}
}

func TestSessionParticipants(t *testing.T) {
	agent1 := agent.Agent{Name: "agent1"}
	agent2 := agent.Agent{Name: "agent2"}
	s := &Session{participants: []protocol.Participant{agent1, agent2}}

	participants := s.Participants()
	if len(participants) != 2 {
		t.Fatalf("Expected 2 participants, got %d", len(participants))
	}

	// Verify it's a copy
	participants[0] = agent.Agent{Name: "modified"}
	if s.participants[0].DisplayName() == "modified" {
		t.Error("Participants should be a copy, not a reference")
	}
}

func TestSessionFacilitator(t *testing.T) {
	fac := agent.Agent{Name: "facilitator"}
	s := &Session{facilitator: &fac}

	retrieved := s.Facilitator()
	if retrieved == nil {
		t.Fatal("Expected non-nil facilitator")
	}

	if retrieved.Name != "facilitator" {
		t.Errorf("Expected facilitator name 'facilitator', got %s", retrieved.Name)
	}
}

func TestSessionFacilitatorNil(t *testing.T) {
	s := &Session{}
	if s.Facilitator() != nil {
		t.Error("Expected nil facilitator")
	}
}

func TestSessionGroups(t *testing.T) {
	agent1 := agent.Agent{Name: "agent1"}
	agent2 := agent.Agent{Name: "agent2"}
	groups := map[string][]protocol.Participant{
		"groupA": {agent1},
		"groupB": {agent2},
	}
	s := &Session{groups: groups}

	retrieved := s.Groups()
	if len(retrieved) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(retrieved))
	}

	if len(retrieved["groupA"]) != 1 {
		t.Error("Expected 1 agent in groupA")
	}

	if len(retrieved["groupB"]) != 1 {
		t.Error("Expected 1 agent in groupB")
	}
}

func TestSessionEmit(t *testing.T) {
	bus := event.NewBus()
	mem := &mockMemory{}
	p := protocol.Solo()

	s := &Session{
		id:       "sess-123",
		bus:      bus,
		memory:   mem,
		protocol: p,
	}

	ev := event.Event{
		Type: event.AgentStarted,
	}

	if err := s.Emit(ev); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Note: Emit modifies a copy of the event, not the original
	// To verify enrichment, we need to subscribe to the bus (see TestSessionEmitEnrichesEvent)
}

func TestSessionEmitEnrichesEvent(t *testing.T) {
	bus := event.NewBus()
	p := protocol.Solo()

	s := &Session{
		id:       "sess-456",
		bus:      bus,
		protocol: p,
	}

	ev := event.Event{
		Type: event.AgentStarted,
	}

	// Subscribe to verify enriched event
	var received event.Event
	done := make(chan bool)
	bus.Subscribe(func(e event.Event) {
		received = e
		done <- true
	})

	if err := s.Emit(ev); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	<-done

	if received.SessionID != "sess-456" {
		t.Errorf("Expected session ID 'sess-456', got %s", received.SessionID)
	}

	if received.ProtocolID != "protocol.solo" {
		t.Errorf("Expected protocol ID 'protocol.solo', got %s", received.ProtocolID)
	}

	if received.ID == "" {
		t.Error("Expected event ID to be generated")
	}

	if received.Time.IsZero() {
		t.Error("Expected event time to be set")
	}
}

func TestSessionRegisterTool(t *testing.T) {
	s := &Session{}

	mockTool := tool.Func{
		IDValue: "test-tool",
		SchemaValue: tool.Schema{
			JSONSchema: []byte(`{"type":"object"}`),
		},
		RunFunc: func(ctx context.Context, call tool.Call, emit tool.Emitter) (tool.Result, error) {
			return tool.TextResult(call, "ok"), nil
		},
	}

	if err := s.RegisterTool(mockTool); err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}

	if s.tools == nil {
		t.Error("Expected tools registry to be initialized")
	}

	// Verify tool was registered
	_, ok := s.tools.Get("test-tool")
	if !ok {
		t.Error("Expected tool to be registered")
	}
}

func TestSessionRegisterToolInvalid(t *testing.T) {
	s := &Session{}

	// Try to register something that's not a tool
	err := s.RegisterTool("not a tool")
	if err == nil {
		t.Error("Expected error when registering non-tool")
	}
}

func TestSessionEmitWithNoMemory(t *testing.T) {
	bus := event.NewBus()
	s := &Session{
		id:     "sess-789",
		bus:    bus,
		memory: nil, // No memory store
	}

	ev := event.Event{
		Type: event.AgentStarted,
	}

	// Should not error even without memory
	if err := s.Emit(ev); err != nil {
		t.Errorf("Emit should not fail without memory: %v", err)
	}
}

func TestSessionEmitPreservesExistingFields(t *testing.T) {
	bus := event.NewBus()
	s := &Session{
		id:  "sess-abc",
		bus: bus,
	}

	customTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	ev := event.Event{
		Type:       event.AgentStarted,
		ID:         "custom-id",
		SessionID:  "custom-session",
		ProtocolID: "custom-protocol",
		Time:       customTime,
	}

	var received event.Event
	done := make(chan bool)
	bus.Subscribe(func(e event.Event) {
		received = e
		done <- true
	})

	if err := s.Emit(ev); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	<-done

	// Verify custom values were preserved
	if received.ID != "custom-id" {
		t.Error("Expected custom ID to be preserved")
	}

	if received.SessionID != "custom-session" {
		t.Error("Expected custom session ID to be preserved")
	}

	if received.ProtocolID != "custom-protocol" {
		t.Error("Expected custom protocol ID to be preserved")
	}

	if !received.Time.Equal(customTime) {
		t.Error("Expected custom time to be preserved")
	}
}

type errorMemoryStore struct{}

func (e *errorMemoryStore) Append(_ context.Context, _ string, _ event.Event) error {
	return errors.New("append failed")
}
func (e *errorMemoryStore) Query(_ context.Context, _ memory.Query) ([]memory.Item, error) {
	return nil, nil
}
func (e *errorMemoryStore) Summarize(_ context.Context, _ string, _ memory.Policy) (memory.Summary, error) {
	return memory.Summary{}, nil
}
func (e *errorMemoryStore) Stats(_ context.Context, _ string, _ memory.Policy) (memory.EventStats, error) {
	return memory.EventStats{}, nil
}

func TestSessionEmitReturnsMemoryError(t *testing.T) {
	bus := event.NewBus()
	s := &Session{
		id:     "sess-error",
		bus:    bus,
		memory: &errorMemoryStore{},
	}

	err := s.Emit(event.New(event.AgentStarted, s.id, nil))
	if err == nil {
		t.Fatalf("expected error from memory append")
	}
}
