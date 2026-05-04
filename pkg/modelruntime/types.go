package modelruntime

import (
	"encoding/json"
	"strings"
)

// Role identifies the sender role of a message.
type Role string

// Supported message roles.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// PartType identifies the type of content in one message part.
type PartType string

// Supported content part types.
const (
	PartText     PartType = "text"
	PartImage    PartType = "image"
	PartAudio    PartType = "audio"
	PartVideo    PartType = "video"
	PartFile     PartType = "file"
	PartResource PartType = "resource"
	PartJSON     PartType = "json"
)

// Part is one multimodal content fragment within a message.
type Part struct {
	Type     PartType       `json:"type"`
	Text     string         `json:"text,omitempty"`
	URI      string         `json:"uri,omitempty"`
	Data     []byte         `json:"data,omitempty"`
	MIMEType string         `json:"mime_type,omitempty"`
	Name     string         `json:"name,omitempty"`
	Size     *int64         `json:"size,omitempty"`
	JSON     any            `json:"json,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Message is one normalized model-runtime message.
type Message struct {
	Role       Role           `json:"role"`
	Parts      []Part         `json:"parts,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Text returns the combined text from text parts only.
func (m Message) Text() string {
	var builder strings.Builder
	for _, part := range m.Parts {
		if part.Type != PartText || part.Text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(part.Text)
	}
	return builder.String()
}

// ToolDefinition describes one tool exposed to the model.
type ToolDefinition struct {
	ID          string
	Description string
	JSONSchema  json.RawMessage
	Tags        []string
}

// ToolCall is one tool invocation emitted by the model provider.
type ToolCall struct {
	ID        string
	ToolID    string
	Arguments json.RawMessage
}

// Request is one normalized model request.
type Request struct {
	Model    string
	Messages []Message
	Tools    []ToolDefinition
	Params   map[string]any
}

// EventType describes a streaming provider event.
type EventType string

// Streaming provider event types.
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

// Event is one normalized streaming provider event.
type Event struct {
	Type      EventType
	Message   Message
	Delta     string
	ToolCalls []ToolCall
	Raw       json.RawMessage
	Err       error
}
