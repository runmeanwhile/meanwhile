package planning

import (
	"context"
	"fmt"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

// Planner creates structured implementation plans.
type Planner struct {
	agent        agent.Agent
	systemPrompt string
	storage      StorageConfig

	mu    sync.RWMutex
	state State
}

// State represents the current planning state.
type State string

const (
	StateIdle     State = "idle"
	StatePlanning State = "planning"
	StateComplete State = "complete"
	StateFailed   State = "failed"
)

// Storage defines where to persist plans.
type Storage string

const (
	StorageReturn  Storage = "return"  // Just return plan to caller
	StorageSession Storage = "session" // Store in session metadata
	StorageMemory  Storage = "memory"  // Persist in memory store
	StorageCustom  Storage = "custom"  // Use custom storage function
)

// StorageConfig configures plan storage.
type StorageConfig struct {
	Strategy Storage
	Key      string // Metadata key for session storage
	Custom   func(context.Context, protocol.Session, *Plan) error
}

// New creates a planner with the given agent and options.
func New(agent agent.Agent, opts ...Option) *Planner {
	p := &Planner{
		agent:   agent,
		storage: StorageConfig{Strategy: StorageReturn},
		state:   StateIdle,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Option configures planner behavior.
type Option func(*Planner)

// WithSystemPrompt sets a custom system prompt for planning.
func WithSystemPrompt(prompt string) Option {
	return func(p *Planner) {
		p.systemPrompt = prompt
	}
}

// WithStorage configures plan storage strategy.
func WithStorage(strategy Storage, key string) Option {
	return func(p *Planner) {
		p.storage = StorageConfig{
			Strategy: strategy,
			Key:      key,
		}
	}
}

// WithCustomStorage sets a custom storage function.
func WithCustomStorage(fn func(context.Context, protocol.Session, *Plan) error) Option {
	return func(p *Planner) {
		p.storage = StorageConfig{
			Strategy: StorageCustom,
			Custom:   fn,
		}
	}
}

// State returns the current planner state.
func (p *Planner) State() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

func (p *Planner) setState(state State) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = state
}

// CreatePlan invokes the planning agent to create a structured plan.
func (p *Planner) CreatePlan(ctx context.Context, sess protocol.Session, msg agent.Message) (*Plan, error) {
	p.setState(StatePlanning)

	// Emit planning started event
	_ = sess.EmitWithContext(ctx, event.New("planning.started", sess.ID(), map[string]any{
		"agent": p.agent.Name,
	}))

	// Build system messages
	systemMessages := p.systemMessages()

	// Run planning agent
	resp, err := sess.RunAgent(ctx, p.agent, protocol.RunRequest{
		Messages:          []agent.Message{msg},
		SystemMessages:    systemMessages,
		MaxToolIterations: 1,
	})
	if err != nil {
		p.setState(StateFailed)
		return nil, fmt.Errorf("run planning agent: %w", err)
	}

	// Parse plan from response
	plan, err := ParsePlan(resp.Text())
	if err != nil {
		p.setState(StateFailed)
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	// Emit plan created event
	_ = sess.EmitWithContext(ctx, event.New("planning.plan_created", sess.ID(), map[string]any{
		"plan": plan,
	}))

	// Store plan
	if err := p.storePlan(ctx, sess, plan); err != nil {
		p.setState(StateFailed)
		return nil, fmt.Errorf("store plan: %w", err)
	}

	p.setState(StateComplete)
	return plan, nil
}

// systemMessages returns system messages for planning.
func (p *Planner) systemMessages() []agent.Message {
	prompt := p.systemPrompt
	if prompt == "" {
		prompt = defaultPlanningPrompt
	}
	return []agent.Message{message.System(prompt)}
}

const defaultPlanningPrompt = `You are a planning specialist. Create a detailed, structured implementation plan.

Output the plan as a JSON object with this structure:
{
    "title": "Plan title",
    "summary": "Brief overview of the plan",
    "steps": [
        {
            "id": "step-1",
            "title": "Step title",
            "description": "Detailed description of what to do",
            "dependencies": []
        }
    ]
}

Be specific and actionable. Break down complex tasks into clear, sequential steps.
Each step should have a clear title and description of what needs to be done.`

// storePlan stores the plan based on configured strategy.
func (p *Planner) storePlan(ctx context.Context, sess protocol.Session, plan *Plan) error {
	switch p.storage.Strategy {
	case StorageReturn:
		// No storage, just return
		return nil

	case StorageSession:
		// Store in session metadata
		key := p.storage.Key
		if key == "" {
			key = "plan"
		}
		sess.Metadata()[key] = plan
		return nil

	case StorageMemory:
		// Persist in memory store as event
		return sess.EmitWithContext(ctx, event.New("planning.plan_stored", sess.ID(), map[string]any{
			"plan": plan,
			"type": "plan_artifact",
		}))

	case StorageCustom:
		// Use custom storage function
		if p.storage.Custom == nil {
			return fmt.Errorf("custom storage function not configured")
		}
		return p.storage.Custom(ctx, sess, plan)

	default:
		return fmt.Errorf("unknown storage strategy: %s", p.storage.Strategy)
	}
}

// GetPlan retrieves a plan from session metadata.
func GetPlan(sess protocol.Session, key string) (*Plan, bool) {
	if key == "" {
		key = "plan"
	}
	p, ok := sess.Metadata()[key].(*Plan)
	return p, ok
}
