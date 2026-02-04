package caucus

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/roundtable"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

const defaultRounds = 1

var (
	// ErrNoParticipants indicates a caucus without participants.
	ErrNoParticipants = errors.New("caucus requires at least one participant")
	// ErrNoTurnBuilder indicates a missing turn builder.
	ErrNoTurnBuilder = errors.New("caucus requires a turn builder")
	// ErrInvalidRounds indicates an invalid round count.
	ErrInvalidRounds = errors.New("caucus requires at least one round")
)

// Runner executes a single agent run.
type Runner func(ctx context.Context, agent agent.Agent, req protocol.RunRequest) (agent.Message, error)

// TurnBuilder builds the per-agent run request for a round.
type TurnBuilder func(agent agent.Agent, thread []agent.Message, round, maxRounds int) protocol.RunRequest

// Config configures caucus execution.
type Config struct {
	Rounds        int
	MaxConcurrent int
	SeedMessages  []agent.Message
}

// Option configures a caucus run.
type Option func(*Config)

// WithRounds sets the number of private rounds per participant.
func WithRounds(rounds int) Option {
	return func(cfg *Config) {
		if rounds > 0 {
			cfg.Rounds = rounds
		}
	}
}

// WithMaxConcurrent limits concurrent agent runs.
func WithMaxConcurrent(max int) Option {
	return func(cfg *Config) {
		if max > 0 {
			cfg.MaxConcurrent = max
		}
	}
}

// WithSeedMessage sets a single seed message for each participant.
func WithSeedMessage(msg agent.Message) Option {
	return func(cfg *Config) {
		cfg.SeedMessages = []agent.Message{msg}
	}
}

// WithSeedMessages sets seed messages for each participant.
func WithSeedMessages(msgs ...agent.Message) Option {
	return func(cfg *Config) {
		if len(msgs) == 0 {
			return
		}
		cfg.SeedMessages = append([]agent.Message(nil), msgs...)
	}
}

// Thread captures a private thread for one participant.
type Thread struct {
	Agent    agent.Agent
	Messages []agent.Message
}

// Result captures caucus threads for all participants.
type Result struct {
	Threads []Thread
}

// Latest returns the most recent message per participant.
func (r Result) Latest() []agent.Message {
	out := make([]agent.Message, 0, len(r.Threads))
	for _, thread := range r.Threads {
		if len(thread.Messages) == 0 {
			continue
		}
		msg := thread.Messages[len(thread.Messages)-1]
		if msg.Name == "" {
			msg.Name = thread.Agent.Name
		}
		out = append(out, msg)
	}
	return out
}

// Brief returns a concise text brief of the latest message per participant.
func (r Result) Brief() string {
	if len(r.Threads) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, thread := range r.Threads {
		if len(thread.Messages) == 0 {
			continue
		}
		msg := thread.Messages[len(thread.Messages)-1]
		summary := strings.TrimSpace(msg.Summary())
		if summary == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", thread.Agent.Name, summary))
	}
	return strings.TrimSpace(sb.String())
}

// Run executes a private caucus for each participant.
func Run(ctx context.Context, run Runner, participants []agent.Agent, builder TurnBuilder, opts ...Option) (Result, error) {
	if len(participants) == 0 {
		return Result{}, ErrNoParticipants
	}
	if builder == nil {
		return Result{}, ErrNoTurnBuilder
	}

	cfg := Config{Rounds: defaultRounds}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Rounds <= 0 {
		return Result{}, ErrInvalidRounds
	}

	threads := make([]Thread, len(participants))
	for i, participant := range participants {
		seed := append([]agent.Message(nil), cfg.SeedMessages...)
		threads[i] = Thread{Agent: participant, Messages: seed}
	}

	runner := runnerAdapter(run)

	for round := 1; round <= cfg.Rounds; round++ {
		snapshots := make([][]agent.Message, len(threads))
		for i, thread := range threads {
			snapshots[i] = append([]agent.Message(nil), thread.Messages...)
		}

		turns := make([]roundtable.Turn, len(threads))
		for i, thread := range threads {
			req := builder(thread.Agent, snapshots[i], round, cfg.Rounds)
			turns[i] = roundtable.Turn{
				Agent:             thread.Agent,
				Messages:          req.Messages,
				SystemMessages:    req.SystemMessages,
				Params:            req.Params,
				MaxToolIterations: req.MaxToolIterations,
				Tools:             req.Tools,
			}
		}

		results, err := roundtable.RunParallel(ctx, runner, turns, roundtable.ParallelConfig{MaxConcurrent: cfg.MaxConcurrent})
		if err != nil {
			return Result{}, err
		}

		for i, result := range results {
			msg := result.Message
			if msg.Name == "" {
				msg.Name = threads[i].Agent.Name
			}
			threads[i].Messages = append(threads[i].Messages, msg)
		}
	}

	return Result{Threads: threads}, nil
}

func runnerAdapter(run Runner) roundtable.Runner {
	return func(ctx context.Context, ag agent.Agent, req roundtable.RunRequest) (agent.Message, error) {
		return run(ctx, ag, protocol.RunRequest{
			Messages:          req.Messages,
			SystemMessages:    req.SystemMessages,
			Params:            req.Params,
			MaxToolIterations: req.MaxToolIterations,
			Tools:             req.Tools,
		})
	}
}
