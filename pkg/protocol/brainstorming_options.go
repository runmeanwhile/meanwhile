package protocol

import (
	"maps"
	"strings"
)

const (
	defaultDivergentRounds   = 1
	defaultInteractionRounds = 2
	defaultIdeaTarget        = 5
	defaultShortlistSize     = 3
	defaultVotesPerAgent     = 3
)

var defaultVoteWeights = []int{3, 2, 1}

// BrainstormingOption configures brainstorming behavior.
type BrainstormingOption func(*brainstormingConfig)

// ScopeRefinementPrompt builds system + user prompts for scope refinement.
type ScopeRefinementPrompt func(userQuestion, configuredScope string) (systemPrompt, userPrompt string)

// ScopeFallback builds a scope message when refinement is not configured.
type ScopeFallback func(userQuestion, configuredScope string) string

// brainstormingConfig holds brainstorming protocol configuration.
type brainstormingConfig struct {
	MaxConcurrent         int
	DivergentRounds       int
	InteractionRounds     int
	IdeaTarget            int
	ShortlistSize         int
	VoteEnabled           bool
	VotesPerAgent         int
	VoteWeights           []int
	Params                map[string]any
	Scope                 string
	Outcome               string
	Briefs                []string
	InterventionPoints    []float64
	ScopeRefinement       ScopeRefinementPrompt
	ScopeFallback         ScopeFallback
	DivergentPrompt       DivergentPromptBuilder
	InteractionPrompt     InteractionPromptBuilder
	VotePrompt            VotePromptBuilder
	ContextMessage        ContextMessageBuilder
	ModeratorOpening      ModeratorOpeningBuilder
	ModeratorSynthesis    ModeratorSynthesisBuilder
	ModeratorInterjection ModeratorInterjectionBuilder
	ModeratorShortlist    ModeratorShortlistBuilder
	ModeratorClosing      ModeratorClosingBuilder
}

// WithBrainstormingConcurrency limits concurrent agent runs.
func WithBrainstormingConcurrency(maxConcurrent int) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if maxConcurrent > 0 {
			cfg.MaxConcurrent = maxConcurrent
		}
	}
}

// WithBrainstormingDivergentRounds sets the number of private divergent rounds.
func WithBrainstormingDivergentRounds(rounds int) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if rounds > 0 {
			cfg.DivergentRounds = rounds
		}
	}
}

// WithBrainstormingInteractionRounds sets the number of interactive discussion rounds.
// Use 0 to skip interaction entirely.
func WithBrainstormingInteractionRounds(rounds int) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if rounds >= 0 {
			cfg.InteractionRounds = rounds
		}
	}
}

// WithBrainstormingIdeaTarget sets the number of ideas per participant in divergence.
func WithBrainstormingIdeaTarget(count int) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if count > 0 {
			cfg.IdeaTarget = count
		}
	}
}

// WithBrainstormingShortlistSize sets the number of ideas to shortlist.
func WithBrainstormingShortlistSize(count int) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if count >= 0 {
			cfg.ShortlistSize = count
		}
	}
}

// WithBrainstormingVoting enables or disables voting.
func WithBrainstormingVoting(enabled bool) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		cfg.VoteEnabled = enabled
	}
}

// WithBrainstormingVotesPerAgent sets the number of votes per participant.
func WithBrainstormingVotesPerAgent(count int) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if count >= 0 {
			cfg.VotesPerAgent = count
		}
	}
}

// WithBrainstormingVoteWeights sets the weighting for ranked votes.
func WithBrainstormingVoteWeights(weights ...int) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if len(weights) == 0 {
			return
		}
		out := make([]int, 0, len(weights))
		for _, weight := range weights {
			if weight <= 0 {
				continue
			}
			out = append(out, weight)
		}
		if len(out) > 0 {
			cfg.VoteWeights = out
		}
	}
}

// WithBrainstormingParams sets default run parameters for agent turns (e.g., temperature).
// Any params explicitly set on an agent will take precedence.
func WithBrainstormingParams(params map[string]any) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if len(params) == 0 {
			return
		}
		cfg.Params = cloneParams(params)
	}
}

// WithBrainstormingScope sets the brainstorming scope description.
func WithBrainstormingScope(scope string) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if strings.TrimSpace(scope) != "" {
			cfg.Scope = scope
		}
	}
}

// WithBrainstormingOutcome sets the intended outcome.
func WithBrainstormingOutcome(outcome string) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if strings.TrimSpace(outcome) != "" {
			cfg.Outcome = outcome
		}
	}
}

// WithBrainstormingBriefs adds brief context snippets.
func WithBrainstormingBriefs(briefs ...string) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		for _, brief := range briefs {
			if strings.TrimSpace(brief) == "" {
				continue
			}
			cfg.Briefs = append(cfg.Briefs, brief)
		}
	}
}

// WithBrainstormingModeratorInterventions sets moderator intervention points.
func WithBrainstormingModeratorInterventions(points ...float64) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		cfg.InterventionPoints = append([]float64(nil), points...)
	}
}

// WithBrainstormingDisableInterjections turns off moderator interjections.
func WithBrainstormingDisableInterjections() BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		cfg.InterventionPoints = []float64{}
	}
}

// WithBrainstormingScopeRefinementPrompt sets the scope refinement prompt builder.
func WithBrainstormingScopeRefinementPrompt(prompt ScopeRefinementPrompt) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if prompt != nil {
			cfg.ScopeRefinement = prompt
		}
	}
}

// WithBrainstormingScopeFallback sets the fallback scope builder.
func WithBrainstormingScopeFallback(fallback ScopeFallback) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if fallback != nil {
			cfg.ScopeFallback = fallback
		}
	}
}

// WithBrainstormingDivergentPrompt sets the divergent prompt builder.
func WithBrainstormingDivergentPrompt(builder DivergentPromptBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.DivergentPrompt = builder
		}
	}
}

// WithBrainstormingInteractionPrompt sets the interaction prompt builder.
func WithBrainstormingInteractionPrompt(builder InteractionPromptBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.InteractionPrompt = builder
		}
	}
}

// WithBrainstormingVotePrompt sets the vote prompt builder.
func WithBrainstormingVotePrompt(builder VotePromptBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.VotePrompt = builder
		}
	}
}

// WithBrainstormingContextMessageBuilder sets the context message builder.
func WithBrainstormingContextMessageBuilder(builder ContextMessageBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.ContextMessage = builder
		}
	}
}

// WithBrainstormingModeratorOpening sets the moderator opening prompt builder.
func WithBrainstormingModeratorOpening(builder ModeratorOpeningBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.ModeratorOpening = builder
		}
	}
}

// WithBrainstormingModeratorSynthesis sets the moderator synthesis prompt builder.
func WithBrainstormingModeratorSynthesis(builder ModeratorSynthesisBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.ModeratorSynthesis = builder
		}
	}
}

// WithBrainstormingModeratorInterjection sets the moderator interjection prompt builder.
func WithBrainstormingModeratorInterjection(builder ModeratorInterjectionBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.ModeratorInterjection = builder
		}
	}
}

// WithBrainstormingModeratorShortlist sets the moderator shortlist prompt builder.
func WithBrainstormingModeratorShortlist(builder ModeratorShortlistBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.ModeratorShortlist = builder
		}
	}
}

// WithBrainstormingModeratorClosing sets the moderator closing prompt builder.
func WithBrainstormingModeratorClosing(builder ModeratorClosingBuilder) BrainstormingOption {
	return func(cfg *brainstormingConfig) {
		if builder != nil {
			cfg.ModeratorClosing = builder
		}
	}
}

func defaultBrainstormingConfig() brainstormingConfig {
	return brainstormingConfig{
		DivergentRounds:       defaultDivergentRounds,
		InteractionRounds:     defaultInteractionRounds,
		IdeaTarget:            defaultIdeaTarget,
		ShortlistSize:         defaultShortlistSize,
		VoteEnabled:           true,
		VotesPerAgent:         defaultVotesPerAgent,
		VoteWeights:           append([]int(nil), defaultVoteWeights...),
		Params:                nil,
		Scope:                 "Generate and refine ideas, then converge on top directions.",
		InterventionPoints:    []float64{0.45, 0.8},
		ScopeRefinement:       defaultBrainstormScopeRefinementPrompt,
		ScopeFallback:         defaultBrainstormScopeFallback,
		DivergentPrompt:       defaultDivergentPrompt,
		InteractionPrompt:     defaultInteractionPrompt,
		VotePrompt:            defaultVotePrompt,
		ContextMessage:        defaultBrainstormingContextMessage,
		ModeratorOpening:      defaultModeratorOpeningPrompt,
		ModeratorSynthesis:    defaultModeratorSynthesisPrompt,
		ModeratorInterjection: defaultModeratorInterjectionPrompt,
		ModeratorShortlist:    defaultModeratorShortlistPrompt,
		ModeratorClosing:      defaultModeratorClosingPrompt,
	}
}

func (c brainstormingConfig) asConfig() Config {
	out := Config{
		"max_concurrent":      c.MaxConcurrent,
		"divergent_rounds":    c.DivergentRounds,
		"interaction_rounds":  c.InteractionRounds,
		"idea_target":         c.IdeaTarget,
		"shortlist_size":      c.ShortlistSize,
		"vote_enabled":        c.VoteEnabled,
		"votes_per_agent":     c.VotesPerAgent,
		"vote_weights":        append([]int(nil), c.VoteWeights...),
		"params":              cloneParams(c.Params),
		"scope":               c.Scope,
		"outcome":             c.Outcome,
		"briefs":              append([]string(nil), c.Briefs...),
		"intervention_points": append([]float64(nil), c.InterventionPoints...),
	}
	return out
}

func cloneParams(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	maps.Copy(out, in)
	return out
}
