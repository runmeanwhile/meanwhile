package agent

import (
"testing"
)

type mockToolWithID struct {
id string
}

func (mt mockToolWithID) ID() string { return mt.id }

func TestBuilderToolMethod(t *testing.T) {
mock := &mockEngine{}

testTool := mockToolWithID{id: "test_tool"}

agent := NewBuilder(mock, "TestAgent").
Prompt("Test").
Tool(testTool). // Should register AND add to agent
Build()

// Verify tool was registered with engine
if len(mock.tools) != 1 {
t.Errorf("Expected 1 tool registered, got %d", len(mock.tools))
}

// Verify tool was added to agent
if len(agent.Tools) != 1 {
t.Errorf("Expected 1 tool on agent, got %d", len(agent.Tools))
}
if agent.Tools[0] != "test_tool" {
t.Errorf("Expected tool ID 'test_tool', got %q", agent.Tools[0])
}
}

func TestBuilderToolWithoutID(t *testing.T) {
mock := &mockEngine{}

type toolNoID struct{}
testTool := toolNoID{}

agent := NewBuilder(mock, "TestAgent").
Prompt("Test").
Tool(testTool). // Should register but NOT add to agent (no ID method)
Build()

// Tool was registered with engine
if len(mock.tools) != 1 {
t.Errorf("Expected 1 tool registered, got %d", len(mock.tools))
}

// But NOT added to agent (no ID() method)
if len(agent.Tools) != 0 {
t.Errorf("Expected 0 tools on agent (no ID method), got %d", len(agent.Tools))
}
}
