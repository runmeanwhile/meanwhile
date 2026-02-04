package engine

import (
	"errors"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func toolIDsFromTools(tools []tool.Tool) []string {
	if len(tools) == 0 {
		return nil
	}
	ids := make([]string, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		id := t.ID()
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func toolMessagesFromResults(results []tool.Result) []agent.Message {
	if len(results) == 0 {
		return nil
	}
	msgs := make([]agent.Message, 0, len(results))
	for _, res := range results {
		if isAwaitingResult(res) {
			continue
		}
		msgs = append(msgs, toolMessageFromResult(res))
	}
	return msgs
}

func toolAllowed(allowed map[string]struct{}, toolID string) bool {
	if allowed == nil {
		return false
	}
	_, ok := allowed[toolID]
	return ok
}

func mergeResultMeta(base map[string]any, extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]any, len(extra))
	}
	for key, value := range extra {
		base[key] = value
	}
	return base
}

func isAwaitingResult(res tool.Result) bool {
	if res.Meta == nil {
		return false
	}
	awaiting, ok := res.Meta["awaiting"].(bool)
	return ok && awaiting
}

func toolAwaiting(err error) *tool.AwaitingResultError {
	var awaiting *tool.AwaitingResultError
	if errors.As(err, &awaiting) {
		return awaiting
	}
	return nil
}
