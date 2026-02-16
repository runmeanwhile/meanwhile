package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/evidencegate"
	"github.com/runmeanwhile/meanwhile/pkg/collab/ideationops"
	"github.com/runmeanwhile/meanwhile/pkg/collab/insightpack"
	"github.com/runmeanwhile/meanwhile/pkg/collab/minutes"
	"github.com/runmeanwhile/meanwhile/pkg/collab/portfolio"
	"github.com/runmeanwhile/meanwhile/pkg/collab/reframer"
	"github.com/runmeanwhile/meanwhile/pkg/collab/roundtable"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
)

// brainstormingLab is a context-aware brainstorm optimized for experiment-ready outcomes.
type brainstormingLab struct {
	cfg        brainstormingLabConfig
	lastResult map[string]any
}

// BrainstormingLab creates a context-aware brainstorming protocol that integrates
// reframing and evidence gating.
func BrainstormingLab(opts ...BrainstormingLabOption) Protocol {
	cfg := defaultBrainstormingLabConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &brainstormingLab{cfg: cfg}
}

func (p *brainstormingLab) ID() string { return "protocol.brainstorming_lab" }

func (p *brainstormingLab) Participants() []Participant { return nil }

func (p *brainstormingLab) Config() Config { return p.cfg.asConfig() }

func (p *brainstormingLab) Init(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}

func (p *brainstormingLab) OnMessage(ctx context.Context, sess Session, msg agent.Message) error {
	participants := sess.Participants()
	if len(participants) == 0 {
		return errBrainstormNoParticipants
	}

	agents, err := participantsToAgents(participants)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return fmt.Errorf("brainstorming lab requires agent participants")
	}

	p.lastResult = nil

	// Create HMW registry and register tool
	hmwRegistry := reframer.NewRegistry()
	hmwTool, err := hmwRegistry.Tool()
	if err != nil {
		return fmt.Errorf("create hmw tool: %w", err)
	}
	if err := sess.RegisterTool(hmwTool); err != nil {
		return fmt.Errorf("register hmw tool: %w", err)
	}

	scope, err := p.resolveScope(ctx, sess, msg)
	if err != nil {
		return fmt.Errorf("refine scope: %w", err)
	}
	if strings.TrimSpace(scope) == "" {
		scope = p.cfg.Scope
	}

	plan := p.cfg.ContextPlan
	if plan.Strategy == "" {
		plan = insightpack.DefaultPlan()
	}
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("context plan: %w", err)
	}

	if err := p.runKickoffFraming(ctx, sess, agents, scope); err != nil {
		return err
	}

	discoveryThread, err := p.runDiscoveryRounds(ctx, sess, agents, scope, plan, msg)
	if err != nil {
		return err
	}

	challengeThread, err := p.runChallengeRounds(ctx, sess, agents, scope, plan, discoveryThread)
	if err != nil {
		return err
	}

	hmwRaw, hmwThread, frames, err := p.runHMWWorkshop(ctx, sess, agents, scope, plan, hmwRegistry, discoveryThread, challengeThread)
	if err != nil {
		return err
	}

	if err := p.runReframeBridge(ctx, sess, agents, frames); err != nil {
		return err
	}

	conceptThread, err := p.runConceptRounds(ctx, sess, agents, scope, frames, plan, msg, discoveryThread, challengeThread)
	if err != nil {
		return err
	}

	critiqueThread, err := p.runConceptCritique(ctx, sess, agents, scope, conceptThread)
	if err != nil {
		return err
	}

	evidenceThread := append(append([]agent.Message(nil), conceptThread...), critiqueThread...)
	gateRaw, cards, eligible, rejected, gateIssues, err := p.runEvidenceGate(ctx, sess, agents, scope, evidenceThread)
	if err != nil {
		return err
	}

	finalists := p.pickFinalists(eligible, rejected)
	portfolioBets := portfolio.Build(finalists, p.cfg.FinalistCount)
	closing, err := p.runClosing(ctx, sess, agents, scope, plan, frames, discoveryThread, portfolioBets)
	if err != nil {
		return err
	}

	mins := minutes.New()
	mins.Add("scope", scope)
	mins.Add("participants", participantNames(agents))
	mins.Add("context_plan", plan)
	mins.Add("discovery", map[string]any{
		"thread": discoveryThread,
	})
	mins.Add("challenge", map[string]any{
		"thread": challengeThread,
	})
	mins.Add("hmw_workshop", map[string]any{
		"raw":    hmwRaw,
		"thread": hmwThread,
		"frames": frames,
	})
	mins.Add("concept_thread", conceptThread)
	mins.Add("critique_thread", critiqueThread)
	mins.Add("evidence_gate", map[string]any{
		"raw":      gateRaw,
		"cards":    cards,
		"eligible": eligible,
		"rejected": rejected,
		"issues":   gateIssues,
	})
	mins.Add("finalists", finalists)
	mins.Add("portfolio", portfolioBets)
	if closing != "" {
		mins.SetSummary(closing)
	}

	payload := mins.Payload()
	p.lastResult = payload
	if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload)); err != nil {
		return fmt.Errorf("emit brainstorm lab: %w", err)
	}
	return nil
}

func (p *brainstormingLab) OnEvent(ctx context.Context, sess Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}

func (p *brainstormingLab) Shutdown(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}

func (p *brainstormingLab) Result() map[string]any {
	return cloneResult(p.lastResult)
}

func (p *brainstormingLab) runDiscoveryRounds(ctx context.Context, sess Session, agents []agent.Agent, scope string, plan insightpack.Plan, seed agent.Message) ([]agent.Message, error) {
	rounds := p.cfg.DiscoveryRounds
	if rounds <= 0 {
		rounds = 1
	}

	// Use plan's tool budget, with a reasonable per-turn allocation
	turnToolBudget := max(plan.MaxToolIterations()/max(1, rounds*len(agents)),
		// At minimum, allow research + follow-up
		2)

	rt := roundtable.New(roundtable.WithMaxRounds(rounds))
	rt.Record(seed)
	rt.Record(message.User("Before we jump into solutions, investigate the problem from evidence and open questions."))

	// Track tool findings across all agents to prevent redundant calls
	sharedFindings := NewSharedFindings()

	for rt.CurrentRound() < rt.MaxRounds() {
		currentRound := rt.IncrementRound()
		ordered := rotateAgentOrder(agents, currentRound)
		for idx, participant := range ordered {
			thread := recentThread(rt.Thread(), 10)
			messages := append([]agent.Message(nil), thread...)
			prompt := fmt.Sprintf("Your discovery turn (%d/%d): investigate the problem and hand off a question to the next person.", idx+1, len(ordered))
			messages = append(messages, message.User(prompt))

			// Include the participant's persona to preserve their unique voice
			basePrompt := participantBasePrompt(participant)
			personaSection := ""
			if basePrompt != "" {
				personaSection = fmt.Sprintf("YOUR PERSONA:\n%s\n\n", basePrompt)
			}

			toolSection := formatToolsSection(plan)

			// Include prior findings from other agents
			findingsSection := ""
			if sharedFindings.Len() > 0 {
				findingsSection = sharedFindings.FormatForPrompt() + "\n"
			}

			system := fmt.Sprintf(`%sBRAINSTORM LAB: DISCOVERY ROUND %d/%d
Scope:
"""
%s
"""

GOAL: Investigate the problem before proposing solutions. Use tools to find relevant context, then share what you learn.

%s%s
YOUR WORKFLOW:
- Review prior findings above—do NOT re-query what others already found
- If you need NEW information, use tools with a DIFFERENT question
- Build on others' insights, add your unique perspective
- When sharing findings, use format: [FINDING: tool_name] your summary
- Hand off an open question to the next person

Respond in your own voice.
`, personaSection, currentRound, rounds, scope, findingsSection, toolSection)

			resp, err := sess.RunAgent(ctx, participant, RunRequest{
				Messages:          messages,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: turnToolBudget,
				Tools:             plan.AllowedToolIDs(),
				ToolPolicy:        plan.ToolPolicy(),
			})
			if err != nil {
				return nil, fmt.Errorf("discovery round (%s): %w", participant.Name, err)
			}

			resp.Name = participant.Name
			rt.Record(resp)

			// Extract and share any findings from this agent's turn
			findings := ExtractFindingsFromResponse(participant.Name, resp)
			for _, f := range findings {
				sharedFindings.Add(f)
			}
		}
	}

	return rt.Thread(), nil
}

func (p *brainstormingLab) runChallengeRounds(ctx context.Context, sess Session, agents []agent.Agent, scope string, plan insightpack.Plan, discoveryThread []agent.Message) ([]agent.Message, error) {
	rounds := p.cfg.ChallengeRounds
	if rounds <= 0 {
		return nil, nil
	}

	thread := make([]agent.Message, 0, len(agents)*rounds)
	// Challenge rounds need tools to fact-check claims
	turnToolBudget := 2

	for round := 1; round <= rounds; round++ {
		ordered := rotateAgentOrder(agents, round)
		for idx, participant := range ordered {
			target := challengeTarget(ordered, idx)

			ctxThread := append([]agent.Message(nil), recentThread(discoveryThread, 8)...)
			ctxThread = append(ctxThread, recentThread(thread, 6)...)
			ctxThread = append(ctxThread, message.User("Your turn. Challenge assumptions and request missing proof."))

			// Include persona for distinct voice
			basePrompt := participantBasePrompt(participant)
			personaSection := ""
			if basePrompt != "" {
				personaSection = fmt.Sprintf("YOUR PERSONA:\n%s\n\n", basePrompt)
			}

			system := fmt.Sprintf(`%sBRAINSTORM LAB: CHALLENGE ROUND %d/%d
Scope: %s
Challenge target: %s

GOAL: Pressure-test %s's claims. Use tools to fact-check if needed. Call out weak evidence or hidden assumptions.

Be direct. Be constructive. 2-4 sentences in your own voice.`, personaSection, round, rounds, scope, target, target)

			resp, err := sess.RunAgent(ctx, participant, RunRequest{
				Messages:          ctxThread,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: turnToolBudget,
				Tools:             plan.AllowedToolIDs(),
				ToolPolicy:        plan.ToolPolicy(),
			})
			if err != nil {
				return nil, fmt.Errorf("challenge round (%s): %w", participant.Name, err)
			}
			resp.Name = participant.Name
			thread = append(thread, resp)
		}
	}

	return thread, nil
}

func (p *brainstormingLab) runHMWWorkshop(ctx context.Context, sess Session, agents []agent.Agent, scope string, plan insightpack.Plan, hmwRegistry *reframer.Registry, discoveryThread, challengeThread []agent.Message) (string, []agent.Message, []reframer.Frame, error) {
	runner := p.selectRunner(sess, agents)
	transitionSystem := `You are the moderator.
Move the team into an explicit How-Might-We phase after discovery and challenge.
Write 2-3 natural sentences:
1) summarize what we learned,
2) say "Let's create several How Might We statements",
3) invite radically different HMW angles.
No bullet points, no markdown, no JSON.`
	transitionUser := fmt.Sprintf("Scope: %s\nDiscovery recap:\n%s\nChallenge recap:\n%s", scope, strings.Join(recentMessages(discoveryThread, 4), "\n"), strings.Join(recentMessages(challengeThread, 4), "\n"))
	if _, err := sess.RunAgent(ctx, runner, RunRequest{
		Messages:          []agent.Message{message.User(transitionUser)},
		SystemMessages:    []agent.Message{message.System(transitionSystem)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	}); err != nil {
		return "", nil, nil, fmt.Errorf("hmw transition: %w", err)
	}

	rt := roundtable.New(roundtable.WithMaxRounds(1))
	rt.Record(message.User("HMW Workshop begins. Use the submit_hmw tool to register your How Might We statements."))
	for _, msg := range recentThread(discoveryThread, 3) {
		rt.Record(msg)
	}
	for _, msg := range recentThread(challengeThread, 3) {
		rt.Record(msg)
	}

	// Include submit_hmw in allowed tools
	toolIDs := append(plan.AllowedToolIDs(), "submit_hmw")

	for idx, participant := range rotateAgentOrder(agents, 1) {
		thread := recentThread(rt.Thread(), 8)
		messages := append([]agent.Message(nil), thread...)
		messages = append(messages, message.User("Your turn. Think about the problem from your perspective and submit 2 HMW framings using the submit_hmw tool."))
		system := fmt.Sprintf(`BRAINSTORM LAB: HMW WORKSHOP TURN %d/%d
Scope: %s

Think about how to reframe this problem. Then call submit_hmw for each distinct framing you want to contribute.

Each HMW should:
- Use a different lens (operational, behavioral, emotional, economic, trust, workflow, adoption)
- Open a new design direction
- Be specific enough to guide ideation, broad enough to allow creativity

Submit 2 HMWs using the submit_hmw tool.`, idx+1, len(agents), scope)

		resp, err := sess.RunAgent(ctx, participant, RunRequest{
			Messages:          messages,
			SystemMessages:    []agent.Message{message.System(system)},
			Params:            p.runParamsFor(participant),
			MaxToolIterations: 3, // Allow 2 tool calls + 1 for thinking
			Tools:             toolIDs,
			ToolPolicy:        plan.ToolPolicy(),
		})
		if err != nil {
			return "", nil, nil, fmt.Errorf("hmw workshop (%s): %w", participant.Name, err)
		}
		resp.Name = participant.Name
		rt.Record(resp)
	}

	raw := ""
	for _, msg := range recentThread(rt.Thread(), len(agents)+2) {
		if msg.Role == agent.RoleAssistant && strings.TrimSpace(msg.Name) != "" {
			raw += strings.TrimSpace(msg.Text()) + "\n"
		}
	}
	return raw, rt.Thread(), hmwRegistry.Frames(), nil
}

func (p *brainstormingLab) runReframeBridge(ctx context.Context, sess Session, agents []agent.Agent, frames []reframer.Frame) error {
	runner := p.selectRunner(sess, agents)
	frameText := formatFrames(limitFrames(frames, 4))
	system := `You are the moderator.
Before ideation, bridge from discovery into "How Might We" prompts.
Write 2-3 conversational sentences that:
1) summarize what changed in understanding,
2) name the most promising HMW angles,
3) invite the team to generate radically different concepts.
No lists, no JSON.`
	user := "Selected HMW frames:\n" + frameText

	_, err := sess.RunAgent(ctx, runner, RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	if err != nil {
		return fmt.Errorf("reframe bridge: %w", err)
	}
	return nil
}

func (p *brainstormingLab) runConceptRounds(ctx context.Context, sess Session, agents []agent.Agent, scope string, frames []reframer.Frame, plan insightpack.Plan, seed agent.Message, discoveryThread []agent.Message, challengeThread []agent.Message) ([]agent.Message, error) {
	rt := roundtable.New(roundtable.WithMaxRounds(p.cfg.InteractionRounds))
	rt.Record(seed)
	rt.Record(message.User("Reframed scope: " + scope))
	rt.Record(message.User("Discovery recap: " + summarizeDiscoveryThread(discoveryThread)))
	for _, msg := range recentThread(challengeThread, 3) {
		rt.Record(msg)
	}

	frameText := formatFrames(frames)
	turnToolBudget := 1

	for rt.CurrentRound() < rt.MaxRounds() {
		currentRound := rt.IncrementRound()
		ordered := rotateAgentOrder(agents, currentRound)
		ops := ideationops.ForRound(currentRound-1, len(ordered))
		for idx, participant := range ordered {
			thread := recentThread(rt.Thread(), 10)
			messages := append([]agent.Message(nil), thread...)
			buildTarget := recentPeer(thread, participant.Name)
			if buildTarget == "" {
				buildTarget = "the last speaker"
			}
			messages = append(messages, message.User("Your turn. Build on one concrete prior point and push the concept forward."))

			op := ideationops.Operator{Name: "Constraint Remix", Prompt: "Keep key constraints fixed and redesign behavior."}
			if idx < len(ops) {
				op = ops[idx]
			}

			// Include persona for distinct voice
			basePrompt := participantBasePrompt(participant)
			personaSection := ""
			if basePrompt != "" {
				personaSection = fmt.Sprintf("YOUR PERSONA:\n%s\n\n", basePrompt)
			}

			system := fmt.Sprintf(`%sBRAINSTORM LAB: IDEATION SPRINT %d/%d
Scope: %s

Active reframes:
%s

%s

GOAL: Build on what %s said. Push the concept forward from your perspective. Contribute something concrete and actionable.

3-5 sentences in your own voice. No templates.`, personaSection, rt.CurrentRound(), rt.MaxRounds(), scope, frameText, ideationops.PromptBlock(op), buildTarget)

			resp, err := sess.RunAgent(ctx, participant, RunRequest{
				Messages:          messages,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: turnToolBudget,
			})
			if err != nil {
				return nil, fmt.Errorf("concept round (%s): %w", participant.Name, err)
			}
			resp.Name = participant.Name
			rt.Record(resp)
		}
	}
	return rt.Thread(), nil
}

func (p *brainstormingLab) runConceptCritique(ctx context.Context, sess Session, agents []agent.Agent, scope string, conceptThread []agent.Message) ([]agent.Message, error) {
	rounds := p.cfg.CritiqueRounds
	if rounds <= 0 {
		return nil, nil
	}

	out := make([]agent.Message, 0, len(agents)*rounds)
	for round := 1; round <= rounds; round++ {
		ordered := rotateAgentOrder(agents, round)
		for _, participant := range ordered {
			target := recentPeer(conceptThread, participant.Name)
			if target == "" {
				target = "another participant"
			}
			messages := append([]agent.Message(nil), recentThread(conceptThread, 8)...)
			messages = append(messages, recentThread(out, 4)...)
			messages = append(messages, message.User("Your critique turn. Pressure-test one concept to improve decision quality."))

			// Include persona for distinct voice
			basePrompt := participantBasePrompt(participant)
			personaSection := ""
			if basePrompt != "" {
				personaSection = fmt.Sprintf("YOUR PERSONA:\n%s\n\n", basePrompt)
			}

			system := fmt.Sprintf(`%sBRAINSTORM LAB: IDEA CRITIQUE %d/%d
Scope: %s
Target: %s

GOAL: Stress-test %s's concept. What's the weakest assumption? What evidence would change your mind?

2-4 sentences. Be direct, be useful.`, personaSection, round, rounds, scope, target, target)

			resp, err := sess.RunAgent(ctx, participant, RunRequest{
				Messages:          messages,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: 1,
			})
			if err != nil {
				return nil, fmt.Errorf("idea critique (%s): %w", participant.Name, err)
			}
			resp.Name = participant.Name
			out = append(out, resp)
		}
	}
	return out, nil
}

func (p *brainstormingLab) runEvidenceGate(ctx context.Context, sess Session, agents []agent.Agent, scope string, thread []agent.Message) (string, []evidencegate.Card, []evidencegate.Card, []evidencegate.Card, []evidencegate.ValidationIssue, error) {
	runner := p.selectRunner(sess, agents)
	snippets := recentMessages(thread, 12)
	system := `BRAINSTORM LAB: EVIDENCE GATE
Convert discussion into experiment cards.
Return JSON only as an array of objects with keys:
- title
- concept
- core_assumption
- cheapest_test
- target_signal
- success_threshold
- failure_threshold
- time_to_learn
- risk_level
- evidence_refs
- confidence
- unknowns`
	user := fmt.Sprintf("Scope: %s\n\nRecent concept discussion:\n%s\n\nProduce 4-6 candidate cards.", scope, strings.Join(snippets, "\n"))

	resp, err := sess.RunAgent(ctx, runner, RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
		Silent:            true,
	})
	if err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("evidence gate: %w", err)
	}

	raw := strings.TrimSpace(resp.Text())
	if raw == "" {
		raw = strings.TrimSpace(resp.Summary())
	}
	cards := parseEvidenceCards(raw)
	if len(cards) == 0 {
		repaired, repErr := p.repairEvidenceCards(ctx, sess, runner, raw)
		if repErr == nil && strings.TrimSpace(repaired) != "" {
			if parsed := parseEvidenceCards(repaired); len(parsed) > 0 {
				raw = repaired
				cards = parsed
			}
		}
	}
	if len(cards) == 0 {
		cards = cardsFromThread(thread)
	}
	eligible, rejected, issues := evidencegate.EligibleCards(cards, 6)
	return raw, cards, eligible, rejected, issues, nil
}

func (p *brainstormingLab) pickFinalists(eligible, rejected []evidencegate.Card) []evidencegate.Card {
	if len(eligible) == 0 && len(rejected) == 0 {
		return nil
	}
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

func (p *brainstormingLab) runClosing(ctx context.Context, sess Session, agents []agent.Agent, scope string, plan insightpack.Plan, frames []reframer.Frame, discoveryThread []agent.Message, bets []portfolio.Bet) (string, error) {
	runner := p.selectRunner(sess, agents)
	var frameLines []string
	for _, frame := range frames {
		frameLines = append(frameLines, fmt.Sprintf("- [%s] %s", frame.Lens, frame.HMW))
	}
	var discoveryLines []string
	for _, line := range recentMessages(discoveryThread, 5) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		discoveryLines = append(discoveryLines, "- "+truncateForContext(line, 180))
	}
	var betLines []string
	for _, bet := range bets {
		card := bet.Card
		betLines = append(betLines, fmt.Sprintf("- [%s] %s | test: %s | signal: %s | confidence: %s", strings.ToUpper(string(bet.Type)), card.Title, card.CheapestTest, card.TargetSignal, card.Confidence))
	}
	system := `BRAINSTORM LAB CLOSING
Write a concise markdown report with sections:
## Goal / Problem
## What We Learned Before Ideation
## Reframes (How Might We)
## Concept Portfolio (Safe / Adjacent / Bold)
## Key Doubts and Missing Proof
## Recommended Next Experiment`
	user := fmt.Sprintf("Scope: %s\nContext strategy: %s\n\nDiscovery:\n%s\n\nFrames:\n%s\n\nPortfolio Bets:\n%s", scope, plan.Strategy, strings.Join(discoveryLines, "\n"), strings.Join(frameLines, "\n"), strings.Join(betLines, "\n"))

	resp, err := sess.RunAgent(ctx, runner, RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	if err != nil {
		return "", fmt.Errorf("closing summary: %w", err)
	}
	return strings.TrimSpace(resp.Text()), nil
}

func (p *brainstormingLab) runKickoffFraming(ctx context.Context, sess Session, agents []agent.Agent, scope string) error {
	runner := p.selectRunner(sess, agents)

	system := `
You are the brainstorm moderator. Start the session naturally.

Write in your own voice:
1) Describe the problem in plain language. Assume no one on the team knows why they are here or what the problem is.
2) The team has access to tools to investigate the problem. Encourage them to use these tools to find evidence before jumping into solutions.
3) Ask the team to challenge the problem framing and find their own insights before ideating.

Sound like a curious, direct moderator. No templates or lists. Just talk to the team.`
	user := fmt.Sprintf("Scope: %s", scope)

	_, err := sess.RunAgent(ctx, runner, RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	if err != nil {
		return fmt.Errorf("kickoff framing: %w", err)
	}
	return nil
}

func formatFrames(frames []reframer.Frame) string {
	if len(frames) == 0 {
		return "(none)"
	}
	var sb strings.Builder
	for _, frame := range frames {
		sb.WriteString("- [")
		sb.WriteString(strings.TrimSpace(frame.Lens))
		sb.WriteString("] ")
		sb.WriteString(strings.TrimSpace(frame.HMW))
		if strings.TrimSpace(frame.Rationale) != "" {
			sb.WriteString(" (")
			sb.WriteString(strings.TrimSpace(frame.Rationale))
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func limitFrames(frames []reframer.Frame, limit int) []reframer.Frame {
	if len(frames) == 0 || limit <= 0 || len(frames) <= limit {
		return frames
	}
	return append([]reframer.Frame(nil), frames[:limit]...)
}

func summarizeDiscoveryThread(thread []agent.Message) string {
	lines := recentMessages(thread, 4)
	if len(lines) == 0 {
		return "No explicit discovery notes were captured."
	}
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, truncateForContext(strings.TrimSpace(line), 100))
	}
	if len(parts) == 0 {
		return "Discovery completed; key questions were implied but not explicit."
	}
	return strings.Join(parts, " | ")
}

func recentPeer(thread []agent.Message, exclude string) string {
	for i := len(thread) - 1; i >= 0; i-- {
		name := strings.TrimSpace(thread[i].Name)
		if name == "" || strings.EqualFold(name, strings.TrimSpace(exclude)) {
			continue
		}
		return name
	}
	return ""
}

func challengeTarget(ordered []agent.Agent, idx int) string {
	if len(ordered) == 0 {
		return "another participant"
	}
	target := ordered[(idx+len(ordered)-1)%len(ordered)].Name
	target = strings.TrimSpace(target)
	if target == "" {
		return "another participant"
	}
	return target
}

func knownSourceIDs(plan insightpack.Plan) []string {
	out := make([]string, 0, len(plan.Sources))
	seen := make(map[string]struct{}, len(plan.Sources))
	for _, src := range plan.Sources {
		id := strings.TrimSpace(src.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// formatToolsSection formats the plan's sources into a tools section for prompts.
func formatToolsSection(plan insightpack.Plan) string {
	if len(plan.Sources) == 0 {
		return "TOOLS: No specific tools configured. Use any available tools to gather evidence."
	}
	var sb strings.Builder
	sb.WriteString("AVAILABLE TOOLS:\n")
	for _, src := range plan.Sources {
		if len(src.ToolIDs) == 0 {
			continue
		}
		desc := strings.TrimSpace(src.Description)
		if desc == "" {
			desc = string(src.Type) + " source"
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", strings.Join(src.ToolIDs, ", "), desc))
	}
	if sb.Len() == len("AVAILABLE TOOLS:\n") {
		return "TOOLS: No specific tools configured. Use any available tools to gather evidence."
	}
	sb.WriteString("\nUse these tools to find concrete evidence. Cite sources when sharing findings.")
	return sb.String()
}

// participantBasePrompt extracts the persona prompt from a participant's profile.
func participantBasePrompt(participant agent.Agent) string {
	if participant.Profile != nil && strings.TrimSpace(participant.Profile.Prompt) != "" {
		return strings.TrimSpace(participant.Profile.Prompt)
	}
	return ""
}

func (p *brainstormingLab) repairEvidenceCards(ctx context.Context, sess Session, runner agent.Agent, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	system := `BRAINSTORM LAB: EVIDENCE JSON REPAIR
Convert the given content into STRICT JSON only:
- output must be an array of objects,
- preserve only these keys:
  title, concept, core_assumption, cheapest_test, target_signal, success_threshold, failure_threshold, time_to_learn, risk_level, evidence_refs, confidence, unknowns
- if no usable cards exist, return [].
Return JSON only.`
	user := "Content to convert:\n" + raw
	resp, err := sess.RunAgent(ctx, runner, RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
		Silent:            true,
	})
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(resp.Text())
	if text == "" {
		text = strings.TrimSpace(resp.Summary())
	}
	return text, nil
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
	var wrapped struct {
		Cards []evidencegate.Card `json:"cards"`
	}
	if err := json.Unmarshal([]byte(trimmed), &wrapped); err == nil && len(wrapped.Cards) > 0 {
		return wrapped.Cards
	}
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

func cardsFromThread(thread []agent.Message) []evidencegate.Card {
	cards := make([]evidencegate.Card, 0)
	for _, msg := range thread {
		if msg.Role != agent.RoleAssistant || strings.TrimSpace(msg.Name) == "" {
			continue
		}
		text := strings.TrimSpace(msg.Text())
		if text == "" {
			continue
		}
		title := truncateForContext(text, 72)
		cards = append(cards, evidencegate.Card{
			Title:            title,
			Concept:          text,
			CoreAssumption:   "Assumption to validate from concept discussion",
			CheapestTest:     "Rapid prototype or simulated workflow walkthrough",
			TargetSignal:     "User adoption or follow-through shift",
			SuccessThreshold: "Meaningful positive movement in target signal",
			FailureThreshold: "No measurable improvement after pilot",
			TimeToLearn:      "1-2 weeks",
			Confidence:       "low",
			Unknowns:         "Need direct user evidence before scaling",
		})
	}
	if len(cards) > 6 {
		cards = cards[:6]
	}
	return cards
}

func (p *brainstormingLab) selectRunner(sess Session, agents []agent.Agent) agent.Agent {
	if facilitator := sess.Facilitator(); facilitator != nil {
		return *facilitator
	}
	return agents[0]
}

func (p *brainstormingLab) runParamsFor(participant agent.Agent) map[string]any {
	if len(p.cfg.Params) == 0 {
		return nil
	}
	if len(participant.Params) == 0 {
		return cloneParams(p.cfg.Params)
	}
	out := make(map[string]any, len(p.cfg.Params))
	for key, value := range p.cfg.Params {
		if _, ok := participant.Params[key]; ok {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (p *brainstormingLab) resolveScope(ctx context.Context, sess Session, msg agent.Message) (string, error) {
	userQuestion := msg.Text()
	if strings.TrimSpace(userQuestion) == "" {
		userQuestion = msg.Summary()
	}

	if facilitator := sess.Facilitator(); facilitator != nil && p.cfg.ScopeRefinement != nil {
		systemPrompt, userPrompt := p.cfg.ScopeRefinement(userQuestion, p.cfg.Scope)
		if strings.TrimSpace(userPrompt) == "" {
			userPrompt = userQuestion
		}
		scopePrompt := PromptWithMedia(userPrompt, msg)
		req := RunRequest{
			Messages:          []agent.Message{scopePrompt},
			MaxToolIterations: 1,
			Silent:            true,
		}
		if strings.TrimSpace(systemPrompt) != "" {
			req.SystemMessages = []agent.Message{message.System(systemPrompt)}
		}
		resp, err := sess.RunAgent(ctx, *facilitator, req)
		if err != nil {
			return "", err
		}
		if scopeText := strings.TrimSpace(resp.Text()); scopeText != "" {
			return scopeText, nil
		}
	}

	if p.cfg.ScopeFallback != nil {
		scope := strings.TrimSpace(p.cfg.ScopeFallback(userQuestion, p.cfg.Scope))
		if scope != "" {
			return scope, nil
		}
	}
	if strings.TrimSpace(p.cfg.Scope) != "" {
		return p.cfg.Scope, nil
	}
	return userQuestion, nil
}
