package tool

import (
	"context"
	"encoding/json"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

// Schema describes tool input requirements.
type Schema struct {
	JSONSchema json.RawMessage
}

// Definition describes a tool available to a provider.
type Definition struct {
	ID string
	// Description provides a hint for the model about tool behavior.
	Description string
	Schema      Schema
	// Tags provide optional tool categorization for policy enforcement.
	Tags []string
}

// Call represents a tool invocation.
type Call struct {
	ID        string
	ToolID    string
	Arguments json.RawMessage
	AgentID   string // Caller agent ID
}

// Result represents a tool result.
type Result struct {
	ID     string
	ToolID string
	Parts  []agent.ContentPart
	Output any
	Error  *Error
	Meta   map[string]any
}

// Error represents a structured tool error.
type Error struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// Text returns a best-effort text rendering of the result.
func (r Result) Text() string {
	if len(r.Parts) > 0 {
		return agent.TextFromParts(r.Parts)
	}
	if r.Output != nil {
		if raw, err := json.Marshal(r.Output); err == nil {
			return string(raw)
		}
	}
	if r.Error != nil {
		return r.Error.Message
	}
	return ""
}

// Emitter streams tool events to the caller.
type Emitter interface {
	Emit(eventType string, payload any) error
}

// Tool defines a callable tool.
type Tool interface {
	ID() string
	Schema() Schema
	Run(ctx context.Context, call Call, emit Emitter) (Result, error)
}

// Tagger optionally exposes tool tags for policy enforcement.
type Tagger interface {
	Tags() []string
}

// DefinitionFromTool builds a Definition from a Tool.
func DefinitionFromTool(t Tool) Definition {
	def := Definition{ID: t.ID(), Schema: t.Schema()}
	if describer, ok := t.(interface{ Description() string }); ok {
		def.Description = describer.Description()
	}
	if tagger, ok := t.(Tagger); ok {
		def.Tags = append([]string(nil), tagger.Tags()...)
	}
	return def
}
