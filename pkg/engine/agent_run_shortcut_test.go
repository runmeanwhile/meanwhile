package engine

import (
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/message"
)

func TestAgentBuilderRun(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Test using Run() shortcut
	msg, err := eng.Agent("TestAgent").
		Prompt("You are a test agent").
		Model("test-model").
		Run(message.User("Hello"))

	// Will fail with "no default provider" without a real provider
	// This is expected - we're testing that the plumbing works
	if err != nil {
		if err.Error() != "protocol message: no default provider configured" {
			t.Fatalf("Unexpected error: %v", err)
		}
		// Expected error - no provider configured
		return
	}

	// If somehow it works (shouldn't with test setup)
	if msg.Role != agent.RoleAssistant {
		t.Errorf("Expected assistant role, got %v", msg.Role)
	}
}

func TestAgentBuilderRunNoMessages(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = eng.Agent("TestAgent").
		Prompt("You are a test agent").
		Model("test-model").
		Run()

	if err == nil {
		t.Error("Expected error when running with no messages")
	}
}
