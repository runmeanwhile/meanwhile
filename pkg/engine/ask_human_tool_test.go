package engine

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/provider"
)

type askHumanProtocol struct {
	agent agent.Agent
	tools []string
}

func (p *askHumanProtocol) ID() string { return "ask-human-protocol" }
func (p *askHumanProtocol) Participants() []protocol.Participant {
	return []protocol.Participant{p.agent}
}
func (p *askHumanProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (p *askHumanProtocol) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	_, err := sess.RunAgent(ctx, p.agent, protocol.RunRequest{
		Messages: []agent.Message{msg},
		Tools:    p.tools,
	})
	return err
}
func (p *askHumanProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (p *askHumanProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func TestAskHumanToolPausesSession(t *testing.T) {
	prov := &askHumanProvider{
		arguments: json.RawMessage(`{"question":"Need your take","participant":"User"}`),
	}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	human := eng.Human("User").Build()
	moderator := agent.Agent{Name: "Moderator", Model: "test"}
	proto := &askHumanProtocol{agent: moderator, tools: []string{AskHumanToolID}}

	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     proto,
		Participants: []protocol.Participant{moderator, human},
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	askTool, err := sess.AskHumanTool()
	if err != nil {
		t.Fatalf("AskHumanTool() error = %v", err)
	}
	if err := sess.RegisterTool(askTool); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	result, err := sess.Run(context.Background(), message.User("start"))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusAwaitingInput {
		t.Fatalf("expected awaiting status, got %s", result.Status)
	}
	if result.AwaitingInput == nil || result.AwaitingInput.RequestID == "" {
		t.Fatalf("expected awaiting input details")
	}

	foundRequest := false
	for _, ev := range result.Events {
		if ev.Type == event.HumanRequestCreated {
			foundRequest = true
			break
		}
	}
	if !foundRequest {
		t.Fatalf("expected human request created event")
	}
}

func TestAskHumanToolOptionalContinues(t *testing.T) {
	prov := &askHumanProvider{
		arguments: json.RawMessage(`{"question":"Need your take","participant":"User","required":false}`),
		message: agent.Message{
			Role:  agent.RoleAssistant,
			Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "done"}},
		},
	}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	human := eng.Human("User").Build()
	moderator := agent.Agent{Name: "Moderator", Model: "test"}
	proto := &askHumanProtocol{agent: moderator, tools: []string{AskHumanToolID}}

	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     proto,
		Participants: []protocol.Participant{moderator, human},
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	askTool, err := sess.AskHumanTool()
	if err != nil {
		t.Fatalf("AskHumanTool() error = %v", err)
	}
	if err := sess.RegisterTool(askTool); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	result, err := sess.Run(context.Background(), message.User("start"))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %s", result.Status)
	}
	if result.Final != "done" {
		t.Fatalf("expected final message to be done, got %q", result.Final)
	}
}

type askHumanProvider struct {
	mu        sync.Mutex
	calls     int
	arguments json.RawMessage
	message   agent.Message
}

func (p *askHumanProvider) ID() string { return "ask-human-provider" }

func (p *askHumanProvider) Stream(_ context.Context, _ provider.Request) (provider.Stream, error) {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()

	if call == 1 {
		return &askHumanStream{events: []provider.Event{{
			Type: provider.EventToolCall,
			ToolCalls: []provider.ToolCall{{
				ID:        "call-1",
				ToolID:    AskHumanToolID,
				Arguments: p.arguments,
			}},
		}}}, nil
	}

	return &askHumanStream{events: []provider.Event{{
		Type:    provider.EventMessageCompleted,
		Message: p.message,
	}}}, nil
}

type askHumanStream struct {
	index  int
	events []provider.Event
}

func (s *askHumanStream) Recv() (provider.Event, error) {
	if s.index >= len(s.events) {
		return provider.Event{}, io.EOF
	}
	ev := s.events[s.index]
	s.index++
	return ev, nil
}

func (s *askHumanStream) Close() error { return nil }
