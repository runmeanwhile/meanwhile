package agent

import "testing"

func TestAgentValidate(t *testing.T) {
	agent := Agent{Name: ""}
	if err := agent.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}

	agent = Agent{Name: "Agent 1"}
	if err := agent.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
