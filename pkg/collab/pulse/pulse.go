package pulse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

const (
	defaultMinRounds          = 2
	defaultMaxConditions      = 3
	defaultMaxConditionLength = 150
	defaultMaxReasoningLength = 300
)

// Config configures pulse check behavior.
type Config struct {
	MinRounds          int
	MaxConditions      int
	MaxConditionLength int
	MaxReasoningLength int
	roundProvider      func() int
}

// Option configures the pulse check.
type Option func(*Config)

// WithMinRounds sets the minimum round before signaling.
func WithMinRounds(minRounds int) Option {
	return func(cfg *Config) {
		if minRounds >= 0 {
			cfg.MinRounds = minRounds
		}
	}
}

// WithMaxConditions caps the number of conditions allowed.
func WithMaxConditions(maxConditions int) Option {
	return func(cfg *Config) {
		if maxConditions > 0 {
			cfg.MaxConditions = maxConditions
		}
	}
}

// WithMaxConditionLength caps the length of any single condition.
func WithMaxConditionLength(maxLen int) Option {
	return func(cfg *Config) {
		if maxLen > 0 {
			cfg.MaxConditionLength = maxLen
		}
	}
}

// WithMaxReasoningLength caps the length of the reasoning string.
func WithMaxReasoningLength(maxLen int) Option {
	return func(cfg *Config) {
		if maxLen > 0 {
			cfg.MaxReasoningLength = maxLen
		}
	}
}

// WithRoundProvider supplies the current round for validation.
func WithRoundProvider(provider func() int) Option {
	return func(cfg *Config) {
		cfg.roundProvider = provider
	}
}

// PulseCheck manages position signaling state.
type PulseCheck struct {
	mu        sync.RWMutex
	cfg       Config
	positions map[string]*AgentPosition
}

// Snapshot captures pulse positions for checkpointing.
type Snapshot struct {
	Positions []AgentPosition `json:"positions"`
}

// New creates a pulse check initialized with participants.
func New(participants []agent.Agent, opts ...Option) *PulseCheck {
	cfg := Config{
		MinRounds:          defaultMinRounds,
		MaxConditions:      defaultMaxConditions,
		MaxConditionLength: defaultMaxConditionLength,
		MaxReasoningLength: defaultMaxReasoningLength,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	positions := make(map[string]*AgentPosition, len(participants))
	for _, ag := range participants {
		positions[ag.Name] = &AgentPosition{
			Agent:     ag.Name,
			Position:  PositionPending,
			Timestamp: time.Now(),
		}
	}
	return &PulseCheck{cfg: cfg, positions: positions}
}

// Register registers the pulse tool and adds it as a default tool.
func (p *PulseCheck) Register(sess protocol.Session) error {
	toolImpl, err := p.createSignalTool()
	if err != nil {
		return fmt.Errorf("create signal tool: %w", err)
	}
	if err := sess.RegisterTool(toolImpl); err != nil {
		return fmt.Errorf("register signal tool: %w", err)
	}
	sess.AddDefaultTools(toolImpl.ID())
	return nil
}

// RecordPosition updates an agent's position.
func (p *PulseCheck) RecordPosition(agentName string, pos Position, reasoning string, conditions []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if agentPos, exists := p.positions[agentName]; exists {
		agentPos.Position = pos
		agentPos.Reasoning = reasoning
		agentPos.Conditions = append([]string(nil), conditions...)
		agentPos.Timestamp = time.Now()
	}
}

// HasSignaled checks if a specific agent has signaled a position.
func (p *PulseCheck) HasSignaled(agentName string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if pos, exists := p.positions[agentName]; exists {
		return pos.Position != PositionPending
	}
	return false
}

// AllSignaled checks if all agents have signaled a position.
func (p *PulseCheck) AllSignaled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, pos := range p.positions {
		if pos.Position == PositionPending {
			return false
		}
	}
	return true
}

// HasBlockers checks if any agent has blocked.
func (p *PulseCheck) HasBlockers() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, pos := range p.positions {
		if pos.Position == PositionBlock {
			return true
		}
	}
	return false
}

// BlockingIssues collects all blocking issues.
func (p *PulseCheck) BlockingIssues() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	issues := make([]string, 0)
	for _, pos := range p.positions {
		if pos.Position == PositionBlock && pos.Reasoning != "" {
			issues = append(issues, pos.Agent+": "+pos.Reasoning)
		}
	}
	return issues
}

// Conditions aggregates all conditions from conditional agreements.
func (p *PulseCheck) Conditions() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	conditions := make([]string, 0)
	for _, pos := range p.positions {
		if pos.Position == PositionConditional {
			conditions = append(conditions, pos.Conditions...)
		}
	}
	return conditions
}

// Positions returns a copy of all agent positions.
func (p *PulseCheck) Positions() []AgentPosition {
	p.mu.RLock()
	defer p.mu.RUnlock()

	positions := make([]AgentPosition, 0, len(p.positions))
	for _, pos := range p.positions {
		copyPos := *pos
		if copyPos.Conditions != nil {
			copyPos.Conditions = append([]string(nil), copyPos.Conditions...)
		}
		positions = append(positions, copyPos)
	}
	return positions
}

// Snapshot returns a copy of the current pulse positions.
func (p *PulseCheck) Snapshot() Snapshot {
	return Snapshot{Positions: p.Positions()}
}

// Restore resets pulse positions from a snapshot.
func (p *PulseCheck) Restore(snapshot Snapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.positions == nil {
		p.positions = make(map[string]*AgentPosition)
	} else {
		for key := range p.positions {
			delete(p.positions, key)
		}
	}
	for _, pos := range snapshot.Positions {
		copyPos := pos
		if copyPos.Conditions != nil {
			copyPos.Conditions = append([]string(nil), copyPos.Conditions...)
		}
		p.positions[copyPos.Agent] = &copyPos
	}
}

// State determines the current consensus-style state.
func (p *PulseCheck) State(currentRound, maxRounds int) State {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if currentRound < maxRounds && !p.allSignaledLocked() {
		return StateInProgress
	}

	if p.hasBlockersLocked() {
		return StateBlocked
	}

	allAgree := true
	hasConditional := false
	for _, pos := range p.positions {
		switch pos.Position {
		case PositionBlock:
			return StateBlocked
		case PositionConditional:
			hasConditional = true
		case PositionPending:
			return StateUnresolved
		case PositionAgree, PositionAbstain:
			// continue
		default:
			allAgree = false
		}
	}

	if hasConditional {
		return StateConditional
	}

	if allAgree {
		return StateFullAgreement
	}

	return StateUnresolved
}

func (p *PulseCheck) allSignaledLocked() bool {
	for _, pos := range p.positions {
		if pos.Position == PositionPending {
			return false
		}
	}
	return true
}

func (p *PulseCheck) hasBlockersLocked() bool {
	for _, pos := range p.positions {
		if pos.Position == PositionBlock {
			return true
		}
	}
	return false
}

// SignalPositionArgs defines the arguments for the signal_position tool.
type SignalPositionArgs struct {
	Position   string   `json:"position" description:"Your position: agree, conditional, block, or abstain"`
	Reasoning  string   `json:"reasoning" description:"Explanation of your position"`
	Conditions []string `json:"conditions,omitempty" description:"Required conditions (only for conditional position)"`
}

type signalTool struct {
	id     string
	schema tool.Schema
	pulse  *PulseCheck
}

func (t *signalTool) ID() string {
	return t.id
}

func (t *signalTool) Schema() tool.Schema {
	return t.schema
}

func (t *signalTool) Run(_ context.Context, call tool.Call, _ tool.Emitter) (tool.Result, error) {
	var args SignalPositionArgs
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return tool.ErrorResult(call, fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	currentRound := 0
	if t.pulse.cfg.roundProvider != nil {
		currentRound = t.pulse.cfg.roundProvider()
	}
	if currentRound < t.pulse.cfg.MinRounds {
		return tool.TextResult(call, "Too early to signal. Let's discuss more first - we need to hear everyone's perspectives and concerns before committing to positions. Continue the conversation."), nil
	}

	pos, err := parsePosition(args.Position)
	if err != nil {
		return tool.ErrorResult(call, err.Error()), nil
	}

	if pos == PositionConditional {
		if len(args.Conditions) == 0 {
			return tool.ErrorResult(call, "conditional position requires at least one condition"), nil
		}
		if len(args.Conditions) > t.pulse.cfg.MaxConditions {
			return tool.TextResult(call, fmt.Sprintf("Too many conditions. List max %d high-level principles, not detailed requirements.", t.pulse.cfg.MaxConditions)), nil
		}
		for _, cond := range args.Conditions {
			if len(cond) > t.pulse.cfg.MaxConditionLength {
				return tool.TextResult(call, fmt.Sprintf("Conditions too detailed. Keep each under %d chars. State principles, not implementation.", t.pulse.cfg.MaxConditionLength)), nil
			}
		}
	}

	if len(args.Reasoning) > t.pulse.cfg.MaxReasoningLength {
		return tool.TextResult(call, fmt.Sprintf("Reasoning too long. Keep it under %d characters. Be concise.", t.pulse.cfg.MaxReasoningLength)), nil
	}

	t.pulse.RecordPosition(call.AgentID, pos, args.Reasoning, args.Conditions)

	content := fmt.Sprintf("Position recorded: %s. Thank you for signaling.", pos)
	if pos == PositionConditional {
		content += fmt.Sprintf(" You specified %d condition(s).", len(args.Conditions))
	}

	return tool.TextResult(call, content), nil
}

func (p *PulseCheck) createSignalTool() (tool.Tool, error) {
	typedTool, err := tool.New[SignalPositionArgs, string]("signal_position", func(_ context.Context, _ SignalPositionArgs) (string, error) { return "", nil })
	if err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &signalTool{
		id:     "signal_position",
		schema: typedTool.Schema(),
		pulse:  p,
	}, nil
}

func parsePosition(value string) (Position, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "agree":
		return PositionAgree, nil
	case "conditional":
		return PositionConditional, nil
	case "block":
		return PositionBlock, nil
	case "abstain":
		return PositionAbstain, nil
	default:
		return PositionPending, fmt.Errorf("invalid position: %s (must be: agree, conditional, block, or abstain)", value)
	}
}
