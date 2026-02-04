package contextpolicy

import (
	"context"
	"strings"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

func TestDefaultPolicyNoop(t *testing.T) {
	policy := NewDefaultPolicy()
	system := []agent.Message{{Role: agent.RoleSystem, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "sys"}}}}
	history := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "u1"}}},
		{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "a1"}}},
	}

	out, err := policy.Select(context.Background(), Input{
		SystemMessages: system,
		Messages:       history,
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
	if out[0].Role != agent.RoleSystem || out[1].Role != agent.RoleUser || out[2].Role != agent.RoleAssistant {
		t.Fatalf("unexpected ordering")
	}
}

func TestDefaultPolicyRollingWindow(t *testing.T) {
	policy := NewDefaultPolicy()
	history := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "u1"}}},
		{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "a1"}}},
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "u2"}}},
	}

	out, err := policy.Select(context.Background(), Input{
		Messages:      history,
		RollingWindow: 2,
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	if out[0].Text() != "a1" || out[1].Text() != "u2" {
		t.Fatalf("unexpected rolling window output")
	}
}

func TestDefaultPolicyTokenBudget(t *testing.T) {
	policy := NewDefaultPolicy()
	history := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("a", 80)}}},
		{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("b", 80)}}},
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("c", 80)}}},
	}

	out, err := policy.Select(context.Background(), Input{
		Messages:  history,
		MaxTokens: 40,
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected some messages")
	}
	if out[len(out)-1].Text() != strings.Repeat("c", 80) {
		t.Fatalf("expected most recent message to be kept")
	}
}

func TestDefaultPolicyTruncatesToolOutput(t *testing.T) {
	policy := NewDefaultPolicy()
	history := []agent.Message{
		{
			Role:  agent.RoleTool,
			Name:  "tool",
			Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "1234567890"}},
		},
	}

	out, err := policy.Select(context.Background(), Input{
		Messages:           history,
		MaxToolOutputChars: 5,
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if out[0].Text() == "1234567890" {
		t.Fatalf("expected tool output to be truncated")
	}
	if out[0].Metadata["truncated_tool_output"] != true {
		t.Fatalf("expected truncated metadata flag")
	}
}

func TestDefaultPolicySummarization(t *testing.T) {
	policy := NewDefaultPolicy()
	history := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("u", 8)}}},
		{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("a", 8)}}},
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("u", 8)}}},
	}

	summary := "summary"
	out, err := policy.Select(context.Background(), Input{
		Messages:               history,
		SummarizationEnabled:   true,
		SummarizationThreshold: 1,
		RollingWindow:          1,
		Summarizer: FuncSummarizer(func(ctx context.Context, messages []agent.Message) (string, error) {
			return summary, nil
		}),
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(out) < 2 {
		t.Fatalf("expected summary + recent message")
	}
	if out[0].Metadata[SummaryMetadataKey] != true {
		t.Fatalf("expected summary metadata")
	}
	if out[0].Text() != summary {
		t.Fatalf("unexpected summary content")
	}
}

type stubEstimator struct{}

func (s stubEstimator) EstimateMessageTokens(ctx context.Context, model string, msg agent.Message) (int, error) {
	if msg.Text() == "drop" {
		return 100, nil
	}
	return 1, nil
}

func TestDefaultPolicyUsesTokenEstimator(t *testing.T) {
	policy := NewDefaultPolicy()
	history := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "drop"}}},
		{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "keep"}}},
	}

	out, err := policy.Select(context.Background(), Input{
		Messages:       history,
		MaxTokens:      2,
		TokenEstimator: stubEstimator{},
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if out[0].Text() != "keep" {
		t.Fatalf("expected estimator to influence selection")
	}
}
