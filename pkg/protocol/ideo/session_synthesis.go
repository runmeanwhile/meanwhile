package ideo

import (
	"context"
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
func (p *brainstormIDEO) runSynthesis(
	ctx context.Context,
	sess protocol.Session,
	agents []agent.Agent,
	scope string,
	readiness *ReadinessResult,
	inspiration *InspirationResult,
	reframe *ReframeResult,
	ideation *IdeationResult,
	ideationTransfer *TransferPacket,
	stagePlan *StagePlan,
) (*SynthesisResult, error) {
	rounds := p.cfg.SynthesisRounds
	if rounds <= 0 {
		rounds = 2
	}

	// Kick off with transition from ideation
	kickoff, err := p.runSynthesisKickoff(ctx, sess, agents, scope, ideationTransfer)
	if err != nil {
		return nil, err
	}

	// Run critique rounds
	critiqueThread, err := p.runCritiqueRounds(ctx, sess, agents, scope, ideationTransfer, rounds, kickoff)
	if err != nil {
		return nil, err
	}

	// Run evidence gate
	cards, eligible, rejected, err := p.runEvidenceGate(ctx, sess, agents, scope, ideation, ideationTransfer, critiqueThread, stagePlan)
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
	closing, err := p.runClosingSummary(ctx, sess, agents, closingSummaryInput{
		Scope:         scope,
		Readiness:     readiness,
		Inspiration:   inspiration,
		Reframe:       reframe,
		Ideation:      ideation,
		Transfer:      ideationTransfer,
		Cards:         cards,
		Eligible:      eligible,
		Rejected:      rejected,
		Portfolio:     portfolioBets,
		HumanFeedback: humanFeedback,
		StagePlan:     stagePlan,
	})
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

func (p *brainstormIDEO) runSynthesisKickoff(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, transfer *TransferPacket) (agent.Message, error) {
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

func (p *brainstormIDEO) runCritiqueRounds(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, transfer *TransferPacket, rounds int, kickoff agent.Message) ([]agent.Message, error) {
	rt := roundtable.New(roundtable.WithMaxRounds(rounds))

	// Seed with ideation context
	if transfer != nil && transfer.Summary != "" {
		rt.Record(message.User(fmt.Sprintf("Concepts to critique:\n%s", transfer.Summary)))
	}
	if kickoff.Role != "" {
		rt.Record(kickoff)
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

func (p *brainstormIDEO) runEvidenceGate(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, ideation *IdeationResult, transfer *TransferPacket, critiqueThread []agent.Message, stagePlan *StagePlan) ([]evidencegate.Card, []evidencegate.Card, []evidencegate.Card, error) {
	runner := p.selectRunner(sess, agents)

	concepts := make([]ConceptCard, 0)
	if ideation != nil && len(ideation.Concepts) > 0 {
		concepts = append(concepts, ideation.Concepts...)
	} else if transfer != nil {
		if typed, ok := transfer.Data["concepts"].([]ConceptCard); ok {
			concepts = append(concepts, typed...)
		}
	}

	var conceptSnippets []string
	for _, concept := range concepts {
		conceptSnippets = append(conceptSnippets, fmt.Sprintf(
			"- %s\n  Problem: %s\n  Mechanism: %s\n  Value: %s\n  Risk: %s",
			concept.Title,
			concept.Problem,
			concept.Mechanism,
			concept.Value,
			concept.Risk,
		))
	}
	if len(conceptSnippets) == 0 && transfer != nil && strings.TrimSpace(transfer.Summary) != "" {
		conceptSnippets = append(conceptSnippets, transfer.Summary)
	}

	critiqueSnippets := make([]string, 0, len(critiqueThread))
	for _, msg := range critiqueThread {
		if msg.Role == agent.RoleAssistant && msg.Name != "" {
			text := strings.TrimSpace(agent.TextFromParts(msg.Parts))
			if text == "" {
				text = strings.TrimSpace(msg.Text())
			}
			if text == "" {
				continue
			}
			critiqueSnippets = append(critiqueSnippets, fmt.Sprintf("[%s] %s", msg.Name, truncate(text, 220)))
		}
	}

	type evidenceGateOutput struct {
		Cards []evidencegate.Card `json:"cards" description:"Experiment-ready cards with concrete assumptions, tests, and measurable signals"`
	}

	system := `EVIDENCE GATE: Convert concepts into experiment cards.

Rules:
- Each card must include a concrete, falsifiable core assumption.
- Cheapest test must be executable with minimal cost/time.
- Target/success/failure signals must be measurable.
- baseline_metric, target_metric, expected_delta, segment, and time_to_impact are required.
- Use risk_level as low, medium, or high.
- Include evidence_refs with concrete source references supporting the card.
- Avoid generic placeholders and duplicate cards.

Return only structured output matching the schema.`

	var user strings.Builder
	user.WriteString(fmt.Sprintf("Scope: %s\n\n", scope))
	if stagePlan != nil && len(stagePlan.NonNegotiables) > 0 {
		user.WriteString("Non-negotiables:\n")
		for _, item := range stagePlan.NonNegotiables {
			user.WriteString("- ")
			user.WriteString(item)
			user.WriteString("\n")
		}
		user.WriteString("\n")
	}
	user.WriteString("Concept candidates:\n")
	user.WriteString(strings.Join(conceptSnippets, "\n\n"))
	if len(critiqueSnippets) > 0 {
		user.WriteString("\n\nCritique evidence:\n")
		user.WriteString(strings.Join(critiqueSnippets[:min(18, len(critiqueSnippets))], "\n"))
	}
	user.WriteString("\n\nGenerate 4-6 experiment cards.")

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
		OutputSchema:      evidenceGateOutput{},
		Silent:            true,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("evidence gate: %w", err)
	}

	out, err := parseStructuredOutput[evidenceGateOutput](resp)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("evidence gate parse: %w", err)
	}
	cards := deduplicateEvidenceCards(out.Cards)
	if len(cards) == 0 {
		return nil, nil, nil, fmt.Errorf("evidence gate produced no cards")
	}

	// Filter through evidence gate
	eligible, rejected, _ := evidencegate.EligibleCards(cards, 6)
	eligible, qualityRejected := filterMetricAlignedCards(eligible)
	rejected = append(rejected, qualityRejected...)

	return cards, eligible, rejected, nil
}

func (p *brainstormIDEO) runHumanValidation(ctx context.Context, sess protocol.Session, agents []agent.Agent, eligible []evidencegate.Card) ([]HumanResponse, error) {
	_ = ctx
	_ = sess
	_ = agents
	_ = eligible
	return nil, nil
}

func deduplicateEvidenceCards(cards []evidencegate.Card) []evidencegate.Card {
	seen := make(map[string]struct{})
	cleaned := make([]evidencegate.Card, 0, len(cards))
	for _, card := range cards {
		card.Title = strings.TrimSpace(card.Title)
		card.Concept = strings.TrimSpace(card.Concept)
		card.CoreAssumption = strings.TrimSpace(card.CoreAssumption)
		card.CheapestTest = strings.TrimSpace(card.CheapestTest)
		card.TargetSignal = strings.TrimSpace(card.TargetSignal)
		card.SuccessThreshold = strings.TrimSpace(card.SuccessThreshold)
		card.FailureThreshold = strings.TrimSpace(card.FailureThreshold)
		card.TimeToLearn = strings.TrimSpace(card.TimeToLearn)
		card.RiskLevel = strings.ToLower(strings.TrimSpace(card.RiskLevel))
		card.Confidence = strings.ToLower(strings.TrimSpace(card.Confidence))
		card.Unknowns = strings.TrimSpace(card.Unknowns)
		card.EvidenceRefs = strings.TrimSpace(card.EvidenceRefs)
		card.BaselineMetric = strings.TrimSpace(card.BaselineMetric)
		card.TargetMetric = strings.TrimSpace(card.TargetMetric)
		card.ExpectedDelta = strings.TrimSpace(card.ExpectedDelta)
		card.Segment = strings.TrimSpace(card.Segment)
		card.TimeToImpact = strings.TrimSpace(card.TimeToImpact)

		if card.Title == "" || card.Concept == "" {
			continue
		}
		key := strings.ToLower(card.Title + "|" + card.Concept)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, card)
	}
	return cleaned
}

func filterMetricAlignedCards(cards []evidencegate.Card) (eligible []evidencegate.Card, rejected []evidencegate.Card) {
	eligible = make([]evidencegate.Card, 0, len(cards))
	rejected = make([]evidencegate.Card, 0)
	for _, card := range cards {
		if strings.TrimSpace(card.EvidenceRefs) == "" ||
			strings.TrimSpace(card.BaselineMetric) == "" ||
			strings.TrimSpace(card.TargetMetric) == "" ||
			strings.TrimSpace(card.ExpectedDelta) == "" ||
			strings.TrimSpace(card.Segment) == "" ||
			strings.TrimSpace(card.TimeToImpact) == "" {
			rejected = append(rejected, card)
			continue
		}
		eligible = append(eligible, card)
	}
	return eligible, rejected
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

type closingSummaryInput struct {
	Scope         string
	Readiness     *ReadinessResult
	Inspiration   *InspirationResult
	Reframe       *ReframeResult
	Ideation      *IdeationResult
	StagePlan     *StagePlan
	Transfer      *TransferPacket
	Cards         []evidencegate.Card
	Eligible      []evidencegate.Card
	Rejected      []evidencegate.Card
	Portfolio     []portfolio.Bet
	HumanFeedback []HumanResponse
}

type closingSummaryOutput struct {
	ProblemStatement           string   `json:"problem_statement"`
	WhatWeLearned              []string `json:"what_we_learned"`
	TopHMWs                    []string `json:"top_hmws"`
	SafeBets                   []string `json:"safe_bets"`
	AdjacentBets               []string `json:"adjacent_bets"`
	BoldBets                   []string `json:"bold_bets"`
	KeyAssumptionsToTest       []string `json:"key_assumptions_to_test"`
	RecommendedFirstExperiment string   `json:"recommended_first_experiment"`
	OpenQuestions              []string `json:"open_questions"`
}

func (p *brainstormIDEO) runClosingSummary(ctx context.Context, sess protocol.Session, agents []agent.Agent, input closingSummaryInput) (string, error) {
	runner := p.selectRunner(sess, agents)

	var betLines []string
	var safeBets []string
	var adjacentBets []string
	var boldBets []string
	for _, bet := range input.Portfolio {
		card := bet.Card
		line := fmt.Sprintf("[%s] %s — Test: %s; Signal: %s", strings.ToUpper(string(bet.Type)), card.Title, card.CheapestTest, card.TargetSignal)
		betLines = append(betLines, line)
		switch bet.Type {
		case portfolio.BetSafe:
			safeBets = append(safeBets, line)
		case portfolio.BetAdjacent:
			adjacentBets = append(adjacentBets, line)
		case portfolio.BetBold:
			boldBets = append(boldBets, line)
		}
	}

	system := `IDEO BRAINSTORM: CLOSING SUMMARY

Generate a concise stakeholder-ready recap.
Use only provided context.
Keep claims concrete and decision-relevant.

Return only structured output matching the schema.`

	var user strings.Builder
	user.WriteString(fmt.Sprintf("Scope: %s\n\n", input.Scope))
	if input.Readiness != nil {
		user.WriteString("Readiness context:\n")
		user.WriteString(input.Readiness.Context)
		user.WriteString("\n\n")
	}
	if input.StagePlan != nil && len(input.StagePlan.NonNegotiables) > 0 {
		user.WriteString("Stage plan non-negotiables:\n")
		for _, item := range input.StagePlan.NonNegotiables {
			user.WriteString("- ")
			user.WriteString(item)
			user.WriteString("\n")
		}
		user.WriteString("\n")
	}
	if input.Inspiration != nil {
		user.WriteString("Inspiration tensions:\n")
		for _, tension := range input.Inspiration.Tensions {
			user.WriteString("- ")
			user.WriteString(tension)
			user.WriteString("\n")
		}
		user.WriteString("\nInspiration observations:\n")
		for _, observation := range input.Inspiration.Observations {
			user.WriteString("- ")
			user.WriteString(observation)
			user.WriteString("\n")
		}
		user.WriteString("\n")
	}
	if input.Reframe != nil {
		user.WriteString("Top HMW candidates:\n")
		for _, frame := range input.Reframe.SelectedFrames {
			user.WriteString(fmt.Sprintf("- [%s] %s\n", frame.Lens, frame.HMW))
		}
		user.WriteString("\n")
	}
	if input.Transfer != nil && strings.TrimSpace(input.Transfer.Summary) != "" {
		user.WriteString("Transfer summary from ideation:\n")
		user.WriteString(input.Transfer.Summary)
		user.WriteString("\n\n")
	}
	user.WriteString("Portfolio bets:\n")
	user.WriteString(strings.Join(betLines, "\n"))
	user.WriteString("\n\nEligible experiment cards:\n")
	for _, card := range input.Eligible {
		user.WriteString(fmt.Sprintf("- %s | assumption: %s | test: %s | signal: %s | baseline: %s | target: %s | delta: %s | segment: %s | impact: %s | refs: %s\n",
			card.Title,
			card.CoreAssumption,
			card.CheapestTest,
			card.TargetSignal,
			card.BaselineMetric,
			card.TargetMetric,
			card.ExpectedDelta,
			card.Segment,
			card.TimeToImpact,
			card.EvidenceRefs,
		))
	}
	if len(input.HumanFeedback) > 0 {
		user.WriteString("\n\nHuman feedback:\n")
		for _, fb := range input.HumanFeedback {
			user.WriteString(fmt.Sprintf("- %s (%s): %s\n", fb.Stakeholder.Name, fb.Stakeholder.Role, fb.Response))
		}
	}

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
		OutputSchema:      closingSummaryOutput{},
		Silent:            true,
	})
	if err != nil {
		return "", fmt.Errorf("closing summary: %w", err)
	}

	out, err := parseStructuredOutput[closingSummaryOutput](resp)
	if err != nil {
		return "", fmt.Errorf("closing summary parse: %w", err)
	}

	// Ensure we keep portfolio classification visible even if model omits it.
	if len(out.SafeBets) == 0 {
		out.SafeBets = safeBets
	}
	if len(out.AdjacentBets) == 0 {
		out.AdjacentBets = adjacentBets
	}
	if len(out.BoldBets) == 0 {
		out.BoldBets = boldBets
	}

	return renderClosingSummary(out), nil
}

func renderClosingSummary(out closingSummaryOutput) string {
	var sb strings.Builder
	sb.WriteString("# IDEO BRAINSTORM: CLOSING SUMMARY\n\n")

	sb.WriteString("## Problem Statement\n")
	sb.WriteString(strings.TrimSpace(out.ProblemStatement))
	sb.WriteString("\n\n")

	sb.WriteString("## What We Learned (Inspiration + Reframe)\n")
	for _, item := range out.WhatWeLearned {
		if strings.TrimSpace(item) == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(strings.TrimSpace(item))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## How Might We (Top HMWs)\n")
	for _, hmw := range out.TopHMWs {
		if strings.TrimSpace(hmw) == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(strings.TrimSpace(hmw))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Concept Portfolio\n")
	sb.WriteString("### Safe\n")
	for _, bet := range out.SafeBets {
		sb.WriteString("- ")
		sb.WriteString(strings.TrimSpace(bet))
		sb.WriteString("\n")
	}
	sb.WriteString("\n### Adjacent\n")
	for _, bet := range out.AdjacentBets {
		sb.WriteString("- ")
		sb.WriteString(strings.TrimSpace(bet))
		sb.WriteString("\n")
	}
	sb.WriteString("\n### Bold Bets\n")
	for _, bet := range out.BoldBets {
		sb.WriteString("- ")
		sb.WriteString(strings.TrimSpace(bet))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Key Assumptions to Test\n")
	for _, assumption := range out.KeyAssumptionsToTest {
		if strings.TrimSpace(assumption) == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(strings.TrimSpace(assumption))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Recommended First Experiment\n")
	sb.WriteString(strings.TrimSpace(out.RecommendedFirstExperiment))
	sb.WriteString("\n\n")

	sb.WriteString("## Open Questions\n")
	for _, question := range out.OpenQuestions {
		if strings.TrimSpace(question) == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(strings.TrimSpace(question))
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}
