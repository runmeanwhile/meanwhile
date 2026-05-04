package groq

import (
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
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
	}

	messages := buildMessages([]modelruntime.Message{msg})
	if len(messages) != 2 {
		t.Fatalf("expected tool result and user image attachment, got %d messages", len(messages))
	}
	if messages[0]["role"] != "tool" || messages[0]["content"] != "Image attached." {
		t.Fatalf("unexpected tool message: %#v", messages[0])
	}
	content, ok := messages[1]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected multimodal user content, got %T", messages[1]["content"])
	}
	if len(content) != 2 {
		t.Fatalf("expected text and image parts, got %d", len(content))
	}
	if content[1]["type"] != "image_url" {
		t.Fatalf("expected image_url part, got %#v", content[1])
	}
}
