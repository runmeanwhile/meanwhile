package agent

import (
	"testing"
)

// Mock tool for testing
type mockTool struct {
	id string
}

func (m *mockTool) ID() string { return m.id }

func TestBuilderToolsWithInstances(t *testing.T) {
	mock := &mockEngine{}
	b := NewBuilder(mock, "test")

	tool1 := &mockTool{id: "tool1"}
	tool2 := &mockTool{id: "tool2"}

	// Test: .Tools() with instances
	b.Tools(tool1, tool2)

	agent := b.Build()
	if len(agent.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(agent.Tools))
	}
	if agent.Tools[0] != "tool1" || agent.Tools[1] != "tool2" {
		t.Errorf("unexpected tool IDs: %v", agent.Tools)
	}
	if !mock.registerToolCalled {
		t.Error("expected RegisterTool to be called")
	}
}

func TestBuilderToolsWithStrings(t *testing.T) {
	mock := &mockEngine{}
	b := NewBuilder(mock, "test")

	// Test: .Tools() with string IDs (existing behavior)
	b.Tools("tool1", "tool2")

	agent := b.Build()
	if len(agent.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(agent.Tools))
	}
	if agent.Tools[0] != "tool1" || agent.Tools[1] != "tool2" {
		t.Errorf("unexpected tool IDs: %v", agent.Tools)
	}
}

func TestBuilderToolsMixed(t *testing.T) {
	mock := &mockEngine{}
	b := NewBuilder(mock, "test")

	tool1 := &mockTool{id: "instance_tool"}

	// Test: Mixed string IDs and instances
	b.Tools(tool1, "string_tool", &mockTool{id: "another_instance"})

	agent := b.Build()
	if len(agent.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(agent.Tools))
	}
	expected := []string{"instance_tool", "string_tool", "another_instance"}
	for i, id := range expected {
		if agent.Tools[i] != id {
			t.Errorf("tool[%d]: expected %s, got %s", i, id, agent.Tools[i])
		}
	}
}

func TestBuilderToolsChaining(t *testing.T) {
	mock := &mockEngine{}
	b := NewBuilder(mock, "test")

	tool1 := &mockTool{id: "tool1"}
	tool2 := &mockTool{id: "tool2"}

	// Test: Chaining multiple .Tools() calls
	b.Tools(tool1).Tools("tool2_id").Tools(tool2)

	agent := b.Build()
	if len(agent.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(agent.Tools))
	}
}
