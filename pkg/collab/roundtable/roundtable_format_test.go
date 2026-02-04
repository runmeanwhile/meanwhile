package roundtable

import (
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

func TestFormatThreadPreservesContent(t *testing.T) {
	msg := agent.Message{
		Role: agent.RoleAssistant,
		Name: "Alice",
		Parts: []agent.ContentPart{
			{Type: agent.ContentPartText, Text: "Hello"},
			{Type: agent.ContentPartJSON, JSON: map[string]any{"k": "v"}},
		},
	}

	formatted := FormatThread([]agent.Message{msg})
	if len(formatted) != 1 {
		t.Fatalf("expected 1 formatted message, got %d", len(formatted))
	}

	text := formatted[0].Text()
	if !strings.Contains(text, "<agent:Alice>") {
		t.Fatalf("expected agent tag, got %q", text)
	}
	if !strings.Contains(text, "Hello") {
		t.Fatalf("expected text content to be preserved, got %q", text)
	}
	if !strings.Contains(text, "\"k\"") {
		t.Fatalf("expected JSON content to be preserved, got %q", text)
	}
}
