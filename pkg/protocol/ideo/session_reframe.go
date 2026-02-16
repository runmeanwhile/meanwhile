package ideo

import (
	"context"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/reframer"
	"github.com/runmeanwhile/meanwhile/pkg/collab/roundtable"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

// ReframeResult contains outputs from the reframe phase.
type ReframeResult struct {
	// Frames are the HMW questions generated
	Frames []reframer.Frame `json:"frames"`

	// SelectedFrames are the top frames chosen for ideation
	SelectedFrames []reframer.Frame `json:"selected_frames"`

	// Artifacts created during reframing
	Artifacts []Artifact `json:"artifacts,omitempty"`

	// Raw thread for debugging
	Thread []agent.Message `json:"-"`
}

// runReframe executes the reframe phase.
// Goal: Generate diverse HMW framings across multiple lenses.
func (p *brainstormIDEO) runReframe(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, inspirationTransfer *TransferPacket, stagePlan *StagePlan) (*ReframeResult, error) {
	rounds := p.cfg.ReframeRounds
	if rounds <= 0 {
		rounds = 3
	}

	// Create HMW registry and register tool
	hmwRegistry := reframer.NewRegistry()
	hmwTool, err := hmwRegistry.Tool()
	if err != nil {
		return nil, fmt.Errorf("create hmw tool: %w", err)
	}
	if err := sess.RegisterTool(hmwTool); err != nil {
		return nil, fmt.Errorf("register hmw tool: %w", err)
	}

	lenses := p.cfg.LensCatalog
	if stagePlan != nil && len(stagePlan.Lenses) > 0 {
		lenses = stagePlan.Lenses
	}
	lensGroups := buildRoundLensGroups(lenses, rounds)

	rt := roundtable.New(roundtable.WithMaxRounds(rounds))

	// Seed with inspiration context
	if inspirationTransfer != nil && p.cfg.TransferStrategy != TransferSummaryOnly {
		for _, msg := range inspirationTransfer.PriorMessages {
			rt.Record(msg)
		}
	}

	// Kick off with transition from inspiration
	transition, err := p.runReframeTransition(ctx, sess, agents, scope, inspirationTransfer, stagePlan)
	if err != nil {
		return nil, err
	}
	if transition.Role != "" {
		rt.Record(transition)
	}
	rt.Record(message.User("Reframe phase begins. Generate diverse How-Might-We questions using submit_hmw."))

	for rt.CurrentRound() < rt.MaxRounds() {
		currentRound := rt.IncrementRound()
		ordered := rotateAgentOrder(agents, currentRound)

		// Select lenses for this round
		lensGroup := lensGroups[(currentRound-1)%len(lensGroups)]

		// Diversity nudge based on round
		userVantage := selectNudge(p.cfg.UserVantagePoints, currentRound)

		for idx, participant := range ordered {
			thread := recentThread(rt.Thread(), 8)
			messages := append([]agent.Message(nil), thread...)
			messages = append(messages, message.User(fmt.Sprintf(
				"Your reframe turn (%d/%d). Submit 2 HMW framings using different lenses.",
				idx+1, len(ordered),
			)))

			system := p.buildReframePrompt(participant, scope, inspirationTransfer, currentRound, rounds, lensGroup, userVantage, stagePlan)

			resp, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          messages,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: 3, // Allow 2 HMW submissions + 1 for reasoning
				Tools:             []string{"submit_hmw"},
			})
			if err != nil {
				return nil, fmt.Errorf("reframe round %d (%s): %w", currentRound, participant.Name, err)
			}

			resp.Name = participant.Name
			rt.Record(resp)
		}

		// After each round, moderator synthesizes and nudges for gaps
		if currentRound < rounds {
			bridge, err := p.runReframeRoundBridge(ctx, sess, agents, hmwRegistry.Frames(), currentRound, rounds, lenses)
			if err != nil {
				return nil, err
			}
			if bridge.Role != "" {
				rt.Record(bridge)
			}
		}
	}

	// Select top frames for ideation
	frames := hmwRegistry.Frames()
	selectedFrames, err := p.selectTopFrames(ctx, sess, agents, scope, frames, stagePlan)
	if err != nil {
		return nil, err
	}

	// Final synthesis by moderator
	summaryMsg, err := p.runReframeSynthesis(ctx, sess, agents, selectedFrames)
	if err != nil {
		return nil, err
	}
	if summaryMsg.Role != "" {
		rt.Record(summaryMsg)
	}

	return &ReframeResult{
		Frames:         frames,
		SelectedFrames: selectedFrames,
		Thread:         rt.Thread(),
	}, nil
}

func (p *brainstormIDEO) runReframeTransition(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, transfer *TransferPacket, stagePlan *StagePlan) (agent.Message, error) {
	runner := p.selectRunner(sess, agents)

	system := `You are the moderator transitioning from INSPIRATION to REFRAME phase.

Write 2-3 natural sentences that:
1. Acknowledge what we learned in inspiration (mention specific findings)
2. Explain we're now going to reframe the problem as "How Might We" questions
3. Emphasize that good HMWs open new directions, not close them down
4. Invite complementary angles that fit the current problem context

Sound like a curious facilitator. No bullet points or templates.`

	var userPrompt strings.Builder
	userPrompt.WriteString(fmt.Sprintf("Scope: %s\n\n", scope))
	if transfer != nil && transfer.Summary != "" {
		userPrompt.WriteString("From Inspiration:\n")
		userPrompt.WriteString(transfer.Summary)
	}
	if stagePlan != nil && len(stagePlan.Lenses) > 0 {
		userPrompt.WriteString("\nPreferred lenses for this run:\n")
		for _, lens := range stagePlan.Lenses {
			userPrompt.WriteString("- ")
			userPrompt.WriteString(strings.TrimSpace(lens))
			userPrompt.WriteString("\n")
		}
	}
	userPrompt.WriteString("\nTransition to reframe phase.")

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(userPrompt.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	if err != nil {
		return agent.Message{}, err
	}
	resp.Name = runner.Name
	return resp, nil
}

func (p *brainstormIDEO) buildReframePrompt(participant agent.Agent, scope string, transfer *TransferPacket, round, totalRounds int, lenses []string, userVantage string, stagePlan *StagePlan) string {
	var sb strings.Builder

	// Include persona if available
	if participant.Profile != nil && strings.TrimSpace(participant.Profile.Prompt) != "" {
		sb.WriteString("YOUR PERSONA:\n")
		sb.WriteString(strings.TrimSpace(participant.Profile.Prompt))
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf(`IDEO BRAINSTORM: REFRAME PHASE (Round %d/%d)

PROBLEM SCOPE:
"""
%s
"""

`, round, totalRounds, scope))

	// Include inspiration context
	if transfer != nil && transfer.Summary != "" {
		sb.WriteString("WHAT WE LEARNED:\n")
		sb.WriteString(transfer.Summary)
		sb.WriteString("\n")
	}

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
		sb.WriteString("KEY QUESTIONS TO ADDRESS:\n")
		for _, question := range stagePlan.Questions {
			sb.WriteString("- ")
			sb.WriteString(question)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`PHASE GOAL: Generate diverse "How Might We" questions that open new design directions.

GOOD HMW QUESTIONS:
- Are specific enough to guide ideation, broad enough to allow creativity
- Are free of embedded solutions or assumptions
- Open new directions the team hasn't considered
- Challenge the original framing of the problem

BAD HMW QUESTIONS:
- "How might we build a better X" (too vague)
- "How might we add feature Y" (embedded solution)
- "How might we make users like it more" (unmeasurable)

`)

	// Add lens guidance
	if len(lenses) > 0 {
		sb.WriteString(fmt.Sprintf("LENSES FOR THIS ROUND: %s\n", strings.Join(lenses, ", ")))
		sb.WriteString("Try to frame HMWs through at least one of these perspectives.\n\n")
	}

	// Add user vantage point
	if userVantage != "" {
		sb.WriteString(fmt.Sprintf("USER TO CONSIDER: %s\n\n", userVantage))
	}

	sb.WriteString(`YOUR TASK:
1. Think about the problem from your unique perspective
2. Generate 2 distinct HMW framings using different lenses
3. Call submit_hmw for each one (include the lens and brief rationale)

Don't just restate the problem—reframe it to open new possibilities.`)

	return sb.String()
}

func (p *brainstormIDEO) runReframeRoundBridge(ctx context.Context, sess protocol.Session, agents []agent.Agent, frames []reframer.Frame, round, totalRounds int, selectedLenses []string) (agent.Message, error) {
	runner := p.selectRunner(sess, agents)

	// Summarize frames so far
	var framesSummary strings.Builder
	lensCount := make(map[string]int)
	for _, frame := range frames {
		lensCount[strings.ToLower(frame.Lens)]++
		framesSummary.WriteString(fmt.Sprintf("- [%s] %s\n", frame.Lens, truncate(frame.HMW, 100)))
	}

	// Find underrepresented lenses from the semantic stage plan/config selection.
	allLenses := deduplicateStrings(selectedLenses)
	if len(allLenses) == 0 {
		allLenses = defaultLensCatalog()
	}
	var gaps []string
	for _, lens := range allLenses {
		if lensCount[strings.ToLower(strings.TrimSpace(lens))] == 0 {
			gaps = append(gaps, lens)
		}
	}

	system := `You are the moderator between reframe rounds.

Write 2-3 sentences that:
1. Acknowledge the HMWs generated so far (be specific, name patterns)
2. Identify which lenses are missing or underrepresented
3. Invite the team to fill the gaps in the next round

Be direct and constructive. No lists.`

	var user strings.Builder
	user.WriteString(fmt.Sprintf("Round %d/%d complete.\n\n", round, totalRounds))
	user.WriteString("HMWs so far:\n")
	user.WriteString(framesSummary.String())
	if len(gaps) > 0 {
		user.WriteString(fmt.Sprintf("\nUnderrepresented lenses: %s", strings.Join(gaps, ", ")))
	}

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	if err != nil {
		return agent.Message{}, err
	}
	resp.Name = runner.Name
	return resp, nil
}

func (p *brainstormIDEO) selectTopFrames(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, frames []reframer.Frame, stagePlan *StagePlan) ([]reframer.Frame, error) {
	target := p.cfg.TargetHMWs
	if target <= 0 {
		target = 8
	}

	if len(frames) <= target {
		return frames, nil
	}

	runner := p.selectRunner(sess, agents)
	type frameSelection struct {
		Indices []int `json:"indices" description:"Zero-based indexes of the strongest HMW frames to carry forward"`
	}

	var frameList strings.Builder
	for idx, frame := range frames {
		frameList.WriteString(fmt.Sprintf("%d) [%s] %s", idx, frame.Lens, frame.HMW))
		if strings.TrimSpace(frame.Rationale) != "" {
			frameList.WriteString(fmt.Sprintf(" -- %s", strings.TrimSpace(frame.Rationale)))
		}
		frameList.WriteString("\n")
	}

	var nonNegotiables strings.Builder
	if stagePlan != nil {
		for _, item := range stagePlan.NonNegotiables {
			if strings.TrimSpace(item) == "" {
				continue
			}
			nonNegotiables.WriteString("- ")
			nonNegotiables.WriteString(strings.TrimSpace(item))
			nonNegotiables.WriteString("\n")
		}
	}

	system := `Select the strongest HMW frames for ideation.

Prioritize frames that:
- address the scope and non-negotiables directly,
- open distinct solution directions,
- are specific enough to drive concrete concepts.

Return only structured output matching the schema.`

	var user strings.Builder
	user.WriteString(fmt.Sprintf("Scope:\n%s\n\n", scope))
	if nonNegotiables.Len() > 0 {
		user.WriteString("Non-negotiables:\n")
		user.WriteString(nonNegotiables.String())
		user.WriteString("\n")
	}
	user.WriteString(fmt.Sprintf("Select %d frames from the list below:\n", target))
	user.WriteString(frameList.String())

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
		OutputSchema:      frameSelection{},
		Silent:            true,
	})
	if err != nil {
		return nil, err
	}

	selection, err := parseStructuredOutput[frameSelection](resp)
	if err != nil {
		return nil, err
	}

	selected := make([]reframer.Frame, 0, target)
	seen := make(map[int]struct{})
	for _, idx := range selection.Indices {
		if idx < 0 || idx >= len(frames) {
			continue
		}
		if _, exists := seen[idx]; exists {
			continue
		}
		seen[idx] = struct{}{}
		selected = append(selected, frames[idx])
		if len(selected) >= target {
			break
		}
	}
	if len(selected) == 0 {
		selected = append(selected, frames[:min(target, len(frames))]...)
	}
	return selected, nil
}

func (p *brainstormIDEO) runReframeSynthesis(ctx context.Context, sess protocol.Session, agents []agent.Agent, selectedFrames []reframer.Frame) (agent.Message, error) {
	runner := p.selectRunner(sess, agents)

	var framesList strings.Builder
	for _, frame := range selectedFrames {
		framesList.WriteString(fmt.Sprintf("- [%s] %s\n", frame.Lens, frame.HMW))
	}

	system := `You are the moderator concluding the REFRAME phase.

Write 2-3 sentences that:
1. Name the most promising HMW directions (be specific)
2. Note what shifted in our understanding of the problem
3. Build anticipation for ideation—invite wild, creative concepts

Sound energized and forward-looking. No bullet points.`

	user := fmt.Sprintf("Selected HMWs for ideation:\n%s\nSynthesize and bridge to ideation.", framesList.String())

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	if err != nil {
		return agent.Message{}, err
	}
	resp.Name = runner.Name
	return resp, nil
}

// buildReframeTransfer creates a transfer packet from reframe results.
func (p *brainstormIDEO) buildReframeTransfer(result *ReframeResult, inspirationTransfer *TransferPacket, scope string) *TransferPacket {
	var summary strings.Builder
	summary.WriteString("## Reframed Problem (How Might We)\n\n")

	for _, frame := range result.SelectedFrames {
		summary.WriteString(fmt.Sprintf("- **[%s]** %s\n", frame.Lens, frame.HMW))
		if frame.Rationale != "" {
			summary.WriteString(fmt.Sprintf("  _Rationale: %s_\n", frame.Rationale))
		}
	}
	summary.WriteString("\n")

	// Include key tensions from inspiration
	if inspirationTransfer != nil {
		if tensions, ok := inspirationTransfer.Data["tensions"].([]string); ok && len(tensions) > 0 {
			summary.WriteString("**Key Tensions to Address:**\n")
			for _, t := range tensions[:min(3, len(tensions))] {
				summary.WriteString(fmt.Sprintf("- %s\n", t))
			}
			summary.WriteString("\n")
		}
	}

	// Build prior messages based on transfer strategy
	var priorMessages []agent.Message
	if p.cfg.TransferStrategy == TransferWithHistory || p.cfg.TransferStrategy == TransferFull {
		for _, msg := range result.Thread {
			if msg.Role == agent.RoleAssistant && msg.Name != "" {
				priorMessages = append(priorMessages, msg)
			}
		}
		if len(priorMessages) > 4 {
			priorMessages = priorMessages[len(priorMessages)-4:]
		}
	}

	return &TransferPacket{
		FromPhase: PhaseReframe,
		Data: map[string]any{
			"scope":           scope,
			"frames":          result.Frames,
			"selected_frames": result.SelectedFrames,
			"artifacts":       result.Artifacts,
		},
		Summary:       summary.String(),
		PriorMessages: priorMessages,
	}
}
