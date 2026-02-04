package event

import (
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

func TestEventString(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	ev := Event{
		ID:         "event-123",
		Type:       AgentStarted,
		Time:       now,
		SessionID:  "sess-456",
		ProtocolID: "protocol.solo",
		AgentID:    "agent-789",
	}

	str := ev.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Check that key fields are included
	expectedSubstrings := []string{
		"2026-01-15",
		string(AgentStarted),
		"session=sess-456",
		"protocol=protocol.solo",
		"agent=agent-789",
	}

	for _, expected := range expectedSubstrings {
		if !contains(str, expected) {
			t.Errorf("Expected string to contain %q, got: %s", expected, str)
		}
	}
}

func TestEventStringMinimal(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	ev := Event{
		Type: AgentStarted,
		Time: now,
	}

	str := ev.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Should still have timestamp and type even without optional fields
	if !contains(str, "2026-01-15") {
		t.Error("Expected timestamp in string")
	}
	if !contains(str, string(AgentStarted)) {
		t.Error("Expected event type in string")
	}
}

func TestSummaryAgentMessageDelta(t *testing.T) {
	ev := Event{
		Type: AgentMessageDelta,
		Payload: map[string]any{
			"delta": "Hello, world!",
		},
	}

	summary := Summary(ev)
	if !contains(summary, "delta=") {
		t.Errorf("Expected summary to contain delta, got: %s", summary)
	}
	if !contains(summary, "Hello, world!") {
		t.Errorf("Expected summary to contain delta content, got: %s", summary)
	}
}

func TestSummaryAgentReasoningDelta(t *testing.T) {
	ev := Event{
		Type: AgentReasoningDelta,
		Payload: map[string]any{
			"delta": "Reasoning step",
		},
	}

	summary := Summary(ev)
	if !contains(summary, "reasoning=") {
		t.Errorf("Expected summary to contain reasoning, got: %s", summary)
	}
	if !contains(summary, "Reasoning step") {
		t.Errorf("Expected summary to contain delta content, got: %s", summary)
	}
}

func TestSummaryAgentReasoningSummaryDelta(t *testing.T) {
	ev := Event{
		Type: AgentReasoningSummaryDelta,
		Payload: map[string]any{
			"delta": "Short summary",
		},
	}

	summary := Summary(ev)
	if !contains(summary, "reasoning_summary=") {
		t.Errorf("Expected summary to contain reasoning summary, got: %s", summary)
	}
	if !contains(summary, "Short summary") {
		t.Errorf("Expected summary to contain delta content, got: %s", summary)
	}
}

func TestSummaryAgentMessageComplete(t *testing.T) {
	ev := Event{
		Type: AgentMessageComplete,
		Payload: agent.Message{
			Role:  agent.RoleAssistant,
			Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "This is a complete message"}},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "message=") {
		t.Errorf("Expected summary to contain message, got: %s", summary)
	}
	if !contains(summary, "This is a complete message") {
		t.Errorf("Expected summary to contain message content, got: %s", summary)
	}
}

func TestSummaryAgentMessageCompleteFromMap(t *testing.T) {
	ev := Event{
		Type: AgentMessageComplete,
		Payload: map[string]any{
			"message": agent.Message{
				Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Message from map"}},
			},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "Message from map") {
		t.Errorf("Expected summary to contain message content, got: %s", summary)
	}
}

func TestSummaryAgentMessageCompleteFromNestedMap(t *testing.T) {
	ev := Event{
		Type: AgentMessageComplete,
		Payload: map[string]any{
			"message": map[string]any{
				"parts": []map[string]any{
					{"type": "text", "text": "Nested message content"},
				},
			},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "Nested message content") {
		t.Errorf("Expected summary to contain nested content, got: %s", summary)
	}
}

func TestSummaryAgentMessageCompleteWithImage(t *testing.T) {
	ev := Event{
		Type: AgentMessageComplete,
		Payload: agent.Message{
			Role: agent.RoleUser,
			Parts: []agent.ContentPart{
				{Type: agent.ContentPartImage, URI: "https://example.com/cat.png"},
			},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "[image]") {
		t.Errorf("Expected summary to contain image marker, got: %s", summary)
	}
}

func TestSummaryToolCallCompleted(t *testing.T) {
	ev := Event{
		Type: ToolCallCompleted,
		Payload: tool.Result{
			Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Tool execution result"}},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "result=") {
		t.Errorf("Expected summary to contain result, got: %s", summary)
	}
	if !contains(summary, "Tool execution result") {
		t.Errorf("Expected summary to contain result content, got: %s", summary)
	}
}

func TestSummaryToolCallError(t *testing.T) {
	ev := Event{
		Type: ToolCallError,
		Payload: tool.Result{
			Error: &tool.Error{Message: "Tool execution failed"},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "error=") {
		t.Errorf("Expected summary to contain error, got: %s", summary)
	}
	if !contains(summary, "Tool execution failed") {
		t.Errorf("Expected summary to contain error message, got: %s", summary)
	}
}

func TestSummaryToolCallCompletedFromMap(t *testing.T) {
	ev := Event{
		Type: ToolCallCompleted,
		Payload: map[string]any{
			"result": tool.Result{
				Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Result from map"}},
			},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "Result from map") {
		t.Errorf("Expected summary to contain result content, got: %s", summary)
	}
}

func TestSummaryToolCallFromNestedMap(t *testing.T) {
	ev := Event{
		Type: ToolCallCompleted,
		Payload: map[string]any{
			"result": map[string]any{
				"parts": []map[string]any{
					{"type": "text", "text": "Nested result content"},
				},
			},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "Nested result content") {
		t.Errorf("Expected summary to contain nested content, got: %s", summary)
	}
}

func TestSummaryToolCallErrorFromNestedMap(t *testing.T) {
	ev := Event{
		Type: ToolCallError,
		Payload: map[string]any{
			"result": map[string]any{
				"error": map[string]any{"message": "Nested error message"},
			},
		},
	}

	summary := Summary(ev)
	if !contains(summary, "Nested error message") {
		t.Errorf("Expected summary to contain nested error, got: %s", summary)
	}
}

func TestSummaryProtocolAction(t *testing.T) {
	ev := Event{
		Type: ProtocolAction,
	}

	summary := Summary(ev)
	if summary != "action" {
		t.Errorf("Expected summary 'action', got: %s", summary)
	}
}

func TestSummaryUnsupportedType(t *testing.T) {
	ev := Event{
		Type: AgentStarted,
	}

	summary := Summary(ev)
	if summary != "" {
		t.Errorf("Expected empty summary for unsupported type, got: %s", summary)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "no truncation needed",
			input:    "short",
			maxLen:   10,
			expected: "short",
		},
		{
			name:     "exact length",
			input:    "exactly10!",
			maxLen:   10,
			expected: "exactly10!",
		},
		{
			name:     "truncate with ellipsis",
			input:    "This is a very long string that needs truncation",
			maxLen:   20,
			expected: "This is a very lo...",
		},
		{
			name:     "maxLen zero",
			input:    "test",
			maxLen:   0,
			expected: "test",
		},
		{
			name:     "maxLen negative",
			input:    "test",
			maxLen:   -5,
			expected: "test",
		},
		{
			name:     "maxLen very small",
			input:    "test",
			maxLen:   2,
			expected: "te",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestNewEvent(t *testing.T) {
	ev := New(AgentStarted, "sess-123", "test payload")

	if ev.ID == "" {
		t.Error("Expected event ID to be generated")
	}

	if ev.Type != AgentStarted {
		t.Errorf("Expected type %s, got %s", AgentStarted, ev.Type)
	}

	if ev.SessionID != "sess-123" {
		t.Errorf("Expected session ID 'sess-123', got %s", ev.SessionID)
	}

	if ev.Payload != "test payload" {
		t.Errorf("Expected payload 'test payload', got %v", ev.Payload)
	}

	if ev.Time.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
