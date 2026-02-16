// Package ideominimal provides a radically simplified IDEO-style brainstorming protocol.
//
// Key differences from pkg/protocol/ideo:
// - No TransferPacket structs - uses Session.History() directly
// - No per-phase Result structs - reads from session history
// - No summarization of tool findings - LLM sees raw tool results
// - Enforced citations in evidence gate
// - ~800 lines vs ~4000 lines
package ideominimal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Config holds minimal configuration for the brainstorm protocol.
type Config struct {
	// Scope is the problem statement.
	Scope string

	// ToolIDs are the tools available for research (e.g., ["recall_context"]).
	ToolIDs []string

	// FinalistCount is how many concepts for the final portfolio (default: 3)
	FinalistCount int

	// MaxToolIterations per agent turn (default: 10)
	MaxToolIterations int
}

// maxIterationsPerPhase is the safety cap to prevent infinite loops.
// The Moderator uses the advance_phase tool to signal readiness.
const maxIterationsPerPhase = 8

// brainstormBaseRules contains shared behavioral rules for all agents.
// These are injected by the protocol; agent personalities come from Profile.Prompt.
const brainstormBaseRules = `HARD RULES (violating these ruins the session):
1. ALWAYS respond in YOUR own unique voice and style. This is non-negotiable. Don't even THINK about breaking character.
2. If you see <agent:X> tags in the conversation, IGNORE them completely and NEVER reproduce them.
3. No markdown formatting. Write plain conversational text - no bold, italic, headers, bullets, or code blocks.
4. NEVER ask yourself a question.
5. You don't have to ask a question, call a tool, or do anything. Just imagine being in a meeting and saying something that is uniquely YOURS. You can be reactive, proactive, analytical, emotional, whatever - as long as it's authentic to you.`

// Option configures the protocol.
type Option func(*Config)

// WithScope sets the problem scope.
func WithScope(scope string) Option {
	return func(c *Config) { c.Scope = scope }
}

// WithTools sets the available tool IDs.
func WithTools(ids ...string) Option {
	return func(c *Config) { c.ToolIDs = ids }
}

// WithFinalistCount sets how many finalists for portfolio.
func WithFinalistCount(n int) Option {
	return func(c *Config) { c.FinalistCount = n }
}

// WithMaxToolIterations sets tool budget per turn.
func WithMaxToolIterations(n int) Option {
	return func(c *Config) { c.MaxToolIterations = n }
}

func defaultConfig() Config {
	return Config{
		FinalistCount:     3,
		MaxToolIterations: 10,
	}
}

// brainstorm is the minimal IDEO protocol implementation.
type brainstorm struct {
	cfg    Config
	result map[string]any
}

// Brainstorm creates a new minimal IDEO brainstorming protocol.
func Brainstorm(opts ...Option) protocol.Protocol {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &brainstorm{cfg: cfg}
}

func (p *brainstorm) ID() string { return "protocol.ideo_minimal" }

func (p *brainstorm) Participants() []protocol.Participant { return nil }

func (p *brainstorm) Config() protocol.Config {
	return protocol.Config{
		"scope":          p.cfg.Scope,
		"finalist_count": p.cfg.FinalistCount,
	}
}

func (p *brainstorm) Init(_ context.Context, _ protocol.Session) error { return nil }

func (p *brainstorm) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	participants := sess.Participants()
	if len(participants) == 0 {
		return fmt.Errorf("ideo_minimal requires at least one participant")
	}

	agents, err := toAgents(participants)
	if err != nil {
		return err
	}

	scope := p.resolveScope(msg)
	runner := p.selectRunner(sess, agents)

	// Register HMW tool for reframe phase
	hmwReg := newHMWRegistry()
	hmwTool, err := hmwReg.tool()
	if err != nil {
		return fmt.Errorf("create hmw tool: %w", err)
	}
	if err := sess.RegisterTool(hmwTool); err != nil {
		return fmt.Errorf("register hmw tool: %w", err)
	}

	// Register concept tool for ideation phase
	conceptReg := newConceptRegistry()
	conceptTool, err := conceptReg.tool()
	if err != nil {
		return fmt.Errorf("create concept tool: %w", err)
	}
	if err := sess.RegisterTool(conceptTool); err != nil {
		return fmt.Errorf("register concept tool: %w", err)
	}

	// Register phase progression tool for Moderator
	progress := newPhaseProgress()
	progressTool, err := progress.tool()
	if err != nil {
		return fmt.Errorf("create progress tool: %w", err)
	}
	if err := sess.RegisterTool(progressTool); err != nil {
		return fmt.Errorf("register progress tool: %w", err)
	}

	// PHASE 0: READINESS - gather context
	if err := p.runReadiness(ctx, sess, runner, scope); err != nil {
		return fmt.Errorf("readiness: %w", err)
	}

	// PHASE 1: INSPIRATION - research and observe
	if err := p.runInspiration(ctx, sess, agents, scope, progress); err != nil {
		return fmt.Errorf("inspiration: %w", err)
	}

	// PHASE 2: REFRAME - generate HMW questions
	if err := p.runReframe(ctx, sess, agents, scope, progress); err != nil {
		return fmt.Errorf("reframe: %w", err)
	}

	// PHASE 3: IDEATION - generate concepts
	if err := p.runIdeation(ctx, sess, agents, scope, progress); err != nil {
		return fmt.Errorf("ideation: %w", err)
	}

	// PHASE 4: SYNTHESIS - critique and build experiment cards
	portfolio, err := p.runSynthesis(ctx, sess, agents, scope, runner, conceptReg.concepts())
	if err != nil {
		return fmt.Errorf("synthesis: %w", err)
	}

	// Build final result
	p.result = map[string]any{
		"scope":     scope,
		"hmws":      hmwReg.hmws(),
		"concepts":  conceptReg.concepts(),
		"portfolio": portfolio,
	}

	return sess.Emit(event.New(event.ProtocolAction, sess.ID(), p.result))
}

func (p *brainstorm) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}

func (p *brainstorm) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func (p *brainstorm) Result() map[string]any {
	if p.result == nil {
		return nil
	}
	out := make(map[string]any, len(p.result))
	for k, v := range p.result {
		out[k] = v
	}
	return out
}

// ============================================================================
// PHASE 0: READINESS
// ============================================================================

func (p *brainstorm) runReadiness(ctx context.Context, sess protocol.Session, runner agent.Agent, scope string) error {
	system := `You're the Moderator. Gather context before the team starts.

Use your tools to find:
- Current metrics and baselines
- Known constraints
- User feedback and pain points
- What's been tried

Then brief the team conversationally with actual numbers. List data points inline or separated by commas.

GOOD: "Pulled the numbers: 16% activation rate, 68% churning before first workflow, 43% blocked by OAuth. That's our baseline. Let's dig in."

BAD: "**Summary of Findings:**\n**Current State:**\n- Point 1..."

NO MARKDOWN. No asterisks, no bullet symbols, no headers. Just talk like a person running a meeting.

End by saying the team is ready.`

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(fmt.Sprintf("Topic: %s\n\nGather context, then brief the team.", scope))},
		SystemMessages:    []agent.Message{message.System(system)},
		MaxToolIterations: p.cfg.MaxToolIterations,
		Tools:             p.cfg.ToolIDs,
	})
	return err
}

// ============================================================================
// PHASE 1: INSPIRATION
// ============================================================================

func (p *brainstorm) runInspiration(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, progress *phaseProgress) error {
	runner := p.selectRunner(sess, agents)

	// Determine first speaker for this phase
	firstSpeaker := rotateOrder(agents, 1)[0].ID

	// Kick off inspiration phase
	teamNames := agentNames(agents)
	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages: []agent.Message{message.User("Begin INSPIRATION phase. The team will now observe and investigate before proposing solutions.")},
		SystemMessages: []agent.Message{message.System(fmt.Sprintf(`Kick off inspiration. 2-3 sentences max.

TEAM MEMBERS: %s
(Use these exact names when addressing people - do not invent other names.)

Remind the team: We're investigating, not solving. Find numbers, tensions, surprises. No solutions yet.

End by calling on %s.`, teamNames, firstSpeaker))},
		MaxToolIterations: 1,
	})
	if err != nil {
		return err
	}

	// Loop until Moderator signals readiness to move on
	for iteration := 1; iteration <= maxIterationsPerPhase; iteration++ {
		// Each agent takes a turn
		for _, participant := range rotateOrder(agents, iteration) {
			history := p.recentHistory(ctx, sess, 10)

			// Reduce tool iterations after first round - agents should build on existing findings
			toolIterations := 3
			if iteration == 1 {
				toolIterations = p.cfg.MaxToolIterations
			}

			phaseContext := fmt.Sprintf(`PHASE: INSPIRATION (investigating, not solving)
TOPIC: %s

Your turn: Based on the conversation so far, post a response that is uniquely YOURS. You can do anything you feel is appropriate right now.
`, scope)
			system := buildAgentSystem(participant, phaseContext)

			_, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          append(history, message.User("Your turn.")),
				SystemMessages:    []agent.Message{message.System(system)},
				MaxToolIterations: toolIterations,
				Tools:             p.cfg.ToolIDs,
			})
			if err != nil {
				return err
			}
		}

		// Moderator checkpoint: should we continue or progress?
		// Only allow advancing after at least 2 rounds of exploration
		progress.reset()
		history := p.recentHistory(ctx, sess, 8)

		var checkpointTools []string
		var checkpointPrompt string
		if iteration >= 2 {
			checkpointTools = []string{"advance_phase"}
			checkpointPrompt = fmt.Sprintf(`YOU decide when to move on. Don't ask permission.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

Check: Do we have 3+ findings with real numbers and sources? Are people repeating themselves?

If we have enough OR people are circling: call advance_phase. Say: "We've got [X], [Y], [Z]. Moving to reframe."

If thin: Tell ONE person what to find.`, agentNames(agents))
		} else {
			checkpointTools = nil
			checkpointPrompt = fmt.Sprintf(`We're still exploring.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

Quick comment on what's surprising. Call on someone to dig deeper.`, agentNames(agents))
		}

		_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
			Messages:          append(history, message.User(fmt.Sprintf("Round %d complete.", iteration))),
			SystemMessages:    []agent.Message{message.System(checkpointPrompt)},
			Tools:             checkpointTools,
			MaxToolIterations: 1,
		})
		if err != nil {
			return err
		}

		if progress.shouldAdvance() {
			break
		}
	}

	return nil
}

// ============================================================================
// PHASE 2: REFRAME
// ============================================================================

func (p *brainstorm) runReframe(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, progress *phaseProgress) error {
	runner := p.selectRunner(sess, agents)

	// Determine first speaker for this phase
	firstSpeaker := rotateOrder(agents, 1)[0].ID

	// Transition to reframe
	teamNames := agentNames(agents)
	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages: []agent.Message{message.User("Transition to REFRAME phase. The team will now generate How-Might-We questions.")},
		SystemMessages: []agent.Message{message.System(fmt.Sprintf(`Transition to REFRAME. 3-4 sentences max.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

Recap 2-3 key findings with numbers. Then explain HMWs: embed the evidence IN the question.

End by calling on %s.`, teamNames, firstSpeaker))},
		MaxToolIterations: 1,
	})
	if err != nil {
		return err
	}

	lenses := []string{"user_experience", "business_model", "technology", "behavioral", "operational", "emotional"}

	// Loop until Moderator signals readiness to move on
	for iteration := 1; iteration <= maxIterationsPerPhase; iteration++ {
		for i, participant := range rotateOrder(agents, iteration) {
			history := p.recentHistory(ctx, sess, 8)

			lens := lenses[(iteration+i)%len(lenses)]

			phaseContext := fmt.Sprintf(`PHASE: REFRAME (turning findings into HMW questions)
TOPIC: %s
YOUR LENS: %s

Your turn: Based on the conversation so far, post a response that is uniquely YOURS. You can do anything you feel is appropriate right now. Just keep in mind at the very high level that our goal right now is to post a reframe of the problem/opportunity/issue. You will lean towards obeying, but if there is a strong reason to go off-script, feel free to do so but in that case you must be explicit about the why, and have a strong, valid, sensible reason.`, scope, lens)
			system := buildAgentSystem(participant, phaseContext)

			// Allow both research and HMW submission
			tools := append([]string{"submit_hmw"}, p.cfg.ToolIDs...)

			_, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          append(history, message.User("Your turn.")),
				SystemMessages:    []agent.Message{message.System(system)},
				MaxToolIterations: 4,
				Tools:             tools,
			})
			if err != nil {
				return err
			}
		}

		// Moderator checkpoint: should we continue or progress?
		// Only allow advancing after at least 2 rounds
		progress.reset()
		history := p.recentHistory(ctx, sess, 6)

		var checkpointTools []string
		var checkpointPrompt string
		if iteration >= 2 {
			checkpointTools = []string{"advance_phase"}
			checkpointPrompt = fmt.Sprintf(`YOU decide. Don't poll.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

We need 4-6 solid HMWs with numbers embedded.

If duplicates appearing or 4+ solid HMWs: call advance_phase and list the best.
If thin: Tell ONE person what angle is missing.`, agentNames(agents))
		} else {
			checkpointTools = nil
			checkpointPrompt = fmt.Sprintf(`Building momentum.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

What HMW angles are covered? What's missing? Challenge someone to think differently.`, agentNames(agents))
		}

		_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
			Messages:          append(history, message.User(fmt.Sprintf("Round %d of HMWs.", iteration))),
			SystemMessages:    []agent.Message{message.System(checkpointPrompt)},
			Tools:             checkpointTools,
			MaxToolIterations: 1,
		})
		if err != nil {
			return err
		}

		if progress.shouldAdvance() {
			break
		}
	}

	return nil
}

// ============================================================================
// PHASE 3: IDEATION
// ============================================================================

func (p *brainstorm) runIdeation(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, progress *phaseProgress) error {
	runner := p.selectRunner(sess, agents)

	// Determine first speaker for this phase
	firstSpeaker := rotateOrder(agents, 1)[0].ID

	// Transition to ideation
	history := p.recentHistory(ctx, sess, 6)
	teamNames := agentNames(agents)
	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages: append(history, message.User("Transition to IDEATION phase. Share the selected HMWs and kick off concept generation.")),
		SystemMessages: []agent.Message{message.System(fmt.Sprintf(`Transition to IDEATION. 3-4 sentences max.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

Mention 2-3 of the strongest HMWs, then tell the team to get creative.

End by calling on %s.`, teamNames, firstSpeaker))},
		MaxToolIterations: 1,
	})
	if err != nil {
		return err
	}

	operators := []string{
		"Reverse assumptions",
		"Combine two unrelated ideas",
		"10x the scale",
		"What would a child do?",
		"Remove the obvious solution",
		"Copy from another industry",
	}

	// Loop until Moderator signals readiness to move on
	for iteration := 1; iteration <= maxIterationsPerPhase; iteration++ {
		for i, participant := range rotateOrder(agents, iteration) {
			history := p.recentHistory(ctx, sess, 8)

			operator := operators[(iteration+i)%len(operators)]

			phaseContext := fmt.Sprintf(`PHASE: IDEATION (generating concepts)
TOPIC: %s
CREATIVE OPERATOR: %s

Your turn: Based on the conversation so far, post a response that is uniquely YOURS. You can do anything you feel is appropriate right now. Just keep in mind that our goal right now is to come up with ideas based on the topic and creative operator. You will lean towards obeying, but if there is a strong reason to go off-script, feel free to do so but in that case you must be explicit about the why, and have a strong, valid, sensible reason.`, scope, operator)
			system := buildAgentSystem(participant, phaseContext)

			// Allow both research and concept submission
			tools := append([]string{"submit_concept"}, p.cfg.ToolIDs...)

			_, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          append(history, message.User("Your turn.")),
				SystemMessages:    []agent.Message{message.System(system)},
				MaxToolIterations: 4,
				Tools:             tools,
			})
			if err != nil {
				return err
			}
		}

		// Moderator checkpoint: should we continue or progress?
		// Only allow advancing after at least 2 rounds
		progress.reset()
		history := p.recentHistory(ctx, sess, 6)

		var checkpointTools []string
		var checkpointPrompt string
		if iteration >= 2 {
			checkpointTools = []string{"advance_phase"}
			checkpointPrompt = fmt.Sprintf(`YOU decide. No polling.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

We need 6-10 diverse concepts.

If repetitive or 6+ concepts: call advance_phase.
If need more wild ideas: Challenge ONE person to go bolder.`, agentNames(agents))
		} else {
			checkpointTools = nil
			checkpointPrompt = fmt.Sprintf(`We're generating.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

Are concepts all safe? All bold? Push for variety. Challenge someone to flip an assumption.`, agentNames(agents))
		}

		_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
			Messages:          append(history, message.User(fmt.Sprintf("Round %d of concepts.", iteration))),
			SystemMessages:    []agent.Message{message.System(checkpointPrompt)},
			Tools:             checkpointTools,
			MaxToolIterations: 1,
		})
		if err != nil {
			return err
		}

		if progress.shouldAdvance() {
			break
		}
	}

	return nil
}

// ============================================================================
// PHASE 4: SYNTHESIS
// ============================================================================

// ExperimentCard represents an experiment-ready concept with citations.
type ExperimentCard struct {
	Title            string     `json:"title"`
	CoreAssumption   string     `json:"core_assumption"`
	CheapestTest     string     `json:"cheapest_test"`
	SuccessSignal    string     `json:"success_signal"`
	FailureSignal    string     `json:"failure_signal"`
	TimeToLearn      string     `json:"time_to_learn"`
	BaselineMetric   string     `json:"baseline_metric"`
	TargetMetric     string     `json:"target_metric"`
	Segment          string     `json:"segment"`
	RiskLevel        string     `json:"risk_level"`
	Evidence         []Citation `json:"evidence"`
	ValidationIssues []string   `json:"validation_issues,omitempty"`
}

// Citation links a claim to a source.
type Citation struct {
	Source string `json:"source"`
	Claim  string `json:"claim"`
}

// PortfolioBet is a final recommended experiment.
type PortfolioBet struct {
	Type string         `json:"type"`
	Card ExperimentCard `json:"card"`
}

func (p *brainstorm) runSynthesis(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, runner agent.Agent, concepts []Concept) ([]PortfolioBet, error) {
	// Determine first speaker for this phase
	firstSpeaker := agents[0].ID
	if len(agents) > 0 {
		firstSpeaker = agents[0].ID
	}

	// Transition to synthesis
	teamNames := agentNames(agents)
	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages: []agent.Message{message.User("Begin SYNTHESIS phase. Time to critique concepts and build experiment cards.")},
		SystemMessages: []agent.Message{message.System(fmt.Sprintf(`Transition to SYNTHESIS. 2-3 sentences max.

TEAM (ONLY these people exist): %s. NEVER use any other names - no invented names like "Sarah" or "Johnny".

Tell the team: be brutal, find flaws. If you agree too easily, you're not thinking.

Call on %s to start.`, teamNames, firstSpeaker))},
		MaxToolIterations: 1,
	})
	if err != nil {
		return nil, err
	}

	// Critique round
	for _, participant := range agents {
		history := p.recentHistory(ctx, sess, 8)

		system := `This is CRITIQUE - stress-testing concepts.

STYLE: 3-4 sentences. No markdown. Be direct.

GOOD: "The template-first concept assumes users know what they want to build. That's shaky—34% said they didn't know what to build [source: feedback.md]. Cheapest test: show 5 users a template picker vs blank canvas, measure completion in 10 minutes. Success = 50%+ complete. Failure = same as baseline."

BAD: "**Critique Analysis:**\n1. First, let me examine...\n2. The core assumption appears to be..."

RULES:
- Be uniquely YOU. What would YOU say to challenge this idea? (if anything?)`

		resp, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
			Messages:          append(history, message.User("Your turn. Attack a concept.")),
			SystemMessages:    []agent.Message{message.System(system)},
			MaxToolIterations: 3,
			Tools:             p.cfg.ToolIDs,
		})
		if err != nil {
			return nil, err
		}
		_ = resp
	}

	// Build experiment cards
	history := p.recentHistory(ctx, sess, 20)

	type cardsOutput struct {
		Cards []ExperimentCard `json:"cards"`
	}

	system := `EVIDENCE GATE: Convert concepts to experiment cards.

REQUIREMENTS (all mandatory):
- core_assumption: Specific, falsifiable
- cheapest_test: Executable in <1 week with minimal resources
- success_signal / failure_signal: Measurable thresholds
- baseline_metric / target_metric: Concrete numbers from evidence
- evidence: Array of {source, claim} - MUST cite actual documents from the research
- risk_level: low, medium, or high

Generate 4-6 experiment cards. Reject any card that lacks citations.`

	var conceptSummary strings.Builder
	conceptSummary.WriteString("Concepts from ideation:\n")
	for _, c := range concepts {
		conceptSummary.WriteString(fmt.Sprintf("- %s: %s (risk: %s)\n", c.Title, c.Problem, c.Risk))
	}

	resp, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          append(history, message.User(conceptSummary.String()+"\n\nGenerate experiment cards.")),
		SystemMessages:    []agent.Message{message.System(system)},
		MaxToolIterations: 1,
		OutputSchema:      cardsOutput{},
	})
	if err != nil {
		return nil, err
	}

	cards, err := parseOutput[cardsOutput](resp)
	if err != nil {
		return nil, fmt.Errorf("parse cards: %w", err)
	}

	// Validate and filter cards
	eligible := make([]ExperimentCard, 0)
	for _, card := range cards.Cards {
		issues := validateCard(card)
		if len(issues) == 0 {
			eligible = append(eligible, card)
		}
	}

	// Build portfolio
	portfolio := buildPortfolio(eligible, p.cfg.FinalistCount)

	return portfolio, nil
}

func validateCard(card ExperimentCard) []string {
	var issues []string
	if strings.TrimSpace(card.CoreAssumption) == "" {
		issues = append(issues, "missing core_assumption")
	}
	if strings.TrimSpace(card.CheapestTest) == "" {
		issues = append(issues, "missing cheapest_test")
	}
	if strings.TrimSpace(card.SuccessSignal) == "" {
		issues = append(issues, "missing success_signal")
	}
	if strings.TrimSpace(card.BaselineMetric) == "" {
		issues = append(issues, "missing baseline_metric")
	}
	if len(card.Evidence) == 0 {
		issues = append(issues, "missing evidence citations")
	}
	for i, cit := range card.Evidence {
		if strings.TrimSpace(cit.Source) == "" {
			issues = append(issues, fmt.Sprintf("evidence[%d] missing source", i))
		}
	}
	return issues
}

func buildPortfolio(cards []ExperimentCard, limit int) []PortfolioBet {
	if len(cards) == 0 || limit <= 0 {
		return nil
	}

	var safe, adjacent, bold []ExperimentCard
	for _, card := range cards {
		switch strings.ToLower(strings.TrimSpace(card.RiskLevel)) {
		case "low", "safe":
			safe = append(safe, card)
		case "high", "bold":
			bold = append(bold, card)
		default:
			adjacent = append(adjacent, card)
		}
	}

	out := make([]PortfolioBet, 0, limit)
	add := func(typ string, cards []ExperimentCard) {
		if len(out) >= limit || len(cards) == 0 {
			return
		}
		out = append(out, PortfolioBet{Type: typ, Card: cards[0]})
	}

	add("safe", safe)
	add("adjacent", adjacent)
	add("bold", bold)

	// Fill remaining slots
	all := append(append(safe, adjacent...), bold...)
	for _, card := range all {
		if len(out) >= limit {
			break
		}
		found := false
		for _, bet := range out {
			if bet.Card.Title == card.Title {
				found = true
				break
			}
		}
		if !found {
			out = append(out, PortfolioBet{Type: classifyRisk(card), Card: card})
		}
	}

	return out
}

func classifyRisk(card ExperimentCard) string {
	switch strings.ToLower(strings.TrimSpace(card.RiskLevel)) {
	case "low", "safe":
		return "safe"
	case "high", "bold":
		return "bold"
	default:
		return "adjacent"
	}
}

// ============================================================================
// HMW REGISTRY
// ============================================================================

// HMW is a How-Might-We question.
type HMW struct {
	Question     string   `json:"question"`
	Lens         string   `json:"lens"`
	EvidenceRefs []string `json:"evidence_refs"`
	Author       string   `json:"author,omitempty"`
}

type hmwRegistry struct {
	mu   sync.Mutex
	list []HMW
}

func newHMWRegistry() *hmwRegistry {
	return &hmwRegistry{list: make([]HMW, 0)}
}

func (r *hmwRegistry) tool() (tool.Tool, error) {
	type input struct {
		HMW          string   `json:"hmw" description:"The How-Might-We question"`
		Lens         string   `json:"lens" description:"The lens/perspective used (e.g., user_experience, business, technology)"`
		EvidenceRefs []string `json:"evidence_refs" description:"Source references supporting this HMW (e.g., 'wiki/product/metrics.md: 42% activate Day 1')"`
	}
	type output struct {
		Status string `json:"status"`
		ID     int    `json:"id"`
	}

	t, err := tool.New("submit_hmw", func(_ context.Context, in input) (output, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.list = append(r.list, HMW{
			Question:     in.HMW,
			Lens:         in.Lens,
			EvidenceRefs: in.EvidenceRefs,
		})
		return output{Status: "registered", ID: len(r.list)}, nil
	})
	if err != nil {
		return nil, err
	}
	return t.WithDescription(`Submit a How-Might-We question for the brainstorm.

Include evidence_refs citing the sources that inspired this HMW.
Example: ["wiki/product/activation-metrics.md: 42% of activations happen Day 1"]`), nil
}

func (r *hmwRegistry) hmws() []HMW {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]HMW, len(r.list))
	copy(out, r.list)
	return out
}

// ============================================================================
// CONCEPT REGISTRY
// ============================================================================

// Concept is a rough idea from ideation.
type Concept struct {
	Title        string   `json:"title"`
	Problem      string   `json:"problem"`
	Mechanism    string   `json:"mechanism"`
	Value        string   `json:"value"`
	Risk         string   `json:"risk"`
	EvidenceRefs []string `json:"evidence_refs"`
	Author       string   `json:"author,omitempty"`
}

type conceptRegistry struct {
	mu   sync.Mutex
	list []Concept
}

func newConceptRegistry() *conceptRegistry {
	return &conceptRegistry{list: make([]Concept, 0)}
}

func (r *conceptRegistry) tool() (tool.Tool, error) {
	type input struct {
		Title        string   `json:"title" description:"Brief name for the concept"`
		Problem      string   `json:"problem" description:"What user problem does this solve? Cite evidence."`
		Mechanism    string   `json:"mechanism" description:"How does it work?"`
		Value        string   `json:"value" description:"What's the benefit?"`
		Risk         string   `json:"risk" description:"What could go wrong?"`
		EvidenceRefs []string `json:"evidence_refs" description:"Source references (e.g., 'wiki/product/metrics.md: users drop at step 4')"`
	}
	type output struct {
		Status string `json:"status"`
		ID     int    `json:"id"`
	}

	t, err := tool.New("submit_concept", func(_ context.Context, in input) (output, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.list = append(r.list, Concept{
			Title:        in.Title,
			Problem:      in.Problem,
			Mechanism:    in.Mechanism,
			Value:        in.Value,
			Risk:         in.Risk,
			EvidenceRefs: in.EvidenceRefs,
		})
		return output{Status: "registered", ID: len(r.list)}, nil
	})
	if err != nil {
		return nil, err
	}
	return t.WithDescription(`Submit a concept for the brainstorm.

Include evidence_refs citing the research that supports this concept.
The problem field should reference specific findings, not generic statements.`), nil
}

func (r *conceptRegistry) concepts() []Concept {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Concept, len(r.list))
	copy(out, r.list)
	return out
}

// ============================================================================
// PHASE PROGRESS TRACKER
// ============================================================================

// phaseProgress tracks calls to the advance_phase tool.
type phaseProgress struct {
	mu     sync.Mutex
	called bool
}

func newPhaseProgress() *phaseProgress {
	return &phaseProgress{}
}

func (p *phaseProgress) tool() (tool.Tool, error) {
	type noArgs struct{}
	t, err := tool.New("advance_phase", func(_ context.Context, _ noArgs) (string, error) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.called = true
		return "Phase advancement confirmed. Moving to next phase.", nil
	})
	if err != nil {
		return nil, err
	}
	return t.WithDescription("Call this when the team is ready to move to the next phase of the brainstorm."), nil
}

func (p *phaseProgress) shouldAdvance() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.called
}

func (p *phaseProgress) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.called = false
}

// ============================================================================
// HELPERS
// ============================================================================

func (p *brainstorm) resolveScope(msg agent.Message) string {
	if text := msg.Text(); text != "" {
		return text
	}
	if p.cfg.Scope != "" {
		return p.cfg.Scope
	}
	return "Brainstorming session"
}

func (p *brainstorm) selectRunner(sess protocol.Session, agents []agent.Agent) agent.Agent {
	if f := sess.Facilitator(); f != nil {
		return *f
	}
	if len(agents) > 0 {
		return agents[0]
	}
	return agent.Agent{}
}

func (p *brainstorm) recentHistory(ctx context.Context, sess protocol.Session, n int) []agent.Message {
	history, err := sess.History(ctx)
	if err != nil || len(history) == 0 {
		return nil
	}
	if len(history) <= n {
		return history
	}
	return history[len(history)-n:]
}

func toAgents(participants []protocol.Participant) ([]agent.Agent, error) {
	agents := make([]agent.Agent, 0, len(participants))
	for _, p := range participants {
		a, ok := p.Agent()
		if !ok {
			continue
		}
		agents = append(agents, a)
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agent participants found")
	}
	return agents, nil
}

func rotateOrder(agents []agent.Agent, round int) []agent.Agent {
	if len(agents) == 0 {
		return agents
	}
	offset := (round - 1) % len(agents)
	result := make([]agent.Agent, len(agents))
	for i := range agents {
		result[i] = agents[(i+offset)%len(agents)]
	}
	return result
}

// agentNames returns a comma-separated list of agent IDs for injection into prompts.
func agentNames(agents []agent.Agent) string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.ID
	}
	return strings.Join(names, ", ")
}

func parseOutput[T any](msg agent.Message) (T, error) {
	var result T
	text := msg.Text()
	if text == "" {
		for _, part := range msg.Parts {
			if part.Type == agent.ContentPartText || part.Type == "text" {
				text = part.Text
				break
			}
		}
	}
	if text == "" {
		return result, fmt.Errorf("no text content in response")
	}

	// Try to find JSON in the response
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		if idx := strings.Index(text, "```"); idx != -1 {
			text = text[:idx]
		}
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.Index(text, "```"); idx != -1 {
			text = text[:idx]
		}
	}
	text = strings.TrimSpace(text)

	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return result, fmt.Errorf("parse JSON: %w", err)
	}
	return result, nil
}

// agentPersonality extracts the personality prompt from an agent's profile.
// Returns empty string if no profile or prompt is set.
func agentPersonality(a agent.Agent) string {
	if a.Profile != nil && a.Profile.Prompt != "" {
		return a.Profile.Prompt
	}
	return ""
}

// buildAgentSystem creates a system prompt combining personality and base rules.
func buildAgentSystem(a agent.Agent, phaseContext string) string {
	personality := agentPersonality(a)
	if personality == "" {
		personality = "Collaborative brainstorm participant."
	}
	return fmt.Sprintf(`You are %s.

%s

=== YOUR VOICE ===
%s

Write your response in this voice. Your FIRST WORDS set the tone - make them distinctly yours.

=== PHASE ===
%s`, a.ID, brainstormBaseRules, personality, phaseContext)
}
