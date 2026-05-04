package anthropic

import (
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
)

func TestToolResultContentCarriesImageParts(t *testing.T) {
	msg := modelruntime.Message{
		Role:       modelruntime.RoleTool,
		Name:       "inspect_image",
		ToolCallID: "toolu_1",
		Parts: []modelruntime.Part{
			{Type: modelruntime.PartText, Text: "Image attached."},
			{Type: modelruntime.PartImage, URI: "data:image/jpeg;base64,abc123"},
		},
		Metadata: map[string]any{"arguments": `{"question":"describe"}`},
	}

	exchange := anthropicToolExchange(msg)
	if len(exchange) != 2 {
		t.Fatalf("expected assistant tool_use and user tool_result, got %d messages", len(exchange))
	}
	userContent, ok := exchange[1]["content"].([]map[string]any)
	if !ok || len(userContent) != 1 {
		t.Fatalf("unexpected tool result content: %#v", exchange[1]["content"])
	}
	resultContent, ok := userContent[0]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected multimodal tool result content, got %T", userContent[0]["content"])
	}
	if len(resultContent) != 2 {
		t.Fatalf("expected text and image tool result parts, got %d", len(resultContent))
	}
	if resultContent[1]["type"] != "image" {
		t.Fatalf("expected image part, got %#v", resultContent[1])
	}
}
