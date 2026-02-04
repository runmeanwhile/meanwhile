package protocol

import (
	"context"
	"fmt"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

// TurnResume resumes a paused human turn with the provided response.
type TurnResume func(ctx context.Context, response agent.Message) error

// TurnOptions configure a participant turn.
type TurnOptions struct {
	Context  string
	Resume   TurnResume
	Timeout  time.Duration
	Deadline time.Time
}

// TurnOption mutates TurnOptions.
type TurnOption func(*TurnOptions)

// WithTurnContext provides context shown to a human during their turn.
func WithTurnContext(context string) TurnOption {
	return func(opts *TurnOptions) {
		opts.Context = context
	}
}

// WithTurnResume sets the callback that resumes the protocol after human input.
func WithTurnResume(resume TurnResume) TurnOption {
	return func(opts *TurnOptions) {
		opts.Resume = resume
	}
}

// WithTurnTimeout sets a relative timeout for a human turn.
func WithTurnTimeout(timeout time.Duration) TurnOption {
	return func(opts *TurnOptions) {
		opts.Timeout = timeout
	}
}

// WithTurnDeadline sets an absolute timeout deadline for a human turn.
func WithTurnDeadline(deadline time.Time) TurnOption {
	return func(opts *TurnOptions) {
		opts.Deadline = deadline
	}
}

// InputOptions configure a human input request.
type InputOptions struct {
	Timeout  time.Duration
	Deadline time.Time
}

// InputOption mutates InputOptions.
type InputOption func(*InputOptions)

// WithInputTimeout sets a relative timeout for a human input request.
func WithInputTimeout(timeout time.Duration) InputOption {
	return func(opts *InputOptions) {
		opts.Timeout = timeout
	}
}

// WithInputDeadline sets an absolute timeout deadline for a human input request.
func WithInputDeadline(deadline time.Time) InputOption {
	return func(opts *InputOptions) {
		opts.Deadline = deadline
	}
}

// InputRequest captures a pending human input request.
type InputRequest struct {
	RequestID       string
	ParticipantID   string
	ParticipantName string
	Context         string
	RequestedAt     time.Time
	TimeoutAt       time.Time
}

// TimeoutRemaining reports the remaining time until timeout, if configured.
func (r InputRequest) TimeoutRemaining(now time.Time) (time.Duration, bool) {
	if r.TimeoutAt.IsZero() {
		return 0, false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return r.TimeoutAt.Sub(now), true
}

// TimedOut reports whether the request has timed out.
func (r InputRequest) TimedOut(now time.Time) bool {
	if r.TimeoutAt.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !now.Before(r.TimeoutAt)
}

// AwaitingInputError signals that execution paused for human input.
type AwaitingInputError struct {
	Request InputRequest
}

func (e *AwaitingInputError) Error() string {
	if e == nil {
		return "awaiting input"
	}
	return fmt.Sprintf("awaiting input: %s", e.Request.RequestID)
}
