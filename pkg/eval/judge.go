package eval

import "context"

// JudgeInput is evaluated by a judge model.
type JudgeInput struct {
	Protocol  string
	Model     string
	Variant   string
	Scenario  Scenario
	Turns     []TranscriptTurn
	Shortlist []string
	Final     string
}

// Judge scores one run.
type Judge interface {
	Score(ctx context.Context, input JudgeInput) (JudgeScore, error)
}

// DefaultDimensionWeights returns sensible defaults for brainstorming quality.
func DefaultDimensionWeights() DimensionScores {
	return DimensionScores{
		FlowArc:            0.18,
		PersonaSeparation:  0.18,
		Responsiveness:     0.14,
		Naturalness:        0.14,
		IdeaQuality:        0.14,
		ConvergenceQuality: 0.14,
		ReportQuality:      0.08,
	}
}

// CriticalDimensionNames returns default critical dimensions for regression gates.
func CriticalDimensionNames() []string {
	return []string{"flow_arc", "persona_separation", "naturalness", "convergence_quality"}
}

// DimensionValue resolves a score by stable field name.
func DimensionValue(scores DimensionScores, name string) float64 {
	switch name {
	case "flow_arc":
		return scores.FlowArc
	case "persona_separation":
		return scores.PersonaSeparation
	case "responsiveness":
		return scores.Responsiveness
	case "naturalness":
		return scores.Naturalness
	case "idea_quality":
		return scores.IdeaQuality
	case "convergence_quality":
		return scores.ConvergenceQuality
	case "report_quality":
		return scores.ReportQuality
	default:
		return 0
	}
}
