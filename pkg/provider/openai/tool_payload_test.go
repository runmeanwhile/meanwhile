package openai

import (
	"encoding/json"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

func TestBuildRequestPayloadUsesObjectSchema(t *testing.T) {
	payload, err := buildRequestPayload(provider.Request{
		Model: "gpt-test",
		Tools: []tool.Definition{{ID: "do_thing"}},
	})
	if err != nil {
		t.Fatalf("buildRequestPayload: %v", err)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	toolsVal, ok := decoded["tools"].([]any)
	if !ok || len(toolsVal) != 1 {
		t.Fatalf("expected tools array, got %T", decoded["tools"])
	}

	toolMap := toolsVal[0].(map[string]any)
	params := toolMap["parameters"]
	if _, ok := params.(map[string]any); !ok {
		t.Fatalf("expected parameters to be an object, got %T", params)
	}
}

func TestBuildToolOutputItemsUsesArgumentsFromMetadata(t *testing.T) {
	args := json.RawMessage(`{"x":1}`)
	msg := agent.Message{
		Role:       agent.RoleTool,
		Name:       "calc",
		ToolCallID: "call-1",
		Parts:      []agent.ContentPart{{Type: agent.ContentPartText, Text: "done"}},
		Metadata:   map[string]any{"arguments": args},
	}

	items, err := buildToolOutputItems(msg)
	if err != nil {
		t.Fatalf("buildToolOutputItems: %v", err)
	}

	if len(items) == 0 {
		t.Fatalf("expected items")
	}

	first := items[0]
	if first["type"] != "function_call" {
		t.Fatalf("expected function_call item, got %v", first["type"])
	}

	argVal, ok := first["arguments"].(string)
	if !ok {
		t.Fatalf("expected arguments to be string, got %T", first["arguments"])
	}
	if argVal == "{}" {
		t.Fatalf("expected arguments to be preserved from metadata")
	}
}
