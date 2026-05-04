package ideationops

import "fmt"

// Operator defines a divergent ideation method.
type Operator struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Prompt  string `json:"prompt"`
	Outcome string `json:"outcome,omitempty"`
}

// DefaultOperators returns a reusable set of divergent operators.
func DefaultOperators() []Operator {
	return []Operator{
		{
			ID:      "analogy_transfer",
			Name:    "Analogy Transfer",
			Prompt:  "Borrow one mechanic from a different domain and map it to this problem.",
			Outcome: "fresh mechanism from adjacent industry",
		},
		{
			ID:      "inversion_flip",
			Name:    "Inversion Flip",
			Prompt:  "Describe the worst possible approach, then invert it into a strong concept.",
			Outcome: "reveals hidden assumptions",
		},
		{
			ID:      "constraint_remix",
			Name:    "Constraint Remix",
			Prompt:  "Keep the toughest constraint fixed, then redesign the experience around it.",
			Outcome: "shippable idea under real constraints",
		},
		{
			ID:      "trust_repair",
			Name:    "Trust Repair",
			Prompt:  "Assume user trust is fragile; design a concept that earns trust quickly.",
			Outcome: "behavioral adoption gains",
		},
		{
			ID:      "speed_to_learning",
			Name:    "Speed to Learning",
			Prompt:  "Maximize what the team can learn in one week with minimal build effort.",
			Outcome: "test-ready experiment concept",
		},
		{
			ID:      "ritual_design",
			Name:    "Ritual Design",
			Prompt:  "Design one recurring team ritual that changes behavior without new software.",
			Outcome: "operational behavior change",
		},
	}
}

// ForRound returns one operator per participant in a deterministic rotation.
func ForRound(round int, participants int) []Operator {
	ops := DefaultOperators()
	if participants <= 0 || len(ops) == 0 {
		return nil
	}
	if round < 0 {
		round = 0
	}
	out := make([]Operator, 0, participants)
	start := round % len(ops)
	for i := 0; i < participants; i++ {
		idx := (start + i) % len(ops)
		out = append(out, ops[idx])
	}
	return out
}

// PromptBlock renders a compact prompt line for an operator.
func PromptBlock(op Operator) string {
	if op.Outcome == "" {
		return fmt.Sprintf("Operator %q: %s", op.Name, op.Prompt)
	}
	return fmt.Sprintf("Operator %q: %s Target outcome: %s.", op.Name, op.Prompt, op.Outcome)
}
