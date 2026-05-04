package openai

import (
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
)

func TestBuildInputSupportsImageParts(t *testing.T) {
	msg := modelruntime.Message{
		Role: modelruntime.RoleUser,
		Parts: []modelruntime.Part{
			{Type: modelruntime.PartText, Text: "Describe this image."},
			{Type: modelruntime.PartImage, URI: "https://example.com/cat.png"},
		},
	}

	input, err := buildInput([]modelruntime.Message{msg})
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	if len(input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(input))
	}

	content, ok := input[0]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected content parts, got %T", input[0]["content"])
	}
	if len(content) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(content))
	}

	if content[0]["type"] != "input_text" || content[0]["text"] != "Describe this image." {
		t.Fatalf("unexpected text part: %#v", content[0])
	}
	if content[1]["type"] != "input_image" || content[1]["image_url"] != "https://example.com/cat.png" {
		t.Fatalf("unexpected image part: %#v", content[1])
	}
}

func TestBuildInputPassesToolImageOutputAsUserImage(t *testing.T) {
	msg := modelruntime.Message{
		Role:       modelruntime.RoleTool,
		Name:       "inspect_image",
		ToolCallID: "call-123",
		Parts: []modelruntime.Part{
			{Type: modelruntime.PartText, Text: "Original photo attached."},
			{Type: modelruntime.PartImage, URI: "data:image/jpeg;base64,abc123", Detail: "low"},
		},
		Metadata: map[string]any{
			"arguments": `{"question":"what is visible?"}`,
		},
	}

	input, err := buildInput([]modelruntime.Message{msg})
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	if len(input) != 3 {
		t.Fatalf("expected function call, output, and image attachment input; got %d", len(input))
	}

	if input[0]["type"] != "function_call" || input[0]["name"] != "inspect_image" {
		t.Fatalf("unexpected function call item: %#v", input[0])
	}
	if input[1]["type"] != "function_call_output" || input[1]["output"] != "Original photo attached." {
		t.Fatalf("unexpected function output item: %#v", input[1])
	}
	if input[2]["role"] != string(modelruntime.RoleUser) {
		t.Fatalf("expected tool image attachment to be reintroduced as user content, got %#v", input[2])
	}
	content, ok := input[2]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected content parts, got %T", input[2]["content"])
	}
	if len(content) != 2 {
		t.Fatalf("expected text and image attachment parts, got %d", len(content))
	}
	if content[1]["type"] != "input_image" || content[1]["image_url"] != "data:image/jpeg;base64,abc123" || content[1]["detail"] != "low" {
		t.Fatalf("unexpected tool image part: %#v", content[1])
	}
}

func TestBuildInputRequiresToolCallID(t *testing.T) {
	msg := modelruntime.Message{
		Role:  modelruntime.RoleTool,
		Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: "result"}},
	}

	if _, err := buildInput([]modelruntime.Message{msg}); err == nil {
		t.Fatalf("expected error for tool message without call id")
	}
}

func TestBuildInputWrapsNamedUserMessage(t *testing.T) {
	msg := modelruntime.Message{
		Role:  modelruntime.RoleUser,
		Name:  "Marketing",
		Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: "We should focus on week-3 retention."}},
	}

	input, err := buildInput([]modelruntime.Message{msg})
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	if len(input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(input))
	}
	if input[0]["role"] != string(modelruntime.RoleUser) {
		t.Fatalf("expected user role to be preserved, got %v", input[0]["role"])
	}

	content, ok := input[0]["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected non-empty content parts, got %#v", input[0]["content"])
	}
	text, _ := content[0]["text"].(string)
	if !strings.Contains(text, "<agent:Marketing>") {
		t.Fatalf("expected named message wrapper, got %q", text)
	}
}
