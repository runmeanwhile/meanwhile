package engine

import (
	"context"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/protocol/consensus"
)

func TestSessionBuilder(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create agents for testing
	alice := eng.Agent("Alice").
		Prompt("You are Alice").
		Model("test-model").
		Build()

	bob := eng.Agent("Bob").
		Prompt("You are Bob").
		Model("test-model").
		Build()

	// Build session with fluent API
	sess, err := eng.Session("Test Session").
		Tags("test", "example").
		Metadata("key", "value").
		Protocol(protocol.Solo()).
		Participants(alice, bob).
		Start(context.Background())

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if sess.Name() != "Test Session" {
		t.Errorf("Expected name 'Test Session', got %q", sess.Name())
	}

	tags := sess.Tags()
	if len(tags) != 2 || tags[0] != "test" || tags[1] != "example" {
		t.Errorf("Expected tags [test, example], got %v", tags)
	}

	meta := sess.Metadata()
	if meta["key"] != "value" {
		t.Errorf("Expected metadata key='value', got %v", meta["key"])
	}
}

func TestSessionBuilderParticipant(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	alice := eng.Agent("Alice").
		Prompt("You are Alice").
		Model("test-model").
		Build()

	bob := eng.Agent("Bob").
		Prompt("You are Bob").
		Model("test-model").
		Build()

	// Use Participant() convenience method
	sess, err := eng.Session("Test").
		Participant(alice).
		Participant(bob).
		Start(context.Background())

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	participants := sess.Participants()
	if len(participants) != 2 {
		t.Errorf("Expected 2 participants, got %d", len(participants))
	}
}

func TestSessionBuilderDefaultProtocol(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	alice := eng.Agent("Alice").
		Prompt("You are Alice").
		Model("test-model").
		Build()

	// Don't specify protocol - should default to Solo
	sess, err := eng.Session("Test").
		Participant(alice).
		Start(context.Background())

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if sess == nil {
		t.Fatal("Expected session to be created with default protocol")
	}
}

func TestSessionBuilderFacilitator(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	alice := eng.Agent("Alice").
		Prompt("You are Alice").
		Model("test-model").
		Build()

	bob := eng.Agent("Bob").
		Prompt("You are Bob").
		Model("test-model").
		Build()

	facilitator := eng.Agent("Facilitator").
		Prompt("You are the facilitator").
		Model("test-model").
		Build()

	sess, err := eng.Session("Test").
		Participants(alice, bob).
		Facilitator(facilitator).
		Protocol(consensus.Consensus()).
		Start(context.Background())

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if sess.Facilitator() == nil {
		t.Error("Expected facilitator to be set")
	} else if sess.Facilitator().Name != "Facilitator" {
		t.Errorf("Expected facilitator name 'Facilitator', got %q", sess.Facilitator().Name)
	}
}

func TestSessionBuilderGroups(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	a1 := eng.Agent("A1").Prompt("A1").Model("test").Build()
	a2 := eng.Agent("A2").Prompt("A2").Model("test").Build()
	b1 := eng.Agent("B1").Prompt("B1").Model("test").Build()
	b2 := eng.Agent("B2").Prompt("B2").Model("test").Build()

	// Use Group() convenience method
	sess, err := eng.Session("Test").
		Participants(a1, a2, b1, b2).
		Group("GroupA", a1, a2).
		Group("GroupB", b1, b2).
		Protocol(protocol.Breakout()).
		Start(context.Background())

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	groups := sess.Groups()
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}

	if len(groups["GroupA"]) != 2 {
		t.Errorf("Expected 2 members in GroupA, got %d", len(groups["GroupA"]))
	}
}
func TestSessionBuilderRun(t *testing.T) {
	t.Skip("TODO: Fix mock provider stream for full integration test")
	eng, err := New(WithProvider(&mockProvider{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	alice := eng.Agent("Alice").
		Prompt("You are Alice").
		Model("test-model").
		Build()

	ctx := context.Background()
	msg := message.User("Hello")

	// Test Run method (creates ephemeral session, runs, then closes)
	result, err := eng.Session("ephemeral").
		Participant(alice).
		Protocol(protocol.Solo()).
		Run(ctx, msg)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Result should have been collected
	if len(result.Events) == 0 {
		t.Error("Expected events to be collected")
	}
}

func TestEngineRunProtocol(t *testing.T) {
	t.Skip("TODO: Fix mock provider stream for full integration test")
	eng, err := New(WithProvider(&mockProvider{}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	msg := message.User("Test message")

	alice := eng.Agent("Alice").Model("test-model").Build()

	// Test RunProtocol convenience method (uses ephemeral session internally)
	// RunProtocol uses Solo protocol but we need to give it participants
	result, err := eng.Session("test").
		Participant(alice).
		Protocol(protocol.Solo()).
		Run(ctx, msg)

	if err != nil {
		t.Fatalf("RunProtocol() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check events were collected
	if len(result.Events) == 0 {
		t.Error("Expected events to be collected")
	}
}
