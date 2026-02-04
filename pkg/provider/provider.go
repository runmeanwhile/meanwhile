package provider

import (
	"context"
	"encoding/json"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

// EventType describes a provider event.
type EventType string

// Provider event types emitted by streaming providers.
const (
	EventMessageDelta          EventType = "message.delta"
	EventMessageCompleted      EventType = "message.completed"
	EventReasoningDelta        EventType = "reasoning.delta"
	EventReasoningSummaryDelta EventType = "reasoning.summary.delta"
	EventToolCall              EventType = "tool.call"
	EventToolResult            EventType = "tool.result"
	EventError                 EventType = "error"
	EventRaw                   EventType = "raw"
)

// ToolCall represents a tool call emitted by the provider.
type ToolCall struct {
	ID        string
	ToolID    string
	Arguments json.RawMessage
}

// Event is emitted by provider streams.
type Event struct {
	Type      EventType
	Message   agent.Message
	Delta     string
	ToolCalls []ToolCall
	Raw       json.RawMessage
	Err       error
}

// Request describes an LLM request.
type Request struct {
	Model    string
	Messages []agent.Message
	Tools    []tool.Definition
	Params   map[string]any
}

// Stream receives provider events.
type Stream interface {
	Recv() (Event, error)
	Close() error
}

// Provider starts streams for a given request.
type Provider interface {
	ID() string
	Stream(ctx context.Context, req Request) (Stream, error)
}

// TokenEstimator optionally provides provider-specific token estimates.
type TokenEstimator interface {
	EstimateMessageTokens(ctx context.Context, model string, msg agent.Message) (int, error)
}
