package ideo

import (
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/insightpack"
)

// Phase identifies a brainstorming phase.
type Phase string

const (
	PhaseInspiration Phase = "inspiration"
	PhaseReframe     Phase = "reframe"
	PhaseIdeation    Phase = "ideation"
	PhaseSynthesis   Phase = "synthesis"
)

// TransferStrategy controls how context moves between sessions.
type TransferStrategy string

const (
	// TransferSummaryOnly includes only a text summary in the next session's system prompt.
	TransferSummaryOnly TransferStrategy = "summary"

	// TransferWithHistory injects key messages into session memory plus summary.
	TransferWithHistory TransferStrategy = "with_history"

	// TransferFull makes all prior data available (for debugging/special cases).
	TransferFull TransferStrategy = "full"
)

// TimeoutBehavior defines what happens when a human doesn't respond.
type TimeoutBehavior string

const (
	// TimeoutContinue proceeds without the human's input.
	TimeoutContinue TimeoutBehavior = "continue"

	// TimeoutFail stops the session.
	TimeoutFail TimeoutBehavior = "fail"

	// TimeoutUseDefault uses a placeholder response.
	TimeoutUseDefault TimeoutBehavior = "use_default"
)

// Stakeholder is a human who can be consulted during synthesis.
type Stakeholder struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Context string `json:"context"` // What they know about
}

// Config holds all configuration for the IDEO brainstorming protocol.
type Config struct {
	// Scope is the problem statement or topic.
	Scope string

	// Session round counts
	InspirationRounds int // Default: 2
	ReframeRounds     int // Default: 3
	IdeationRounds    int // Default: 2
	SynthesisRounds   int // Default: 2

	// Output targets
	TargetHMWs     int // Default: 8
	TargetConcepts int // Default: 15
	FinalistCount  int // Default: 3

	// Context transfer
	TransferStrategy TransferStrategy // Default: summary

	// Tool configuration
	ContextPlan   insightpack.Plan
	ArtifactTools bool // Enable sketch_* tools

	// Human-in-the-loop
	HumanInLoop     bool
	Stakeholders    []Stakeholder
	HumanTimeout    time.Duration   // Default: 5 minutes
	TimeoutBehavior TimeoutBehavior // Default: continue

	// Diversity injection - rotated across rounds
	DisciplineNudges   []string
	MentalModelPrompts []string
	UserVantagePoints  []string
	LensCatalog        []string

	// Runtime params passed to agent runs
	Params map[string]any

	// MaxConcurrent limits parallel agent execution.
	MaxConcurrent int
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		InspirationRounds:  2,
		ReframeRounds:      3,
		IdeationRounds:     2,
		SynthesisRounds:    2,
		TargetHMWs:         8,
		TargetConcepts:     15,
		FinalistCount:      3,
		TransferStrategy:   TransferSummaryOnly,
		ArtifactTools:      true,
		HumanInLoop:        false,
		HumanTimeout:       5 * time.Minute,
		TimeoutBehavior:    TimeoutContinue,
		ContextPlan:        insightpack.DefaultPlan(),
		DisciplineNudges:   defaultDisciplineNudges(),
		MentalModelPrompts: defaultMentalModelPrompts(),
		UserVantagePoints:  defaultUserVantagePoints(),
		LensCatalog:        defaultLensCatalog(),
		MaxConcurrent:      4,
	}
}

func defaultDisciplineNudges() []string {
	return []string{
		"Consider this from an operations perspective—what processes would change?",
		"What would a behavioral economist notice about the incentives here?",
		"How would a service designer map the end-to-end experience?",
		"What does the data tell us vs. what do users actually say?",
		"Think like a systems engineer—what are the dependencies and failure modes?",
		"What would an anthropologist observe about how people actually behave?",
	}
}

func defaultMentalModelPrompts() []string {
	return []string{
		"Systems thinking: What feedback loops exist here?",
		"Jobs-to-be-done: What job is the user hiring this to do?",
		"First principles: What must be true for this to work?",
		"Constraint analysis: What if we removed the biggest constraint?",
		"Second-order effects: What happens after the obvious outcome?",
		"Inversion: What would make this fail spectacularly?",
	}
}

func defaultUserVantagePoints() []string {
	return []string{
		"Power users who know every shortcut",
		"Newcomers seeing this for the first time",
		"Users on the happy path",
		"Users hitting edge cases and errors",
		"Individual contributors working alone",
		"Teams collaborating together",
		"Mobile users on the go",
		"Desktop users with full attention",
	}
}

func defaultLensCatalog() []string {
	return []string{
		"activation",
		"operational",
		"behavioral",
		"workflow",
		"adoption",
		"trust",
		"learning",
		"friction",
		"messaging",
		"economic",
		"systemic",
		"radical",
	}
}

// Option configures the IDEO brainstorming protocol.
type Option func(*Config)

// WithScope sets the problem statement.
func WithScope(scope string) Option {
	return func(c *Config) { c.Scope = scope }
}

// WithInspirationRounds sets rounds for the inspiration phase.
func WithInspirationRounds(n int) Option {
	return func(c *Config) { c.InspirationRounds = n }
}

// WithReframeRounds sets rounds for the reframe phase.
func WithReframeRounds(n int) Option {
	return func(c *Config) { c.ReframeRounds = n }
}

// WithIdeationRounds sets rounds for the ideation phase.
func WithIdeationRounds(n int) Option {
	return func(c *Config) { c.IdeationRounds = n }
}

// WithSynthesisRounds sets rounds for the synthesis phase.
func WithSynthesisRounds(n int) Option {
	return func(c *Config) { c.SynthesisRounds = n }
}

// WithTargetHMWs sets the target number of HMW frames.
func WithTargetHMWs(n int) Option {
	return func(c *Config) { c.TargetHMWs = n }
}

// WithTargetConcepts sets the target number of concepts.
func WithTargetConcepts(n int) Option {
	return func(c *Config) { c.TargetConcepts = n }
}

// WithFinalistCount sets how many concepts make the final portfolio.
func WithFinalistCount(n int) Option {
	return func(c *Config) { c.FinalistCount = n }
}

// WithTransferStrategy sets how context moves between sessions.
func WithTransferStrategy(s TransferStrategy) Option {
	return func(c *Config) { c.TransferStrategy = s }
}

// WithContextPlan sets the insight pack configuration.
func WithContextPlan(plan insightpack.Plan) Option {
	return func(c *Config) { c.ContextPlan = plan }
}

// WithArtifactTools enables or disables sketch_* tools.
func WithArtifactTools(enabled bool) Option {
	return func(c *Config) { c.ArtifactTools = enabled }
}

// WithHumanInLoop enables human consultation during synthesis.
func WithHumanInLoop(enabled bool) Option {
	return func(c *Config) { c.HumanInLoop = enabled }
}

// WithStakeholder adds a human stakeholder for consultation.
func WithStakeholder(s Stakeholder) Option {
	return func(c *Config) { c.Stakeholders = append(c.Stakeholders, s) }
}

// WithStakeholders sets all stakeholders.
func WithStakeholders(stakeholders []Stakeholder) Option {
	return func(c *Config) { c.Stakeholders = stakeholders }
}

// WithHumanTimeout sets how long to wait for human responses.
func WithHumanTimeout(d time.Duration) Option {
	return func(c *Config) { c.HumanTimeout = d }
}

// WithTimeoutBehavior sets what happens when humans don't respond.
func WithTimeoutBehavior(b TimeoutBehavior) Option {
	return func(c *Config) { c.TimeoutBehavior = b }
}

// WithDisciplineNudges sets the discipline prompts for diversity.
func WithDisciplineNudges(nudges []string) Option {
	return func(c *Config) { c.DisciplineNudges = nudges }
}

// WithMentalModelPrompts sets the mental model prompts.
func WithMentalModelPrompts(prompts []string) Option {
	return func(c *Config) { c.MentalModelPrompts = prompts }
}

// WithUserVantagePoints sets the user perspective prompts.
func WithUserVantagePoints(points []string) Option {
	return func(c *Config) { c.UserVantagePoints = points }
}

// WithLensCatalog sets the candidate lenses for semantic stage planning.
func WithLensCatalog(lenses []string) Option {
	return func(c *Config) { c.LensCatalog = lenses }
}

// WithParams sets runtime parameters for agent execution.
func WithParams(params map[string]any) Option {
	return func(c *Config) { c.Params = params }
}

// WithMaxConcurrent sets parallel execution limit.
func WithMaxConcurrent(n int) Option {
	return func(c *Config) { c.MaxConcurrent = n }
}

// TransferPacket carries curated context between sessions.
type TransferPacket struct {
	// Phase that produced this packet
	FromPhase Phase `json:"from_phase"`

	// Structured data for programmatic access
	Data map[string]any `json:"data"`

	// Human-readable summary for system prompts
	Summary string `json:"summary"`

	// Optional: messages to inject into next session's context
	PriorMessages []agent.Message `json:"prior_messages,omitempty"`
}

// Artifact is a structured output from sketch tools.
type Artifact struct {
	Type    string `json:"type"` // mermaid, concept_card, journey, table
	Title   string `json:"title"`
	Content any    `json:"content"` // Type-specific content
	Author  string `json:"author"`
	Context string `json:"context,omitempty"` // Why this artifact was created
}

// ConceptCard is a structured concept artifact.
type ConceptCard struct {
	Title     string `json:"title"`
	Problem   string `json:"problem"`
	Mechanism string `json:"mechanism"`
	Value     string `json:"value"`
	Risk      string `json:"risk"`
}

// StagePlan captures moderator-selected planning constraints for subsequent phases.
type StagePlan struct {
	ProblemStatement string   `json:"problem_statement,omitempty"`
	NonNegotiables   []string `json:"non_negotiables,omitempty"`
	Lenses           []string `json:"lenses,omitempty"`
	ToolIDs          []string `json:"tool_ids,omitempty"`
	Questions        []string `json:"questions,omitempty"`
	Rationale        string   `json:"rationale,omitempty"`
}

// JourneyStage is one step in a user journey.
type JourneyStage struct {
	Name       string `json:"name"`
	UserAction string `json:"user_action"`
	Emotion    string `json:"emotion"`
	Touchpoint string `json:"touchpoint"`
}

// Journey is a user journey artifact.
type Journey struct {
	Title  string         `json:"title"`
	Stages []JourneyStage `json:"stages"`
}
