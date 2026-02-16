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
func (p *brainstormIDEO) runReframe(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, inspirationTransfer *TransferPacket) (*ReframeResult, error) {
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

	// Kick off with transition from inspiration
	if err := p.runReframeTransition(ctx, sess, agents, scope, inspirationTransfer); err != nil {
		return nil, err
	}

	// Run multiple reframe rounds
	// Each round focuses on different lenses
	lensGroups := [][]string{
		{"operational", "behavioral", "workflow"},
		{"emotional", "trust", "adoption"},
		{"economic", "systemic", "radical"},
	}

	rt := roundtable.New(roundtable.WithMaxRounds(rounds))

	// Seed with inspiration context
	if inspirationTransfer != nil && p.cfg.TransferStrategy != TransferSummaryOnly {
		for _, msg := range inspirationTransfer.PriorMessages {
			rt.Record(msg)
		}
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

			system := p.buildReframePrompt(participant, scope, inspirationTransfer, currentRound, rounds, lensGroup, userVantage)

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
			if err := p.runReframeRoundBridge(ctx, sess, agents, hmwRegistry.Frames(), currentRound, rounds); err != nil {
				return nil, err
			}
		}
	}

	// Select top frames for ideation
	frames := hmwRegistry.Frames()
	selectedFrames := p.selectTopFrames(frames)

	// Final synthesis by moderator
	if err := p.runReframeSynthesis(ctx, sess, agents, selectedFrames); err != nil {
		return nil, err
	}

	return &ReframeResult{
		Frames:         frames,
		SelectedFrames: selectedFrames,
		Thread:         rt.Thread(),
	}, nil
}

func (p *brainstormIDEO) runReframeTransition(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, transfer *TransferPacket) error {
	runner := p.selectRunner(sess, agents)

	system := `You are the moderator transitioning from INSPIRATION to REFRAME phase.

Write 2-3 natural sentences that:
1. Acknowledge what we learned in inspiration (mention specific findings)
2. Explain we're now going to reframe the problem as "How Might We" questions
3. Emphasize that good HMWs open new directions, not close them down
4. Invite radically different angles—operational, emotional, behavioral, economic

Sound like a curious facilitator. No bullet points or templates.`

	var userPrompt strings.Builder
	userPrompt.WriteString(fmt.Sprintf("Scope: %s\n\n", scope))
	if transfer != nil && transfer.Summary != "" {
		userPrompt.WriteString("From Inspiration:\n")
		userPrompt.WriteString(transfer.Summary)
	}
	userPrompt.WriteString("\nTransition to reframe phase.")

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(userPrompt.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	return err
}

func (p *brainstormIDEO) buildReframePrompt(participant agent.Agent, scope string, transfer *TransferPacket, round, totalRounds int, lenses []string, userVantage string) string {
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

func (p *brainstormIDEO) runReframeRoundBridge(ctx context.Context, sess protocol.Session, agents []agent.Agent, frames []reframer.Frame, round, totalRounds int) error {
	runner := p.selectRunner(sess, agents)

	// Summarize frames so far
	var framesSummary strings.Builder
	lensCount := make(map[string]int)
	for _, frame := range frames {
		lensCount[strings.ToLower(frame.Lens)]++
		framesSummary.WriteString(fmt.Sprintf("- [%s] %s\n", frame.Lens, truncate(frame.HMW, 100)))
	}

	// Find underrepresented lenses
	allLenses := []string{"operational", "behavioral", "emotional", "economic", "trust", "workflow", "adoption", "systemic", "radical"}
	var gaps []string
	for _, lens := range allLenses {
		if lensCount[lens] == 0 {
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

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	return err
}

func (p *brainstormIDEO) selectTopFrames(frames []reframer.Frame) []reframer.Frame {
	target := p.cfg.TargetHMWs
	if target <= 0 {
		target = 8
	}

	if len(frames) <= target {
		return frames
	}

	// Prioritize lens diversity
	lensMap := make(map[string][]reframer.Frame)
	for _, frame := range frames {
		lens := strings.ToLower(strings.TrimSpace(frame.Lens))
		if lens == "" {
			lens = "general"
		}
		lensMap[lens] = append(lensMap[lens], frame)
	}

	// Round-robin selection from each lens
	selected := make([]reframer.Frame, 0, target)
	for len(selected) < target {
		added := false
		for lens, lensFrames := range lensMap {
			if len(lensFrames) == 0 {
				continue
			}
			selected = append(selected, lensFrames[0])
			lensMap[lens] = lensFrames[1:]
			added = true
			if len(selected) >= target {
				break
			}
		}
		if !added {
			break
		}
	}

	return selected
}

func (p *brainstormIDEO) runReframeSynthesis(ctx context.Context, sess protocol.Session, agents []agent.Agent, selectedFrames []reframer.Frame) error {
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

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	return err
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
