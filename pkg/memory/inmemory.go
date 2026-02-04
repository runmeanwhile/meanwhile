package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/event"
)

// InMemoryStore stores events in memory.
type InMemoryStore struct {
	mu    sync.RWMutex
	items map[string][]event.Event
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{items: make(map[string][]event.Event)}
}

// Append stores an event.
func (s *InMemoryStore) Append(_ context.Context, sessionID string, ev event.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[sessionID] = append(s.items[sessionID], ev)
	return nil
}

// Query returns stored events matching the query.
func (s *InMemoryStore) Query(_ context.Context, query Query) ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.items[query.SessionID]
	if len(events) == 0 {
		return nil, nil
	}

	filtered := make([]Item, 0, len(events))
	typeFilter := make(map[event.Type]struct{}, len(query.Types))
	for _, t := range query.Types {
		typeFilter[t] = struct{}{}
	}

	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if len(typeFilter) > 0 {
			if _, ok := typeFilter[ev.Type]; !ok {
				continue
			}
		}
		filtered = append(filtered, Item{Event: ev})
		if query.Limit > 0 && len(filtered) >= query.Limit {
			break
		}
	}

	// reverse to chronological order
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered, nil
}

// Summarize builds a naive summary by concatenating event types.
//
// Deprecated: Use Stats instead for structured event statistics.
func (s *InMemoryStore) Summarize(ctx context.Context, sessionID string, policy Policy) (Summary, error) {
	items, err := s.Query(ctx, Query{SessionID: sessionID, Types: policy.Types, Limit: policy.MaxItems})
	if err != nil {
		return Summary{}, err
	}

	var sb strings.Builder
	for i, item := range items {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(string(item.Event.Type))
	}

	return Summary{Text: sb.String(), EventCount: len(items)}, nil
}

// Stats calculates structured statistics about stored events.
func (s *InMemoryStore) Stats(ctx context.Context, sessionID string, policy Policy) (EventStats, error) {
	items, err := s.Query(ctx, Query{SessionID: sessionID, Types: policy.Types, Limit: policy.MaxItems})
	if err != nil {
		return EventStats{}, err
	}

	if len(items) == 0 {
		return EventStats{
			SessionID:   sessionID,
			TotalEvents: 0,
			EventCounts: make(map[event.Type]int),
		}, nil
	}

	stats := EventStats{
		SessionID:   sessionID,
		TotalEvents: len(items),
		EventCounts: make(map[event.Type]int),
		FirstEvent:  items[0].Event.Time,
		LastEvent:   items[len(items)-1].Event.Time,
	}

	for _, item := range items {
		stats.EventCounts[item.Event.Type]++
	}

	return stats, nil
}
