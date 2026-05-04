package provider

import (
	"context"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// EventType describes a provider event.
type EventType = modelruntime.EventType

// Provider event types emitted by streaming providers.
const (
	EventMessageDelta          = modelruntime.EventMessageDelta
	EventMessageCompleted      = modelruntime.EventMessageCompleted
	EventReasoningDelta        = modelruntime.EventReasoningDelta
	EventReasoningSummaryDelta = modelruntime.EventReasoningSummaryDelta
	EventToolCall              = modelruntime.EventToolCall
	EventToolResult            = modelruntime.EventToolResult
	EventError                 = modelruntime.EventError
	EventRaw                   = modelruntime.EventRaw
)

// ToolCall represents a tool call emitted by the provider.
type ToolCall = modelruntime.ToolCall

// Event is emitted by provider streams.
type Event = modelruntime.Event

// Request describes an LLM request.
type Request = modelruntime.Request

// Stream receives provider events.
type Stream = modelruntime.Stream

// Provider starts streams for a given request.
type Provider = modelruntime.Provider

// TokenEstimator optionally provides provider-specific token estimates.
type TokenEstimator interface {
	EstimateMessageTokens(ctx context.Context, model string, msg agent.Message) (int, error)
}

// ToolDefinitionFromTool converts a Meanwhile tool definition into the neutral runtime shape.
func ToolDefinitionFromTool(def tool.Definition) modelruntime.ToolDefinition {
	return modelruntime.ToolDefinition{
		ID:          def.ID,
		Description: def.Description,
		JSONSchema:  append([]byte(nil), def.Schema.JSONSchema...),
		Tags:        append([]string(nil), def.Tags...),
	}
}
