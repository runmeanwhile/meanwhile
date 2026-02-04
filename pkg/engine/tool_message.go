package engine

import (
	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

func toolMessageFromResult(res tool.Result) agent.Message {
	parts := make([]agent.ContentPart, 0, len(res.Parts)+1)
	parts = append(parts, res.Parts...)

	if res.Output != nil && !hasPartType(parts, agent.ContentPartJSON) {
		parts = append(parts, agent.ContentPart{Type: agent.ContentPartJSON, JSON: res.Output})
	}

	metadata := map[string]any{}
	if res.Meta != nil {
		for k, v := range res.Meta {
			metadata[k] = v
		}
	}
	if res.Error != nil {
		metadata["error"] = res.Error
		if !hasPartType(parts, agent.ContentPartText) && res.Error.Message != "" {
			parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: res.Error.Message})
		}
	}

	return agent.Message{
		Role:       agent.RoleTool,
		Parts:      parts,
		Name:       res.ToolID,
		ToolCallID: res.ID,
		Metadata:   metadata,
	}
}

func hasPartType(parts []agent.ContentPart, partType agent.ContentPartType) bool {
	for _, part := range parts {
		if part.Type == partType {
			return true
		}
	}
	return false
}
