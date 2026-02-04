package engine

import (
	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

// RunResult contains the outcome of a session run.
type RunResult struct {
	// Status is the run outcome status.
	Status RunStatus

	// RequestID is set when awaiting human input.
	RequestID string

	// Context carries a short explanation for the awaited human turn.
	Context string

	// AwaitingInput captures details for a pending human input request.
	AwaitingInput *protocol.InputRequest
	// AwaitingTool captures details for a pending tool result.
	AwaitingTool *tool.Request

	// Final is the last assistant message content
	Final string

	// Transcript contains all messages exchanged during the run
	Transcript []agent.Message

	// Events contains all events emitted during the run
	Events []event.Event

	// Metadata contains protocol-specific data
	Metadata map[string]any
}
