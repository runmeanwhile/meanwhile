package contextpolicy

import (
	"context"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

func TestAutoSummarizePolicy_SummarizesWhenThresholdExceeded(t *testing.T) {
	policy := NewAutoSummarizePolicy(NewDefaultPolicy(), AutoSummarizeConfig{
		SummarizeAtTokens: 1,
		MinKeepMessages:   1,
	})
	summarizer := FuncSummarizer(func(ctx context.Context, messages []agent.Message) (string, error) {
		_ = ctx
		if len(messages) == 0 {
			t.Fatalf("expected messages to summarize")
		}
		return "summary", nil
	})

	input := Input{
		Model:         "test",
		Messages:      []agent.Message{{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "alpha"}}}, {Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "beta"}}}},
		RollingWindow: 1,
		Summarizer:    summarizer,
	}

	out, err := policy.Select(context.Background(), input)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(out) < 2 {
		t.Fatalf("expected summary + recent message, got %d", len(out))
	}
	if out[0].Metadata[SummaryMetadataKey] != true {
		t.Fatalf("expected summary metadata")
	}
	if out[0].Text() != "summary" {
		t.Fatalf("unexpected summary text: %q", out[0].Text())
	}
}
