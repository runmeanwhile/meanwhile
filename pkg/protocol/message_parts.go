package protocol

import (
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

// PromptWithMedia wraps a text prompt with any non-text content parts from the source message.
func PromptWithMedia(text string, source agent.Message) agent.Message {
	msg := agent.Message{Role: agent.RoleUser}
	if text != "" {
		msg.Parts = []agent.ContentPart{{Type: agent.ContentPartText, Text: text}}
	}
	if len(source.Parts) == 0 {
		return msg
	}

	mediaParts := filterNonTextParts(source.Parts)
	if len(mediaParts) == 0 {
		return msg
	}

	parts := make([]agent.ContentPart, 0, len(mediaParts)+1)
	if text != "" {
		parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: text})
	}
	parts = append(parts, mediaParts...)
	msg.Parts = parts

	return msg
}

func filterNonTextParts(parts []agent.ContentPart) []agent.ContentPart {
	out := make([]agent.ContentPart, 0, len(parts))
	for _, part := range parts {
		if isTextPart(part) {
			continue
		}
		out = append(out, part)
	}
	return out
}

func isTextPart(part agent.ContentPart) bool {
	switch strings.ToLower(string(part.Type)) {
	case "text", "input_text":
		return true
	default:
		return false
	}
}
