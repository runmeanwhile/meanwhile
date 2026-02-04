package chair

import (
	"context"
	"fmt"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

const (
	defaultInterventionPoint1 = 0.5
	defaultInterventionPoint2 = 0.8
	defaultInterventionPoint3 = 0.9
)

// Config controls chair behavior.
type Config struct {
	InterventionPoints []float64
}

// Option configures the chair.
type Option func(*Config)

// WithInterventions sets intervention points as progress percentages.
func WithInterventions(points ...float64) Option {
	return func(cfg *Config) {
		if len(points) > 0 {
			cfg.InterventionPoints = append([]float64(nil), points...)
		}
	}
}

// Prompt defines the input for a facilitator run.
type Prompt struct {
	System            string
	User              string
	MaxToolIterations int
}

// Chair moderates interventions in a discussion.
type Chair struct {
	mu                sync.Mutex
	interventionPts   []float64
	interventionsDone map[float64]bool
}

// State captures completed interventions for checkpointing.
type State struct {
	InterventionsDone []float64 `json:"interventions_done"`
}

// New creates a chair with default or configured intervention points.
func New(opts ...Option) *Chair {
	cfg := Config{InterventionPoints: []float64{defaultInterventionPoint1, defaultInterventionPoint2, defaultInterventionPoint3}}
	for _, opt := range opts {
		opt(&cfg)
	}
	points := cfg.InterventionPoints
	if len(points) == 0 {
		points = []float64{defaultInterventionPoint1, defaultInterventionPoint2, defaultInterventionPoint3}
	}
	return &Chair{interventionPts: points, interventionsDone: make(map[float64]bool)}
}

// ShouldInterject checks if the chair should speak at current progress.
func (c *Chair) ShouldInterject(currentRound, maxRounds int) (bool, float64) {
	if maxRounds == 0 {
		return false, 0
	}

	progress := float64(currentRound) / float64(maxRounds)

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, point := range c.interventionPts {
		if progress >= point && !c.interventionsDone[point] {
			c.interventionsDone[point] = true
			return true, point
		}
	}

	return false, 0
}

// State returns a snapshot of chair progress.
func (c *Chair) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	done := make([]float64, 0, len(c.interventionsDone))
	for point := range c.interventionsDone {
		done = append(done, point)
	}
	return State{InterventionsDone: done}
}

// Restore resets chair progress from a snapshot.
func (c *Chair) Restore(state State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interventionsDone = make(map[float64]bool, len(state.InterventionsDone))
	for _, point := range state.InterventionsDone {
		c.interventionsDone[point] = true
	}
}

// RunPrompt executes a facilitator run with the provided prompt.
func (c *Chair) RunPrompt(ctx context.Context, sess protocol.Session, facilitator agent.Agent, prompt Prompt) (string, error) {
	req := protocol.RunRequest{
		Messages:          []agent.Message{message.User(prompt.User)},
		MaxToolIterations: prompt.MaxToolIterations,
	}
	if req.MaxToolIterations <= 0 {
		req.MaxToolIterations = 1
	}
	if prompt.System != "" {
		req.SystemMessages = []agent.Message{message.System(prompt.System)}
	}

	resp, err := sess.RunAgent(ctx, facilitator, req)
	if err != nil {
		return "", fmt.Errorf("run chair prompt: %w", err)
	}
	return resp.Text(), nil
}
