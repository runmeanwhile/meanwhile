package contextpolicy

import (
	"context"
	"errors"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

// AutoSummarizeConfig controls automatic summarization thresholds.
type AutoSummarizeConfig struct {
	SummarizeAtTokens int
	MinKeepMessages   int
}

// AutoSummarizePolicy wraps another policy with automatic summarization.
type AutoSummarizePolicy struct {
	base Policy
	cfg  AutoSummarizeConfig
}

// NewAutoSummarizePolicy creates an auto-summarizing policy wrapper.
func NewAutoSummarizePolicy(base Policy, cfg AutoSummarizeConfig) *AutoSummarizePolicy {
	return &AutoSummarizePolicy{base: base, cfg: cfg}
}

// Base returns the wrapped policy.
func (p *AutoSummarizePolicy) Base() Policy {
	if p == nil {
		return nil
	}
	return p.base
}

// Select applies auto-summarization before delegating to the base policy.
func (p *AutoSummarizePolicy) Select(ctx context.Context, input Input) ([]agent.Message, error) {
	if p == nil || p.base == nil {
		return nil, errors.New("base policy required")
	}
	if input.Summarizer == nil || p.cfg.SummarizeAtTokens <= 0 {
		return p.base.Select(ctx, input)
	}

	messages := cloneMessages(input.Messages)
	totalTokens := estimateTokens(ctx, input.Model, messages, input.TokenEstimator)
	if totalTokens <= p.cfg.SummarizeAtTokens {
		return p.base.Select(ctx, input)
	}

	keep := p.cfg.MinKeepMessages
	if keep <= 0 {
		keep = input.RollingWindow
	}
	if keep <= 0 {
		keep = 6
	}
	if len(messages) <= keep {
		return p.base.Select(ctx, input)
	}

	summaryText, err := input.Summarizer.Summarize(ctx, messages[:len(messages)-keep])
	if err != nil {
		return p.base.Select(ctx, input)
	}
	summaryText = strings.TrimSpace(summaryText)
	if summaryText == "" {
		return p.base.Select(ctx, input)
	}

	summary := agent.Message{
		Role:  agent.RoleSystem,
		Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: summaryText}},
		Metadata: map[string]any{
			SummaryMetadataKey: true,
		},
	}
	input.Messages = append([]agent.Message{summary}, messages[len(messages)-keep:]...)
	input.RollingWindow = 0
	return p.base.Select(ctx, input)
}
