package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// ContextOption configures context building behavior.
type ContextOption func(*contextConfig)

// contextConfig holds configuration for BuildConversationContext.
type contextConfig struct {
	tokenLimit   int
	messageTypes []event.Type
	recentCount  int
	maxMessages  int
}

// WithTokenLimit sets the maximum token count for conversation context.
// When exceeded, older messages are truncated. Use 0 for unlimited.
// Note: Token counting is approximate (4 chars ≈ 1 token).
func WithTokenLimit(limit int) ContextOption {
	return func(cfg *contextConfig) {
		cfg.tokenLimit = limit
	}
}

// WithMessageTypes filters events to specific types before extraction.
// By default, all event types are included.
func WithMessageTypes(types ...event.Type) ContextOption {
	return func(cfg *contextConfig) {
		cfg.messageTypes = types
	}
}

// WithRecent limits context to the N most recent messages.
// Use 0 for no limit.
func WithRecent(count int) ContextOption {
	return func(cfg *contextConfig) {
		cfg.recentCount = count
	}
}

// WithMaxMessages sets an absolute maximum on returned messages.
// Useful to prevent unbounded context growth.
func WithMaxMessages(maxMessages int) ContextOption {
	return func(cfg *contextConfig) {
		cfg.maxMessages = maxMessages
	}
}

// BuildConversationContext extracts agent messages from memory to build
// conversation context suitable for prompt inclusion.
//
// It queries the memory store for events containing messages, extracts
// user and assistant messages, and returns them in chronological order.
//
// The function respects token limits, message counts, and type filters
// to ensure the context fits within model constraints.
//
// Example:
//
//	history, err := memory.BuildConversationContext(
//	    ctx, store, sessionID,
//	    memory.WithTokenLimit(4000),
//	    memory.WithRecent(10),
//	)
//	// Prepend history to new prompt
func BuildConversationContext(
	ctx context.Context,
	store Store,
	sessionID string,
	opts ...ContextOption,
) ([]agent.Message, error) {
	if sessionID == "" {
		return nil, errors.New("sessionID cannot be empty")
	}

	cfg := &contextConfig{
		tokenLimit:  0, // Unlimited by default
		recentCount: 0, // All messages
		maxMessages: 0, // No limit
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Build query
	query := Query{
		SessionID: sessionID,
		Types:     cfg.messageTypes,
	}

	// If recent count is set, use it as limit
	if cfg.recentCount > 0 {
		query.Limit = cfg.recentCount * 2 // Approximate: account for user + assistant pairs
	}

	items, err := store.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query memory: %w", err)
	}

	// Ensure chronological ordering regardless of store ordering.
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Event.Time.Before(items[j].Event.Time)
	})

	// Extract messages from events
	var messages []agent.Message
	for _, item := range items {
		msg, ok := extractMessage(item.Event)
		if ok {
			messages = append(messages, msg)
		}
	}

	// Apply recent count filter (on actual messages, not events)
	if cfg.recentCount > 0 && len(messages) > cfg.recentCount {
		messages = messages[len(messages)-cfg.recentCount:]
	}

	// Apply max messages limit
	if cfg.maxMessages > 0 && len(messages) > cfg.maxMessages {
		messages = messages[len(messages)-cfg.maxMessages:]
	}

	// Apply token limit if specified
	if cfg.tokenLimit > 0 {
		messages = truncateToTokenLimit(messages, cfg.tokenLimit)
	}

	return messages, nil
}

// extractMessage attempts to extract an agent.Message from an event.
// Returns the message and true if extraction succeeded.
func extractMessage(ev event.Event) (agent.Message, bool) {
	// Handle AgentMessageComplete events
	if ev.Type == event.AgentMessageComplete {
		if msg, ok := ev.Payload.(agent.Message); ok {
			return msg, true
		}
		if payload, ok := ev.Payload.(map[string]any); ok {
			if msgData, ok := payload["message"].(map[string]any); ok {
				return agent.MessageFromMap(msgData), true
			}
			if msg, ok := payload["message"].(agent.Message); ok {
				return msg, true
			}
		}
	}

	// Handle tool results
	if ev.Type == event.ToolCallCompleted || ev.Type == event.ToolCallError {
		if payload, ok := ev.Payload.(map[string]any); ok {
			if result, ok := payload["result"].(tool.Result); ok {
				parts := append([]agent.ContentPart(nil), result.Parts...)
				if result.Output != nil && !hasPartType(parts, agent.ContentPartJSON) {
					parts = append(parts, agent.ContentPart{Type: agent.ContentPartJSON, JSON: result.Output})
				}
				if result.Error != nil && !hasPartType(parts, agent.ContentPartText) && result.Error.Message != "" {
					parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: result.Error.Message})
				}
				return agent.Message{
					Role:       agent.RoleTool,
					Name:       result.ToolID,
					Parts:      parts,
					ToolCallID: result.ID,
				}, true
			}
		}
	}

	return agent.Message{}, false
}

// parseMessageFromMap converts a map to agent.Message.
// Handles JSON-unmarshaled message data.
func parseMessageFromMap(data map[string]any) agent.Message {
	return agent.MessageFromMap(data)
}

// truncateToTokenLimit removes older messages until token count is under limit.
// Uses approximate token counting: 4 characters ≈ 1 token.
func truncateToTokenLimit(messages []agent.Message, limit int) []agent.Message {
	if limit <= 0 {
		return messages
	}

	// Calculate approximate tokens
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += estimateMessageTokens(msg)
	}

	// If under limit, return all messages
	if totalTokens <= limit {
		return messages
	}

	// Truncate from the beginning (keep recent messages)
	result := make([]agent.Message, 0, len(messages))
	currentTokens := 0

	// Work backwards from most recent
	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := estimateMessageTokens(messages[i])
		if currentTokens+msgTokens > limit {
			// This message would exceed limit, stop here
			break
		}
		currentTokens += msgTokens
		// Prepend to result to maintain chronological order
		result = append([]agent.Message{messages[i]}, result...)
	}

	return result
}

// estimateTokens provides a rough estimate of token count.
// Uses the common heuristic: 1 token ≈ 4 characters.
func estimateTokens(text string) int {
	return len(text) / 4
}

const imageTokenEstimate = 256

func estimateMessageTokens(msg agent.Message) int {
	textTokens := estimateTokens(msg.Text())
	imageTokens := msg.ImageCount() * imageTokenEstimate
	return textTokens + imageTokens
}

func hasPartType(parts []agent.ContentPart, partType agent.ContentPartType) bool {
	for _, part := range parts {
		if part.Type == partType {
			return true
		}
	}
	return false
}

// SemanticContextBuilder builds conversation context using semantic search.
type SemanticContextBuilder struct {
	store         Store
	semanticStore *SemanticStore
}

// NewSemanticContextBuilder creates a builder for semantic context.
func NewSemanticContextBuilder(store Store) *SemanticContextBuilder {
	// If store is already a SemanticStore, use it directly
	if ss, ok := store.(*SemanticStore); ok {
		return &SemanticContextBuilder{
			store:         store,
			semanticStore: ss,
		}
	}

	// Otherwise, wrap it (requires semantic store)
	return &SemanticContextBuilder{
		store: store,
	}
}

// SemanticContextOption configures semantic context building.
type SemanticContextOption func(*semanticContextConfig)

type semanticContextConfig struct {
	query         string
	recentCount   int
	relevantCount int
	tokenLimit    int
	threshold     float64
	deduplicate   bool
}

// WithQuery sets the semantic query text.
func WithQuery(query string) SemanticContextOption {
	return func(cfg *semanticContextConfig) {
		cfg.query = query
	}
}

// WithRecentMessages sets the number of recent messages to include.
func WithRecentMessages(count int) SemanticContextOption {
	return func(cfg *semanticContextConfig) {
		cfg.recentCount = count
	}
}

// WithRelevantMessages sets the number of semantically relevant messages.
func WithRelevantMessages(count int) SemanticContextOption {
	return func(cfg *semanticContextConfig) {
		cfg.relevantCount = count
	}
}

// WithTokenLimitForSemantic sets the maximum token count.
func WithTokenLimitForSemantic(limit int) SemanticContextOption {
	return func(cfg *semanticContextConfig) {
		cfg.tokenLimit = limit
	}
}

// WithSimilarityThreshold sets the minimum similarity score (0-1).
func WithSimilarityThreshold(threshold float64) SemanticContextOption {
	return func(cfg *semanticContextConfig) {
		cfg.threshold = threshold
	}
}

// WithDeduplication enables/disables automatic deduplication of messages.
func WithDeduplication(enabled bool) SemanticContextOption {
	return func(cfg *semanticContextConfig) {
		cfg.deduplicate = enabled
	}
}

// BuildSemanticContext builds conversation context combining recent and relevant messages.
//
// This function intelligently combines:
// 1. Most recent N messages (for immediate context)
// 2. Most relevant M messages (for historical context)
// 3. Automatic deduplication (to avoid repeating messages)
// 4. Token-aware truncation (to fit within model limits)
//
// Example:
//
//	builder := memory.NewSemanticContextBuilder(semanticStore)
//	history, err := builder.BuildSemanticContext(
//	    ctx, sessionID,
//	    memory.WithQuery("What did we discuss about authentication?"),
//	    memory.WithRecentMessages(5),
//	    memory.WithRelevantMessages(10),
//	    memory.WithTokenLimitForSemantic(4000),
//	    memory.WithSimilarityThreshold(0.7),
//	)
func (b *SemanticContextBuilder) BuildSemanticContext(
	ctx context.Context,
	sessionID string,
	opts ...SemanticContextOption,
) ([]agent.Message, error) {
	if b.semanticStore == nil {
		return nil, errors.New("semantic store not configured")
	}

	cfg := &semanticContextConfig{
		recentCount:   5,
		relevantCount: 10,
		tokenLimit:    0,
		threshold:     0.7,
		deduplicate:   true,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Get recent messages (chronological)
	recentMessages, err := BuildConversationContext(
		ctx, b.store, sessionID,
		WithRecent(cfg.recentCount),
	)
	if err != nil {
		return nil, fmt.Errorf("get recent messages: %w", err)
	}

	var combinedMessages []agent.Message

	// Add relevant messages if query is provided
	if cfg.query != "" {
		results, err := b.semanticStore.QuerySemantic(ctx, SemanticQuery{
			SessionID: sessionID,
			Text:      cfg.query,
			Limit:     cfg.relevantCount,
			Threshold: cfg.threshold,
		})
		if err != nil {
			return nil, fmt.Errorf("semantic query: %w", err)
		}

		// Extract messages from semantic results
		for _, result := range results {
			msg, ok := extractMessage(result.Item.Event)
			if ok {
				combinedMessages = append(combinedMessages, msg)
			}
		}
	}

	// Append recent messages
	combinedMessages = append(combinedMessages, recentMessages...)

	// Deduplicate if enabled
	if cfg.deduplicate {
		combinedMessages = deduplicateMessages(combinedMessages)
	}

	// Apply token limit
	if cfg.tokenLimit > 0 {
		combinedMessages = truncateToTokenLimit(combinedMessages, cfg.tokenLimit)
	}

	return combinedMessages, nil
}

// deduplicateMessages removes duplicate messages based on content.
// Keeps the first occurrence of each unique message.
func deduplicateMessages(messages []agent.Message) []agent.Message {
	seen := make(map[string]bool)
	result := make([]agent.Message, 0, len(messages))

	for _, msg := range messages {
		key := msg.DedupeKey()
		if !seen[key] {
			seen[key] = true
			result = append(result, msg)
		}
	}

	return result
}
