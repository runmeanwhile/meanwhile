package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

type staticStore struct {
	items []Item
}

func (s *staticStore) Append(_ context.Context, _ string, _ event.Event) error { return nil }
func (s *staticStore) Query(_ context.Context, _ Query) ([]Item, error) {
	return append([]Item(nil), s.items...), nil
}
func (s *staticStore) Summarize(_ context.Context, _ string, _ Policy) (Summary, error) {
	return Summary{}, nil
}
func (s *staticStore) Stats(_ context.Context, _ string, _ Policy) (EventStats, error) {
	return EventStats{}, nil
}

func textMessageMap(role, text string) map[string]any {
	return map[string]any{
		"role": role,
		"parts": []any{
			map[string]any{
				"type": "text",
				"text": text,
			},
		},
	}
}

func TestBuildConversationContext(t *testing.T) {
	ctx := context.Background()

	t.Run("extracts messages from AgentMessageComplete events", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		// Add user message event
		userMsg := textMessageMap("user", "Hello, how are you?")
		ev1 := event.New(event.AgentMessageComplete, sessionID, map[string]any{
			"message": userMsg,
		})
		if err := store.Append(ctx, sessionID, ev1); err != nil {
			t.Fatalf("failed to append: %v", err)
		}

		// Add assistant message event
		assistantMsg := textMessageMap("assistant", "I'm doing well, thank you!")
		ev2 := event.New(event.AgentMessageComplete, sessionID, map[string]any{
			"message": assistantMsg,
		})
		if err := store.Append(ctx, sessionID, ev2); err != nil {
			t.Fatalf("failed to append: %v", err)
		}

		messages, err := BuildConversationContext(ctx, store, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(messages))
		}

		if messages[0].Role != agent.RoleUser {
			t.Errorf("expected user role, got %s", messages[0].Role)
		}
		if messages[0].Text() != "Hello, how are you?" {
			t.Errorf("unexpected content: %s", messages[0].Text())
		}

		if messages[1].Role != agent.RoleAssistant {
			t.Errorf("expected assistant role, got %s", messages[1].Role)
		}
	})

	t.Run("returns empty for non-message events", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		// Add non-message events
		ev := event.New(event.AgentStarted, sessionID, nil)
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("failed to append: %v", err)
		}

		messages, err := BuildConversationContext(ctx, store, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(messages) != 0 {
			t.Errorf("expected 0 messages, got %d", len(messages))
		}
	})

	t.Run("rejects empty session ID", func(t *testing.T) {
		store := NewInMemoryStore()

		_, err := BuildConversationContext(ctx, store, "")
		if err == nil {
			t.Error("expected error for empty session ID")
		}
	})

	t.Run("respects recent count limit", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		// Add 10 messages
		for i := 0; i < 10; i++ {
			msg := textMessageMap("user", "Message "+string(rune('A'+i)))
			ev := event.New(event.AgentMessageComplete, sessionID, map[string]any{
				"message": msg,
			})
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}

		messages, err := BuildConversationContext(ctx, store, sessionID,
			WithRecent(3))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(messages) != 3 {
			t.Errorf("expected 3 messages, got %d", len(messages))
		}

		// Should have the last 3 messages
		if !strings.Contains(messages[0].Text(), "H") {
			t.Errorf("expected recent message, got: %s", messages[0].Text())
		}
	})

	t.Run("respects max messages limit", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		// Add 20 messages
		for i := 0; i < 20; i++ {
			msg := textMessageMap("user", "Message number "+string(rune('0'+i)))
			ev := event.New(event.AgentMessageComplete, sessionID, map[string]any{
				"message": msg,
			})
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}

		messages, err := BuildConversationContext(ctx, store, sessionID,
			WithMaxMessages(5))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(messages) != 5 {
			t.Errorf("expected 5 messages, got %d", len(messages))
		}
	})

	t.Run("truncates to token limit", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		// Add messages with known lengths
		messages := []string{
			strings.Repeat("a", 400), // ~100 tokens
			strings.Repeat("b", 400), // ~100 tokens
			strings.Repeat("c", 400), // ~100 tokens
		}

		for _, content := range messages {
			msg := textMessageMap("user", content)
			ev := event.New(event.AgentMessageComplete, sessionID, map[string]any{
				"message": msg,
			})
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}

		// Request 250 tokens - should get last 2 messages (~200 tokens)
		result, err := BuildConversationContext(ctx, store, sessionID,
			WithTokenLimit(250))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("expected 2 messages after token limit, got %d", len(result))
		}

		// Should keep the most recent messages (b and c)
		if !strings.Contains(result[0].Text(), "b") {
			t.Error("expected message with 'b' content")
		}
	})

	t.Run("filters by message types", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		// Add different event types
		msg1 := textMessageMap("user", "message 1")
		ev1 := event.New(event.AgentMessageComplete, sessionID, map[string]any{"message": msg1})
		store.Append(ctx, sessionID, ev1)

		ev2 := event.New(event.AgentStarted, sessionID, nil)
		store.Append(ctx, sessionID, ev2)

		msg2 := textMessageMap("assistant", "message 2")
		ev3 := event.New(event.AgentMessageComplete, sessionID, map[string]any{"message": msg2})
		store.Append(ctx, sessionID, ev3)

		// Filter only AgentMessageComplete
		messages, err := BuildConversationContext(ctx, store, sessionID,
			WithMessageTypes(event.AgentMessageComplete))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(messages))
		}
	})

	t.Run("handles tool call results", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		// Add tool call completed event
		ev := event.New(event.ToolCallCompleted, sessionID, map[string]any{
			"result": tool.Result{
				ID:     "call_123",
				ToolID: "tool.test",
				Parts:  []agent.ContentPart{{Type: agent.ContentPartText, Text: "Tool executed successfully"}},
			},
		})
		store.Append(ctx, sessionID, ev)

		messages, err := BuildConversationContext(ctx, store, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}

		if messages[0].Role != agent.RoleTool {
			t.Errorf("expected tool role, got %s", messages[0].Role)
		}
		if messages[0].ToolCallID != "call_123" {
			t.Errorf("expected call ID call_123, got %s", messages[0].ToolCallID)
		}
	})

	t.Run("preserves message metadata", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		msg := textMessageMap("assistant", "Test message")
		msg["name"] = "TestAgent"
		msg["metadata"] = map[string]any{
			"confidence": 0.95,
			"source":     "test",
		}
		ev := event.New(event.AgentMessageComplete, sessionID, map[string]any{
			"message": msg,
		})
		store.Append(ctx, sessionID, ev)

		messages, err := BuildConversationContext(ctx, store, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}

		if messages[0].Name != "TestAgent" {
			t.Errorf("expected name TestAgent, got %s", messages[0].Name)
		}

		if messages[0].Metadata == nil {
			t.Fatal("expected metadata to be preserved")
		}
		if confidence, ok := messages[0].Metadata["confidence"].(float64); !ok || confidence != 0.95 {
			t.Errorf("expected confidence 0.95, got %v", messages[0].Metadata["confidence"])
		}
	})

	t.Run("returns messages in chronological order", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "test-session"

		messages := []string{"first", "second", "third"}
		for _, content := range messages {
			msg := textMessageMap("user", content)
			ev := event.New(event.AgentMessageComplete, sessionID, map[string]any{
				"message": msg,
			})
			store.Append(ctx, sessionID, ev)
		}

		result, err := BuildConversationContext(ctx, store, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(result))
		}

		if result[0].Text() != "first" {
			t.Errorf("expected first message first, got %s", result[0].Text())
		}
		if result[1].Text() != "second" {
			t.Errorf("expected second message second, got %s", result[1].Text())
		}
		if result[2].Text() != "third" {
			t.Errorf("expected third message third, got %s", result[2].Text())
		}
	})

	t.Run("orders messages chronologically even when store is unordered", func(t *testing.T) {
		sessionID := "test-session"
		store := &staticStore{}

		older := event.New(event.AgentMessageComplete, sessionID, map[string]any{
			"message": textMessageMap("user", "older"),
		})
		older.Time = time.Now().Add(-2 * time.Hour)
		newer := event.New(event.AgentMessageComplete, sessionID, map[string]any{
			"message": textMessageMap("user", "newer"),
		})
		newer.Time = time.Now()

		store.items = []Item{{Event: newer}, {Event: older}}

		messages, err := BuildConversationContext(ctx, store, sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(messages))
		}
		if messages[0].Text() != "older" {
			t.Fatalf("expected older message first, got %q", messages[0].Text())
		}
		if messages[1].Text() != "newer" {
			t.Fatalf("expected newer message second, got %q", messages[1].Text())
		}
	})
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "short text",
			text:     "Hello",
			expected: 1, // 5 chars / 4 = 1
		},
		{
			name:     "medium text",
			text:     strings.Repeat("a", 400),
			expected: 100, // 400 / 4 = 100
		},
		{
			name:     "long text",
			text:     strings.Repeat("test ", 1000),
			expected: 1250, // 5000 / 4 = 1250
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("expected %d tokens, got %d", tt.expected, result)
			}
		})
	}
}

// TestBuildSemanticContext tests the semantic context builder.
func TestBuildSemanticContext(t *testing.T) {
	mockEmbedder := &mockEmbeddingProvider{
		dimensions: 3,
		model:      "test-model",
		embedFn: func(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
			embeddings := make([][]float64, len(req.Texts))
			for i, text := range req.Texts {
				// Create embeddings based on text content
				var vec []float64
				if len(text) > 10 {
					vec = []float64{1.0, 0.0, 0.0} // Long text
				} else {
					vec = []float64{0.0, 1.0, 0.0} // Short text
				}
				if req.Normalized {
					vec = NormalizeVector(vec)
				}
				embeddings[i] = vec
			}
			return &EmbeddingResponse{
				Embeddings: embeddings,
				Dimensions: 3,
				Model:      "test-model",
			}, nil
		},
	}

	store := NewSemanticStore(mockEmbedder)
	ctx := context.Background()
	sessionID := "test-session"

	// Append some test events
	events := []struct {
		text string
		role string
	}{
		{"Hello world", "user"},
		{"Hi there! How can I help you?", "assistant"},
		{"What is authentication?", "user"},
		{"Authentication is the process of verifying identity", "assistant"},
		{"Tell me about OAuth", "user"},
		{"OAuth is an open standard for access delegation", "assistant"},
		{"Can you explain JWT?", "user"},
		{"JWT stands for JSON Web Token", "assistant"},
	}

	for i, e := range events {
		ev := event.New(event.AgentMessageComplete, sessionID, map[string]any{
			"message": textMessageMap(e.role, e.text),
		})
		ev.Time = time.Now().Add(time.Duration(i) * time.Second)
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	builder := NewSemanticContextBuilder(store)

	t.Run("with semantic query", func(t *testing.T) {
		messages, err := builder.BuildSemanticContext(
			ctx, sessionID,
			WithQuery("authentication and authorization"),
			WithRecentMessages(2),
			WithRelevantMessages(3),
			WithSimilarityThreshold(0.0),
		)
		if err != nil {
			t.Fatalf("BuildSemanticContext failed: %v", err)
		}

		if len(messages) == 0 {
			t.Error("expected messages, got none")
		}

		// Should include both relevant and recent messages
		t.Logf("Got %d messages", len(messages))
	})

	t.Run("without query", func(t *testing.T) {
		messages, err := builder.BuildSemanticContext(
			ctx, sessionID,
			WithRecentMessages(3),
		)
		if err != nil {
			t.Fatalf("BuildSemanticContext failed: %v", err)
		}

		// Should only include recent messages
		if len(messages) > 3 {
			t.Errorf("expected <= 3 messages, got %d", len(messages))
		}
	})

	t.Run("with token limit", func(t *testing.T) {
		messages, err := builder.BuildSemanticContext(
			ctx, sessionID,
			WithQuery("test"),
			WithRecentMessages(10),
			WithRelevantMessages(10),
			WithTokenLimitForSemantic(50), // Very low limit
		)
		if err != nil {
			t.Fatalf("BuildSemanticContext failed: %v", err)
		}

		// Calculate total tokens
		totalTokens := 0
		for _, msg := range messages {
			totalTokens += estimateMessageTokens(msg)
		}

		if totalTokens > 50 {
			t.Errorf("total tokens %d exceeds limit 50", totalTokens)
		}
	})

	t.Run("with deduplication", func(t *testing.T) {
		messages, err := builder.BuildSemanticContext(
			ctx, sessionID,
			WithQuery("authentication"),
			WithRecentMessages(5),
			WithRelevantMessages(5),
			WithDeduplication(true),
		)
		if err != nil {
			t.Fatalf("BuildSemanticContext failed: %v", err)
		}

		// Check for duplicates
		seen := make(map[string]bool)
		for _, msg := range messages {
			key := msg.DedupeKey()
			if seen[key] {
				t.Errorf("duplicate message found: %s", msg.Text())
			}
			seen[key] = true
		}
	})
}

func TestDeduplicateMessages(t *testing.T) {
	messages := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Hello"}}},
		{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Hi"}}},
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Hello"}}}, // Duplicate
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Goodbye"}}},
	}

	result := deduplicateMessages(messages)

	if len(result) != 3 {
		t.Errorf("expected 3 unique messages, got %d", len(result))
	}

	// Check that first occurrence is kept
	if result[0].Text() != "Hello" {
		t.Errorf("expected first message to be 'Hello', got %q", result[0].Text())
	}
}

func TestTruncateToTokenLimit(t *testing.T) {
	t.Run("returns all messages under limit", func(t *testing.T) {
		messages := []agent.Message{
			{Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "short"}}},
			{Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "text"}}},
		}

		result := truncateToTokenLimit(messages, 1000)
		if len(result) != 2 {
			t.Errorf("expected 2 messages, got %d", len(result))
		}
	})

	t.Run("truncates from beginning when over limit", func(t *testing.T) {
		messages := []agent.Message{
			{Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("a", 400)}}}, // ~100 tokens
			{Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("b", 400)}}}, // ~100 tokens
			{Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("c", 400)}}}, // ~100 tokens
		}

		result := truncateToTokenLimit(messages, 250)
		if len(result) != 2 {
			t.Errorf("expected 2 messages, got %d", len(result))
		}

		// Should keep the last two messages
		if len(result) > 0 && !strings.Contains(result[0].Text(), "b") {
			t.Error("expected message with 'b'")
		}
		if len(result) > 1 && !strings.Contains(result[1].Text(), "c") {
			t.Error("expected message with 'c'")
		}
	})

	t.Run("returns empty when no messages fit", func(t *testing.T) {
		messages := []agent.Message{
			{Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("a", 4000)}}}, // ~1000 tokens
		}

		result := truncateToTokenLimit(messages, 100)
		if len(result) != 0 {
			t.Errorf("expected 0 messages, got %d", len(result))
		}
	})

	t.Run("handles zero limit", func(t *testing.T) {
		messages := []agent.Message{
			{Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "test"}}},
		}

		result := truncateToTokenLimit(messages, 0)
		if len(result) != 1 {
			t.Errorf("expected all messages with 0 limit, got %d", len(result))
		}
	})
}

func TestExtractMessage(t *testing.T) {
	t.Run("extracts from AgentMessageComplete", func(t *testing.T) {
		ev := event.New(event.AgentMessageComplete, "session", map[string]any{
			"message": textMessageMap("user", "Hello"),
		})

		msg, ok := extractMessage(ev)
		if !ok {
			t.Fatal("expected successful extraction")
		}

		if msg.Role != agent.RoleUser {
			t.Errorf("expected user role, got %s", msg.Role)
		}
		if msg.Text() != "Hello" {
			t.Errorf("expected Hello content, got %s", msg.Text())
		}
	})

	t.Run("extracts from ToolCallCompleted", func(t *testing.T) {
		ev := event.New(event.ToolCallCompleted, "session", map[string]any{
			"result": tool.Result{
				ID:     "call_456",
				ToolID: "tool.test",
				Parts:  []agent.ContentPart{{Type: agent.ContentPartText, Text: "Success"}},
			},
		})

		msg, ok := extractMessage(ev)
		if !ok {
			t.Fatal("expected successful extraction")
		}

		if msg.Role != agent.RoleTool {
			t.Errorf("expected tool role, got %s", msg.Role)
		}
		if msg.ToolCallID != "call_456" {
			t.Errorf("expected call_456, got %s", msg.ToolCallID)
		}
	})

	t.Run("returns false for non-message events", func(t *testing.T) {
		ev := event.New(event.AgentStarted, "session", nil)

		_, ok := extractMessage(ev)
		if ok {
			t.Error("expected extraction to fail")
		}
	})

	t.Run("returns false for malformed payloads", func(t *testing.T) {
		ev := event.New(event.AgentMessageComplete, "session", map[string]any{
			"invalid": "payload",
		})

		_, ok := extractMessage(ev)
		if ok {
			t.Error("expected extraction to fail")
		}
	})

	t.Run("copies event AgentID to message Name", func(t *testing.T) {
		ev := event.New(event.AgentMessageComplete, "session", map[string]any{
			"message": textMessageMap("assistant", "Hello from agent"),
		})
		ev.AgentID = "Strategist"

		msg, ok := extractMessage(ev)
		if !ok {
			t.Fatal("expected successful extraction")
		}

		if msg.Name != "Strategist" {
			t.Errorf("expected Name to be Strategist (from AgentID), got %q", msg.Name)
		}
	})

	t.Run("preserves message Name over event AgentID", func(t *testing.T) {
		ev := event.New(event.AgentMessageComplete, "session", map[string]any{
			"message": map[string]any{
				"role": "assistant",
				"parts": []any{map[string]any{"type": "text", "text": "Hello"}},
				"name": "OriginalName",
			},
		})
		ev.AgentID = "DifferentAgent"

		msg, ok := extractMessage(ev)
		if !ok {
			t.Fatal("expected successful extraction")
		}

		if msg.Name != "OriginalName" {
			t.Errorf("expected Name to preserve OriginalName, got %q", msg.Name)
		}
	})
}

func TestParseMessageFromMap(t *testing.T) {
	t.Run("parses complete message", func(t *testing.T) {
		data := map[string]any{
			"role":         "assistant",
			"parts":        []any{map[string]any{"type": "text", "text": "Test content"}},
			"name":         "TestBot",
			"tool_call_id": "call_789",
			"metadata": map[string]any{
				"key": "value",
			},
		}

		msg := parseMessageFromMap(data)

		if msg.Role != agent.RoleAssistant {
			t.Errorf("expected assistant role, got %s", msg.Role)
		}
		if msg.Text() != "Test content" {
			t.Errorf("expected Test content, got %s", msg.Text())
		}
		if msg.Name != "TestBot" {
			t.Errorf("expected TestBot, got %s", msg.Name)
		}
		if msg.ToolCallID != "call_789" {
			t.Errorf("expected call_789, got %s", msg.ToolCallID)
		}
		if msg.Metadata["key"] != "value" {
			t.Error("expected metadata to be parsed")
		}
	})

	t.Run("handles minimal message", func(t *testing.T) {
		data := map[string]any{
			"role":  "user",
			"parts": []any{map[string]any{"type": "text", "text": "Hi"}},
		}

		msg := parseMessageFromMap(data)

		if msg.Role != agent.RoleUser {
			t.Errorf("expected user role, got %s", msg.Role)
		}
		if msg.Text() != "Hi" {
			t.Errorf("expected Hi, got %s", msg.Text())
		}
	})

	t.Run("parses content parts", func(t *testing.T) {
		data := map[string]any{
			"role": "user",
			"parts": []any{
				map[string]any{
					"type": "text",
					"text": "Look at this.",
				},
				map[string]any{
					"type": "image",
					"uri":  "https://example.com/image.png",
				},
			},
		}

		msg := parseMessageFromMap(data)

		if len(msg.Parts) != 2 {
			t.Fatalf("expected 2 parts, got %d", len(msg.Parts))
		}
		if msg.Text() != "Look at this." {
			t.Errorf("expected content from parts, got %s", msg.Text())
		}
		if msg.ImageCount() != 1 {
			t.Errorf("expected 1 image part, got %d", msg.ImageCount())
		}
	})

	t.Run("handles empty map", func(t *testing.T) {
		data := map[string]any{}

		msg := parseMessageFromMap(data)

		if msg.Role != "" {
			t.Errorf("expected empty role, got %s", msg.Role)
		}
		if msg.Text() != "" {
			t.Errorf("expected empty content, got %s", msg.Text())
		}
	})
}
