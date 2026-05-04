package ideo

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

type stagePlanRegistry struct {
	mu   sync.Mutex
	plan StagePlan
	set  bool
}

func newStagePlanRegistry() *stagePlanRegistry {
	return &stagePlanRegistry{}
}

func (r *stagePlanRegistry) Tool() (tool.Tool, error) {
	type result struct {
		Status string    `json:"status"`
		Plan   StagePlan `json:"plan"`
	}
	t, err := tool.New("set_stage_plan", func(_ context.Context, input StagePlan) (result, error) {
		r.mu.Lock()
		r.plan = input
		r.set = true
		r.mu.Unlock()
		return result{
			Status: "planned",
			Plan:   input,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return t.WithDescription(`Set the strategic stage plan for the brainstorming run.

Use this once after reviewing scope + evidence to define:
- Non-negotiable outcomes and constraints
- The most relevant lenses to reframe the problem
- Which tools should be prioritized for evidence gathering
- Key questions the team must answer

Choose only lenses that are truly relevant to the current problem.`), nil
}

func (r *stagePlanRegistry) Plan() (StagePlan, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.set {
		return StagePlan{}, false
	}
	return r.plan, true
}

func (p *brainstormIDEO) runStagePlan(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, readiness *ReadinessResult) (*StagePlan, error) {
	runner := p.selectRunner(sess, agents)
	registry := newStagePlanRegistry()
	planTool, err := registry.Tool()
	if err != nil {
		return nil, fmt.Errorf("create stage plan tool: %w", err)
	}
	if err := sess.RegisterTool(planTool); err != nil {
		return nil, fmt.Errorf("register stage plan tool: %w", err)
	}

	availableTools := deduplicateStrings(p.cfg.ContextPlan.AllowedToolIDs())
	lensCatalog := deduplicateStrings(p.cfg.LensCatalog)
	if len(lensCatalog) == 0 {
		lensCatalog = defaultLensCatalog()
	}

	system := `You are setting the strategic stage plan before collaboration begins.

Use the available evidence and scope to define:
1) non-negotiable outcomes/constraints,
2) the most relevant reframing lenses,
3) which tools should be prioritized,
4) key questions the team must answer.

Call set_stage_plan exactly once.`

	var user strings.Builder
	user.WriteString(fmt.Sprintf("Original scope:\n%s\n\n", scope))
	if readiness != nil && strings.TrimSpace(readiness.RefinedScope) != "" {
		user.WriteString("Refined framing from readiness:\n")
		user.WriteString(strings.TrimSpace(readiness.RefinedScope))
		user.WriteString("\n\n")
	}
	if readiness != nil && strings.TrimSpace(readiness.Context) != "" {
		user.WriteString("Readiness context:\n")
		user.WriteString(strings.TrimSpace(readiness.Context))
		user.WriteString("\n\n")
	}
	if readiness != nil && len(readiness.Assumptions) > 0 {
		user.WriteString("Known assumptions:\n")
		for _, assumption := range readiness.Assumptions {
			user.WriteString("- ")
			user.WriteString(assumption)
			user.WriteString("\n")
		}
		user.WriteString("\n")
	}
	if len(p.cfg.ContextPlan.Questions) > 0 {
		user.WriteString("Existing focus questions:\n")
		for _, question := range p.cfg.ContextPlan.Questions {
			user.WriteString("- ")
			user.WriteString(question)
			user.WriteString("\n")
		}
		user.WriteString("\n")
	}
	user.WriteString(fmt.Sprintf("Lens catalog: %s\n", strings.Join(lensCatalog, ", ")))
	user.WriteString(fmt.Sprintf("Available tools: %s\n", strings.Join(availableTools, ", ")))
	user.WriteString("Now call set_stage_plan.")

	_, err = sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 3,
		Tools:             []string{"set_stage_plan"},
	})
	if err != nil {
		return nil, err
	}

	plan, ok := registry.Plan()
	if !ok {
		plan = StagePlan{}
	}

	normalized := normalizeStagePlan(plan, scope, readiness, lensCatalog, availableTools, p.cfg.ContextPlan.Questions)
	return &normalized, nil
}

func normalizeStagePlan(plan StagePlan, scope string, readiness *ReadinessResult, lensCatalog, availableTools, defaultQuestions []string) StagePlan {
	normalized := StagePlan{
		ProblemStatement: strings.TrimSpace(plan.ProblemStatement),
		NonNegotiables:   deduplicateStrings(plan.NonNegotiables),
		Lenses:           deduplicateStrings(plan.Lenses),
		ToolIDs:          deduplicateStrings(plan.ToolIDs),
		Questions:        deduplicateStrings(plan.Questions),
		Rationale:        strings.TrimSpace(plan.Rationale),
	}

	if normalized.ProblemStatement == "" {
		normalized.ProblemStatement = strings.TrimSpace(scope)
	}
	if readiness != nil && strings.TrimSpace(readiness.RefinedScope) != "" && !strings.EqualFold(readiness.RefinedScope, normalized.ProblemStatement) {
		normalized.NonNegotiables = deduplicateStrings(append(normalized.NonNegotiables, "Refined framing: "+strings.TrimSpace(readiness.RefinedScope)))
	}
	if readiness != nil && len(readiness.Assumptions) > 0 {
		prefixed := make([]string, 0, len(readiness.Assumptions))
		for _, assumption := range readiness.Assumptions {
			prefixed = append(prefixed, "Assumption: "+strings.TrimSpace(assumption))
		}
		normalized.NonNegotiables = deduplicateStrings(append(normalized.NonNegotiables, prefixed...))
	}

	if len(normalized.Lenses) == 0 {
		normalized.Lenses = append(normalized.Lenses, lensCatalog...)
	}
	if len(normalized.Lenses) > 8 {
		normalized.Lenses = normalized.Lenses[:8]
	}

	if len(normalized.ToolIDs) == 0 {
		normalized.ToolIDs = append(normalized.ToolIDs, availableTools...)
	} else {
		allowed := make(map[string]struct{}, len(availableTools))
		for _, toolID := range availableTools {
			allowed[toolID] = struct{}{}
		}
		filtered := make([]string, 0, len(normalized.ToolIDs))
		for _, toolID := range normalized.ToolIDs {
			if _, ok := allowed[toolID]; ok {
				filtered = append(filtered, toolID)
			}
		}
		if len(filtered) == 0 {
			filtered = append(filtered, availableTools...)
		}
		normalized.ToolIDs = filtered
	}

	if len(normalized.Questions) == 0 {
		normalized.Questions = append(normalized.Questions, defaultQuestions...)
	}
	return normalized
}
