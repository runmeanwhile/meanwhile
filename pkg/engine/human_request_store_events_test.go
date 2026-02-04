package engine

import (
	"context"
	"testing"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/integration"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

type inboxRecordingIntegration struct {
	ch chan integration.Request
}

func (r *inboxRecordingIntegration) ID() string      { return "recording" }
func (r *inboxRecordingIntegration) Channel() string { return "email" }
func (r *inboxRecordingIntegration) Send(_ context.Context, req integration.Request) error {
	r.ch <- req
	return nil
}

func TestHumanRequestStoreLifecycle(t *testing.T) {
	store := NewInMemoryHumanRequestStore()
	recorder := &inboxRecordingIntegration{ch: make(chan integration.Request, 1)}

	eng, err := New(
		WithIntegration(recorder),
		WithHumanRequestStore(store),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	human := eng.Human("User").ID("user").ContactVia("email", "user@example.com").PreferredChannel("email").Build()
	agentParticipant := agent.Agent{Name: "Moderator", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     integrationNoopProtocol{},
		Participants: []protocol.Participant{agentParticipant, human},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
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
		t.Fatalf("emit: %v", err)
	}

	select {
	case <-recorder.ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected integration dispatch")
	}

	waitForStatus(t, store, "req-123", HumanRequestStatusSent)

	resp := HumanResponse{
		RequestID:       "req-123",
		SessionID:       sess.ID(),
		ProtocolID:      sess.ProtocolID(),
		ParticipantID:   "user",
		ParticipantName: "User",
		Response:        message.User("ok"),
		ReceivedAt:      time.Now().UTC(),
	}
	if err := sess.Emit(event.New(event.HumanResponseReceived, sess.ID(), resp)); err != nil {
		t.Fatalf("emit response: %v", err)
	}

	waitForStatus(t, store, "req-123", HumanRequestStatusAnswered)
}

func waitForStatus(t *testing.T, store HumanRequestStore, requestID string, status HumanRequestStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		record, err := store.Get(context.Background(), requestID)
		if err == nil && record.Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := store.Get(context.Background(), requestID)
	t.Fatalf("expected status %s, got %s", status, record.Status)
}
