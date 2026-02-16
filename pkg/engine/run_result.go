package engine

import (
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
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

	// ProtocolSummary is a protocol-authored summary when available
	// (e.g. from minutes payload "summary"), distinct from Final.
	ProtocolSummary string

	// Transcript contains all messages exchanged during the run
	Transcript []agent.Message

	// Events contains all events emitted during the run
	Events []event.Event

	// Metadata contains protocol-specific data
	Metadata map[string]any
}

func applyProtocolSummary(result *RunResult) {
	if result == nil {
		return
	}
	result.ProtocolSummary = extractProtocolSummary(result.Metadata)
}

func extractProtocolSummary(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	if summary, ok := metadata["summary"].(string); ok {
		if trimmed := strings.TrimSpace(summary); trimmed != "" {
			return trimmed
		}
	}
	if synthesis, ok := metadata["synthesis"].(map[string]any); ok {
		if closing, ok := synthesis["closing"].(string); ok {
			if trimmed := strings.TrimSpace(closing); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
