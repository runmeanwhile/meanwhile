package protocol

import (
	"context"
	"errors"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
)

func TestHandoffProtocol_ID(t *testing.T) {
	caller := agent.Agent{Name: "Caller"}
	callee := agent.Agent{Name: "Callee"}
	p := Handoff(caller, callee)

	if p.ID() != "protocol.handoff" {
		t.Errorf("expected ID 'protocol.handoff', got '%s'", p.ID())
	}
}

func TestHandoffProtocol_Participants(t *testing.T) {
	caller := agent.Agent{Name: "Caller"}
	callee := agent.Agent{Name: "Callee"}
	p := Handoff(caller, callee)

	participants := p.Participants()
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}

	if participants[0].DisplayName() != "Caller" {
		t.Errorf("expected first participant 'Caller', got '%s'", participants[0].DisplayName())
	}

	if participants[1].DisplayName() != "Callee" {
		t.Errorf("expected second participant 'Callee', got '%s'", participants[1].DisplayName())
	}
}

func TestHandoffProtocol_Init(t *testing.T) {
	caller := agent.Agent{Name: "Caller"}
	callee := agent.Agent{Name: "Callee"}
	p := Handoff(caller, callee)
	sess := &mockSession{id: "test"}

	err := p.Init(context.Background(), sess)
	if err != nil {
		t.Errorf("Init() failed: %v", err)
	}
}

func TestHandoffProtocol_OnMessage_Success(t *testing.T) {
	caller := agent.Agent{Name: "Caller"}
	callee := agent.Agent{Name: "Callee"}
	p := Handoff(caller, callee)

	var capturedAgent agent.Agent
	sess := &mockSession{
		id:            "test",
		emittedEvents: []event.Event{},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			capturedAgent = ag
			return message.Assistant("delegated response"), nil
		},
	}

	msg := message.User("task")
	err := p.OnMessage(context.Background(), sess, msg)

	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if capturedAgent.Name != "Callee" {
		t.Errorf("expected RunAgent called with Callee, got %s", capturedAgent.Name)
	}

	if len(sess.emittedEvents) != 1 {
		t.Errorf("expected 1 emitted event, got %d", len(sess.emittedEvents))
	}

	if sess.emittedEvents[0].Type != event.ProtocolAction {
		t.Errorf("expected event type ProtocolAction, got %v", sess.emittedEvents[0].Type)
	}

	payload, ok := sess.emittedEvents[0].Payload.(map[string]any)
	if !ok {
		t.Fatal("expected payload to be map[string]any")
	}

	if payload["caller"] != "Caller" {
		t.Errorf("expected caller 'Caller', got %v", payload["caller"])
	}

	if payload["callee"] != "Callee" {
		t.Errorf("expected callee 'Callee', got %v", payload["callee"])
	}
}

func TestHandoffProtocol_OnMessage_RunAgentError(t *testing.T) {
	caller := agent.Agent{Name: "Caller"}
	callee := agent.Agent{Name: "Callee"}
	p := Handoff(caller, callee)

	sess := &mockSession{
		id: "test",
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			return agent.Message{}, errors.New("delegation failed")
		},
	}

	msg := message.User("task")
	err := p.OnMessage(context.Background(), sess, msg)

	if err == nil {
		t.Error("expected error from RunAgent, got nil")
	}

	if err.Error() != "delegation failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandoffProtocol_OnEvent(t *testing.T) {
	caller := agent.Agent{Name: "Caller"}
	callee := agent.Agent{Name: "Callee"}
	p := Handoff(caller, callee)
	sess := &mockSession{id: "test"}

	err := p.OnEvent(context.Background(), sess, event.Event{})
	if err != nil {
		t.Errorf("OnEvent() failed: %v", err)
	}
}

func TestHandoffProtocol_Shutdown(t *testing.T) {
	caller := agent.Agent{Name: "Caller"}
	callee := agent.Agent{Name: "Callee"}
	p := Handoff(caller, callee)
	sess := &mockSession{id: "test"}

	err := p.Shutdown(context.Background(), sess)
	if err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

func TestHandoffProtocol_Lifecycle(t *testing.T) {
	caller := agent.Agent{Name: "Caller"}
	callee := agent.Agent{Name: "Callee"}
	p := Handoff(caller, callee)

	sess := &mockSession{
		id:            "test",
		emittedEvents: []event.Event{},
	}
	ctx := context.Background()

	// Init
	if err := p.Init(ctx, sess); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// OnMessage
	msg := message.User("delegate this")
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
