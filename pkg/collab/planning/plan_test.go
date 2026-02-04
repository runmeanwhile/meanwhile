package planning

import (
	"testing"
	"time"
)

func TestNewPlan(t *testing.T) {
	steps := []Step{
		{Title: "Step 1", Description: "Do thing 1"},
		{Title: "Step 2", Description: "Do thing 2"},
	}

	plan := NewPlan("Test Plan", "A test plan", steps)

	if plan.ID == "" {
		t.Error("expected plan ID to be generated")
	}
	if plan.Title != "Test Plan" {
		t.Errorf("expected title 'Test Plan', got %q", plan.Title)
	}
	if plan.Summary != "A test plan" {
		t.Errorf("expected summary 'A test plan', got %q", plan.Summary)
	}
	if len(plan.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(plan.Steps))
	}
	if plan.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestPlan_Format(t *testing.T) {
	steps := []Step{
		{ID: "step-1", Title: "Setup", Description: "Setup environment"},
		{ID: "step-2", Title: "Build", Description: "Build project", Dependencies: []string{"step-1"}},
	}

	plan := &Plan{
		ID:        "test-id",
		Title:     "Build Plan",
		Summary:   "Plan for building",
		Steps:     steps,
		CreatedAt: time.Now(),
	}

	formatted := plan.Format()

	if formatted == "" {
		t.Error("expected non-empty formatted output")
	}
	if !contains(formatted, "Build Plan") {
		t.Error("expected formatted output to contain title")
	}
	if !contains(formatted, "Setup") {
		t.Error("expected formatted output to contain step titles")
	}
	if !contains(formatted, "Dependencies") {
		t.Error("expected formatted output to show dependencies")
	}
}

func TestPlan_JSON(t *testing.T) {
	steps := []Step{
		{ID: "step-1", Title: "Step 1"},
	}

	plan := &Plan{
		ID:        "test-id",
		Title:     "Test",
		Summary:   "Summary",
		Steps:     steps,
		CreatedAt: time.Now(),
	}

	data, err := plan.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON output")
	}
	if !contains(string(data), "Test") {
		t.Error("expected JSON to contain title")
	}
}

func TestParsePlan(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid JSON plan",
			input: `Here is the plan:
{
    "title": "Feature Plan",
    "summary": "Build a feature",
    "steps": [
        {
            "title": "Step 1",
            "description": "Do thing"
        }
    ]
}
Let me know if this works.`,
			wantErr: false,
		},
		{
			name: "plan with existing IDs",
			input: `{
    "id": "custom-id",
    "title": "Test",
    "summary": "Test plan",
    "steps": [
        {
            "id": "custom-step",
            "title": "Step"
        }
    ]
}`,
			wantErr: false,
		},
		{
			name:    "no JSON",
			input:   "This is just text without JSON",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `{"title": "Test", "steps": [}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := ParsePlan(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if plan.Title == "" {
				t.Error("expected title to be parsed")
			}
			if plan.ID == "" {
				t.Error("expected ID to be generated")
			}
			if plan.CreatedAt.IsZero() {
				t.Error("expected CreatedAt to be set")
			}

			for i, step := range plan.Steps {
				if step.ID == "" {
					t.Errorf("step %d: expected ID to be generated", i)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && s != substr && anyContains(s, substr)
}

func anyContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
