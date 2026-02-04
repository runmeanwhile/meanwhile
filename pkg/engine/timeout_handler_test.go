package engine

import (
	"context"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

type timeoutProtocol struct {
	participant protocol.Participant
	responseCh  chan agent.Message
	timeoutAt   time.Time
}

func (p *timeoutProtocol) ID() string { return "protocol.timeout_input" }
func (p *timeoutProtocol) Participants() []protocol.Participant {
	return []protocol.Participant{p.participant}
}
func (p *timeoutProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (p *timeoutProtocol) OnMessage(ctx context.Context, sess protocol.Session, _ agent.Message) error {
	return sess.AwaitInput(ctx, p.participant, "need input", func(_ context.Context, resp agent.Message) error {
		p.responseCh <- resp
		return nil
	}, protocol.WithInputDeadline(p.timeoutAt))
}
func (p *timeoutProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (p *timeoutProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func TestHandleTimeoutUsesDefaultPolicy(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	human := eng.Human("User").Build()
	responseCh := make(chan agent.Message, 1)
	proto := &timeoutProtocol{
		participant: human,
		responseCh:  responseCh,
		timeoutAt:   time.Now().Add(-time.Second),
	}

	sess, err := eng.Session("Timeout Test").
		Participant(human).
		Protocol(proto).
		TimeoutPolicy(ContinueWithNote("timed out")).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	result, err := sess.Run(context.Background(), message.User("start"))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusAwaitingInput {
		t.Fatalf("expected awaiting input status")
	}

	if _, err := sess.HandleTimeout(context.Background(), result.RequestID); err != nil {
		t.Fatalf("HandleTimeout() error = %v", err)
	}

	select {
	case msg := <-responseCh:
		if msg.Role != agent.RoleSystem {
			t.Fatalf("expected system message, got %s", msg.Role)
		}
		if msg.Text() != "timed out" {
			t.Fatalf("unexpected timeout note: %s", msg.Text())
		}
		if msg.Metadata == nil || msg.Metadata["timeout"] != true {
			t.Fatalf("expected timeout metadata")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected timeout response")
	}
}

func TestHandleTimeoutRequiresPolicy(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	human := eng.Human("User").Build()
	responseCh := make(chan agent.Message, 1)
	proto := &timeoutProtocol{
		participant: human,
		responseCh:  responseCh,
		timeoutAt:   time.Now().Add(-time.Second),
	}

	sess, err := eng.Session("Timeout Missing Policy").
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

	if _, err := sess.HandleTimeout(context.Background(), result.RequestID); err != ErrTimeoutPolicyRequired {
		t.Fatalf("expected ErrTimeoutPolicyRequired, got %v", err)
	}
}
