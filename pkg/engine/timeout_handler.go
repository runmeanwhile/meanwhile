package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
)

// HandleTimeout resolves a pending request timeout by session ID.
func (e *Engine) HandleTimeout(ctx context.Context, sessionID, requestID string) (*RunResult, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session id required")
	}
	if requestID == "" {
		return nil, fmt.Errorf("request id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sess, err := e.session(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return sess.HandleTimeout(ctx, requestID)
}

// HandleTimeout resolves a pending request timeout using the session default policy.
func (s *Session) HandleTimeout(ctx context.Context, requestID string) (*RunResult, error) {
	if requestID == "" {
		return nil, fmt.Errorf("request id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pending, ok := s.pendingRequest(requestID)
	if !ok {
		return nil, ErrRequestNotFound
	}
	if !pending.resumable {
		s.clearPendingRequest(ctx, requestID)
		return nil, ErrSessionNotResumable
	}
	now := time.Now().UTC()
	if err := validateTimeout(pending.request, now); err != nil {
		return nil, err
	}
	_ = s.EmitWithContext(ctx, event.New(event.HumanRequestTimedOut, s.id, pending.request))
	if s.timeoutPolicy == nil {
		return nil, ErrTimeoutPolicyRequired
	}
	policy := *s.timeoutPolicy
	return s.Respond(ctx, requestID, agent.Message{}, OnTimeout(policy))
}
