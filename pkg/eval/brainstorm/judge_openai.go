package brainstorm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/eval"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

// OpenAIJudge evaluates transcript quality with a judge model.
type OpenAIJudge struct {
	provider *openai.Client
	model    string
}

// NewOpenAIJudge creates a judge backed by the configured OpenAI provider.
func NewOpenAIJudge(provider *openai.Client, model string) (*OpenAIJudge, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("judge model required")
	}
	return &OpenAIJudge{provider: provider, model: strings.TrimSpace(model)}, nil
}

func (j *OpenAIJudge) Score(ctx context.Context, input eval.JudgeInput) (eval.JudgeScore, error) {
	eng, err := engine.New(
		engine.WithProvider(j.provider),
		engine.WithDefaultModel(j.model),
	)
	if err != nil {
		return eval.JudgeScore{}, fmt.Errorf("new engine: %w", err)
	}

	judgeAgent := eng.Agent("Protocol Eval Judge").
		Prompt(judgeSystemPrompt).
		OutputSchema(judgeResponse{}).
		Build()

	sess, err := eng.Session("Protocol Eval Judge Session").
		Participant(judgeAgent).
		Protocol(protocol.Solo()).
		Start(ctx)
	if err != nil {
		return eval.JudgeScore{}, fmt.Errorf("start judge session: %w", err)
	}

	resp, err := sess.RunAgent(ctx, judgeAgent, protocol.RunRequest{
		Messages:          []agent.Message{message.User(buildJudgeUserPrompt(input))},
		MaxToolIterations: 1,
	})
	if err != nil {
		return eval.JudgeScore{}, fmt.Errorf("judge run: %w", err)
	}

	raw := strings.TrimSpace(agent.TextFromParts(resp.Parts))
	if raw == "" {
		raw = strings.TrimSpace(resp.Text())
	}
	parsed, err := parseJudgeResponse(raw)
	if err != nil {
		return eval.JudgeScore{}, fmt.Errorf("parse judge response: %w", err)
	}

	dims := eval.DimensionScores{
		FlowArc:            clampScore(parsed.FlowArc),
		PersonaSeparation:  clampScore(parsed.PersonaSeparation),
		Responsiveness:     clampScore(parsed.Responsiveness),
		Naturalness:        clampScore(parsed.Naturalness),
		IdeaQuality:        clampScore(parsed.IdeaQuality),
		ConvergenceQuality: clampScore(parsed.ConvergenceQuality),
		ReportQuality:      clampScore(parsed.ReportQuality),
	}
	overall := clampScore(parsed.Overall)
	if overall == 0 {
		overall = dims.WeightedAverage(eval.DefaultDimensionWeights())
	}

	return eval.JudgeScore{
		Model:      j.model,
		Dimensions: dims,
		Overall:    overall,
		Summary:    strings.TrimSpace(parsed.Summary),
		StrengthA:  strings.TrimSpace(parsed.StrengthA),
		StrengthB:  strings.TrimSpace(parsed.StrengthB),
		RiskA:      strings.TrimSpace(parsed.RiskA),
		RiskB:      strings.TrimSpace(parsed.RiskB),
	}, nil
}

type judgeResponse struct {
	FlowArc            float64 `json:"flow_arc"`
	PersonaSeparation  float64 `json:"persona_separation"`
	Responsiveness     float64 `json:"responsiveness"`
	Naturalness        float64 `json:"naturalness"`
	IdeaQuality        float64 `json:"idea_quality"`
	ConvergenceQuality float64 `json:"convergence_quality"`
	ReportQuality      float64 `json:"report_quality"`
	Overall            float64 `json:"overall"`
	Summary            string  `json:"summary"`
	StrengthA          string  `json:"strength_a"`
	StrengthB          string  `json:"strength_b"`
	RiskA              string  `json:"risk_a"`
	RiskB              string  `json:"risk_b"`
}

const judgeSystemPrompt = `You are a strict evaluation judge for multi-agent brainstorming transcripts.
Score each dimension on a 1.0 to 5.0 scale where 5.0 is excellent and 1.0 is poor.
Use the rubric:
- flow_arc: natural progression through phases (explore -> diverge -> present -> converge -> vote/report)
- persona_separation: each speaker has distinct voice and role behavior
- responsiveness: speakers directly react to each other, not parallel monologues
- naturalness: human-like cadence and language, low templating/robotic style
- idea_quality: specificity, feasibility, and novelty of ideas
- convergence_quality: discussion narrows effectively to shortlist/final picks
- report_quality: final summary is clear, client-facing, and faithful to discussion

Return only JSON matching the schema.`

func buildJudgeUserPrompt(input eval.JudgeInput) string {
	var b strings.Builder
	b.WriteString("Protocol: ")
	b.WriteString(input.Protocol)
	b.WriteString("\n")
	b.WriteString("Model under test: ")
	b.WriteString(input.Model)
	b.WriteString("\n")
	b.WriteString("Variant: ")
	b.WriteString(input.Variant)
	b.WriteString("\n")
	b.WriteString("Scenario: ")
	b.WriteString(input.Scenario.ID)
	b.WriteString("\n")
	if desc := strings.TrimSpace(input.Scenario.Description); desc != "" {
		b.WriteString("Scenario description: ")
		b.WriteString(desc)
		b.WriteString("\n")
	}
	b.WriteString("Prompt:\n")
	b.WriteString(strings.TrimSpace(input.Scenario.Prompt))
	b.WriteString("\n\n")

	b.WriteString("Transcript turns:\n")
	maxTurns := len(input.Turns)
	if maxTurns > 80 {
		maxTurns = 80
	}
	for i := 0; i < maxTurns; i++ {
		turn := input.Turns[i]
		b.WriteString(fmt.Sprintf("%02d. [%s] %s\n", i+1, turn.Speaker, strings.TrimSpace(turn.Text)))
	}
	if len(input.Turns) > maxTurns {
		b.WriteString(fmt.Sprintf("... (%d additional turns omitted)\n", len(input.Turns)-maxTurns))
	}

	if len(input.Shortlist) > 0 {
		b.WriteString("\nShortlist:\n")
		for i, item := range input.Shortlist {
			b.WriteString(fmt.Sprintf("%d) %s\n", i+1, item))
		}
	}

	if final := strings.TrimSpace(input.Final); final != "" {
		b.WriteString("\nFinal moderator report:\n")
		b.WriteString(final)
		b.WriteString("\n")
	}

	b.WriteString(`
Return JSON with keys:
flow_arc, persona_separation, responsiveness, naturalness, idea_quality, convergence_quality, report_quality, overall, summary, strength_a, strength_b, risk_a, risk_b.
All score fields must be numbers in [1.0, 5.0].`)

	return b.String()
}

func parseJudgeResponse(raw string) (judgeResponse, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return judgeResponse{}, fmt.Errorf("empty response")
	}
	var out judgeResponse
	if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
		return out, nil
	}
	candidate := extractJSONObject(trimmed)
	if candidate == "" {
		return judgeResponse{}, fmt.Errorf("no parseable json object")
	}
	if err := json.Unmarshal([]byte(candidate), &out); err != nil {
		return judgeResponse{}, err
	}
	return out, nil
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}

func clampScore(v float64) float64 {
	if v < 1 {
		return 1
	}
	if v > 5 {
		return 5
	}
	return v
}
