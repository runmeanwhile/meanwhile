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
	system := `You are the brainstorm moderator. Before convening the team, gather context about the problem.

Use tools to find:
- Current metrics and baselines
- Known constraints and prior decisions
- User feedback and pain points
- What's been tried before

After gathering context, share what you learned with the team. Be specific - include actual numbers, source references, and concrete findings. Do NOT summarize away the details.

End with: "The team is ready to begin inspiration."`

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(fmt.Sprintf("Topic: %s\n\nGather context using your tools, then brief the team.", scope))},
		SystemMessages:    []agent.Message{message.System(system)},
		MaxToolIterations: p.cfg.MaxToolIterations * 2,
		Tools:             p.cfg.ToolIDs,
	})
	return err
}

// ============================================================================
// PHASE 1: INSPIRATION
// ============================================================================

func (p *brainstorm) runInspiration(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, progress *phaseProgress) error {
	runner := p.selectRunner(sess, agents)

	// Kick off inspiration phase
	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages: []agent.Message{message.User("Begin INSPIRATION phase. The team will now observe and investigate before proposing solutions.")},
		SystemMessages: []agent.Message{message.System(`Kick off inspiration. No markdown, just talk.

Quick reminder to the team: We're investigating first, not solving. Find tensions, contradictions, real numbers. No solutions yet.

End with something like: "Alright, let's dig in. Strategist, kick us off - what have you found?"`)},
		MaxToolIterations: 1,
	})
	if err != nil {
		return err
	}

	// Loop until Moderator signals readiness to move on
	for iteration := 1; iteration <= maxIterationsPerPhase; iteration++ {
		// Each agent takes a turn
		for i, participant := range rotateOrder(agents, iteration) {
			history := p.recentHistory(ctx, sess, 15)

			system := fmt.Sprintf(`You're in a brainstorm about: %s

This is INSPIRATION - we're investigating, not solving yet.

STYLE: Talk like you're in a real meeting. No markdown, no headers, no bullet points. Just speak naturally in 2-4 sentences.

MUST DO:
- Respond to the previous speaker by name. Agree, disagree, or build.
- Share ONE concrete finding with a source: "I found that 68%% churn at blank canvas [source: onboarding-state.md]"
- End with a question to someone specific: "Critic, what do you think about...?"

Keep it punchy. One insight, one question.`, scope)

			_, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          append(history, message.User(fmt.Sprintf("Your turn (%d/%d). Investigate and share findings.", i+1, len(agents)))),
				SystemMessages:    []agent.Message{message.System(system)},
				MaxToolIterations: p.cfg.MaxToolIterations,
				Tools:             p.cfg.ToolIDs,
			})
			if err != nil {
				return err
			}
		}

		// Moderator checkpoint: should we continue or progress?
		progress.reset()
		history := p.recentHistory(ctx, sess, 12)
		_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
			Messages: append(history, message.User("Quick check: Are we ready to reframe, or do we need more digging?")),
			SystemMessages: []agent.Message{message.System(`Quick pulse check. No markdown.

Do we have concrete numbers and sources? Are we finding new stuff or repeating ourselves?

If we have enough evidence, call the advance_phase tool.
If not, tell [Name] what we still need and keep going.`)},
			Tools:             []string{"advance_phase"},
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

	// Transition to reframe
	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages: []agent.Message{message.User("Transition to REFRAME phase. The team will now generate How-Might-We questions.")},
		SystemMessages: []agent.Message{message.System(`Transition to REFRAME. No markdown, just talk.

Quickly recap what we found - mention 2-3 specific numbers and sources. Then explain: now we turn problems into "How Might We" questions. The key is embedding the evidence IN the question - not "How might we improve onboarding" but "Given that 68% abandon at blank canvas, how might we..."

End with: "Builder, start us off - give me an HMW from a practical angle."`)},
		MaxToolIterations: 1,
	})
	if err != nil {
		return err
	}

	lenses := []string{"user_experience", "business_model", "technology", "behavioral", "operational", "emotional"}

	// Loop until Moderator signals readiness to move on
	for iteration := 1; iteration <= maxIterationsPerPhase; iteration++ {
		for i, participant := range rotateOrder(agents, iteration) {
			history := p.recentHistory(ctx, sess, 12)

			lens := lenses[(iteration+i)%len(lenses)]

			system := fmt.Sprintf(`You're in a brainstorm about: %s
Your lens this turn: %s

This is REFRAME - we're turning findings into "How Might We" questions.

STYLE: Talk like you're in a meeting. No markdown. Speak naturally in 2-3 sentences after submitting.

MUST DO:
- React to the previous speaker's HMW first. "I like [Name]'s framing..." or "I'd push back on that because..."
- Submit 1-2 HMWs using submit_hmw tool
- CRITICAL: Embed the number IN the question. Not "How might we improve onboarding?" but "Given that 68%% abandon at blank canvas, how might we..."
- Include evidence_refs with source path

After submitting, briefly explain your thinking in casual speech.`, scope, lens)

			_, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          append(history, message.User(fmt.Sprintf("Your turn (%d/%d). Submit HMWs using submit_hmw.", i+1, len(agents)))),
				SystemMessages:    []agent.Message{message.System(system)},
				MaxToolIterations: 4,
				Tools:             []string{"submit_hmw"},
			})
			if err != nil {
				return err
			}
		}

		// Moderator checkpoint: should we continue or progress?
		progress.reset()
		history := p.recentHistory(ctx, sess, 10)
		_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
			Messages: append(history, message.User("Quick check: Are our HMWs strong enough for ideation?")),
			SystemMessages: []agent.Message{message.System(`Quick pulse check. No markdown.

Do we have 4-6 HMWs with real numbers embedded? Different angles covered?

If the HMWs are solid, call the advance_phase tool.
If not, tell [Name] we need an HMW from [angle].`)},
			Tools:             []string{"advance_phase"},
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

	// Transition to ideation
	history := p.recentHistory(ctx, sess, 8)
	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages: append(history, message.User("Transition to IDEATION phase. Share the selected HMWs and kick off concept generation.")),
		SystemMessages: []agent.Message{message.System(`Transition to IDEATION. No markdown, just talk.

Mention 2-3 strong HMWs, then tell the team: time to generate concepts. Build on each other, challenge each other, get wild. We'll stress-test later.

End with something like: "Alright, who wants to take a swing at [specific HMW]? Get weird with it."`)},
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
			history := p.recentHistory(ctx, sess, 10)

			operator := operators[(iteration+i)%len(operators)]

			system := fmt.Sprintf(`You're in a brainstorm about: %s
Creative operator this turn: %s

This is IDEATION - we're generating wild concepts.

STYLE: Talk like you're riffing in a meeting. No markdown. Keep it casual, 2-3 sentences after submitting.

MUST DO:
- React to someone's previous concept first. "I love Builder's template idea, but what if we..." or "Wait, that assumes X - let me flip it..."
- Submit 1-2 concepts using submit_concept tool
- Embed the metric: "Since 68%% abandon at blank canvas..." not "users struggle"
- Apply your creative operator to push past the obvious

After submitting, quick explanation of how you built on or challenged others.`, scope, operator)

			_, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          append(history, message.User(fmt.Sprintf("Your turn (%d/%d). Submit concepts using submit_concept.", i+1, len(agents)))),
				SystemMessages:    []agent.Message{message.System(system)},
				MaxToolIterations: 4,
				Tools:             []string{"submit_concept"},
			})
			if err != nil {
				return err
			}
		}

		// Moderator checkpoint: should we continue or progress?
		progress.reset()
		history := p.recentHistory(ctx, sess, 10)
		_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
			Messages: append(history, message.User("Quick check: Enough concepts to stress-test, or keep going?")),
			SystemMessages: []agent.Message{message.System(`Quick pulse check. No markdown.

Got 6-10 diverse concepts? Still generating fresh stuff or getting repetitive?

If we have enough variety, call the advance_phase tool.
If not, tell [Name] to give us something wild using [operator].`)},
			Tools:             []string{"advance_phase"},
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
	// Transition to synthesis
	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages: []agent.Message{message.User("Begin SYNTHESIS phase. Time to critique concepts and build experiment cards.")},
		SystemMessages: []agent.Message{message.System(`Transition to SYNTHESIS. No markdown, just talk.

Now we stress-test ideas. Tell the team: be brutal, find the flaws, challenge each other by name. If you agree too easily, you're not thinking hard enough.

End with something like: "[Name], go first - which concept has the shakiest foundation? Don't be polite."`)},
		MaxToolIterations: 1,
	})
	if err != nil {
		return nil, err
	}

	// Critique round
	for i, participant := range agents {
		history := p.recentHistory(ctx, sess, 12)

		system := `This is CRITIQUE - we're stress-testing concepts before investing.

STYLE: Talk like you're poking holes in a meeting. No markdown. Be direct and brief, 3-4 sentences.

MUST DO:
- Respond to whoever spoke before you. "Critic raised a good point about X, and I'd add..."
- Pick 1-2 concepts and find the flaw
- Name the core assumption crisply
- What's the cheapest test? Under a week.
- What number would tell us success vs failure?

Be constructively brutal. Don't be polite.`

		resp, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
			Messages:          append(history, message.User(fmt.Sprintf("Your critique turn (%d/%d).", i+1, len(agents)))),
			SystemMessages:    []agent.Message{message.System(system)},
			MaxToolIterations: p.cfg.MaxToolIterations,
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
