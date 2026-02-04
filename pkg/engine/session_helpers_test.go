package engine

import (
	"context"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

func TestNewSessionRejectsDuplicateParticipantNames(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	participants := []protocol.Participant{agent.Agent{Name: "dup"}, agent.Agent{Name: "dup"}}
	_, err = eng.NewSession(context.Background(), SessionConfig{
		Name:         "dups",
		Protocol:     protocol.Solo(),
		Participants: participants,
	})
	if err == nil {
		t.Fatalf("expected error for duplicate participant names")
	}
}
