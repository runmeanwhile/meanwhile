package ideo

import (
	"context"
	"encoding/json"
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
func (p *brainstormIDEO) runInspiration(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, seed agent.Message, readiness *ReadinessResult, stagePlan *StagePlan) (*InspirationResult, error) {
	rounds := p.cfg.InspirationRounds
	if rounds <= 0 {
		rounds = 2
	}

	// Run discovery rounds
	rt := roundtable.New(roundtable.WithMaxRounds(rounds))
	rt.Record(seed)

	// Kick off with moderator framing (includes assumptions if any)
	kickoff, err := p.runInspirationKickoff(ctx, sess, agents, scope, readiness)
	if err != nil {
		return nil, err
	}
	if kickoff.Role != "" {
		rt.Record(kickoff)
	}

	// Include assumptions as context for the team
	if readiness != nil && len(readiness.Assumptions) > 0 {
		assumptionsMsg := formatAssumptionsForTeam(readiness)
		rt.Record(message.User(assumptionsMsg))
	}

	rt.Record(message.User("Before proposing solutions, investigate this problem. Use tools to find evidence. Surface tensions and observations."))

	turnToolBudget := max(p.cfg.ContextPlan.MaxToolIterations()/max(1, rounds*len(agents)), 2)
	activeTools := p.cfg.ContextPlan.AllowedToolIDs()
	if stagePlan != nil && len(stagePlan.ToolIDs) > 0 {
		activeTools = stagePlan.ToolIDs
	}

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

			system := p.buildInspirationPromptWithHistory(participant, scope, currentRound, rounds, nudge, mentalModel, toolCalls, stagePlan)

			resp, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          messages,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: turnToolBudget,
				Tools:             activeTools,
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
	result, err := p.summarizeInspiration(ctx, sess, agents, scope, rt.Thread(), stagePlan)
	if err != nil {
		return nil, err
	}
	result.Thread = rt.Thread()

	return result, nil
}

func (p *brainstormIDEO) runInspirationKickoff(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, readiness *ReadinessResult) (agent.Message, error) {
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

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(userBuilder.String())},
		SystemMessages:    []agent.Message{message.System(systemBuilder.String())},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	if err != nil {
		return agent.Message{}, err
	}
	resp.Name = runner.Name
	return resp, nil
}

// toolCallInfo represents a tool call extracted from history.
type toolCallInfo struct {
	ToolID     string
	Query      string
	Findings   []string
	SourceRefs []string
}

// extractToolCallsFromHistory extracts tool call information from session history.
func extractToolCallsFromHistory(history []agent.Message) []toolCallInfo {
	var calls []toolCallInfo
	for _, msg := range history {
		if msg.Role == agent.RoleTool {
			info := toolCallInfo{
				ToolID: strings.TrimSpace(msg.Name),
			}

			info.Query = extractToolQuery(msg.Metadata)

			var recall struct {
				Hits []struct {
					Source  string  `json:"source"`
					Summary string  `json:"summary"`
					Detail  string  `json:"detail,omitempty"`
					Score   float64 `json:"score,omitempty"`
				} `json:"hits"`
				Notes string `json:"notes,omitempty"`
			}
			if parsed, ok := decodeMessageJSONPart[struct {
				Hits []struct {
					Source  string  `json:"source"`
					Summary string  `json:"summary"`
					Detail  string  `json:"detail,omitempty"`
					Score   float64 `json:"score,omitempty"`
				} `json:"hits"`
				Notes string `json:"notes,omitempty"`
			}](msg); ok {
				recall = parsed
			}

			if len(recall.Hits) > 0 {
				for _, hit := range recall.Hits {
					if strings.TrimSpace(hit.Summary) == "" && strings.TrimSpace(hit.Source) == "" {
						continue
					}
					finding := strings.TrimSpace(hit.Summary)
					if finding == "" {
						finding = strings.TrimSpace(hit.Source)
					}
					if detail := strings.TrimSpace(hit.Detail); detail != "" {
						finding = fmt.Sprintf("%s | %s", finding, truncate(detail, 220))
					}
					if hit.Score > 0 {
						finding = fmt.Sprintf("%s (score %.2f)", finding, hit.Score)
					}
					info.Findings = append(info.Findings, truncate(finding, 160))
					if src := strings.TrimSpace(hit.Source); src != "" {
						info.SourceRefs = append(info.SourceRefs, src)
					}
				}
			} else {
				content := strings.TrimSpace(agent.TextFromParts(msg.Parts))
				if content == "" {
					content = strings.TrimSpace(msg.Text())
				}
				if content != "" {
					info.Findings = append(info.Findings, truncate(content, 180))
				}
			}

			info.SourceRefs = deduplicateStrings(info.SourceRefs)
			calls = append(calls, info)
		}
	}
	return calls
}

func extractToolQuery(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	if q, ok := metadata["query"].(string); ok {
		return strings.TrimSpace(q)
	}
	rawArgs, ok := metadata["arguments"]
	if !ok {
		return ""
	}

	var payload []byte
	switch value := rawArgs.(type) {
	case json.RawMessage:
		payload = append([]byte(nil), value...)
	case []byte:
		payload = append([]byte(nil), value...)
	case string:
		payload = []byte(strings.TrimSpace(value))
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		payload = encoded
	}

	var args map[string]any
	if err := json.Unmarshal(payload, &args); err != nil {
		return ""
	}
	return extractQueryFromMap(args)
}

func extractQueryFromMap(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	keys := []string{"query", "q", "question", "search", "term", "text", "prompt"}
	for _, key := range keys {
		value, ok := data[key]
		if !ok {
			continue
		}
		asText, ok := value.(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(asText); trimmed != "" {
			return trimmed
		}
	}
	for _, key := range []string{"input", "args", "parameters"} {
		nested, ok := data[key].(map[string]any)
		if !ok {
			continue
		}
		if query := extractQueryFromMap(nested); query != "" {
			return query
		}
	}
	return ""
}

// formatToolCallSummaryFromHistory creates a message showing what tools have been called.
func formatToolCallSummaryFromHistory(calls []toolCallInfo) string {
	if len(calls) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("📋 TEAM'S TOOL CALLS SO FAR (do NOT repeat these queries):\n")
	for i, call := range calls {
		toolID := call.ToolID
		if toolID == "" {
			toolID = "tool"
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] was called", i+1, toolID))
		if call.Query != "" {
			sb.WriteString(fmt.Sprintf("\n   → Query: %s", truncate(call.Query, 120)))
		}
		for idx, finding := range call.Findings {
			if idx >= 2 {
				break
			}
			sb.WriteString(fmt.Sprintf("\n   → Finding: %s", truncate(finding, 150)))
		}
		if len(call.SourceRefs) > 0 {
			sb.WriteString(fmt.Sprintf("\n   → Sources: %s", strings.Join(call.SourceRefs[:min(2, len(call.SourceRefs))], ", ")))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nBuild on these findings. Only call tools if you have a DIFFERENT question.")
	return sb.String()
}

func (p *brainstormIDEO) buildInspirationPromptWithHistory(participant agent.Agent, scope string, round, totalRounds int, nudge, mentalModel string, toolCalls []toolCallInfo, stagePlan *StagePlan) string {
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

	if stagePlan != nil && len(stagePlan.NonNegotiables) > 0 {
		sb.WriteString("NON-NEGOTIABLE OUTCOMES:\n")
		for _, item := range stagePlan.NonNegotiables {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	if stagePlan != nil && len(stagePlan.Questions) > 0 {
		sb.WriteString("KEY QUESTIONS TO ANSWER:\n")
		for _, question := range stagePlan.Questions {
			sb.WriteString("- ")
			sb.WriteString(question)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Add prior tool calls from history - this is CRITICAL for avoiding redundant queries
	if len(toolCalls) > 0 {
		sb.WriteString("🔍 TOOLS ALREADY CALLED (from session history):\n")
		for _, call := range toolCalls {
			toolID := call.ToolID
			if toolID == "" {
				toolID = "tool"
			}
			sb.WriteString(fmt.Sprintf("- [%s]", toolID))
			if len(call.Findings) > 0 {
				sb.WriteString(fmt.Sprintf(" → %s", truncate(call.Findings[0], 120)))
			}
			if len(call.SourceRefs) > 0 {
				sb.WriteString(fmt.Sprintf(" [source: %s]", call.SourceRefs[0]))
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
	if stagePlan != nil && len(stagePlan.ToolIDs) > 0 {
		sb.WriteString("PRIORITIZED TOOLS FOR THIS SESSION:\n")
		for _, toolID := range stagePlan.ToolIDs {
			sb.WriteString("- ")
			sb.WriteString(toolID)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString(formatToolsSection(p.cfg.ContextPlan))
	sb.WriteString("\n\n")

	sb.WriteString(`YOUR WORKFLOW:
1. Review what tools have already been called above—do NOT repeat those calls
2. If you need NEW information, ask a DIFFERENT question
3. If everything relevant is already found, synthesize and add your unique perspective
4. End with an open question for the next person

Respond in your own voice. Be specific and grounded. 3-5 sentences.`)

	if p.cfg.ContextPlan.RequireCitation {
		sb.WriteString("\n\nEvidence discipline: cite concrete sources in-line like [wiki/product/activation-metrics.md].")
	}

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

func (p *brainstormIDEO) summarizeInspiration(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, thread []agent.Message, stagePlan *StagePlan) (*InspirationResult, error) {
	runner := p.selectRunner(sess, agents)

	history, err := sess.History(ctx)
	if err != nil {
		return nil, err
	}
	toolCalls := extractToolCallsFromHistory(history)

	var snippets []string
	for _, msg := range recentThread(thread, 18) {
		text := strings.TrimSpace(agent.TextFromParts(msg.Parts))
		if text == "" {
			text = strings.TrimSpace(msg.Text())
		}
		if text == "" {
			continue
		}
		prefix := strings.TrimSpace(string(msg.Role))
		if msg.Name != "" {
			prefix = msg.Name
		}
		snippets = append(snippets, fmt.Sprintf("[%s] %s", prefix, truncate(text, 220)))
	}

	type inspirationSynthesis struct {
		Tensions     []string `json:"tensions" description:"Most decision-relevant tensions/frictions discovered"`
		Observations []string `json:"observations" description:"Factual observations grounded in evidence"`
		Constraints  []string `json:"constraints" description:"Known constraints that should shape solution design"`
		KeyQuotes    []string `json:"key_quotes,omitempty" description:"Optional direct quotes that reveal user reality"`
	}

	system := `You are synthesizing the INSPIRATION phase.

Goal:
- Extract evidence-grounded tensions, observations, and constraints.
- Prioritize concrete findings that change strategy decisions.
- Keep claims specific and non-generic.

Return only structured output matching the schema.`

	var user strings.Builder
	user.WriteString(fmt.Sprintf("Scope:\n%s\n\n", scope))
	if stagePlan != nil && len(stagePlan.NonNegotiables) > 0 {
		user.WriteString("Non-negotiables:\n")
		for _, item := range stagePlan.NonNegotiables {
			user.WriteString("- ")
			user.WriteString(item)
			user.WriteString("\n")
		}
		user.WriteString("\n")
	}
	if len(toolCalls) > 0 {
		user.WriteString("Tool evidence summary:\n")
		user.WriteString(formatToolCallSummaryFromHistory(toolCalls))
		user.WriteString("\n\n")
	}
	user.WriteString("Discussion snippets:\n")
	user.WriteString(strings.Join(snippets, "\n"))

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
		OutputSchema:      inspirationSynthesis{},
		Silent:            true,
	})
	if err != nil {
		return nil, err
	}

	synth, err := parseStructuredOutput[inspirationSynthesis](resp)
	if err != nil {
		return nil, err
	}

	result := &InspirationResult{
		Tensions:     deduplicateStrings(synth.Tensions),
		Observations: deduplicateStrings(synth.Observations),
		Constraints:  deduplicateStrings(synth.Constraints),
		KeyQuotes:    deduplicateStrings(synth.KeyQuotes),
		Artifacts:    make([]Artifact, 0),
	}

	if len(result.Observations) > 10 {
		result.Observations = result.Observations[:10]
	}
	if len(result.Tensions) > 10 {
		result.Tensions = result.Tensions[:10]
	}
	if len(result.Constraints) > 8 {
		result.Constraints = result.Constraints[:8]
	}
	if len(result.KeyQuotes) > 6 {
		result.KeyQuotes = result.KeyQuotes[:6]
	}

	return result, nil
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
