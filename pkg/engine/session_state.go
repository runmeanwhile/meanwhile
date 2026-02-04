package engine

import (
	"context"
	"sort"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// SessionState captures runtime pause state for a session.
type SessionState struct {
	SessionID    string
	UpdatedAt    time.Time
	Pending      []protocol.InputRequest
	PendingTools []ToolRunState
}

// State returns a snapshot of the session's pause state.
func (s *Session) State() SessionState {
	return SessionState{
		SessionID:    s.id,
		UpdatedAt:    time.Now().UTC(),
		Pending:      s.pendingSnapshot(),
		PendingTools: s.pendingToolSnapshot(),
	}
}

// IsPaused reports whether the session is awaiting human input.
func (s *Session) IsPaused() bool {
	s.pendingMu.Lock()
	pendingInputs := len(s.pending)
	s.pendingMu.Unlock()
	if pendingInputs > 0 {
		return true
	}
	s.pendingToolsMu.Lock()
	defer s.pendingToolsMu.Unlock()
	return len(s.pendingTools) > 0
}

// PendingRequests returns a snapshot of pending input requests.
func (s *Session) PendingRequests() []protocol.InputRequest {
	return s.pendingSnapshot()
}

// PendingToolRequests returns a snapshot of pending tool requests.
func (s *Session) PendingToolRequests() []tool.Request {
	states := s.pendingToolSnapshot()
	if len(states) == 0 {
		return nil
	}
	requests := make([]tool.Request, 0, len(states))
	for _, state := range states {
		requests = append(requests, state.Request)
	}
	return requests
}

// PendingRequest returns a specific pending request by ID.
func (s *Session) PendingRequest(id string) (protocol.InputRequest, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if len(s.pending) == 0 {
		return protocol.InputRequest{}, false
	}
	pending, ok := s.pending[id]
	if !ok {
		return protocol.InputRequest{}, false
	}
	return pending.request, true
}

// TimedOutRequests returns pending requests that have passed their timeout.
func (s *Session) TimedOutRequests(now time.Time) []protocol.InputRequest {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	pending := s.pendingSnapshot()
	if len(pending) == 0 {
		return nil
	}
	timedOut := make([]protocol.InputRequest, 0, len(pending))
	for _, req := range pending {
		if req.TimedOut(now) {
			timedOut = append(timedOut, req)
		}
	}
	if len(timedOut) == 0 {
		return nil
	}
	return timedOut
}

func (s *Session) pendingSnapshot() []protocol.InputRequest {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return pendingRequestsLocked(s.pending)
}

func (s *Session) pendingToolSnapshot() []ToolRunState {
	s.pendingToolsMu.Lock()
	defer s.pendingToolsMu.Unlock()
	return pendingToolsLocked(s.pendingTools)
}

func pendingRequestsLocked(pending map[string]pendingInput) []protocol.InputRequest {
	if len(pending) == 0 {
		return nil
	}
	requests := make([]protocol.InputRequest, 0, len(pending))
	for _, entry := range pending {
		requests = append(requests, entry.request)
	}
	sort.Slice(requests, func(i, j int) bool {
		if !requests[i].RequestedAt.Equal(requests[j].RequestedAt) {
			return requests[i].RequestedAt.Before(requests[j].RequestedAt)
		}
		return requests[i].RequestID < requests[j].RequestID
	})
	return requests
}

func pendingToolsLocked(pending map[string]pendingTool) []ToolRunState {
	if len(pending) == 0 {
		return nil
	}
	states := make([]ToolRunState, 0, len(pending))
	for _, entry := range pending {
		states = append(states, ToolRunState{
			Request:      entry.request,
			Continuation: entry.continuation,
		})
	}
	sort.Slice(states, func(i, j int) bool {
		left := states[i].Request
		right := states[j].Request
		if !left.RequestedAt.Equal(right.RequestedAt) {
			return left.RequestedAt.Before(right.RequestedAt)
		}
		return left.RequestID < right.RequestID
	})
	return states
}

func (s *Session) restorePending(requests []protocol.InputRequest) []protocol.InputRequest {
	if s == nil || len(requests) == 0 {
		return nil
	}
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	s.pending = make(map[string]pendingInput, len(requests))
	restored := make([]protocol.InputRequest, 0, len(requests))
	for _, req := range requests {
		if req.RequestID == "" {
			continue
		}
		s.pending[req.RequestID] = pendingInput{
			request:   req,
			resume:    resumeNotResumable,
			resumable: false,
		}
		restored = append(restored, req)
	}
	if len(s.pending) == 0 {
		s.pending = nil
		return nil
	}
	return restored
}

func (s *Session) restorePendingTools(states []ToolRunState) []ToolRunState {
	if s == nil || len(states) == 0 {
		return nil
	}
	s.pendingToolsMu.Lock()
	defer s.pendingToolsMu.Unlock()
	s.pendingTools = make(map[string]pendingTool, len(states))
	restored := make([]ToolRunState, 0, len(states))
	for _, state := range states {
		req := state.Request
		if req.RequestID == "" {
			continue
		}
		s.pendingTools[req.RequestID] = pendingTool{
			request:      req,
			continuation: state.Continuation,
		}
		restored = append(restored, state)
	}
	if len(s.pendingTools) == 0 {
		s.pendingTools = nil
		return nil
	}
	return restored
}

func resumeNotResumable(_ context.Context, _ agent.Message) error {
	return ErrSessionNotResumable
}
