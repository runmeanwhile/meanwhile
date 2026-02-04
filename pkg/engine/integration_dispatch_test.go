package engine

import (
	"context"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/integration"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

type integrationNoopProtocol struct{}

func (integrationNoopProtocol) ID() string                                       { return "noop.integration" }
func (integrationNoopProtocol) Participants() []protocol.Participant             { return nil }
func (integrationNoopProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (integrationNoopProtocol) OnMessage(_ context.Context, _ protocol.Session, _ agent.Message) error {
	return nil
}
func (integrationNoopProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (integrationNoopProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

type recordingIntegration struct {
	ch chan integration.Request
}

func (r *recordingIntegration) ID() string      { return "recording" }
func (r *recordingIntegration) Channel() string { return "email" }
func (r *recordingIntegration) Send(_ context.Context, req integration.Request) error {
	r.ch <- req
	return nil
}

func TestIntegrationDispatchOnHumanRequestEvent(t *testing.T) {
	recorder := &recordingIntegration{ch: make(chan integration.Request, 1)}
	eng, err := New(WithIntegration(recorder))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	human := eng.Human("User").ID("user").ContactVia("email", "user@example.com").PreferredChannel("email").Build()
	agentParticipant := agent.Agent{Name: "Moderator", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     integrationNoopProtocol{},
		Participants: []protocol.Participant{agentParticipant, human},
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	req := HumanRequest{
		RequestID:       "req-123",
		SessionID:       sess.ID(),
		ProtocolID:      sess.ProtocolID(),
		ParticipantID:   "user",
		ParticipantName: "User",
		Question:        "Need input",
		RequestedAt:     time.Now().UTC(),
	}
	if err := sess.Emit(event.New(event.HumanRequestCreated, sess.ID(), req)); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	select {
	case got := <-recorder.ch:
		if got.RequestID != "req-123" {
			t.Fatalf("unexpected request id %q", got.RequestID)
		}
		if got.Channel != "email" || got.Contact != "user@example.com" {
			t.Fatalf("expected email contact, got %s %s", got.Channel, got.Contact)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected integration dispatch")
	}
}
