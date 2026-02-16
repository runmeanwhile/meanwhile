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
	Decision    ReadinessDecision `json:"decision"`
	Assumptions []string          `json:"assumptions,omitempty"`
	Missing     []string          `json:"missing,omitempty"`
	Rejection   string            `json:"rejection,omitempty"`
	Context     string            `json:"context,omitempty"` // Gathered context summary
	RefinedScope string           `json:"refined_scope,omitempty"`
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

BEFORE convening the team, you must gather context. A good brainstorm requires understanding:
- What is the PRODUCT or service?
- Who are the USERS (segments, personas)?
- What is the MARKET context (competitors, trends)?
- What are the KEY METRICS or KPIs?
- What is the NORTH STAR or strategic goal?
- What CONSTRAINTS exist (time, budget, tech)?

YOUR TASK:
1. Use your tools to gather as much context as possible about this problem
2. Search for organizational memory, past decisions, user research, metrics, strategy docs
3. Be thorough - call multiple tools with different queries if needed
4. Summarize what you found (and what you couldn't find)

After gathering, provide a CONTEXT SUMMARY with sections:
- PRODUCT: What we know about the product/service
- USERS: What we know about users
- MARKET: What we know about competitive/market context  
- METRICS: What we know about KPIs/success measures
- STRATEGY: What we know about goals/north star
- GAPS: What critical information is MISSING

Be factual about what you found vs. what's unknown.`

	userPrompt := fmt.Sprintf(`The team has been asked to brainstorm:
"""
%s
"""

Gather context before we proceed. Use your tools.`, scope)

	// Allow generous tool budget for context gathering
	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{PromptWithMedia(userPrompt, seed)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 6, // Allow multiple tool calls
		Tools:             p.cfg.ContextPlan.AllowedToolIDs(),
		ToolPolicy:        p.cfg.ContextPlan.ToolPolicy(),
	})
	if err != nil {
		return "", err
	}

	return resp.Text(), nil
}

// assessReadiness has the moderator decide whether to proceed based on gathered context.
func (p *brainstormIDEO) assessReadiness(ctx context.Context, sess protocol.Session, runner agent.Agent, scope string, contextSummary string) (*ReadinessResult, error) {
	system := `You are the brainstorm moderator deciding whether to convene the session.

Based on the context you gathered, you must make a DECISION:

1. PROCEED - You have rich context. The problem is well-defined with clear product, users, metrics.

2. PROCEED_WITH_ASSUMPTIONS - Context is incomplete but you can make reasonable assumptions.
   You MUST explicitly state your assumptions so the team (and stakeholders) know what you're working with.
   This protects the team - they'll be judged by results, so assumptions must be transparent.

3. REQUEST_INFO - Critical information is missing that you cannot reasonably assume.
   Specify exactly what you need from the requester.
   
4. REJECT - The request is too vague, inappropriate, or impossible to address productively.
   Explain why.

DECISION CRITERIA:
- Can you define WHO the users are (even if assumed)?
- Can you define WHAT success looks like (even if assumed)?
- Is the problem scoped enough to generate actionable ideas?
- Would the team waste time without more context?

Respond with your decision in this exact format:

DECISION: [PROCEED | PROCEED_WITH_ASSUMPTIONS | REQUEST_INFO | REJECT]

If PROCEED_WITH_ASSUMPTIONS, list each assumption:
ASSUMPTION: [Your assumption about product/users/market/metrics]
ASSUMPTION: [Another assumption]
...

If REQUEST_INFO, list what's needed:
MISSING: [What specific information you need]
MISSING: [Another piece of needed information]
...

If REJECT:
REJECTION: [Why this request cannot proceed]

Finally, if proceeding, provide:
REFINED_SCOPE: [A more specific version of the problem statement incorporating your context/assumptions]`

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
	})
	if err != nil {
		return nil, err
	}

	return parseReadinessDecision(resp.Text(), contextSummary), nil
}

// parseReadinessDecision extracts structured decision from moderator's response.
func parseReadinessDecision(text, contextSummary string) *ReadinessResult {
	result := &ReadinessResult{
		Decision: DecisionProceedWithAssumptions, // Default to proceeding with assumptions
		Context:  contextSummary,
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)

		// Parse decision
		if strings.HasPrefix(upper, "DECISION:") {
			decision := strings.TrimSpace(strings.TrimPrefix(line, "DECISION:"))
			decision = strings.TrimSpace(strings.TrimPrefix(decision, "decision:"))
			switch strings.ToUpper(decision) {
			case "PROCEED":
				result.Decision = DecisionProceed
			case "PROCEED_WITH_ASSUMPTIONS":
				result.Decision = DecisionProceedWithAssumptions
			case "REQUEST_INFO":
				result.Decision = DecisionRequestInfo
			case "REJECT":
				result.Decision = DecisionReject
			}
		}

		// Parse assumptions
		if strings.HasPrefix(upper, "ASSUMPTION:") {
			assumption := strings.TrimSpace(strings.TrimPrefix(line, "ASSUMPTION:"))
			assumption = strings.TrimSpace(strings.TrimPrefix(assumption, "assumption:"))
			if assumption != "" {
				result.Assumptions = append(result.Assumptions, assumption)
			}
		}

		// Parse missing info
		if strings.HasPrefix(upper, "MISSING:") {
			missing := strings.TrimSpace(strings.TrimPrefix(line, "MISSING:"))
			missing = strings.TrimSpace(strings.TrimPrefix(missing, "missing:"))
			if missing != "" {
				result.Missing = append(result.Missing, missing)
			}
		}

		// Parse rejection reason
		if strings.HasPrefix(upper, "REJECTION:") {
			result.Rejection = strings.TrimSpace(strings.TrimPrefix(line, "REJECTION:"))
			result.Rejection = strings.TrimSpace(strings.TrimPrefix(result.Rejection, "rejection:"))
		}

		// Parse refined scope
		if strings.HasPrefix(upper, "REFINED_SCOPE:") {
			result.RefinedScope = strings.TrimSpace(strings.TrimPrefix(line, "REFINED_SCOPE:"))
			result.RefinedScope = strings.TrimSpace(strings.TrimPrefix(result.RefinedScope, "refined_scope:"))
		}
	}

	return result
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
