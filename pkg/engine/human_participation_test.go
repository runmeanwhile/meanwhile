package engine

import (
	"context"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

type awaitProtocol struct {
	participant protocol.Participant
}

func (p *awaitProtocol) ID() string { return "protocol.await_input" }
func (p *awaitProtocol) Participants() []protocol.Participant {
	return []protocol.Participant{p.participant}
}
func (p *awaitProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (p *awaitProtocol) OnMessage(ctx context.Context, sess protocol.Session, _ agent.Message) error {
	return sess.AwaitInput(ctx, p.participant, "need your input", func(ctx context.Context, resp agent.Message) error {
		_ = ctx
		payload := map[string]any{"response": resp.Text()}
		return sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload))
	})
}
func (p *awaitProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (p *awaitProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func TestSessionAwaitInputAndRespond(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	human := eng.Human("User").Build()
	proto := &awaitProtocol{participant: human}
	sess, err := eng.Session("Await Test").
		Participant(human).
		Protocol(proto).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result, err := sess.Run(context.Background(), message.User("start"))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusAwaitingInput {
		t.Fatalf("expected awaiting status, got %s", result.Status)
	}
	if result.RequestID == "" {
		t.Fatalf("expected request ID to be set")
	}
	if result.Context != "need your input" {
		t.Fatalf("expected context to be propagated")
	}
	if result.AwaitingInput == nil || result.AwaitingInput.ParticipantName != "User" {
		t.Fatalf("expected awaiting input details to be set")
	}

	foundAwait := false
	foundPaused := false
	for _, ev := range result.Events {
		if ev.Type == event.AwaitingUserInput {
			foundAwait = true
		}
		if ev.Type == event.SessionPaused {
			foundPaused = true
		}
	}
	if !foundAwait {
		t.Fatalf("expected awaiting user input event")
	}
	if !foundPaused {
		t.Fatalf("expected session paused event")
	}

	resp, err := sess.Respond(context.Background(), result.RequestID, message.User("ok"))
	if err != nil {
		t.Fatalf("Respond() error = %v", err)
	}
	if resp.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %s", resp.Status)
	}
	if resp.Metadata["response"] != "ok" {
		t.Fatalf("expected response in metadata, got %v", resp.Metadata["response"])
	}

	foundResumed := false
	foundResponse := false
	for _, ev := range resp.Events {
		if ev.Type == event.SessionResumed {
			foundResumed = true
		}
		if ev.Type == event.HumanResponseReceived {
			foundResponse = true
		}
	}
	if !foundResumed {
		t.Fatalf("expected session resumed event")
	}
	if !foundResponse {
		t.Fatalf("expected human response received event")
	}

	if _, err := sess.Respond(context.Background(), result.RequestID, message.User("again")); err == nil {
		t.Fatalf("expected error for reused request ID")
	}
}
