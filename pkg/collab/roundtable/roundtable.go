package roundtable

import (
	"fmt"
	"strings"
	"sync"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

const defaultMaxRounds = 10

// Config configures roundtable behavior.
type Config struct {
	MaxRounds int
}

// Option configures a roundtable.
type Option func(*Config)

// WithMaxRounds sets the maximum number of rounds.
func WithMaxRounds(maxRounds int) Option {
	return func(cfg *Config) {
		if maxRounds > 0 {
			cfg.MaxRounds = maxRounds
		}
	}
}

// Roundtable tracks discussion rounds and messages.
type Roundtable struct {
	mu           sync.RWMutex
	cfg          Config
	currentRound int
	thread       []agent.Message
}

// State captures roundtable state for checkpointing.
type State struct {
	CurrentRound int             `json:"current_round"`
	Thread       []agent.Message `json:"thread"`
}

// New creates a roundtable with options applied.
func New(opts ...Option) *Roundtable {
	cfg := Config{MaxRounds: defaultMaxRounds}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Roundtable{cfg: cfg, thread: make([]agent.Message, 0)}
}

// MaxRounds returns the configured max rounds.
func (r *Roundtable) MaxRounds() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg.MaxRounds
}

// CurrentRound returns the current round number.
func (r *Roundtable) CurrentRound() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentRound
}

// IncrementRound advances to the next round and returns it.
func (r *Roundtable) IncrementRound() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentRound++
	return r.currentRound
}

// Record appends a message to the thread.
func (r *Roundtable) Record(msg agent.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.thread = append(r.thread, msg)
}

// Thread returns a copy of the message thread.
func (r *Roundtable) Thread() []agent.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	thread := make([]agent.Message, len(r.thread))
	copy(thread, r.thread)
	return thread
}

// State returns a snapshot of the roundtable.
func (r *Roundtable) State() State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	thread := make([]agent.Message, len(r.thread))
	copy(thread, r.thread)
	return State{
		CurrentRound: r.currentRound,
		Thread:       thread,
	}
}

// Restore resets the roundtable to a prior state.
func (r *Roundtable) Restore(state State) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentRound = state.CurrentRound
	r.thread = append([]agent.Message(nil), state.Thread...)
}

// FormatThread wraps assistant messages with agent tags for clearer attribution.
func FormatThread(thread []agent.Message) []agent.Message {
	messages := make([]agent.Message, 0, len(thread))
	for _, msg := range thread {
		wrapped := msg
		if msg.Role == agent.RoleAssistant && msg.Name != "" {
			content := formatMessageContent(msg)
			wrapped.Parts = []agent.ContentPart{{Type: agent.ContentPartText, Text: fmt.Sprintf("<agent:%s>\n%s\n</agent:%s>", msg.Name, content, msg.Name)}}
			wrapped.Name = ""
		}
		messages = append(messages, wrapped)
	}
	return messages
}

func formatMessageContent(msg agent.Message) string {
	text := agent.TextFromParts(msg.Parts)
	if strings.TrimSpace(text) != "" {
		return text
	}
	return msg.Summary()
}
