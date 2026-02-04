package engine

import "github.com/runmeanwhile/meanwhile/pkg/tool"

// ToolRunState captures pending tool execution for persistence.
type ToolRunState struct {
	Request      tool.Request     `json:"request"`
	Continuation toolContinuation `json:"continuation"`
}
