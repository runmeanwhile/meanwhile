package memory

import (
	"context"
	"testing"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/event"
)

func TestSemanticStore_AppendAndQuery(t *testing.T) {
	mockEmbedder := &mockEmbeddingProvider{
		dimensions: 3,
		model:      "test-model",
		embedFn: func(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
			embeddings := make([][]float64, len(req.Texts))
			for i, text := range req.Texts {
				// Create simple embeddings based on text length
				embeddings[i] = []float64{
					float64(len(text)),
					float64(len(text)) * 2,
					float64(len(text)) * 3,
				}
				if req.Normalized {
					embeddings[i] = NormalizeVector(embeddings[i])
				}
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

	// Append some events
	events := []event.Event{
		{
			Type:      event.AgentStarted,
			SessionID: sessionID,
			Time:      time.Now(),
			Payload: map[string]any{
				"text": "Hello world",
			},
		},
		{
			Type:      event.AgentMessageComplete,
			SessionID: sessionID,
			Time:      time.Now(),
			Payload: map[string]any{
				"text": "How are you doing today?",
			},
		},
		{
			Type:      event.ToolCallCompleted,
			SessionID: sessionID,
			Time:      time.Now(),
			Payload: map[string]any{
				"text": "Weather is sunny",
			},
		},
	}

	for _, ev := range events {
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Test traditional query
	t.Run("traditional query", func(t *testing.T) {
		query := Query{
			SessionID: sessionID,
			Types:     []event.Type{event.AgentMessageComplete},
		}

		items, err := store.Query(ctx, query)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(items) != 1 {
			t.Errorf("got %d items, want 1", len(items))
		}

		if len(items) > 0 && items[0].Event.Type != event.AgentMessageComplete {
			t.Errorf("event type = %s, want %s", items[0].Event.Type, event.AgentMessageComplete)
		}
	})

	// Test semantic query
	t.Run("semantic query", func(t *testing.T) {
		query := SemanticQuery{
			SessionID: sessionID,
			Text:      "greeting message",
			Limit:     2,
			Threshold: 0.0,
		}

		results, err := store.QuerySemantic(ctx, query)
		if err != nil {
			t.Fatalf("QuerySemantic failed: %v", err)
		}

		if len(results) > 2 {
			t.Errorf("got %d results, want <= 2", len(results))
		}

		// Results should be sorted by score
		for i := 1; i < len(results); i++ {
			if results[i].Score > results[i-1].Score {
				t.Errorf("results not sorted: score[%d] = %f > score[%d] = %f",
					i, results[i].Score, i-1, results[i-1].Score)
			}
		}

		// Scores should be between 0 and 1
		for i, r := range results {
			if r.Score < 0 || r.Score > 1 {
				t.Errorf("results[%d].Score = %f, want 0-1", i, r.Score)
			}
		}
	})

	// Test semantic query with threshold
	t.Run("semantic query with threshold", func(t *testing.T) {
		query := SemanticQuery{
			SessionID: sessionID,
			Text:      "test query",
			Threshold: 0.9, // High threshold
		}

		results, err := store.QuerySemantic(ctx, query)
		if err != nil {
			t.Fatalf("QuerySemantic failed: %v", err)
		}

		// All results should have score >= threshold
		for i, r := range results {
			if r.Score < query.Threshold {
				t.Errorf("results[%d].Score = %f, want >= %f",
					i, r.Score, query.Threshold)
			}
		}
	})

	// Test semantic query with event type filter
	t.Run("semantic query with event type filter", func(t *testing.T) {
		query := SemanticQuery{
			SessionID:  sessionID,
			Text:       "any text",
			EventTypes: []event.Type{event.ToolCallCompleted},
			Threshold:  0.0,
		}

		results, err := store.QuerySemantic(ctx, query)
		if err != nil {
			t.Fatalf("QuerySemantic failed: %v", err)
		}

		// All results should match event type
		for i, r := range results {
			if r.Item.Event.Type != event.ToolCallCompleted {
				t.Errorf("results[%d].Event.Type = %s, want %s",
					i, r.Item.Event.Type, event.ToolCallCompleted)
			}
		}
	})

	// Test stats
	t.Run("stats", func(t *testing.T) {
		stats, err := store.Stats(ctx, sessionID, Policy{})
		if err != nil {
			t.Fatalf("Stats failed: %v", err)
		}

		if stats.TotalEvents != 3 {
			t.Errorf("TotalEvents = %d, want 3", stats.TotalEvents)
		}

		if stats.EventCounts[event.AgentStarted] != 1 {
			t.Errorf("EventCounts[AgentStarted] = %d, want 1",
				stats.EventCounts[event.AgentStarted])
		}

		if stats.FirstEvent.IsZero() || stats.LastEvent.IsZero() {
			t.Error("FirstEvent or LastEvent is zero")
		}
	})
}

func TestSemanticStore_EmptySession(t *testing.T) {
	mockEmbedder := &mockEmbeddingProvider{
		dimensions: 3,
		model:      "test-model",
	}

	store := NewSemanticStore(mockEmbedder)
	ctx := context.Background()

	t.Run("query empty session", func(t *testing.T) {
		query := Query{
			SessionID: "nonexistent",
		}

		items, err := store.Query(ctx, query)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(items) != 0 {
			t.Errorf("got %d items, want 0", len(items))
		}
	})

	t.Run("semantic query empty session", func(t *testing.T) {
		query := SemanticQuery{
			SessionID: "nonexistent",
			Text:      "test",
		}

		results, err := store.QuerySemantic(ctx, query)
		if err != nil {
			t.Fatalf("QuerySemantic failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})

	t.Run("stats empty session", func(t *testing.T) {
		stats, err := store.Stats(ctx, "nonexistent", Policy{})
		if err != nil {
			t.Fatalf("Stats failed: %v", err)
		}

		if stats.TotalEvents != 0 {
			t.Errorf("TotalEvents = %d, want 0", stats.TotalEvents)
		}
	})
}

func TestSemanticStore_ConcurrentAccess(t *testing.T) {
	mockEmbedder := &mockEmbeddingProvider{
		dimensions: 3,
		model:      "test-model",
	}

	store := NewSemanticStore(mockEmbedder)
	ctx := context.Background()
	sessionID := "test-session"

	// Concurrent appends
	const numGoroutines = 10
	const eventsPerGoroutine = 10

	done := make(chan bool)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < eventsPerGoroutine; j++ {
				ev := event.Event{
					Type:      event.AgentStarted,
					SessionID: sessionID,
					Time:      time.Now(),
					Payload: map[string]any{
						"text": "test message",
					},
				}
				if err := store.Append(ctx, sessionID, ev); err != nil {
					t.Errorf("Append failed: %v", err)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify total count
	stats, err := store.Stats(ctx, sessionID, Policy{})
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	expected := numGoroutines * eventsPerGoroutine
	if stats.TotalEvents != expected {
		t.Errorf("TotalEvents = %d, want %d", stats.TotalEvents, expected)
	}
}

func TestExtractEventText(t *testing.T) {
	tests := []struct {
		name     string
		event    event.Event
		expected string
	}{
		{
			name: "text field",
			event: event.Event{
				Type: event.AgentMessageComplete,
				Payload: map[string]any{
					"text": "Hello world",
				},
			},
			expected: "Hello world",
		},
		{
			name: "parts field",
			event: event.Event{
				Type: event.AgentMessageComplete,
				Payload: map[string]any{
					"parts": []map[string]any{
						{"type": "text", "text": "Test content"},
					},
				},
			},
			expected: "Test content",
		},
		{
			name: "message field",
			event: event.Event{
				Type: event.AgentMessageComplete,
				Payload: map[string]any{
					"message": "Test message",
				},
			},
			expected: "Test message",
		},
		{
			name: "nested message.parts (agent events)",
			event: event.Event{
				Type: event.AgentMessageComplete,
				Payload: map[string]any{
					"message": map[string]any{
						"role": "user",
						"parts": []map[string]any{
							{"type": "text", "text": "What is OAuth 2.0?"},
						},
					},
				},
			},
			expected: "What is OAuth 2.0?",
		},
		{
			name: "no text fields",
			event: event.Event{
				Type: event.AgentStarted,
				ID:   "test-id",
				Payload: map[string]any{
					"other": "data",
				},
			},
			expected: "agent.started test-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEventText(tt.event)
			if result != tt.expected {
				t.Errorf("extractEventText() = %q, want %q", result, tt.expected)
			}
		})
	}
}
