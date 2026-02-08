package protocol

import (
	"context"
	"errors"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/message"
)

type mockSession struct {
	id            string
	participants  []Participant
	facilitator   *agent.Agent
	emittedEvents []event.Event
	runAgentFunc  func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error)
}

func (m *mockSession) ID() string                       { return m.id }
func (m *mockSession) Name() string                     { return "test-session" }
func (m *mockSession) Tags() []string                   { return nil }
func (m *mockSession) Metadata() map[string]any         { return nil }
func (m *mockSession) ProtocolID() string               { return "test-protocol" }
func (m *mockSession) Participants() []Participant      { return m.participants }
func (m *mockSession) Facilitator() *agent.Agent        { return m.facilitator }
func (m *mockSession) Groups() map[string][]Participant { return nil }
func (m *mockSession) DefaultTools() []string           { return nil }
func (m *mockSession) RegisterTool(t any) error         { return nil }
func (m *mockSession) RegisterTools(tools ...any) error { return nil }
func (m *mockSession) AddDefaultTools(ids ...string)    {}

func (m *mockSession) Emit(ev event.Event) error {
	m.emittedEvents = append(m.emittedEvents, ev)
	return nil
}

func (m *mockSession) EmitWithContext(ctx context.Context, ev event.Event) error {
	_ = ctx
	return m.Emit(ev)
}

func (m *mockSession) History(_ context.Context, _ ...memory.ContextOption) ([]agent.Message, error) {
	return nil, nil // Mock returns empty history
}

func (m *mockSession) RunAgent(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
	if m.runAgentFunc != nil {
		return m.runAgentFunc(ctx, ag, req)
	}
	return message.Assistant("response"), nil
}

func (m *mockSession) RunTurn(ctx context.Context, participant Participant, req RunRequest, _ ...TurnOption) (agent.Message, error) {
	ag, ok := participant.Agent()
	if !ok {
		return agent.Message{}, errors.New("human turn not supported")
	}
	return m.RunAgent(ctx, ag, req)
}

func (m *mockSession) AwaitInput(ctx context.Context, participant Participant, context string, resume TurnResume, _ ...InputOption) error {
	_ = ctx
	_ = participant
	_ = context
	_ = resume
	return errors.New("human input not supported")
}

func TestSoloProtocol_ID(t *testing.T) {
	p := Solo()
	if p.ID() != "protocol.solo" {
		t.Errorf("expected ID 'protocol.solo', got '%s'", p.ID())
	}
}

func TestSoloProtocol_Participants(t *testing.T) {
	p := Solo()
	participants := p.Participants()
	if participants != nil {
		t.Errorf("expected nil participants, got %v", participants)
	}
}

func TestSoloProtocol_Init(t *testing.T) {
	p := Solo()
	sess := &mockSession{id: "test"}

	err := p.Init(context.Background(), sess)
	if err != nil {
		t.Errorf("Init() failed: %v", err)
	}
}

func TestSoloProtocol_OnMessage_Success(t *testing.T) {
	p := Solo()
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
		},
		emittedEvents: []event.Event{},
	}

	msg := message.User("hello")
	err := p.OnMessage(context.Background(), sess, msg)

	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if len(sess.emittedEvents) != 1 {
		t.Errorf("expected 1 emitted event, got %d", len(sess.emittedEvents))
	}

	if sess.emittedEvents[0].Type != event.ProtocolAction {
		t.Errorf("expected event type ProtocolAction, got %v", sess.emittedEvents[0].Type)
	}
}

func TestSoloProtocol_OnMessage_NoParticipants(t *testing.T) {
	p := Solo()
	sess := &mockSession{
		id:           "test",
		participants: []Participant{},
	}

	msg := message.User("hello")
	err := p.OnMessage(context.Background(), sess, msg)

	if err == nil {
		t.Error("expected error for no participants, got nil")
	}

	if err.Error() != "solo protocol requires at least one participant" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSoloProtocol_OnMessage_RunAgentError(t *testing.T) {
	p := Solo()
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
		},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			return agent.Message{}, errors.New("agent error")
		},
	}

	msg := message.User("hello")
	err := p.OnMessage(context.Background(), sess, msg)

	if err == nil {
		t.Error("expected error from RunAgent, got nil")
	}
}

func TestSoloProtocol_OnEvent(t *testing.T) {
	p := Solo()
	sess := &mockSession{id: "test"}

	err := p.OnEvent(context.Background(), sess, event.Event{})
	if err != nil {
		t.Errorf("OnEvent() failed: %v", err)
	}
}

func TestSoloProtocol_Shutdown(t *testing.T) {
	p := Solo()
	sess := &mockSession{id: "test"}

	err := p.Shutdown(context.Background(), sess)
	if err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

func TestSoloProtocol_Lifecycle(t *testing.T) {
	p := Solo()
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
		},
		emittedEvents: []event.Event{},
	}
	ctx := context.Background()

	// Init
	if err := p.Init(ctx, sess); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// OnMessage
	msg := message.User("test")
	if err := p.OnMessage(ctx, sess, msg); err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	// OnEvent
	if err := p.OnEvent(ctx, sess, event.Event{}); err != nil {
		t.Fatalf("OnEvent() failed: %v", err)
	}

	// Shutdown
	if err := p.Shutdown(ctx, sess); err != nil {
		t.Fatalf("Shutdown() failed: %v", err)
	}
}
