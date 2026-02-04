package consensus

import (
	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/agenda"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/chair"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/pulse"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/roundtable"
)

// ContextMessageBuilder builds a context message for participants.
type ContextMessageBuilder func(thread []agent.Message, currentRound, maxRounds int) agent.Message

// AgentPromptBuilder builds a participant system prompt.
type AgentPromptBuilder func(basePrompt, scope string, currentRound, maxRounds int) string

// InterjectionPromptBuilder builds a chair interjection prompt.
type InterjectionPromptBuilder func(input InterjectionInput) chair.Prompt

// ClosingSummaryPromptBuilder builds a closing summary prompt.
type ClosingSummaryPromptBuilder func(input ClosingSummaryInput) chair.Prompt

// BrevityReminder builds a brevity reminder message.
type BrevityReminder func(participant agent.Agent) string

// Config holds consensus protocol configuration.
type Config struct {
	MaxRounds          int
	AgendaOptions      []agenda.Option
	ChairOptions       []chair.Option
	PulseOptions       []pulse.Option
	RoundtableOptions  []roundtable.Option
	ScopeRefinement    agenda.RefinePrompt
	ScopeFallback      agenda.Fallback
	InterjectionPrompt InterjectionPromptBuilder
	ClosingPrompt      ClosingSummaryPromptBuilder
	AgentPrompt        AgentPromptBuilder
	ContextMessage     ContextMessageBuilder
	BrevityReminder    BrevityReminder
	BrevityMinRound    int
	BrevityMaxChars    int
}

// Option configures consensus behavior.
type Option func(*Config)

// WithMaxRounds sets the maximum number of discussion rounds.
func WithMaxRounds(rounds int) Option {
	return func(c *Config) {
		if rounds > 0 {
			c.MaxRounds = rounds
		}
	}
}

// WithScope sets the discussion scope description.
func WithScope(scope string) Option {
	return WithAgenda(agenda.WithScope(scope))
}

// WithScopeRefinementPrompt sets the scope refinement prompt builder.
func WithScopeRefinementPrompt(prompt agenda.RefinePrompt) Option {
	return func(c *Config) {
		if prompt != nil {
			c.ScopeRefinement = prompt
		}
	}
}

// WithScopeFallback sets the fallback scope builder.
func WithScopeFallback(fallback agenda.Fallback) Option {
	return func(c *Config) {
		if fallback != nil {
			c.ScopeFallback = fallback
		}
	}
}

// WithInterjectionPrompt sets the interjection prompt builder.
func WithInterjectionPrompt(builder InterjectionPromptBuilder) Option {
	return func(c *Config) {
		if builder != nil {
			c.InterjectionPrompt = builder
		}
	}
}

// WithClosingSummaryPrompt sets the closing summary prompt builder.
func WithClosingSummaryPrompt(builder ClosingSummaryPromptBuilder) Option {
	return func(c *Config) {
		if builder != nil {
			c.ClosingPrompt = builder
		}
	}
}

// WithAgentPromptBuilder sets the participant system prompt builder.
func WithAgentPromptBuilder(builder AgentPromptBuilder) Option {
	return func(c *Config) {
		if builder != nil {
			c.AgentPrompt = builder
		}
	}
}

// WithContextMessageBuilder sets the context message builder.
func WithContextMessageBuilder(builder ContextMessageBuilder) Option {
	return func(c *Config) {
		if builder != nil {
			c.ContextMessage = builder
		}
	}
}

// WithBrevityReminder sets the brevity reminder builder.
func WithBrevityReminder(reminder BrevityReminder) Option {
	return func(c *Config) {
		c.BrevityReminder = reminder
	}
}

// WithBrevityLimits sets the brevity threshold and minimum round.
func WithBrevityLimits(minRound, maxChars int) Option {
	return func(c *Config) {
		if minRound > 0 {
			c.BrevityMinRound = minRound
		}
		if maxChars > 0 {
			c.BrevityMaxChars = maxChars
		}
	}
}

// WithModeratorInterventions sets intervention points as progress percentages.
// For example, []float64{0.5, 0.8, 0.9} means moderator will interject at
// 50%, 80%, and 90% of max rounds.
func WithModeratorInterventions(points ...float64) Option {
	return WithChair(chair.WithInterventions(points...))
}

// WithAgenda configures the agenda component.
func WithAgenda(opts ...agenda.Option) Option {
	return func(c *Config) {
		c.AgendaOptions = append(c.AgendaOptions, opts...)
	}
}

// WithChair configures the chair component.
func WithChair(opts ...chair.Option) Option {
	return func(c *Config) {
		c.ChairOptions = append(c.ChairOptions, opts...)
	}
}

// WithPulseCheck configures the pulse check component.
func WithPulseCheck(opts ...pulse.Option) Option {
	return func(c *Config) {
		c.PulseOptions = append(c.PulseOptions, opts...)
	}
}

// WithRoundtable configures the roundtable component.
func WithRoundtable(opts ...roundtable.Option) Option {
	return func(c *Config) {
		c.RoundtableOptions = append(c.RoundtableOptions, opts...)
	}
}

// defaultConfig returns default consensus configuration.
func defaultConfig() Config {
	return Config{
		MaxRounds:          10,
		AgendaOptions:      []agenda.Option{agenda.WithScope("Reach consensus on the topic")},
		ScopeRefinement:    defaultScopeRefinementPrompt,
		ScopeFallback:      defaultScopeFallback,
		InterjectionPrompt: defaultInterjectionPrompt,
		ClosingPrompt:      defaultClosingSummaryPrompt,
		AgentPrompt:        buildAgentRefinementPrompt,
		ContextMessage:     defaultContextMessage,
		BrevityReminder:    defaultBrevityReminder,
		BrevityMinRound:    3,
		BrevityMaxChars:    400,
	}
}
