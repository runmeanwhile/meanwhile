package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// mockEngine implements engineRef for testing.
type mockEngine struct {
	profiles           []Profile
	tools              []any
	registerToolCalled bool
	runCalled          bool
	runAgent           Agent
	runMessage         Message
}

func (m *mockEngine) RegisterProfile(profile Profile) {
	m.profiles = append(m.profiles, profile)
}

func (m *mockEngine) RegisterTool(t any) error {
	m.registerToolCalled = true
	m.tools = append(m.tools, t)
	return nil
}

func (m *mockEngine) RunAgent(agent Agent, messages ...Message) (Message, error) {
	m.runCalled = true
	m.runAgent = agent
	if len(messages) > 0 {
		m.runMessage = messages[0]
	}
	return Message{Role: RoleAssistant, Parts: []ContentPart{{Type: ContentPartText, Text: "Test response"}}}, nil
}

func (m *mockEngine) RunAgentWithContext(_ context.Context, agent Agent, messages ...Message) (Message, error) {
	return m.RunAgent(agent, messages...)
}

func TestBuilderRun(t *testing.T) {
	mock := &mockEngine{}

	builder := NewBuilder(mock, "TestAgent").
		Prompt("Test prompt").
		Model("test-model").
		Tools("tool1", "tool2")

	// Call Run
	msg, err := builder.Run(Message{Role: RoleUser, Parts: []ContentPart{{Type: ContentPartText, Text: "Hello"}}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Check that RunAgent was called
	if !mock.runCalled {
		t.Error("Expected RunAgent to be called")
	}

	// Check agent configuration
	if mock.runAgent.Name != "TestAgent" {
		t.Errorf("Expected agent name 'TestAgent', got %q", mock.runAgent.Name)
	}
	if mock.runAgent.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got %q", mock.runAgent.Model)
	}

	// Check message was passed
	if mock.runMessage.Text() != "Hello" {
		t.Errorf("Expected message 'Hello', got %q", mock.runMessage.Text())
	}

	// Check response
	if msg.Role != RoleAssistant || msg.Text() != "Test response" {
		t.Errorf("Expected assistant message 'Test response', got %v", msg)
	}
}

func TestBuilderBuild(t *testing.T) {
	mock := &mockEngine{}

	type TestOutput struct {
		Result string `json:"result"`
	}

	agent := NewBuilder(mock, "TestAgent").
		Prompt("Test prompt").
		Model("test-model").
		Tools("tool1", "tool2").
		Param("temp", 0.7).
		OutputSchema(TestOutput{}).
		Build()

	if agent.Name != "TestAgent" {
		t.Errorf("Expected name 'TestAgent', got %q", agent.Name)
	}
	if agent.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got %q", agent.Model)
	}
	if len(agent.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(agent.Tools))
	}
	if agent.Params["temp"] != 0.7 {
		t.Errorf("Expected temp param 0.7, got %v", agent.Params["temp"])
	}
	if agent.OutputSchema == nil {
		t.Error("Expected OutputSchema to be set")
	}

	// Check profile was registered
	if len(mock.profiles) != 1 {
		t.Errorf("Expected 1 profile registered, got %d", len(mock.profiles))
	}
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestGenerateProfileIDFallback(t *testing.T) {
	original := randReader
	randReader = failingReader{}
	defer func() { randReader = original }()

	id := generateProfileID("Test Agent")
	expectedPrefix := "profile-" + sanitizeName("Test Agent") + "-"
	if !strings.HasPrefix(id, expectedPrefix) {
		t.Fatalf("unexpected profile id: %s", id)
	}
}
