package memory

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

// SemanticStore provides semantic memory storage with embedding-based retrieval.
//
// It combines traditional event storage with vector embeddings, enabling:
// - Semantic search: Find events by meaning, not just keywords
// - Relevance ranking: Return most relevant events first
// - Context building: Automatically select most pertinent conversation history
//
// Example usage:
//
//	embedder := memory.NewOpenAIEmbeddings(apiKey)
//	store := memory.NewSemanticStore(embedder)
//
//	// Events are automatically embedded on append
//	store.Append(ctx, sessionID, event)
//
//	// Query by semantic similarity
//	items, err := store.QuerySemantic(ctx, memory.SemanticQuery{
//	    SessionID: sessionID,
//	    Text:      "discussions about performance",
//	    Limit:     5,
//	    Threshold: 0.7,
//	})
type SemanticStore struct {
	embedder   EmbeddingProvider
	mu         sync.RWMutex
	events     map[string][]Item      // sessionID -> events
	embeddings map[string][][]float64 // sessionID -> embeddings
}

// NewSemanticStore creates a new semantic memory store.
//
// The embedder is used to generate vector embeddings for each event.
// Events are stored in memory along with their embeddings for fast retrieval.
func NewSemanticStore(embedder EmbeddingProvider) *SemanticStore {
	return &SemanticStore{
		embedder:   embedder,
		events:     make(map[string][]Item),
		embeddings: make(map[string][][]float64),
	}
}

// Append stores an event and generates its embedding.
func (s *SemanticStore) Append(ctx context.Context, sessionID string, ev event.Event) error {
	// Extract text content from event for embedding
	text := extractEventText(ev)
	if text == "" {
		text = fmt.Sprintf("%s event", ev.Type)
	}

	// Generate embedding
	resp, err := s.embedder.Embed(ctx, EmbeddingRequest{
		Texts:      []string{text},
		Normalized: true, // Normalize for cosine similarity
	})
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return fmt.Errorf("no embedding returned")
	}

	// Store event and embedding
	s.mu.Lock()
	defer s.mu.Unlock()

	item := Item{
		Event: ev,
	}

	s.events[sessionID] = append(s.events[sessionID], item)
	s.embeddings[sessionID] = append(s.embeddings[sessionID], resp.Embeddings[0])

	return nil
}

// Query retrieves events using traditional filters (non-semantic).
//
// For semantic search, use QuerySemantic instead.
func (s *SemanticStore) Query(ctx context.Context, query Query) ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items, ok := s.events[query.SessionID]
	if !ok {
		return []Item{}, nil
	}

	// Apply filters
	var results []Item
	for _, item := range items {
		if matchesQuery(item, query) {
			results = append(results, item)
		}
	}

	return results, nil
}

// SemanticQuery defines parameters for semantic search.
type SemanticQuery struct {
	// SessionID is the session to search within.
	SessionID string

	// Text is the query text to search for.
	// Events with similar semantic meaning will be returned.
	Text string

	// Limit is the maximum number of results to return.
	// If 0, all matching results are returned.
	Limit int

	// Threshold is the minimum similarity score (0-1).
	// Only events with similarity >= threshold are returned.
	// Typical values: 0.7 (relevant), 0.8 (very relevant), 0.9 (highly relevant)
	Threshold float64

	// EventTypes filters results to specific event types.
	// If empty, all event types are included.
	EventTypes []event.Type
}

// SemanticResult contains an event and its relevance score.
type SemanticResult struct {
	Item  Item
	Score float64 // Cosine similarity (0-1)
}

// QuerySemantic performs semantic search using embeddings.
//
// Returns events ranked by semantic similarity to the query text.
func (s *SemanticStore) QuerySemantic(ctx context.Context, query SemanticQuery) ([]SemanticResult, error) {
	// Generate query embedding
	resp, err := s.embedder.Embed(ctx, EmbeddingRequest{
		Texts:      []string{query.Text},
		Normalized: true,
	})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no query embedding returned")
	}

	queryEmb := resp.Embeddings[0]

	s.mu.RLock()
	defer s.mu.RUnlock()

	items, ok := s.events[query.SessionID]
	if !ok {
		return []SemanticResult{}, nil
	}

	embeddings, ok := s.embeddings[query.SessionID]
	if !ok || len(embeddings) != len(items) {
		return nil, fmt.Errorf("embedding mismatch for session %s", query.SessionID)
	}

	// Compute similarities
	var results []SemanticResult
	for i, item := range items {
		// Apply event type filter
		if len(query.EventTypes) > 0 {
			match := slices.Contains(query.EventTypes, item.Event.Type)
			if !match {
				continue
			}
		}

		// Compute cosine similarity
		score := CosineSimilarity(queryEmb, embeddings[i])

		// Apply threshold
		if score >= query.Threshold {
			results = append(results, SemanticResult{
				Item:  item,
				Score: score,
			})
		}
	}

	// Sort by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply limit
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

// Stats returns statistics about stored events.
func (s *SemanticStore) Stats(ctx context.Context, sessionID string, policy Policy) (EventStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items, ok := s.events[sessionID]
	if !ok {
		return EventStats{
			TotalEvents: 0,
			EventCounts: make(map[event.Type]int),
			SessionID:   sessionID,
		}, nil
	}

	stats := EventStats{
		TotalEvents: len(items),
		EventCounts: make(map[event.Type]int),
		SessionID:   sessionID,
	}

	for i, item := range items {
		stats.EventCounts[item.Event.Type]++

		if i == 0 {
			stats.FirstEvent = item.Event.Time
		}
		if i == len(items)-1 {
			stats.LastEvent = item.Event.Time
		}
	}

	return stats, nil
}

// Summarize returns a deprecated summary.
//
// Deprecated: Use Stats instead for structured event statistics.
func (s *SemanticStore) Summarize(ctx context.Context, sessionID string, policy Policy) (Summary, error) {
	stats, err := s.Stats(ctx, sessionID, policy)
	if err != nil {
		return Summary{}, err
	}

	return Summary{
		Text:       fmt.Sprintf("Session %s has %d events", sessionID, stats.TotalEvents),
		EventCount: stats.TotalEvents,
	}, nil
}

// extractEventText extracts meaningful text from an event for embedding.
func extractEventText(ev event.Event) string {
	if msg, ok := ev.Payload.(agent.Message); ok {
		if text := msg.Text(); text != "" {
			return text
		}
	}

	// Try to extract message content
	if payload, ok := ev.Payload.(map[string]any); ok {
		// Check for common message fields
		if text, ok := payload["text"].(string); ok && text != "" {
			return text
		}
		if message, ok := payload["message"].(string); ok && message != "" {
			return message
		}
		if parts, ok := payload["parts"]; ok {
			msg := agent.MessageFromMap(map[string]any{"parts": parts})
			if text := msg.Text(); text != "" {
				return text
			}
		}

		// Check for nested message structure (common in agent events)
		if msg, ok := payload["message"].(agent.Message); ok {
			if text := msg.Text(); text != "" {
				return text
			}
		}
		if messageMap, ok := payload["message"].(map[string]any); ok {
			msg := agent.MessageFromMap(messageMap)
			if text := msg.Text(); text != "" {
				return text
			}
		}
	}

	// Fallback to event type and ID
	return fmt.Sprintf("%s %s", ev.Type, ev.ID)
}

// matchesQuery checks if an item matches a query (non-semantic filters).
func matchesQuery(item Item, query Query) bool {
	// Event type filter
	if len(query.Types) > 0 {
		match := false
		for _, et := range query.Types {
			if item.Event.Type == et {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	return true
}
