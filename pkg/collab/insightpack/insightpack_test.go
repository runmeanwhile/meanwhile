package insightpack

import (
	"reflect"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

func TestPlanAllowedToolIDsAndPolicy(t *testing.T) {
	plan := Plan{
		Strategy: StrategyBalanced,
		Sources: []Source{
			{ID: "memory", Type: SourceMemory, ToolIDs: []string{"mem.search", "doc.search"}},
			{ID: "docs", Type: SourceInternal, ToolIDs: []string{"doc.search", "drive.fetch"}},
		},
	}
	got := plan.AllowedToolIDs()
	want := []string{"mem.search", "doc.search", "drive.fetch"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("allowed tools mismatch: got=%v want=%v", got, want)
	}
	policy := plan.ToolPolicy()
	if policy.Mode != tool.PolicyAllowlist {
		t.Fatalf("expected allowlist mode, got %q", policy.Mode)
	}
	if !reflect.DeepEqual(policy.AllowIDs, want) {
		t.Fatalf("allow ids mismatch: got=%v want=%v", policy.AllowIDs, want)
	}
}

func TestPlanValidate(t *testing.T) {
	plan := DefaultPlan()
	plan.Strategy = "unknown"
	if err := plan.Validate(); err == nil {
		t.Fatal("expected error for unknown strategy")
	}
	plan = DefaultPlan()
	plan.Sources = []Source{{Type: SourceMemory}}
	if err := plan.Validate(); err == nil {
		t.Fatal("expected error for missing source id")
	}
}

func TestParseInsights(t *testing.T) {
	in := "- Stale commitments hide risk | Follow-through decays by week 4 | crm.issues\n- Metrics feel repetitive"
	insights := ParseInsights(in)
	if len(insights) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(insights))
	}
	if insights[0].SourceID != "crm.issues" {
		t.Fatalf("unexpected source id: %q", insights[0].SourceID)
	}
	if insights[1].Title != "Metrics feel repetitive" {
		t.Fatalf("unexpected second title: %q", insights[1].Title)
	}
}
