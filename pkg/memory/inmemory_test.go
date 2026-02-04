package memory

import (
	"context"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

func TestInMemoryStoreAppendQuery(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	ev1 := event.New(event.AgentStarted, "s1", nil)
	ev2 := event.New(event.AgentMessageComplete, "s1", nil)

	if err := store.Append(ctx, "s1", ev1); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := store.Append(ctx, "s1", ev2); err != nil {
		t.Fatalf("append: %v", err)
	}

	items, err := store.Query(ctx, Query{SessionID: "s1", Limit: 2})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestInMemoryStoreSummarize(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	if err := store.Append(ctx, "s1", event.New(event.AgentStarted, "s1", nil)); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := store.Append(ctx, "s1", event.New(event.AgentMessageComplete, "s1", nil)); err != nil {
		t.Fatalf("append: %v", err)
	}

	summary, err := store.Summarize(ctx, "s1", Policy{MaxItems: 2})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.EventCount != 2 {
		t.Fatalf("expected 2 events, got %d", summary.EventCount)
	}
	if summary.Text == "" {
		t.Fatal("expected summary text")
	}
}

func TestInMemoryStore_Stats(t *testing.T) {
	ctx := context.Background()

	t.Run("calculates event statistics", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "stats-session"

		// Add various event types
		events := []event.Type{
			event.AgentStarted,
			event.AgentMessageComplete,
			event.AgentStarted,
			event.ToolCallStarted,
			event.AgentMessageComplete,
		}

		for _, evType := range events {
			ev := event.New(evType, sessionID, nil)
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}

		stats, err := store.Stats(ctx, sessionID, Policy{})
		if err != nil {
			t.Fatalf("failed to get stats: %v", err)
		}

		if stats.TotalEvents != 5 {
			t.Errorf("expected 5 total events, got %d", stats.TotalEvents)
		}

		if stats.EventCounts[event.AgentStarted] != 2 {
			t.Errorf("expected 2 AgentStarted events, got %d", stats.EventCounts[event.AgentStarted])
		}

		if stats.EventCounts[event.AgentMessageComplete] != 2 {
			t.Errorf("expected 2 AgentMessageComplete events, got %d", stats.EventCounts[event.AgentMessageComplete])
		}

		if stats.SessionID != sessionID {
			t.Errorf("expected session ID %s, got %s", sessionID, stats.SessionID)
		}

		if stats.FirstEvent.IsZero() {
			t.Error("expected FirstEvent to be set")
		}

		if stats.LastEvent.IsZero() {
			t.Error("expected LastEvent to be set")
		}

		if !stats.FirstEvent.Before(stats.LastEvent) && !stats.FirstEvent.Equal(stats.LastEvent) {
			t.Error("expected FirstEvent to be before or equal to LastEvent")
		}
	})

	t.Run("returns empty stats for non-existent session", func(t *testing.T) {
		store := NewInMemoryStore()

		stats, err := store.Stats(ctx, "non-existent", Policy{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if stats.TotalEvents != 0 {
			t.Errorf("expected 0 events, got %d", stats.TotalEvents)
		}

		if len(stats.EventCounts) != 0 {
			t.Errorf("expected empty EventCounts, got %d entries", len(stats.EventCounts))
		}
	})

	t.Run("respects policy type filter", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "filtered-stats"

		events := []event.Type{
			event.AgentStarted,
			event.AgentMessageComplete,
			event.ToolCallStarted,
		}

		for _, evType := range events {
			ev := event.New(evType, sessionID, nil)
			store.Append(ctx, sessionID, ev)
		}

		stats, err := store.Stats(ctx, sessionID, Policy{
			Types: []event.Type{event.AgentStarted},
		})
		if err != nil {
			t.Fatalf("failed to get stats: %v", err)
		}

		if stats.TotalEvents != 1 {
			t.Errorf("expected 1 event, got %d", stats.TotalEvents)
		}

		if stats.EventCounts[event.AgentStarted] != 1 {
			t.Errorf("expected 1 AgentStarted, got %d", stats.EventCounts[event.AgentStarted])
		}
	})

	t.Run("respects policy max items", func(t *testing.T) {
		store := NewInMemoryStore()
		sessionID := "limited-stats"

		for i := 0; i < 10; i++ {
			ev := event.New(event.AgentStarted, sessionID, nil)
			store.Append(ctx, sessionID, ev)
		}

		stats, err := store.Stats(ctx, sessionID, Policy{
			MaxItems: 5,
		})
		if err != nil {
			t.Fatalf("failed to get stats: %v", err)
		}

		if stats.TotalEvents != 5 {
			t.Errorf("expected 5 events (max items), got %d", stats.TotalEvents)
		}
	})
}
