package protocol

import (
	"context"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
)

func TestBreakoutProtocol_ID(t *testing.T) {
	p := Breakout()
	if p.ID() != "protocol.breakout_reconvene" {
		t.Errorf("expected ID 'protocol.breakout_reconvene', got '%s'", p.ID())
	}
}

func TestBreakoutProtocol_Participants(t *testing.T) {
	p := Breakout()
	participants := p.Participants()
	if participants != nil {
		t.Errorf("expected nil participants, got %v", participants)
	}
}

func TestBreakoutProtocol_Init(t *testing.T) {
	p := Breakout()
	sess := &mockSession{id: "test"}

	err := p.Init(context.Background(), sess)
	if err != nil {
		t.Errorf("Init() failed: %v", err)
	}
}

func TestBreakoutProtocol_OnMessage(t *testing.T) {
	p := Breakout()
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
		},
		emittedEvents: []event.Event{},
	}

	msg := message.User("test")
	err := p.OnMessage(context.Background(), sess, msg)

	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if len(sess.emittedEvents) != 1 {
		t.Errorf("expected 1 emitted event, got %d", len(sess.emittedEvents))
	}
}

func TestBreakoutProtocol_PropagatesImageParts(t *testing.T) {
	p := Breakout()
	var capturedMessages []agent.Message
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
		},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if len(req.Messages) > 0 {
				capturedMessages = append(capturedMessages, req.Messages[0])
			}
			return message.Assistant("response"), nil
		},
	}

	msg := agent.Message{
		Role: agent.RoleUser,
		Parts: []agent.ContentPart{
			{Type: agent.ContentPartText, Text: "Review this."},
			{Type: agent.ContentPartImage, URI: "https://example.com/image.png"},
		},
	}
	err := p.OnMessage(context.Background(), sess, msg)
	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if len(capturedMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(capturedMessages))
	}

	if !breakoutHasImagePart(capturedMessages[0].Parts) {
		t.Fatal("expected image part in breakout prompt")
	}
}

func TestBreakoutProtocol_ReconvenePropagatesImageParts(t *testing.T) {
	p := Breakout()
	var capturedMessages []agent.Message
	facilitator := agent.Agent{Name: "Facilitator"}
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
		},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if len(req.Messages) > 0 {
				capturedMessages = append(capturedMessages, req.Messages[0])
			}
			return message.Assistant("response"), nil
		},
		facilitator: &facilitator,
	}

	msg := agent.Message{
		Role: agent.RoleUser,
		Parts: []agent.ContentPart{
			{Type: agent.ContentPartText, Text: "Review this."},
			{Type: agent.ContentPartImage, URI: "https://example.com/image.png"},
		},
	}
	err := p.OnMessage(context.Background(), sess, msg)
	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if len(capturedMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(capturedMessages))
	}

	if !breakoutHasImagePart(capturedMessages[1].Parts) {
		t.Fatal("expected image part in reconvene prompt")
	}
}

func TestBreakoutProtocol_OnEvent(t *testing.T) {
	p := Breakout()
	sess := &mockSession{id: "test"}

	err := p.OnEvent(context.Background(), sess, event.Event{})
	if err != nil {
		t.Errorf("OnEvent() failed: %v", err)
	}
}

func TestBreakoutProtocol_Shutdown(t *testing.T) {
	p := Breakout()
	sess := &mockSession{id: "test"}

	err := p.Shutdown(context.Background(), sess)
	if err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

func breakoutHasImagePart(parts []agent.ContentPart) bool {
	for _, part := range parts {
		if part.Type == agent.ContentPartImage {
			return true
		}
	}
	return false
}
