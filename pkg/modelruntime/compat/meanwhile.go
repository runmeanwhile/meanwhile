package compat

import (
	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// FromAgentMessage converts a Meanwhile agent message into a neutral runtime message.
func FromAgentMessage(msg agent.Message) modelruntime.Message {
	parts := make([]modelruntime.Part, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		parts = append(parts, modelruntime.Part{
			Type:     modelruntime.PartType(part.Type),
			Text:     part.Text,
			URI:      part.URI,
			Data:     append([]byte(nil), part.Data...),
			MIMEType: part.MIMEType,
			Name:     part.Name,
			Size:     part.Size,
			JSON:     part.JSON,
			Detail:   part.Detail,
			Metadata: cloneMap(part.Metadata),
		})
	}
	return modelruntime.Message{
		Role:       modelruntime.Role(msg.Role),
		Parts:      parts,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
		Metadata:   cloneMap(msg.Metadata),
	}
}

// ToAgentMessage converts a neutral runtime message into a Meanwhile agent message.
func ToAgentMessage(msg modelruntime.Message) agent.Message {
	parts := make([]agent.ContentPart, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		parts = append(parts, agent.ContentPart{
			Type:     agent.ContentPartType(part.Type),
			Text:     part.Text,
			URI:      part.URI,
			Data:     append([]byte(nil), part.Data...),
			MIMEType: part.MIMEType,
			Name:     part.Name,
			Size:     part.Size,
			JSON:     part.JSON,
			Detail:   part.Detail,
			Metadata: cloneMap(part.Metadata),
		})
	}
	return agent.Message{
		Role:       agent.Role(msg.Role),
		Parts:      parts,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
		Metadata:   cloneMap(msg.Metadata),
	}
}

// FromToolDefinition converts one Meanwhile tool definition into a runtime definition.
func FromToolDefinition(def tool.Definition) modelruntime.ToolDefinition {
	return modelruntime.ToolDefinition{
		ID:          def.ID,
		Description: def.Description,
		JSONSchema:  append([]byte(nil), def.Schema.JSONSchema...),
		Tags:        append([]string(nil), def.Tags...),
	}
}

// FromToolDefinitions converts tool definitions into runtime definitions.
func FromToolDefinitions(defs []tool.Definition) []modelruntime.ToolDefinition {
	out := make([]modelruntime.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, FromToolDefinition(def))
	}
	return out
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}
