package protocol

import (
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/collab/insightpack"
)

// BrainstormingLabOption configures the brainstorming lab protocol.
type BrainstormingLabOption func(*brainstormingLabConfig)

type brainstormingLabConfig struct {
	Scope             string
	ScopeRefinement   ScopeRefinementPrompt
	ScopeFallback     ScopeFallback
	ContextPlan       insightpack.Plan
	DiscoveryRounds   int
	DiscoveryRetries  int
	ChallengeRounds   int
	InteractionRounds int
	CritiqueRounds    int
	FrameTarget       int
	FinalistCount     int
	Params            map[string]any
}

// WithBrainstormingLabScope sets the lab scope.
func WithBrainstormingLabScope(scope string) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if strings.TrimSpace(scope) != "" {
			cfg.Scope = scope
		}
	}
}

// WithBrainstormingLabScopeRefinementPrompt overrides scope refinement prompt behavior.
func WithBrainstormingLabScopeRefinementPrompt(prompt ScopeRefinementPrompt) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if prompt != nil {
			cfg.ScopeRefinement = prompt
		}
	}
}

// WithBrainstormingLabScopeFallback overrides scope fallback behavior.
func WithBrainstormingLabScopeFallback(fallback ScopeFallback) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if fallback != nil {
			cfg.ScopeFallback = fallback
		}
	}
}

// WithBrainstormingLabContextPlan sets the context intake plan.
func WithBrainstormingLabContextPlan(plan insightpack.Plan) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		cfg.ContextPlan = plan
	}
}

// WithBrainstormingLabContextStrategy updates only the strategy.
func WithBrainstormingLabContextStrategy(strategy insightpack.Strategy) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		cfg.ContextPlan.Strategy = strategy
	}
}

// WithBrainstormingLabContextSource appends one source to the context plan.
func WithBrainstormingLabContextSource(source insightpack.Source) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if strings.TrimSpace(source.ID) == "" {
			return
		}
		cfg.ContextPlan.Sources = append(cfg.ContextPlan.Sources, source)
	}
}

// WithBrainstormingLabQuestions appends focus questions for context intake.
func WithBrainstormingLabQuestions(questions ...string) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		for _, q := range questions {
			if strings.TrimSpace(q) == "" {
				continue
			}
			cfg.ContextPlan.Questions = append(cfg.ContextPlan.Questions, q)
		}
	}
}

// WithBrainstormingLabToolBudget sets max tool iterations for context intake.
func WithBrainstormingLabToolBudget(maxToolIterations int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if maxToolIterations > 0 {
			cfg.ContextPlan.Budget.MaxToolIterations = maxToolIterations
		}
	}
}

// WithBrainstormingLabSourceBudget sets max number of source chunks for context intake.
func WithBrainstormingLabSourceBudget(maxSources int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if maxSources > 0 {
			cfg.ContextPlan.Budget.MaxSources = maxSources
		}
	}
}

// WithBrainstormingLabInteractionRounds sets concept interaction rounds.
func WithBrainstormingLabInteractionRounds(rounds int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if rounds > 0 {
			cfg.InteractionRounds = rounds
		}
	}
}

// WithBrainstormingLabDiscoveryRounds sets discovery interaction rounds.
func WithBrainstormingLabDiscoveryRounds(rounds int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if rounds > 0 {
			cfg.DiscoveryRounds = rounds
		}
	}
}

// WithBrainstormingLabDiscoveryRetries sets revision attempts for discovery quality gates.
func WithBrainstormingLabDiscoveryRetries(retries int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if retries >= 0 {
			cfg.DiscoveryRetries = retries
		}
	}
}

// WithBrainstormingLabChallengeRounds sets number of explicit challenge rounds.
func WithBrainstormingLabChallengeRounds(rounds int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if rounds >= 0 {
			cfg.ChallengeRounds = rounds
		}
	}
}

// WithBrainstormingLabCritiqueRounds sets number of critique rounds after ideation.
func WithBrainstormingLabCritiqueRounds(rounds int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if rounds >= 0 {
			cfg.CritiqueRounds = rounds
		}
	}
}

// WithBrainstormingLabFrameTarget sets number of reframe candidates to generate.
func WithBrainstormingLabFrameTarget(target int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if target > 0 {
			cfg.FrameTarget = target
		}
	}
}

// WithBrainstormingLabFinalistCount sets number of finalists produced.
func WithBrainstormingLabFinalistCount(count int) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if count > 0 {
			cfg.FinalistCount = count
		}
	}
}

// WithBrainstormingLabParams sets default run params for turns.
func WithBrainstormingLabParams(params map[string]any) BrainstormingLabOption {
	return func(cfg *brainstormingLabConfig) {
		if len(params) == 0 {
			return
		}
		cfg.Params = cloneParams(params)
	}
}

func defaultBrainstormingLabConfig() brainstormingLabConfig {
	return brainstormingLabConfig{
		Scope:             "Generate reframed, experiment-ready concepts grounded in available context.",
		ScopeRefinement:   defaultBrainstormScopeRefinementPrompt,
		ScopeFallback:     defaultBrainstormScopeFallback,
		ContextPlan:       insightpack.DefaultPlan(),
		DiscoveryRounds:   1,
		DiscoveryRetries:  1,
		ChallengeRounds:   1,
		InteractionRounds: 3,
		CritiqueRounds:    1,
		FrameTarget:       8,
		FinalistCount:     3,
		Params:            nil,
	}
}

func (c brainstormingLabConfig) asConfig() Config {
	return Config{
		"discovery_rounds":   c.DiscoveryRounds,
		"discovery_retries":  c.DiscoveryRetries,
		"challenge_rounds":   c.ChallengeRounds,
		"scope":              c.Scope,
		"context_plan":       c.ContextPlan,
		"interaction_rounds": c.InteractionRounds,
		"critique_rounds":    c.CritiqueRounds,
		"frame_target":       c.FrameTarget,
		"finalist_count":     c.FinalistCount,
		"params":             cloneParams(c.Params),
	}
}
