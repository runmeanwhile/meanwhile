package ideo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

func parseStructuredOutput[T any](msg agent.Message) (T, error) {
	var out T

	if parsed, ok := decodeMessageJSONPart[T](msg); ok {
		return parsed, nil
	}

	raw := strings.TrimSpace(agent.TextFromParts(msg.Parts))
	if raw == "" {
		raw = strings.TrimSpace(msg.Text())
	}
	raw = stripCodeFence(raw)
	if raw == "" {
		return out, fmt.Errorf("empty structured output")
	}

	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return out, fmt.Errorf("parse structured output: %w", err)
	}
	return out, nil
}

func decodeMessageJSONPart[T any](msg agent.Message) (T, bool) {
	var out T
	for _, part := range msg.Parts {
		if part.Type != agent.ContentPartJSON || part.JSON == nil {
			continue
		}
		raw, err := json.Marshal(part.JSON)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			continue
		}
		return out, true
	}
	return out, false
}

func stripCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	firstNL := strings.Index(trimmed, "\n")
	if firstNL < 0 {
		return trimmed
	}
	inner := strings.TrimSpace(trimmed[firstNL+1:])
	inner = strings.TrimSuffix(inner, "```")
	return strings.TrimSpace(inner)
}
