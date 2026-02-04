package agenda

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

// Config configures agenda behavior.
type Config struct {
	Scope string
	// Outcome describes the intended result of the meeting.
	Outcome string
	// Briefs are optional context snippets to anchor discussion.
	Briefs []string
	// Items are optional agenda items to sequence.
	Items []Item
	// RefinePrompt builds system + user prompts for scope refinement.
	RefinePrompt RefinePrompt
	// Fallback builds a scope message when no refinement prompt is configured.
	Fallback Fallback
}

// Option configures agenda behavior.
type Option func(*Config)

// Item captures a structured agenda item.
type Item struct {
	Title   string
	Outcome string
	Timebox time.Duration
	Notes   string
}

// RefinePrompt builds system and user prompts for scope refinement.
type RefinePrompt func(userQuestion, configuredScope string) (systemPrompt, userPrompt string)

// Fallback builds a scope message when refinement is not configured.
type Fallback func(userQuestion, configuredScope string) string

// WithScope sets the agenda scope description.
func WithScope(scope string) Option {
	return func(cfg *Config) {
		if strings.TrimSpace(scope) != "" {
			cfg.Scope = scope
		}
	}
}

// WithOutcome sets the intended outcome.
func WithOutcome(outcome string) Option {
	return func(cfg *Config) {
		if strings.TrimSpace(outcome) != "" {
			cfg.Outcome = outcome
		}
	}
}

// WithBrief adds a brief context snippet.
func WithBrief(brief string) Option {
	return func(cfg *Config) {
		if strings.TrimSpace(brief) != "" {
			cfg.Briefs = append(cfg.Briefs, brief)
		}
	}
}

// WithBriefs adds multiple brief context snippets.
func WithBriefs(briefs ...string) Option {
	return func(cfg *Config) {
		for _, brief := range briefs {
			if strings.TrimSpace(brief) == "" {
				continue
			}
			cfg.Briefs = append(cfg.Briefs, brief)
		}
	}
}

// WithItem appends an agenda item.
func WithItem(item Item) Option {
	return func(cfg *Config) {
		if strings.TrimSpace(item.Title) == "" && strings.TrimSpace(item.Notes) == "" {
			return
		}
		cfg.Items = append(cfg.Items, item)
	}
}

// WithItems appends multiple agenda items.
func WithItems(items ...Item) Option {
	return func(cfg *Config) {
		for _, item := range items {
			if strings.TrimSpace(item.Title) == "" && strings.TrimSpace(item.Notes) == "" {
				continue
			}
			cfg.Items = append(cfg.Items, item)
		}
	}
}

// WithRefinementPrompt sets the prompt builder for scope refinement.
func WithRefinementPrompt(prompt RefinePrompt) Option {
	return func(cfg *Config) {
		if prompt != nil {
			cfg.RefinePrompt = prompt
		}
	}
}

// WithFallback sets the fallback scope builder.
func WithFallback(fallback Fallback) Option {
	return func(cfg *Config) {
		if fallback != nil {
			cfg.Fallback = fallback
		}
	}
}

// Agenda refines discussion scope.
type Agenda struct {
	mu           sync.RWMutex
	cfg          Config
	refinedScope string
	itemIndex    int
}

// State captures agenda progress for checkpointing.
type State struct {
	RefinedScope string `json:"refined_scope"`
	ItemIndex    int    `json:"item_index"`
}

// New creates a new agenda with options applied.
func New(opts ...Option) *Agenda {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Agenda{cfg: cfg, itemIndex: 0}
}

// Scope returns the configured agenda scope.
func (a *Agenda) Scope() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Scope
}

// ResolvedScope returns the refined scope when available, otherwise the configured scope.
func (a *Agenda) ResolvedScope() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if strings.TrimSpace(a.refinedScope) != "" {
		return a.refinedScope
	}
	return a.cfg.Scope
}

// Outcome returns the intended outcome.
func (a *Agenda) Outcome() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Outcome
}

// Briefs returns a copy of the brief snippets.
func (a *Agenda) Briefs() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]string(nil), a.cfg.Briefs...)
}

// Brief returns the combined brief text.
func (a *Agenda) Brief() string {
	briefs := a.Briefs()
	if len(briefs) == 0 {
		return ""
	}
	return strings.Join(briefs, "\n")
}

// Items returns a copy of the agenda items.
func (a *Agenda) Items() []Item {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]Item, len(a.cfg.Items))
	copy(out, a.cfg.Items)
	return out
}

// CurrentItem returns the current agenda item.
func (a *Agenda) CurrentItem() (Item, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.cfg.Items) == 0 {
		return Item{}, false
	}
	idx := a.itemIndex
	if idx < 0 || idx >= len(a.cfg.Items) {
		return Item{}, false
	}
	return a.cfg.Items[idx], true
}

// Advance moves to the next agenda item and returns it if present.
func (a *Agenda) Advance() (Item, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.cfg.Items) == 0 {
		return Item{}, false
	}
	if a.itemIndex+1 >= len(a.cfg.Items) {
		a.itemIndex = len(a.cfg.Items)
		return Item{}, false
	}
	a.itemIndex++
	return a.cfg.Items[a.itemIndex], true
}

// Context returns a structured agenda context block.
func (a *Agenda) Context() string {
	var sb strings.Builder
	scope := a.ResolvedScope()
	if strings.TrimSpace(scope) != "" {
		sb.WriteString("Scope: ")
		sb.WriteString(scope)
		sb.WriteString("\n")
	}
	if outcome := a.Outcome(); strings.TrimSpace(outcome) != "" {
		sb.WriteString("Outcome: ")
		sb.WriteString(outcome)
		sb.WriteString("\n")
	}
	if item, ok := a.CurrentItem(); ok {
		if strings.TrimSpace(item.Title) != "" {
			sb.WriteString("Agenda item: ")
			sb.WriteString(item.Title)
			sb.WriteString("\n")
		}
		if strings.TrimSpace(item.Outcome) != "" {
			sb.WriteString("Item outcome: ")
			sb.WriteString(item.Outcome)
			sb.WriteString("\n")
		}
		if item.Timebox > 0 {
			sb.WriteString("Timebox: ")
			sb.WriteString(item.Timebox.String())
			sb.WriteString("\n")
		}
		if strings.TrimSpace(item.Notes) != "" {
			sb.WriteString("Item notes: ")
			sb.WriteString(item.Notes)
			sb.WriteString("\n")
		}
	}
	if brief := a.Brief(); strings.TrimSpace(brief) != "" {
		sb.WriteString("Brief:\n")
		sb.WriteString(brief)
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// State returns a snapshot of agenda progress.
func (a *Agenda) State() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return State{
		RefinedScope: a.refinedScope,
		ItemIndex:    a.itemIndex,
	}
}

// Restore resets agenda progress from a snapshot.
func (a *Agenda) Restore(state State) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.refinedScope = state.RefinedScope
	a.itemIndex = state.ItemIndex
}

// RefineScope uses the facilitator (if available) to refine scope and caches it.
func (a *Agenda) RefineScope(ctx context.Context, sess protocol.Session, msg agent.Message) (string, error) {
	if cached := a.getRefinedScope(); cached != "" {
		return cached, nil
	}

	userQuestion := msg.Text()
	if strings.TrimSpace(userQuestion) == "" {
		userQuestion = msg.Summary()
	}

	cfg := a.configSnapshot()
	facilitator := sess.Facilitator()
	if facilitator == nil || cfg.RefinePrompt == nil {
		scope := fallbackScope(cfg, userQuestion)
		a.setRefinedScope(scope)
		return scope, nil
	}

	systemPrompt, userPrompt := cfg.RefinePrompt(userQuestion, cfg.Scope)
	if strings.TrimSpace(userPrompt) == "" {
		userPrompt = userQuestion
	}

	scopePrompt := protocol.PromptWithMedia(userPrompt, msg)
	req := protocol.RunRequest{
		Messages:          []agent.Message{scopePrompt},
		MaxToolIterations: 1,
	}
	if strings.TrimSpace(systemPrompt) != "" {
		req.SystemMessages = []agent.Message{message.System(systemPrompt)}
	}

	resp, err := sess.RunAgent(ctx, *facilitator, req)
	if err != nil {
		return "", err
	}

	a.setRefinedScope(resp.Text())
	return resp.Text(), nil
}

func (a *Agenda) getRefinedScope() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.refinedScope
}

func (a *Agenda) setRefinedScope(scope string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.refinedScope = scope
}

func (a *Agenda) configSnapshot() Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

func fallbackScope(cfg Config, userQuestion string) string {
	if cfg.Fallback != nil {
		return cfg.Fallback(userQuestion, cfg.Scope)
	}
	if strings.TrimSpace(cfg.Scope) != "" {
		return cfg.Scope
	}
	return userQuestion
}
