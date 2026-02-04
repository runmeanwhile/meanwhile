package contextpolicy

import (
	"context"
	"errors"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

// Policy selects which messages are sent to the provider.
type Policy interface {
	Select(ctx context.Context, input Input) ([]agent.Message, error)
}

// Summarizer produces a summary for older messages.
type Summarizer interface {
	Summarize(ctx context.Context, messages []agent.Message) (string, error)
}

// TokenEstimator optionally provides model-aware token estimates.
type TokenEstimator interface {
	EstimateMessageTokens(ctx context.Context, model string, msg agent.Message) (int, error)
}

// FuncSummarizer adapts a function into a Summarizer.
type FuncSummarizer func(ctx context.Context, messages []agent.Message) (string, error)

// Summarize implements Summarizer.
func (f FuncSummarizer) Summarize(ctx context.Context, messages []agent.Message) (string, error) {
	return f(ctx, messages)
}

// Input describes the full message history and selection config.
type Input struct {
	Model                 string
	SystemMessages        []agent.Message
	Messages              []agent.Message
	MaxTokens             int
	RollingWindow         int
	MaxToolOutputChars    int
	SummarizationEnabled  bool
	SummarizationThreshold int
	Summarizer            Summarizer
	TokenEstimator        TokenEstimator
}

// SummaryMetadataKey marks a generated summary message.
const SummaryMetadataKey = "context_summary"

// DefaultPolicy provides bounded selection with optional summarization.
type DefaultPolicy struct{}

// NewDefaultPolicy returns a new policy instance.
func NewDefaultPolicy() *DefaultPolicy {
	return &DefaultPolicy{}
}

// Select selects messages based on the input configuration.
func (p *DefaultPolicy) Select(ctx context.Context, input Input) ([]agent.Message, error) {
	system := cloneMessages(input.SystemMessages)
	messages := cloneMessages(input.Messages)

	if input.MaxToolOutputChars > 0 {
		messages = truncateToolOutputs(messages, input.MaxToolOutputChars)
	}

	summarized := false
	if input.SummarizationEnabled && input.Summarizer != nil && input.SummarizationThreshold > 0 {
		totalTokens := estimateTokens(ctx, input.Model, system, input.TokenEstimator) + estimateTokens(ctx, input.Model, messages, input.TokenEstimator)
		if totalTokens > input.SummarizationThreshold {
			keep := input.RollingWindow
			if keep <= 0 {
				keep = 6
			}
			if len(messages) > keep && !hasSummary(messages) {
				summaryText, err := input.Summarizer.Summarize(ctx, messages[:len(messages)-keep])
				if err != nil {
					return nil, err
				}
				summaryText = strings.TrimSpace(summaryText)
				if summaryText == "" {
					return nil, errors.New("empty summary")
				}
				summary := agent.Message{
					Role:  agent.RoleSystem,
					Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: summaryText}},
					Metadata: map[string]any{
						SummaryMetadataKey: true,
					},
				}
				messages = append([]agent.Message{summary}, messages[len(messages)-keep:]...)
				summarized = true
			}
		}
	}

	if input.RollingWindow > 0 && !summarized && len(messages) > input.RollingWindow {
		messages = messages[len(messages)-input.RollingWindow:]
	}

	if input.MaxTokens <= 0 {
		return append(system, messages...), nil
	}

	return applyTokenBudget(ctx, input.Model, system, messages, input.MaxTokens, input.TokenEstimator), nil
}

func truncateToolOutputs(messages []agent.Message, maxChars int) []agent.Message {
	if maxChars <= 0 {
		return messages
	}
	out := make([]agent.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != agent.RoleTool {
			out = append(out, msg)
			continue
		}
		text := agent.TextFromParts(msg.Parts)
		if text == "" {
			text = msg.Text()
		}
		if len(text) <= maxChars {
			out = append(out, msg)
			continue
		}
		trimmed := truncateString(text, maxChars)
		metadata := cloneMetadata(msg.Metadata)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["truncated_tool_output"] = true
		out = append(out, agent.Message{
			Role:       agent.RoleTool,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
			Metadata:   metadata,
			Parts:      []agent.ContentPart{{Type: agent.ContentPartText, Text: trimmed}},
		})
	}
	return out
}

func applyTokenBudget(ctx context.Context, model string, system []agent.Message, messages []agent.Message, maxTokens int, estimator TokenEstimator) []agent.Message {
	systemTokens := estimateTokens(ctx, model, system, estimator)
	pinned, others := splitPinned(messages)
	pinnedTokens := estimateTokens(ctx, model, pinned, estimator)

	remaining := maxTokens - systemTokens - pinnedTokens
	if remaining <= 0 {
		return append(system, pinned...)
	}

	selected := make([]agent.Message, 0, len(others))
	tokens := 0
	for i := len(others) - 1; i >= 0; i-- {
		msgTokens := estimateMessageTokens(ctx, model, others[i], estimator)
		if tokens+msgTokens > remaining {
			continue
		}
		tokens += msgTokens
		selected = append(selected, others[i])
	}

	// Reverse to preserve chronological order.
	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}

	out := make([]agent.Message, 0, len(system)+len(pinned)+len(selected))
	out = append(out, system...)
	out = append(out, pinned...)
	out = append(out, selected...)
	return out
}

func splitPinned(messages []agent.Message) ([]agent.Message, []agent.Message) {
	pinned := make([]agent.Message, 0)
	others := make([]agent.Message, 0)
	for _, msg := range messages {
		if isPinned(msg) {
			pinned = append(pinned, msg)
		} else {
			others = append(others, msg)
		}
	}
	return pinned, others
}

func isPinned(msg agent.Message) bool {
	if msg.Metadata == nil {
		return false
	}
	if v, ok := msg.Metadata[SummaryMetadataKey].(bool); ok && v {
		return true
	}
	if v, ok := msg.Metadata["context_pinned"].(bool); ok && v {
		return true
	}
	return false
}

func hasSummary(messages []agent.Message) bool {
	for _, msg := range messages {
		if isPinned(msg) {
			if v, ok := msg.Metadata[SummaryMetadataKey].(bool); ok && v {
				return true
			}
		}
	}
	return false
}

func estimateTokens(ctx context.Context, model string, messages []agent.Message, estimator TokenEstimator) int {
	total := 0
	for _, msg := range messages {
		total += estimateMessageTokens(ctx, model, msg, estimator)
	}
	return total
}

const imageTokenEstimate = 256

func estimateMessageTokens(ctx context.Context, model string, msg agent.Message, estimator TokenEstimator) int {
	if estimator != nil {
		tokens, err := estimator.EstimateMessageTokens(ctx, model, msg)
		if err == nil {
			if tokens > 0 || isMessageEmpty(msg) {
				return tokens
			}
		}
	}
	text := agent.TextFromParts(msg.Parts)
	if text == "" {
		text = msg.Text()
	}
	tokens := len(text) / 4
	tokens += msg.ImageCount() * imageTokenEstimate
	return tokens
}

func isMessageEmpty(msg agent.Message) bool {
	text := agent.TextFromParts(msg.Parts)
	if text == "" {
		text = msg.Text()
	}
	return text == "" && msg.ImageCount() == 0
}

func truncateString(value string, maxChars int) string {
	if maxChars <= 0 || len(value) <= maxChars {
		return value
	}
	if maxChars <= 3 {
		return value[:maxChars]
	}
	return value[:maxChars-3] + "..."
}

func cloneMessages(messages []agent.Message) []agent.Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]agent.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, cloneMessage(msg))
	}
	return out
}

func cloneMessage(msg agent.Message) agent.Message {
	out := msg
	if len(msg.Parts) > 0 {
		parts := make([]agent.ContentPart, len(msg.Parts))
		copy(parts, msg.Parts)
		out.Parts = parts
	}
	out.Metadata = cloneMetadata(msg.Metadata)
	return out
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}
