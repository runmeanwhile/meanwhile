package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

func TestProtocolAsToolSchema(t *testing.T) {
	// Create engine (no provider needed for schema test)
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create a solo protocol
	soloProto := protocol.Solo()

	// Wrap protocol as tool
	protoTool := eng.AsTool(soloProto,
		WithToolName("ask_specialist"),
		WithToolDescription("Ask specialist for help"),
	)

	// Verify tool ID
	if protoTool.ID() != "ask_specialist" {
		t.Errorf("Expected tool ID 'ask_specialist', got %q", protoTool.ID())
	}

	// Verify schema
	schema := protoTool.Schema()

	// Verify schema has required fields
	var schemaMap map[string]any
	if err := json.Unmarshal(schema.JSONSchema, &schemaMap); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Schema missing properties")
	}

	if _, ok := props["task"]; !ok {
		t.Errorf("Schema missing 'task' property")
	}

	if _, ok := props["context"]; !ok {
		t.Errorf("Schema missing 'context' property")
	}

	// Verify required fields
	required, ok := schemaMap["required"].([]any)
	if !ok {
		t.Fatalf("Schema missing required array")
	}

	if len(required) != 1 || required[0] != "task" {
		t.Errorf("Expected required=['task'], got %v", required)
	}
}

func TestProtocolAsToolDefaults(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	soloProto := protocol.Solo()
	protoTool := eng.AsTool(soloProto)

	// Default name
	if protoTool.ID() != "protocol_tool" {
		t.Errorf("Expected default ID 'protocol_tool', got %q", protoTool.ID())
	}

	// Default description can be verified from schema JSON
	schema := protoTool.Schema()
	var schemaMap map[string]any
	if err := json.Unmarshal(schema.JSONSchema, &schemaMap); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	if desc, ok := schemaMap["description"].(string); !ok || desc != "Delegates to a nested session" {
		t.Errorf("Expected default description 'Delegates to a nested session', got %q", desc)
	}
}

func TestProtocolAsToolInvalidArgsHandling(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	soloProto := protocol.Solo()
	protoTool := eng.AsTool(soloProto,
		WithToolName("test_tool"),
	)

	// Invalid JSON should result in error
	call := tool.Call{
		ID:        "call-1",
		ToolID:    "test_tool",
		Arguments: []byte(`{invalid`),
	}

	// Since we have no participants, Run will fail gracefully
	// We're just testing that invalid JSON is caught
	result, _ := protoTool.Run(nil, call, nil)
	if result.Error == nil || result.Error.Message == "" {
		t.Errorf("Expected error in result for invalid JSON")
	}
}

type requiresParticipantsProtocol struct {
	participants []protocol.Participant
}

func (p *requiresParticipantsProtocol) ID() string { return "protocol.requires_participants" }
func (p *requiresParticipantsProtocol) Participants() []protocol.Participant {
	return append([]protocol.Participant(nil), p.participants...)
}
func (p *requiresParticipantsProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (p *requiresParticipantsProtocol) OnMessage(_ context.Context, sess protocol.Session, _ agent.Message) error {
	if len(sess.Participants()) == 0 {
		return fmt.Errorf("participants required")
	}
	return nil
}
func (p *requiresParticipantsProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (p *requiresParticipantsProtocol) Shutdown(_ context.Context, _ protocol.Session) error {
	return nil
}

func TestProtocolAsToolAutoParticipants(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	proto := &requiresParticipantsProtocol{
		participants: []protocol.Participant{agent.Agent{Name: "Alice"}},
	}
	protoTool := eng.AsTool(proto, WithToolName("auto_participants"))

	call := tool.Call{
		ID:        "call-1",
		ToolID:    "auto_participants",
		Arguments: []byte(`{"task":"check participants"}`),
	}

	result, _ := protoTool.Run(context.Background(), call, nil)
	if result.Error != nil {
		t.Fatalf("expected protocol tool to succeed, got error: %s", result.Error.Message)
	}
}
