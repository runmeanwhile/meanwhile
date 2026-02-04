package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

// TimeoutStrategy defines how to proceed when a human request times out.
type TimeoutStrategy string

const (
	TimeoutContinue TimeoutStrategy = "continue"
	TimeoutRetry    TimeoutStrategy = "retry"
	TimeoutFail     TimeoutStrategy = "fail"
)

// TimeoutPolicy configures how to handle a timed-out request.
type TimeoutPolicy struct {
	Strategy         TimeoutStrategy
	Note             string
	RetryParticipant string
}

// ContinueWithNote continues execution with a system note when the request times out.
func ContinueWithNote(note string) TimeoutPolicy {
	return TimeoutPolicy{Strategy: TimeoutContinue, Note: note}
}

// RetryWith retries the request with another human participant when it times out.
func RetryWith(participantID string) TimeoutPolicy {
	return TimeoutPolicy{Strategy: TimeoutRetry, RetryParticipant: participantID}
}

// MarkIncomplete marks the request as incomplete and halts resume.
func MarkIncomplete() TimeoutPolicy {
	return TimeoutPolicy{Strategy: TimeoutFail}
}

type respondOptions struct {
	timeoutPolicy *TimeoutPolicy
}

// RespondOption configures Session.Respond behavior.
type RespondOption func(*respondOptions)

// OnTimeout sets the timeout policy applied when Respond is called without a message.
func OnTimeout(policy TimeoutPolicy) RespondOption {
	return func(opts *respondOptions) {
		opts.timeoutPolicy = &policy
	}
}

func (s *Session) resolveTimeout(ctx context.Context, pending pendingInput, opts respondOptions) (agent.Message, error) {
	now := time.Now().UTC()
	if err := validateTimeout(pending.request, now); err != nil {
		return agent.Message{}, err
	}
	if opts.timeoutPolicy == nil {
		return agent.Message{}, ErrRequestTimedOut
	}

	switch opts.timeoutPolicy.Strategy {
	case TimeoutContinue:
		return timeoutContinueMessage(pending.request, *opts.timeoutPolicy)
	case TimeoutRetry:
		return agent.Message{}, s.retryTimeout(ctx, pending, *opts.timeoutPolicy)
	case TimeoutFail:
		if err := s.markTimeoutIncomplete(ctx, pending); err != nil {
			return agent.Message{}, err
		}
		return agent.Message{}, ErrSessionIncomplete
	default:
		return agent.Message{}, fmt.Errorf("unknown timeout policy %q", opts.timeoutPolicy.Strategy)
	}
}

func timeoutContinueMessage(req protocol.InputRequest, policy TimeoutPolicy) (agent.Message, error) {
	if policy.Note == "" {
		return agent.Message{}, ErrTimeoutNoteRequired
	}
	msg := message.System(policy.Note)
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]any)
	}
	msg.Metadata["timeout"] = true
	msg.Metadata["timeout_policy"] = string(policy.Strategy)
	msg.Metadata["request_id"] = req.RequestID
	msg.Metadata["participant_id"] = req.ParticipantID
	msg.Metadata["participant_name"] = req.ParticipantName
	if !req.TimeoutAt.IsZero() {
		msg.Metadata["timeout_at"] = req.TimeoutAt
	}
	return msg, nil
}

func (s *Session) retryTimeout(ctx context.Context, pending pendingInput, policy TimeoutPolicy) error {
	participantID := policy.RetryParticipant
	if participantID == "" {
		return ErrRetryParticipantRequired
	}
	participant, ok := participantByKey(s.participants, participantID)
	if !ok {
		return fmt.Errorf("retry participant not found: %s", participantID)
	}
	if !participant.IsHuman() {
		return fmt.Errorf("retry participant must be human")
	}

	inputOpts := []protocol.InputOption{}
	if !pending.request.TimeoutAt.IsZero() && !pending.request.RequestedAt.IsZero() {
		timeout := pending.request.TimeoutAt.Sub(pending.request.RequestedAt)
		if timeout > 0 {
			inputOpts = append(inputOpts, protocol.WithInputTimeout(timeout))
		}
	}

	turnContext := pending.request.Context
	if policy.Note != "" {
		if turnContext == "" {
			turnContext = policy.Note
		} else {
			turnContext = fmt.Sprintf("%s\n\n%s", turnContext, policy.Note)
		}
	}

	err := s.AwaitInput(ctx, participant, turnContext, pending.resume, inputOpts...)
	_, _ = s.removePending(pending.request.RequestID)
	return err
}

func (s *Session) markTimeoutIncomplete(ctx context.Context, pending pendingInput) error {
	remaining, _ := s.removePending(pending.request.RequestID)
	if remaining == 0 {
		_ = s.EmitWithContext(ctx, event.New(event.SessionResumed, s.id, s.State()))
	}
	if s.engine != nil {
		_ = s.engine.persistSessionState(ctx, s)
	}
	return nil
}

func isMessageEmpty(msg agent.Message) bool {
	if msg.Role != "" {
		return false
	}
	if msg.Name != "" || msg.ToolCallID != "" {
		return false
	}
	if len(msg.Parts) > 0 || len(msg.Metadata) > 0 {
		return false
	}
	return true
}

func validateTimeout(req protocol.InputRequest, now time.Time) error {
	if req.TimeoutAt.IsZero() {
		return ErrResponseRequired
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if now.Before(req.TimeoutAt) {
		return ErrRequestNotTimedOut
	}
	return nil
}
