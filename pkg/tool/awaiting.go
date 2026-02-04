package tool

import (
	"fmt"
	"time"
)

// Request captures an awaiting tool result request.
type Request struct {
	RequestID   string    `json:"request_id"`
	ToolCallID  string    `json:"tool_call_id"`
	ToolID      string    `json:"tool_id"`
	AgentID     string    `json:"agent_id"`
	Context     string    `json:"context,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
	TimeoutAt   time.Time `json:"timeout_at,omitempty"`
}

// TimeoutRemaining reports the remaining time until timeout, if configured.
func (r Request) TimeoutRemaining(now time.Time) (time.Duration, bool) {
	if r.TimeoutAt.IsZero() {
		return 0, false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return r.TimeoutAt.Sub(now), true
}

// TimedOut reports whether the request has timed out.
func (r Request) TimedOut(now time.Time) bool {
	if r.TimeoutAt.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !now.Before(r.TimeoutAt)
}

// AwaitingResultError signals that tool execution paused for an external result.
type AwaitingResultError struct {
	Request Request
}

func (e *AwaitingResultError) Error() string {
	if e == nil {
		return "awaiting tool result"
	}
	return fmt.Sprintf("awaiting tool result: %s", e.Request.RequestID)
}

// AwaitOption customizes an awaiting request.
type AwaitOption func(*Request, *Result)

// WithRequestID sets the request ID.
func WithRequestID(id string) AwaitOption {
	return func(req *Request, _ *Result) {
		if id != "" {
			req.RequestID = id
		}
	}
}

// WithContext sets a request context description.
func WithContext(ctx string) AwaitOption {
	return func(req *Request, _ *Result) {
		req.Context = ctx
	}
}

// WithTimeout sets a timeout duration from now.
func WithTimeout(timeout time.Duration) AwaitOption {
	return func(req *Request, _ *Result) {
		if timeout > 0 {
			req.TimeoutAt = time.Now().UTC().Add(timeout)
		}
	}
}

// WithDeadline sets an absolute timeout deadline.
func WithDeadline(deadline time.Time) AwaitOption {
	return func(req *Request, _ *Result) {
		if !deadline.IsZero() {
			req.TimeoutAt = deadline.UTC()
		}
	}
}

// WithMeta sets metadata on the tool result.
func WithMeta(meta map[string]any) AwaitOption {
	return func(_ *Request, res *Result) {
		if len(meta) == 0 {
			return
		}
		if res.Meta == nil {
			res.Meta = make(map[string]any, len(meta))
		}
		for key, value := range meta {
			res.Meta[key] = value
		}
	}
}

// Await builds a pending tool result and returns an awaiting error.
func Await(call Call, opts ...AwaitOption) (Result, error) {
	req := Request{
		RequestID:   call.ID,
		ToolCallID:  call.ID,
		ToolID:      call.ToolID,
		AgentID:     call.AgentID,
		RequestedAt: time.Now().UTC(),
	}
	if req.RequestID == "" {
		req.RequestID = fmt.Sprintf("%s-%d", call.ToolID, time.Now().UnixNano())
		req.ToolCallID = req.RequestID
	}
	res := ResultForCall(call)
	res.Meta = map[string]any{
		"awaiting":   true,
		"request_id": req.RequestID,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&req, &res)
		}
	}
	return res, &AwaitingResultError{Request: req}
}
