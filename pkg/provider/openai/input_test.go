package openai

import (
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

func TestBuildInputSupportsImageParts(t *testing.T) {
	msg := agent.Message{
		Role: agent.RoleUser,
		Parts: []agent.ContentPart{
			{Type: agent.ContentPartText, Text: "Describe this image."},
			{Type: agent.ContentPartImage, URI: "https://example.com/cat.png"},
		},
	}

	input, err := buildInput([]agent.Message{msg})
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

func TestBuildInputRequiresToolCallID(t *testing.T) {
	msg := agent.Message{
		Role:  agent.RoleTool,
		Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "result"}},
	}

	if _, err := buildInput([]agent.Message{msg}); err == nil {
		t.Fatalf("expected error for tool message without call id")
	}
}

func TestBuildInputWrapsNamedUserMessage(t *testing.T) {
	msg := agent.Message{
		Role:  agent.RoleUser,
		Name:  "Marketing",
		Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "We should focus on week-3 retention."}},
	}

	input, err := buildInput([]agent.Message{msg})
	if err != nil {
		t.Fatalf("buildInput: %v", err)
	}
	if len(input) != 1 {
		t.Fatalf("expected 1 input item, got %d", len(input))
	}
	if input[0]["role"] != string(agent.RoleUser) {
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
