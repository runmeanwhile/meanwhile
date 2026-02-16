package ideo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/evidencegate"
	"github.com/runmeanwhile/meanwhile/pkg/collab/portfolio"
	"github.com/runmeanwhile/meanwhile/pkg/collab/roundtable"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

// SynthesisResult contains outputs from the synthesis phase.
type SynthesisResult struct {
	// Cards are experiment-ready concept cards
	Cards []evidencegate.Card `json:"cards"`

	// Eligible cards that passed the evidence gate
	Eligible []evidencegate.Card `json:"eligible"`

	// Rejected cards that didn't pass
	Rejected []evidencegate.Card `json:"rejected"`

	// Portfolio is the final allocation of bets
	Portfolio []portfolio.Bet `json:"portfolio"`

	// HumanFeedback captured during synthesis
	HumanFeedback []HumanResponse `json:"human_feedback,omitempty"`

	// Closing is the final summary
	Closing string `json:"closing"`

	// Raw thread for debugging
	Thread []agent.Message `json:"-"`
}

// HumanResponse captures a stakeholder's feedback.
type HumanResponse struct {
	Stakeholder Stakeholder `json:"stakeholder"`
	Question    string      `json:"question"`
	Response    string      `json:"response"`
	Timestamp   string      `json:"timestamp"`
}

// runSynthesis executes the synthesis phase.
// Goal: Converge to experiment-ready portfolio with evidence gates.
func (p *brainstormIDEO) runSynthesis(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, ideationTransfer *TransferPacket) (*SynthesisResult, error) {
	rounds := p.cfg.SynthesisRounds
	if rounds <= 0 {
		rounds = 2
	}

	// Kick off with transition from ideation
	if err := p.runSynthesisKickoff(ctx, sess, agents, scope, ideationTransfer); err != nil {
		return nil, err
	}

	// Run critique rounds
	critiqueThread, err := p.runCritiqueRounds(ctx, sess, agents, scope, ideationTransfer, rounds)
	if err != nil {
		return nil, err
	}

	// Run evidence gate
	cards, eligible, rejected, err := p.runEvidenceGate(ctx, sess, agents, scope, ideationTransfer, critiqueThread)
	if err != nil {
		return nil, err
	}

	// Optional: Human-in-the-loop validation
	var humanFeedback []HumanResponse
	if p.cfg.HumanInLoop && len(p.cfg.Stakeholders) > 0 {
		humanFeedback, err = p.runHumanValidation(ctx, sess, agents, eligible)
		if err != nil {
			// Log but continue - human feedback is optional
			humanFeedback = nil
		}
	}

	// Build portfolio
	finalists := p.selectFinalists(eligible, rejected)
	portfolioBets := portfolio.Build(finalists, p.cfg.FinalistCount)

	// Generate closing summary
	closing, err := p.runClosingSummary(ctx, sess, agents, scope, ideationTransfer, portfolioBets, humanFeedback)
	if err != nil {
		return nil, err
	}

	return &SynthesisResult{
		Cards:         cards,
		Eligible:      eligible,
		Rejected:      rejected,
		Portfolio:     portfolioBets,
		HumanFeedback: humanFeedback,
		Closing:       closing,
		Thread:        critiqueThread,
	}, nil
}

func (p *brainstormIDEO) runSynthesisKickoff(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, transfer *TransferPacket) error {
	runner := p.selectRunner(sess, agents)

	system := `You are the moderator beginning the SYNTHESIS phase.

This is where we get rigorous. Write 2-3 sentences that:
1. Acknowledge the creative concepts generated
2. Explain we're now going to CRITIQUE and BUILD EVIDENCE
3. Set the tone: constructive pressure-testing, not idea-killing
4. Remind them that good ideas should survive scrutiny

Sound focused and professional. The playful part is over.`

	var userPrompt strings.Builder
	userPrompt.WriteString(fmt.Sprintf("Scope: %s\n\n", scope))
	if transfer != nil && transfer.Summary != "" {
		userPrompt.WriteString("From Ideation:\n")
		userPrompt.WriteString(transfer.Summary)
	}
	userPrompt.WriteString("\nBegin synthesis phase.")

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(userPrompt.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	return err
}

func (p *brainstormIDEO) runCritiqueRounds(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, transfer *TransferPacket, rounds int) ([]agent.Message, error) {
	rt := roundtable.New(roundtable.WithMaxRounds(rounds))

	// Seed with ideation context
	if transfer != nil && transfer.Summary != "" {
		rt.Record(message.User(fmt.Sprintf("Concepts to critique:\n%s", transfer.Summary)))
	}
	rt.Record(message.User("Critique phase. Pressure-test concepts constructively."))

	for rt.CurrentRound() < rt.MaxRounds() {
		currentRound := rt.IncrementRound()
		ordered := rotateAgentOrder(agents, currentRound)

		for idx, participant := range ordered {
			thread := recentThread(rt.Thread(), 10)
			messages := append([]agent.Message(nil), thread...)

			target := recentPeer(thread, participant.Name)
			if target == "" {
				target = "a concept from ideation"
			}

			messages = append(messages, message.User(fmt.Sprintf(
				"Your critique turn (%d/%d). Pressure-test %s's idea constructively.",
				idx+1, len(ordered), target,
			)))

			system := p.buildCritiquePrompt(participant, scope, currentRound, rounds, target)

			resp, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          messages,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: 1,
			})
			if err != nil {
				return nil, fmt.Errorf("critique round %d (%s): %w", currentRound, participant.Name, err)
			}

			resp.Name = participant.Name
			rt.Record(resp)
		}
	}

	return rt.Thread(), nil
}

func (p *brainstormIDEO) buildCritiquePrompt(participant agent.Agent, scope string, round, totalRounds int, target string) string {
	var sb strings.Builder

	// Include persona if available
	if participant.Profile != nil && strings.TrimSpace(participant.Profile.Prompt) != "" {
		sb.WriteString("YOUR PERSONA:\n")
		sb.WriteString(strings.TrimSpace(participant.Profile.Prompt))
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf(`IDEO BRAINSTORM: SYNTHESIS PHASE - Critique (Round %d/%d)

PROBLEM SCOPE:
"""
%s
"""

CRITIQUE TARGET: %s

`, round, totalRounds, scope, target))

	sb.WriteString(`CRITIQUE MINDSET:
You are now a constructive skeptic. Your job is to stress-test ideas so the best ones emerge stronger.

GOOD CRITIQUE:
- "What assumption are we making here?"
- "What evidence would change our mind?"
- "What's the weakest link in this approach?"
- "Who would this NOT work for?"

BAD CRITIQUE:
- "I don't like it" (unhelpful)
- "This won't work" (without specifics)
- Attacking the person, not the idea

YOUR TASK:
1. Identify the core assumption behind the concept
2. Name the biggest risk or unknown
3. Suggest what evidence would validate or invalidate it

2-4 sentences. Be direct, be useful.`)

	return sb.String()
}

func (p *brainstormIDEO) runEvidenceGate(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, transfer *TransferPacket, critiqueThread []agent.Message) ([]evidencegate.Card, []evidencegate.Card, []evidencegate.Card, error) {
	runner := p.selectRunner(sess, agents)

	// Gather concept snippets
	var conceptSnippets []string
	if transfer != nil {
		conceptSnippets = append(conceptSnippets, transfer.Summary)
	}
	for _, msg := range critiqueThread {
		if msg.Role == agent.RoleAssistant && msg.Name != "" {
			conceptSnippets = append(conceptSnippets, truncate(msg.Text(), 200))
		}
	}

	system := `EVIDENCE GATE: Convert concepts into experiment cards.

Return JSON only as an array of objects with these keys:
- title: Brief concept name
- concept: What is the idea?
- core_assumption: The critical assumption to test
- cheapest_test: Simplest way to validate
- target_signal: What user behavior would indicate success?
- success_threshold: What counts as success?
- failure_threshold: What counts as failure?
- time_to_learn: How long to run the test?
- risk_level: low/medium/high
- confidence: low/medium/high
- unknowns: What we still don't know

Generate 4-6 cards from the concepts discussed.`

	user := fmt.Sprintf("Scope: %s\n\nConcepts and critique:\n%s\n\nProduce experiment cards.", scope, strings.Join(conceptSnippets, "\n\n"))

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
		Silent:            true,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("evidence gate: %w", err)
	}

	// Parse cards
	cards := parseEvidenceCards(resp.Text())
	if len(cards) == 0 {
		// Fallback: create cards from concepts
		cards = p.fallbackCards(transfer)
	}

	// Filter through evidence gate
	eligible, rejected, _ := evidencegate.EligibleCards(cards, 6)

	return cards, eligible, rejected, nil
}

func (p *brainstormIDEO) runHumanValidation(ctx context.Context, sess protocol.Session, agents []agent.Agent, eligible []evidencegate.Card) ([]HumanResponse, error) {
	if len(p.cfg.Stakeholders) == 0 || len(eligible) == 0 {
		return nil, nil
	}

	// For MVP, we just note that human validation would happen here
	// In production, this would use the ask_human tool integration
	feedback := make([]HumanResponse, 0)

	// Pick the top card to validate
	topCard := eligible[0]
	stakeholder := p.cfg.Stakeholders[0]

	feedback = append(feedback, HumanResponse{
		Stakeholder: stakeholder,
		Question:    fmt.Sprintf("Does this assumption ring true? '%s'", topCard.CoreAssumption),
		Response:    "[Human validation pending - stakeholder: " + stakeholder.Email + "]",
		Timestamp:   "pending",
	})

	return feedback, nil
}

func (p *brainstormIDEO) selectFinalists(eligible, rejected []evidencegate.Card) []evidencegate.Card {
	sortCards := func(cards []evidencegate.Card) {
		sort.SliceStable(cards, func(i, j int) bool {
			si := evidencegate.ScoreCard(cards[i])
			sj := evidencegate.ScoreCard(cards[j])
			if si == sj {
				return cards[i].Title < cards[j].Title
			}
			return si > sj
		})
	}

	elig := append([]evidencegate.Card(nil), eligible...)
	rej := append([]evidencegate.Card(nil), rejected...)
	sortCards(elig)
	sortCards(rej)

	limit := p.cfg.FinalistCount
	if limit <= 0 {
		limit = 3
	}

	out := make([]evidencegate.Card, 0, limit)
	out = append(out, elig...)
	if len(out) < limit {
		out = append(out, rej...)
	}
	if len(out) > limit {
		out = out[:limit]
	}

	return out
}

func (p *brainstormIDEO) runClosingSummary(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, ideationTransfer *TransferPacket, bets []portfolio.Bet, humanFeedback []HumanResponse) (string, error) {
	runner := p.selectRunner(sess, agents)

	var betLines []string
	for _, bet := range bets {
		card := bet.Card
		betLines = append(betLines, fmt.Sprintf("- [%s] %s\n  Test: %s\n  Signal: %s", strings.ToUpper(string(bet.Type)), card.Title, card.CheapestTest, card.TargetSignal))
	}

	system := `IDEO BRAINSTORM: CLOSING SUMMARY

Generate a concise markdown report with these sections:
## Problem Statement
## What We Learned (Inspiration + Reframe)
## How Might We (Top HMWs)
## Concept Portfolio (Safe / Adjacent / Bold bets)
## Key Assumptions to Test
## Recommended First Experiment
## Open Questions

Write for a stakeholder who wasn't in the room. Be concrete and actionable.`

	var user strings.Builder
	user.WriteString(fmt.Sprintf("Scope: %s\n\n", scope))
	if ideationTransfer != nil && ideationTransfer.Summary != "" {
		user.WriteString("From prior phases:\n")
		user.WriteString(ideationTransfer.Summary)
		user.WriteString("\n")
	}
	user.WriteString("Portfolio:\n")
	user.WriteString(strings.Join(betLines, "\n"))
	if len(humanFeedback) > 0 {
		user.WriteString("\n\nHuman feedback:\n")
		for _, fb := range humanFeedback {
			user.WriteString(fmt.Sprintf("- %s (%s): %s\n", fb.Stakeholder.Name, fb.Stakeholder.Role, fb.Response))
		}
	}

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	if err != nil {
		return "", fmt.Errorf("closing summary: %w", err)
	}

	return strings.TrimSpace(resp.Text()), nil
}

func (p *brainstormIDEO) fallbackCards(transfer *TransferPacket) []evidencegate.Card {
	if transfer == nil {
		return nil
	}

	concepts, ok := transfer.Data["concepts"].([]ConceptCard)
	if !ok || len(concepts) == 0 {
		return nil
	}

	cards := make([]evidencegate.Card, 0, len(concepts))
	for _, c := range concepts {
		cards = append(cards, evidencegate.Card{
			Title:            c.Title,
			Concept:          c.Mechanism,
			CoreAssumption:   c.Problem,
			CheapestTest:     "User interview or prototype test",
			TargetSignal:     "User adoption or engagement change",
			SuccessThreshold: "Measurable positive shift",
			FailureThreshold: "No change after pilot",
			TimeToLearn:      "1-2 weeks",
			RiskLevel:        c.Risk,
			Confidence:       "low",
		})
	}

	return cards
}

func parseEvidenceCards(raw string) []evidencegate.Card {
	trimmed := strings.TrimSpace(stripMarkdownCodeFence(raw))
	if trimmed == "" {
		return nil
	}

	cards := make([]evidencegate.Card, 0)
	if err := json.Unmarshal([]byte(trimmed), &cards); err == nil && len(cards) > 0 {
		return cards
	}

	// Try wrapped format
	var wrapped struct {
		Cards []evidencegate.Card `json:"cards"`
	}
	if err := json.Unmarshal([]byte(trimmed), &wrapped); err == nil && len(wrapped.Cards) > 0 {
		return wrapped.Cards
	}

	// Try extracting JSON array
	arrayJSON := extractJSONArray(trimmed)
	if arrayJSON != "" {
		if err := json.Unmarshal([]byte(arrayJSON), &cards); err == nil && len(cards) > 0 {
			return cards
		}
	}

	return nil
}

func stripMarkdownCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	firstNL := strings.Index(trimmed, "\n")
	if firstNL < 0 {
		return trimmed
	}
	inner := strings.TrimSpace(trimmed[firstNL+1:])
	inner = strings.TrimSuffix(inner, "```")
	return strings.TrimSpace(inner)
}

func extractJSONArray(raw string) string {
	start := strings.Index(raw, "[")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}
