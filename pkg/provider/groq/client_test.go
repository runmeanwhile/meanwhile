package groq

import (
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

func TestBuildMessagesCarriesToolImageOutputAsUserContent(t *testing.T) {
	msg := modelruntime.Message{
		Role:       modelruntime.RoleTool,
		Name:       "inspect_image",
		ToolCallID: "call_1",
		Parts: []modelruntime.Part{
			{Type: modelruntime.PartText, Text: "Image attached."},
			{Type: modelruntime.PartImage, URI: "data:image/jpeg;base64,abc123", Detail: "low"},
		},
		Metadata: map[string]any{"arguments": `{"question":"look"}`},
	}

	messages := buildMessages([]modelruntime.Message{msg})
	if len(messages) != 3 {
		t.Fatalf("expected assistant tool call, tool result, and user image attachment, got %d messages", len(messages))
	}
	if messages[0]["role"] != "assistant" || messages[0]["tool_calls"] == nil {
		t.Fatalf("unexpected assistant tool call message: %#v", messages[0])
	}
	if messages[1]["role"] != "tool" || messages[1]["content"] != "Image attached." {
		t.Fatalf("unexpected tool message: %#v", messages[1])
	}
	content, ok := messages[2]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected multimodal user content, got %T", messages[2]["content"])
	}
	if len(content) != 2 {
		t.Fatalf("expected text and image parts, got %d", len(content))
	}
	if content[1]["type"] != "image_url" {
		t.Fatalf("expected image_url part, got %#v", content[1])
	}
}

func TestBuildPayloadOmitToolsAfterToolResult(t *testing.T) {
	payload := buildPayload(providerRequestWithToolResult())
	if _, ok := payload["tools"]; ok {
		t.Fatalf("tools should be omitted after a tool result: %#v", payload["tools"])
	}
	if _, ok := payload["tool_choice"]; ok {
		t.Fatalf("tool_choice should be omitted after a tool result: %#v", payload["tool_choice"])
	}
}

func providerRequestWithToolResult() provider.Request {
	return provider.Request{
		Model: "meta-llama/llama-4-scout-17b-16e-instruct",
		Messages: []modelruntime.Message{{
			Role:       modelruntime.RoleTool,
			Name:       "inspect_image",
			ToolCallID: "call_1",
			Parts:      []modelruntime.Part{{Type: modelruntime.PartText, Text: "done"}},
		}},
		Tools: []modelruntime.ToolDefinition{{
			ID: "inspect_image",
		}},
	}
}
