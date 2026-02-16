package ideo

import (
	"context"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/roundtable"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

// InspirationResult contains outputs from the inspiration phase.
type InspirationResult struct {
	// Tensions are problems, frictions, or contradictions observed
	Tensions []string `json:"tensions"`

	// Observations are factual findings from research
	Observations []string `json:"observations"`

	// Constraints are known limitations or requirements
	Constraints []string `json:"constraints"`

	// KeyQuotes are notable statements from research
	KeyQuotes []string `json:"key_quotes,omitempty"`

	// Artifacts created during inspiration
	Artifacts []Artifact `json:"artifacts,omitempty"`

	// Raw thread for debugging
	Thread []agent.Message `json:"-"`
}

// runInspiration executes the inspiration phase.
// Goal: Empathize, observe, gather tensions before jumping to solutions.
func (p *brainstormIDEO) runInspiration(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, seed agent.Message, readiness *ReadinessResult) (*InspirationResult, error) {
	rounds := p.cfg.InspirationRounds
	if rounds <= 0 {
		rounds = 2
	}

	// Kick off with moderator framing (includes assumptions if any)
	if err := p.runInspirationKickoff(ctx, sess, agents, scope, readiness); err != nil {
		return nil, err
	}

	// Run discovery rounds
	rt := roundtable.New(roundtable.WithMaxRounds(rounds))
	rt.Record(seed)

	// Include assumptions as context for the team
	if readiness != nil && len(readiness.Assumptions) > 0 {
		assumptionsMsg := formatAssumptionsForTeam(readiness)
		rt.Record(message.User(assumptionsMsg))
	}

	rt.Record(message.User("Before proposing solutions, investigate this problem. Use tools to find evidence. Surface tensions and observations."))

	turnToolBudget := max(p.cfg.ContextPlan.MaxToolIterations()/max(1, rounds*len(agents)), 2)

	for rt.CurrentRound() < rt.MaxRounds() {
		currentRound := rt.IncrementRound()
		ordered := rotateAgentOrder(agents, currentRound)

		// Select diversity nudge for this round
		nudge := selectNudge(p.cfg.DisciplineNudges, currentRound)
		mentalModel := selectNudge(p.cfg.MentalModelPrompts, currentRound)

		for idx, participant := range ordered {
			// Get full history including tool calls from Session memory
			history, err := sess.History(ctx)
			if err != nil {
				return nil, fmt.Errorf("getting history: %w", err)
			}

			// Extract tool calls from history for the prompt
			toolCalls := extractToolCallsFromHistory(history)

			// Use roundtable thread for recent context (text responses)
			thread := recentThread(rt.Thread(), 10)
			messages := append([]agent.Message(nil), thread...)

			// Inject tool call summary so agent knows what's been queried
			if len(toolCalls) > 0 {
				toolSummary := formatToolCallSummaryFromHistory(toolCalls)
				messages = append(messages, message.User(toolSummary))
			}

			messages = append(messages, message.User(fmt.Sprintf(
				"Your discovery turn (%d/%d). Investigate, then hand off an open question to the next person.",
				idx+1, len(ordered),
			)))

			system := p.buildInspirationPromptWithHistory(participant, scope, currentRound, rounds, nudge, mentalModel, toolCalls)

			resp, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          messages,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: turnToolBudget,
				Tools:             p.cfg.ContextPlan.AllowedToolIDs(),
				ToolPolicy:        p.cfg.ContextPlan.ToolPolicy(),
			})
			if err != nil {
				return nil, fmt.Errorf("inspiration round %d (%s): %w", currentRound, participant.Name, err)
			}

			resp.Name = participant.Name
			rt.Record(resp)
		}
	}

	// Extract structured insights from the thread
	result := p.extractInspirationInsights(rt.Thread())
	result.Thread = rt.Thread()

	return result, nil
}

func (p *brainstormIDEO) runInspirationKickoff(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, readiness *ReadinessResult) error {
	runner := p.selectRunner(sess, agents)

	// Build system prompt - include assumptions if proceeding with them
	var systemBuilder strings.Builder
	systemBuilder.WriteString(`You are the brainstorm moderator beginning the INSPIRATION phase.

Your job is to set up empathetic, evidence-based problem exploration. Write naturally:
1. Frame the problem in plain language—assume no one knows why they're here
2. Emphasize that you want to UNDERSTAND before SOLVING
3. Encourage the team to use their tools to find real evidence
4. Ask them to surface tensions, contradictions, and user pain points
5. Remind them to defer judgment—no solutions yet, just observations

`)

	// If we have assumptions, instruct moderator to share them
	if readiness != nil && len(readiness.Assumptions) > 0 {
		systemBuilder.WriteString(`IMPORTANT: You gathered context earlier and made assumptions to proceed.
You MUST share these assumptions with the team so they understand the working constraints.
Be transparent - the team's recommendations will be judged, so they need to know what's assumed vs. known.

`)
	}

	systemBuilder.WriteString(`Sound curious and direct. No templates or bullet points. Talk to the team like a curious facilitator.`)

	// Build user prompt
	var userBuilder strings.Builder
	userBuilder.WriteString(fmt.Sprintf("Topic: %s\n\n", scope))

	if readiness != nil && readiness.Context != "" {
		userBuilder.WriteString("Context gathered:\n")
		userBuilder.WriteString(readiness.Context)
		userBuilder.WriteString("\n\n")
	}

	if readiness != nil && len(readiness.Assumptions) > 0 {
		userBuilder.WriteString("Working assumptions (share these with the team):\n")
		for i, a := range readiness.Assumptions {
			userBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, a))
		}
		userBuilder.WriteString("\n")
	}

	userBuilder.WriteString("Begin the inspiration phase.")

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(userBuilder.String())},
		SystemMessages:    []agent.Message{message.System(systemBuilder.String())},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	return err
}

// toolCallInfo represents a tool call extracted from history.
type toolCallInfo struct {
	AgentID string
	ToolID  string
	Content string // Truncated content from tool result
}

// extractToolCallsFromHistory extracts tool call information from session history.
func extractToolCallsFromHistory(history []agent.Message) []toolCallInfo {
	var calls []toolCallInfo
	for _, msg := range history {
		if msg.Role == agent.RoleTool {
			content := msg.Text()
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			calls = append(calls, toolCallInfo{
				AgentID: msg.Name, // Tool ID is stored in Name for tool messages
				ToolID:  msg.Name,
				Content: content,
			})
		}
	}
	return calls
}

// formatToolCallSummaryFromHistory creates a message showing what tools have been called.
func formatToolCallSummaryFromHistory(calls []toolCallInfo) string {
	if len(calls) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("📋 TEAM'S TOOL CALLS SO FAR (do NOT repeat these queries):\n")
	for i, call := range calls {
		sb.WriteString(fmt.Sprintf("%d. [%s] was called", i+1, call.ToolID))
		if call.Content != "" {
			content := call.Content
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			sb.WriteString(fmt.Sprintf("\n   → Result: %s", content))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nBuild on these findings. Only call tools if you have a DIFFERENT question.")
	return sb.String()
}

func (p *brainstormIDEO) buildInspirationPromptWithHistory(participant agent.Agent, scope string, round, totalRounds int, nudge, mentalModel string, toolCalls []toolCallInfo) string {
	var sb strings.Builder

	// Include persona if available
	if participant.Profile != nil && strings.TrimSpace(participant.Profile.Prompt) != "" {
		sb.WriteString("YOUR PERSONA:\n")
		sb.WriteString(strings.TrimSpace(participant.Profile.Prompt))
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf(`IDEO BRAINSTORM: INSPIRATION PHASE (Round %d/%d)

PROBLEM SCOPE:
"""
%s
"""

`, round, totalRounds, scope))

	sb.WriteString(`PHASE GOAL: Understand before solving. Empathize with users. Surface tensions.

MINDSET:
- You are an observer, not a problem-solver (yet)
- Look for what's actually happening, not what should happen
- Notice contradictions, workarounds, and pain points
- Defer judgment—no solutions, just observations

`)

	// Add prior tool calls from history - this is CRITICAL for avoiding redundant queries
	if len(toolCalls) > 0 {
		sb.WriteString("🔍 TOOLS ALREADY CALLED (from session history):\n")
		for _, call := range toolCalls {
			sb.WriteString(fmt.Sprintf("- [%s]", call.ToolID))
			if call.Content != "" {
				content := call.Content
				if len(content) > 100 {
					content = content[:100] + "..."
				}
				sb.WriteString(fmt.Sprintf(" → %s", content))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n⚠️ DO NOT repeat these queries. Build on what's been found or ask DIFFERENT questions.\n\n")
	}

	// Add diversity nudge
	if nudge != "" {
		sb.WriteString(fmt.Sprintf("PERSPECTIVE NUDGE: %s\n\n", nudge))
	}

	// Add mental model prompt
	if mentalModel != "" {
		sb.WriteString(fmt.Sprintf("LENS TO APPLY: %s\n\n", mentalModel))
	}

	// Add tools section
	sb.WriteString(formatToolsSection(p.cfg.ContextPlan))
	sb.WriteString("\n\n")

	sb.WriteString(`YOUR WORKFLOW:
1. Review what tools have already been called above—do NOT repeat those calls
2. If you need NEW information, ask a DIFFERENT question
3. If everything relevant is already found, synthesize and add your unique perspective
4. End with an open question for the next person

Respond in your own voice. Be specific and grounded. 3-5 sentences.`)

	return sb.String()
}

// Keep the old function for backward compatibility
func (p *brainstormIDEO) buildInspirationPrompt(participant agent.Agent, scope string, round, totalRounds int, nudge, mentalModel string, findings *SharedFindings) string {
	var sb strings.Builder

	// Include persona if available
	if participant.Profile != nil && strings.TrimSpace(participant.Profile.Prompt) != "" {
		sb.WriteString("YOUR PERSONA:\n")
		sb.WriteString(strings.TrimSpace(participant.Profile.Prompt))
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf(`IDEO BRAINSTORM: INSPIRATION PHASE (Round %d/%d)

PROBLEM SCOPE:
"""
%s
"""

`, round, totalRounds, scope))

	sb.WriteString(`PHASE GOAL: Understand before solving. Empathize with users. Surface tensions.

MINDSET:
- You are an observer, not a problem-solver (yet)
- Look for what's actually happening, not what should happen
- Notice contradictions, workarounds, and pain points
- Defer judgment—no solutions, just observations

`)

	// Add shared findings from prior agents (critical for avoiding redundant tool calls)
	if findings != nil {
		if findingsSection := findings.FormatForPrompt(); findingsSection != "" {
			sb.WriteString(findingsSection)
			sb.WriteString("\n")
		}
	}

	// Add diversity nudge
	if nudge != "" {
		sb.WriteString(fmt.Sprintf("PERSPECTIVE NUDGE: %s\n\n", nudge))
	}

	// Add mental model prompt
	if mentalModel != "" {
		sb.WriteString(fmt.Sprintf("LENS TO APPLY: %s\n\n", mentalModel))
	}

	// Add tools section
	sb.WriteString(formatToolsSection(p.cfg.ContextPlan))
	sb.WriteString("\n\n")

	sb.WriteString(`YOUR WORKFLOW:
1. Review prior findings above—do NOT re-query what others already found
2. If you need NEW information, use tools with a DIFFERENT question
3. Build on others' insights, add your unique perspective
4. When sharing findings, use format: [FINDING: tool_name] your summary
5. End with an open question for the next person

Respond in your own voice. Be specific and grounded. 3-5 sentences.`)

	return sb.String()
}

func (p *brainstormIDEO) extractInspirationInsights(thread []agent.Message) *InspirationResult {
	result := &InspirationResult{
		Tensions:     make([]string, 0),
		Observations: make([]string, 0),
		Constraints:  make([]string, 0),
		KeyQuotes:    make([]string, 0),
		Artifacts:    make([]Artifact, 0),
	}

	// Extract insights from assistant messages
	// This is a simplified extraction - in production you might use LLM-based extraction
	for _, msg := range thread {
		if msg.Role != agent.RoleAssistant || msg.Name == "" {
			continue
		}

		text := strings.TrimSpace(msg.Text())
		if text == "" {
			continue
		}

		// Look for tension/friction indicators
		lower := strings.ToLower(text)
		if strings.Contains(lower, "tension") ||
			strings.Contains(lower, "friction") ||
			strings.Contains(lower, "problem") ||
			strings.Contains(lower, "pain point") ||
			strings.Contains(lower, "struggle") {
			// Add as observation (could be more sophisticated)
			result.Observations = append(result.Observations, truncate(text, 200))
		} else {
			result.Observations = append(result.Observations, truncate(text, 200))
		}
	}

	// Deduplicate and limit
	result.Observations = deduplicateStrings(result.Observations)
	if len(result.Observations) > 10 {
		result.Observations = result.Observations[:10]
	}

	return result
}

// buildInspirationTransfer creates a transfer packet from inspiration results.
func (p *brainstormIDEO) buildInspirationTransfer(result *InspirationResult, scope string) *TransferPacket {
	var summary strings.Builder
	summary.WriteString("## What We Learned in Inspiration\n\n")

	if len(result.Tensions) > 0 {
		summary.WriteString("**Key Tensions:**\n")
		for _, t := range result.Tensions {
			summary.WriteString(fmt.Sprintf("- %s\n", t))
		}
		summary.WriteString("\n")
	}

	if len(result.Observations) > 0 {
		summary.WriteString("**Observations:**\n")
		for _, o := range result.Observations {
			summary.WriteString(fmt.Sprintf("- %s\n", o))
		}
		summary.WriteString("\n")
	}

	if len(result.Constraints) > 0 {
		summary.WriteString("**Constraints:**\n")
		for _, c := range result.Constraints {
			summary.WriteString(fmt.Sprintf("- %s\n", c))
		}
		summary.WriteString("\n")
	}

	// Build prior messages based on transfer strategy
	var priorMessages []agent.Message
	if p.cfg.TransferStrategy == TransferWithHistory || p.cfg.TransferStrategy == TransferFull {
		// Include key messages from the thread
		for _, msg := range result.Thread {
			if msg.Role == agent.RoleAssistant && msg.Name != "" {
				priorMessages = append(priorMessages, msg)
			}
		}
		// Limit to most recent
		if len(priorMessages) > 6 {
			priorMessages = priorMessages[len(priorMessages)-6:]
		}
	}

	return &TransferPacket{
		FromPhase: PhaseInspiration,
		Data: map[string]any{
			"scope":        scope,
			"tensions":     result.Tensions,
			"observations": result.Observations,
			"constraints":  result.Constraints,
			"key_quotes":   result.KeyQuotes,
			"artifacts":    result.Artifacts,
		},
		Summary:       summary.String(),
		PriorMessages: priorMessages,
	}
}
