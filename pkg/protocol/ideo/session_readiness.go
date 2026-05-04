package ideo

import (
	"context"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

// ReadinessDecision represents the moderator's decision about proceeding.
type ReadinessDecision string

const (
	// DecisionProceed indicates the session has enough context to proceed.
	DecisionProceed ReadinessDecision = "proceed"

	// DecisionProceedWithAssumptions indicates proceeding with stated assumptions.
	DecisionProceedWithAssumptions ReadinessDecision = "proceed_with_assumptions"

	// DecisionRequestInfo indicates more information is needed from the user.
	DecisionRequestInfo ReadinessDecision = "request_info"

	// DecisionReject indicates the request is too vague or inappropriate.
	DecisionReject ReadinessDecision = "reject"
)

// ReadinessResult contains the outcome of the readiness gate.
type ReadinessResult struct {
	Decision     ReadinessDecision `json:"decision"`
	Assumptions  []string          `json:"assumptions,omitempty"`
	Missing      []string          `json:"missing,omitempty"`
	Rejection    string            `json:"rejection,omitempty"`
	Context      string            `json:"context,omitempty"` // Gathered context summary
	RefinedScope string            `json:"refined_scope,omitempty"`
}

type readinessContextSummary struct {
	Product     string   `json:"product" description:"What is known about the product/service in this problem context"`
	Users       string   `json:"users" description:"What is known about user segments/personas and behavior"`
	Market      string   `json:"market" description:"What is known about market/competitive context"`
	Metrics     string   `json:"metrics" description:"What is known about KPIs, baselines, and targets"`
	Strategy    string   `json:"strategy" description:"What is known about goals, north star, and strategic intent"`
	Constraints []string `json:"constraints" description:"Known constraints that should shape recommendations"`
	Gaps        []string `json:"gaps" description:"Critical unknowns or missing context"`
	SourceRefs  []string `json:"source_refs,omitempty" description:"Source identifiers or document paths used for this summary"`
	KeyFacts    []string `json:"key_facts,omitempty" description:"Most decision-relevant factual findings from available context"`
}

func (s readinessContextSummary) Render() string {
	var sb strings.Builder
	sb.WriteString("PRODUCT:\n")
	sb.WriteString(strings.TrimSpace(s.Product))
	sb.WriteString("\n\nUSERS:\n")
	sb.WriteString(strings.TrimSpace(s.Users))
	sb.WriteString("\n\nMARKET:\n")
	sb.WriteString(strings.TrimSpace(s.Market))
	sb.WriteString("\n\nMETRICS:\n")
	sb.WriteString(strings.TrimSpace(s.Metrics))
	sb.WriteString("\n\nSTRATEGY:\n")
	sb.WriteString(strings.TrimSpace(s.Strategy))
	if len(s.Constraints) > 0 {
		sb.WriteString("\n\nCONSTRAINTS:\n")
		for _, constraint := range s.Constraints {
			if strings.TrimSpace(constraint) == "" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(constraint))
			sb.WriteString("\n")
		}
	}
	if len(s.KeyFacts) > 0 {
		sb.WriteString("\nKEY FACTS:\n")
		for _, fact := range s.KeyFacts {
			if strings.TrimSpace(fact) == "" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(fact))
			sb.WriteString("\n")
		}
	}
	if len(s.Gaps) > 0 {
		sb.WriteString("\nGAPS:\n")
		for _, gap := range s.Gaps {
			if strings.TrimSpace(gap) == "" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(gap))
			sb.WriteString("\n")
		}
	}
	if len(s.SourceRefs) > 0 {
		sb.WriteString("\nSOURCES:\n")
		for _, ref := range s.SourceRefs {
			if strings.TrimSpace(ref) == "" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(ref))
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

type readinessAssessment struct {
	Decision     ReadinessDecision `json:"decision" description:"One of: proceed, proceed_with_assumptions, request_info, reject"`
	Assumptions  []string          `json:"assumptions,omitempty" description:"Explicit assumptions required to proceed safely"`
	Missing      []string          `json:"missing,omitempty" description:"Information needed from the requester before proceeding"`
	Rejection    string            `json:"rejection,omitempty" description:"Reason to reject the request if decision is reject"`
	RefinedScope string            `json:"refined_scope,omitempty" description:"Sharper problem framing for the team if proceeding"`
	Rationale    string            `json:"rationale,omitempty" description:"Why this decision is appropriate for team productivity and quality"`
}

// runReadinessGate has the moderator assess whether there's enough context to proceed.
// This is a semantic gate - the moderator uses judgment, not a checklist.
func (p *brainstormIDEO) runReadinessGate(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, seed agent.Message) (*ReadinessResult, error) {
	runner := p.selectRunner(sess, agents)

	// Phase 1: Gather context
	contextSummary, err := p.gatherReadinessContext(ctx, sess, runner, scope, seed)
	if err != nil {
		return nil, fmt.Errorf("gather context: %w", err)
	}

	// Phase 2: Assess readiness and decide
	result, err := p.assessReadiness(ctx, sess, runner, scope, contextSummary)
	if err != nil {
		return nil, fmt.Errorf("assess readiness: %w", err)
	}

	return result, nil
}

// gatherReadinessContext has the moderator use tools to gather context about the problem.
func (p *brainstormIDEO) gatherReadinessContext(ctx context.Context, sess protocol.Session, runner agent.Agent, scope string, seed agent.Message) (string, error) {
	system := `You are the brainstorm moderator preparing to convene an IDEO-style session.

Gather context before convening the team:
- product/service definition and onboarding context
- users/personas/segments
- market + competitive positioning
- baseline metrics, targets, and funnel bottlenecks
- strategic intent + constraints

Use tools first; gather concrete facts and source references.
Return only structured output matching the schema.`

	userPrompt := fmt.Sprintf(`The team has been asked to brainstorm:
"""
%s
"""

Gather context before we proceed. Use your tools.`, scope)

	// Use configured context tool budget (can be raised by the example/config).
	toolBudget := p.cfg.ContextPlan.MaxToolIterations()
	if toolBudget <= 0 {
		toolBudget = 6
	}

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{PromptWithMedia(userPrompt, seed)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: toolBudget,
		Tools:             p.cfg.ContextPlan.AllowedToolIDs(),
		ToolPolicy:        p.cfg.ContextPlan.ToolPolicy(),
		OutputSchema:      readinessContextSummary{},
	})
	if err != nil {
		return "", err
	}

	summary, err := parseStructuredOutput[readinessContextSummary](resp)
	if err != nil {
		return "", err
	}

	return summary.Render(), nil
}

// assessReadiness has the moderator decide whether to proceed based on gathered context.
func (p *brainstormIDEO) assessReadiness(ctx context.Context, sess protocol.Session, runner agent.Agent, scope string, contextSummary string) (*ReadinessResult, error) {
	system := `You are the brainstorm moderator deciding whether to convene the session.

Decide one of:
- proceed
- proceed_with_assumptions
- request_info
- reject

Rules:
- Prefer proceed when context is sufficient for actionable work.
- Use proceed_with_assumptions when assumptions are explicit and testable.
- Use request_info when missing inputs block responsible recommendations.
- Use reject only when scope is unsuitable or incoherent.
- Preserve explicit business outcomes and targets from the original request when refining scope.

Return only structured output matching the schema.`

	userPrompt := fmt.Sprintf(`Original request:
"""
%s
"""

Context gathered:
"""
%s
"""

Make your decision. Remember: the team will be judged by their results. 
Be a responsible gatekeeper - don't let them waste time on vague requests, 
but also don't be so restrictive that you block reasonable work.`, scope, contextSummary)

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(userPrompt)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
		OutputSchema:      readinessAssessment{},
	})
	if err != nil {
		return nil, err
	}

	assessment, err := parseStructuredOutput[readinessAssessment](resp)
	if err != nil {
		return nil, err
	}

	switch assessment.Decision {
	case DecisionProceed, DecisionProceedWithAssumptions, DecisionRequestInfo, DecisionReject:
	default:
		return nil, fmt.Errorf("invalid readiness decision %q", assessment.Decision)
	}

	return &ReadinessResult{
		Decision:     assessment.Decision,
		Assumptions:  deduplicateStrings(assessment.Assumptions),
		Missing:      deduplicateStrings(assessment.Missing),
		Rejection:    strings.TrimSpace(assessment.Rejection),
		Context:      contextSummary,
		RefinedScope: strings.TrimSpace(assessment.RefinedScope),
	}, nil
}

// formatAssumptionsForTeam creates a prompt section informing the team of assumptions.
func formatAssumptionsForTeam(result *ReadinessResult) string {
	if result == nil || len(result.Assumptions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("⚠️ WORKING ASSUMPTIONS (stated by moderator due to incomplete context):\n")
	for i, assumption := range result.Assumptions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, assumption))
	}
	sb.WriteString("\nThese assumptions should be validated. Challenge them if you have contradicting information.\n")
	return sb.String()
}
