package memory

import (
	"context"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

// Store defines memory storage behavior.
type Store interface {
	Append(ctx context.Context, sessionID string, ev event.Event) error
	Query(ctx context.Context, query Query) ([]Item, error)
	Summarize(ctx context.Context, sessionID string, policy Policy) (Summary, error)
	Stats(ctx context.Context, sessionID string, policy Policy) (EventStats, error)
}

// Item is a stored event.
type Item struct {
	Event event.Event
}

// Query describes a memory query.
type Query struct {
	SessionID string
	Types     []event.Type
	Limit     int
}

// Policy configures summarization.
type Policy struct {
	MaxItems int
	Types    []event.Type
}

// Summary is a compact representation of memory.
//
// Deprecated: Use Stats instead for structured event statistics.
type Summary struct {
	Text       string
	EventCount int
}

// EventStats provides structured statistics about session events.
type EventStats struct {
	TotalEvents int
	EventCounts map[event.Type]int
	FirstEvent  time.Time
	LastEvent   time.Time
	SessionID   string
}
