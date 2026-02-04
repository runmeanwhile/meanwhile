package planning

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Plan is a structured implementation plan.
type Plan struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Summary   string         `json:"summary"`
	Steps     []Step         `json:"steps"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Step is a single step in a plan.
type Step struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Description  string         `json:"description,omitempty"`
	Dependencies []string       `json:"dependencies,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// NewPlan creates a plan with generated ID and timestamp.
func NewPlan(title, summary string, steps []Step) *Plan {
	return &Plan{
		ID:        uuid.New().String(),
		Title:     title,
		Summary:   summary,
		Steps:     steps,
		CreatedAt: time.Now(),
	}
}

// Format returns a human-readable representation of the plan.
func (p *Plan) Format() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", p.Title))
	if p.Summary != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", p.Summary))
	}
	sb.WriteString("## Steps\n\n")
	for i, step := range p.Steps {
		sb.WriteString(fmt.Sprintf("%d. **%s**", i+1, step.Title))
		if step.Description != "" {
			sb.WriteString(fmt.Sprintf(": %s", step.Description))
		}
		sb.WriteString("\n")
		if len(step.Dependencies) > 0 {
			sb.WriteString(fmt.Sprintf("   - Dependencies: %s\n", strings.Join(step.Dependencies, ", ")))
		}
	}
	return sb.String()
}

// JSON returns the plan as JSON bytes.
func (p *Plan) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ParsePlan attempts to parse a plan from agent response text.
// It looks for JSON in the response and extracts plan structure.
func ParsePlan(text string) (*Plan, error) {
	// Try to find JSON in the response
	jsonStart := strings.Index(text, "{")
	jsonEnd := strings.LastIndex(text, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonStart >= jsonEnd {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonText := text[jsonStart : jsonEnd+1]

	var plan Plan
	if err := json.Unmarshal([]byte(jsonText), &plan); err != nil {
		return nil, fmt.Errorf("parse plan JSON: %w", err)
	}

	// Generate ID and timestamp if not present
	if plan.ID == "" {
		plan.ID = uuid.New().String()
	}
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}

	// Generate step IDs if missing
	for i := range plan.Steps {
		if plan.Steps[i].ID == "" {
			plan.Steps[i].ID = fmt.Sprintf("step-%d", i+1)
		}
	}

	return &plan, nil
}
