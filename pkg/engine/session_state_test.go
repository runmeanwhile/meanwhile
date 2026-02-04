package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

type stateProtocol struct{}

func (stateProtocol) ID() string                                       { return "protocol.state" }
func (stateProtocol) Participants() []protocol.Participant             { return nil }
func (stateProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (stateProtocol) OnMessage(_ context.Context, _ protocol.Session, _ agent.Message) error {
	return nil
}
func (stateProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (stateProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func TestSessionStateRoundTrip(t *testing.T) {
	state := SessionState{
		SessionID: "sess-1",
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
		Pending: []protocol.InputRequest{
			{
				RequestID:       "req-1",
				ParticipantID:   "user",
				ParticipantName: "User",
				Context:         "context",
				RequestedAt:     time.Now().UTC().Add(-time.Minute).Truncate(time.Second),
				TimeoutAt:       time.Now().UTC().Add(time.Minute).Truncate(time.Second),
			},
		},
	}

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal state failed: %v", err)
	}

	var decoded SessionState
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal state failed: %v", err)
	}

	if decoded.SessionID != state.SessionID {
		t.Fatalf("expected session id %q, got %q", state.SessionID, decoded.SessionID)
	}
	if !decoded.UpdatedAt.Equal(state.UpdatedAt) {
		t.Fatalf("expected updated at %v, got %v", state.UpdatedAt, decoded.UpdatedAt)
	}
	if len(decoded.Pending) != 1 || decoded.Pending[0].RequestID != "req-1" {
		t.Fatalf("expected pending request to survive round trip")
	}
	if !decoded.Pending[0].TimeoutAt.Equal(state.Pending[0].TimeoutAt) {
		t.Fatalf("expected timeout to survive round trip")
	}
}

func TestSessionPendingRequests(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	human := eng.Human("User").Build()
	sess, err := eng.Session("pending").
		Participant(human).
		Protocol(stateProtocol{}).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	_ = sess.AwaitInput(context.Background(), human, "first", func(_ context.Context, _ agent.Message) error {
		return nil
	})
	_ = sess.AwaitInput(context.Background(), human, "second", func(_ context.Context, _ agent.Message) error {
		return nil
	})

	if !sess.IsPaused() {
		t.Fatalf("expected session to be paused")
	}

	pending := sess.PendingRequests()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending requests, got %d", len(pending))
	}
	contexts := map[string]bool{
		pending[0].Context: true,
		pending[1].Context: true,
	}
	if !contexts["first"] || !contexts["second"] {
		t.Fatalf("expected pending contexts to match")
	}
}

func TestSessionTimedOutRequests(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	human := eng.Human("User").Build()
	sess, err := eng.Session("timeouts").
		Participant(human).
		Protocol(stateProtocol{}).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	past := time.Now().UTC().Add(-time.Minute)
	_ = sess.AwaitInput(context.Background(), human, "expired", func(_ context.Context, _ agent.Message) error {
		return nil
	}, protocol.WithInputDeadline(past))

	timedOut := sess.TimedOutRequests(time.Now().UTC())
	if len(timedOut) != 1 || timedOut[0].Context != "expired" {
		t.Fatalf("expected expired request to be detected")
	}
}

func TestRespondTimeoutContinueWithNote(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	human := eng.Human("User").Build()
	sess, err := eng.Session("timeout-continue").
		Participant(human).
		Protocol(stateProtocol{}).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	var received agent.Message
	past := time.Now().UTC().Add(-time.Minute)
	err = sess.AwaitInput(context.Background(), human, "context", func(_ context.Context, msg agent.Message) error {
		received = msg
		return nil
	}, protocol.WithInputDeadline(past))
	awaitErr := &protocol.AwaitingInputError{}
	if !errors.As(err, &awaitErr) {
		t.Fatalf("expected awaiting input error")
	}

	result, err := sess.Respond(context.Background(), awaitErr.Request.RequestID, agent.Message{}, OnTimeout(ContinueWithNote("no response")))
	if err != nil {
		t.Fatalf("Respond failed: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed status, got %s", result.Status)
	}
	if received.Role != agent.RoleSystem {
		t.Fatalf("expected system message for timeout, got %s", received.Role)
	}
	if received.Text() != "no response" {
		t.Fatalf("expected timeout note to be passed through")
	}
	if sess.IsPaused() {
		t.Fatalf("expected session to be resumed")
	}
}

func TestRespondTimeoutRetry(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	human := eng.Human("User").Build()
	sess, err := eng.Session("timeout-retry").
		Participant(human).
		Protocol(stateProtocol{}).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	past := time.Now().UTC().Add(-time.Minute)
	err = sess.AwaitInput(context.Background(), human, "context", func(_ context.Context, _ agent.Message) error {
		t.Fatalf("resume should not be called on retry")
		return nil
	}, protocol.WithInputDeadline(past))
	if err == nil {
		t.Fatalf("expected awaiting input error")
	}

	pending := sess.PendingRequests()
	if len(pending) != 1 {
		t.Fatalf("expected a pending request")
	}
	result, err := sess.Respond(context.Background(), pending[0].RequestID, agent.Message{}, OnTimeout(RetryWith("User")))
	if err != nil {
		t.Fatalf("Respond failed: %v", err)
	}
	if result.Status != StatusAwaitingInput {
		t.Fatalf("expected awaiting input status, got %s", result.Status)
	}
	if result.RequestID == pending[0].RequestID {
		t.Fatalf("expected new request ID after retry")
	}
}
