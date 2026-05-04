package engine

import (
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

const telemetryImageMarker = "[image data omitted]"

func llmEventAttrs(sessionID, agentID string, ev event.Type, delta string) map[string]any {
	return map[string]any{
		"session_id": sessionID,
		"agent_id":   agentID,
		"event_type": string(ev),
		"text":       redactTraceText(delta),
	}
}

func messageCompleteAttrs(sessionID, agentID string, msg agent.Message) map[string]any {
	return map[string]any{
		"session_id": sessionID,
		"agent_id":   agentID,
		"event_type": string(event.AgentMessageComplete),
		"role":       string(msg.Role),
		"text":       redactTraceText(msg.Text()),
		"summary":    redactTraceText(msg.Summary()),
		"parts":      len(msg.Parts),
	}
}

func providerToolCallAttrs(sessionID, agentID string, calls []provider.ToolCall) map[string]any {
	attrs := map[string]any{
		"session_id": sessionID,
		"agent_id":   agentID,
		"event_type": string(provider.EventToolCall),
		"count":      len(calls),
	}
	if len(calls) == 1 {
		attrs["tool_call_id"] = calls[0].ID
		attrs["tool_id"] = calls[0].ToolID
		attrs["arguments"] = string(calls[0].Arguments)
	}
	return attrs
}

func toolCallAttrs(sessionID, agentID string, call tool.Call) map[string]any {
	return map[string]any{
		"session_id":     sessionID,
		"agent_id":       agentID,
		"tool_id":        call.ToolID,
		"tool_call_id":   call.ID,
		"tool_arguments": string(call.Arguments),
	}
}

func toolResultAttrs(sessionID, agentID string, res tool.Result, eventType event.Type) map[string]any {
	attrs := map[string]any{
		"session_id":   sessionID,
		"agent_id":     agentID,
		"event_type":   string(eventType),
		"tool_id":      res.ToolID,
		"tool_call_id": res.ID,
		"result":       redactTraceText(res.Text()),
	}
	if res.Error != nil {
		attrs["error"] = res.Error.Message
	}
	return attrs
}

func rawProviderEventAttrs(sessionID, agentID string, raw []byte) map[string]any {
	return map[string]any{
		"session_id": sessionID,
		"agent_id":   agentID,
		"event_type": string(event.ProviderRawEvent),
		"raw":        redactTraceText(string(raw)),
	}
}

func redactRuntimeMessage(msg modelruntime.Message) modelruntime.Message {
	msg.Parts = redactRuntimeParts(msg.Parts)
	return msg
}

func redactRuntimeParts(parts []modelruntime.Part) []modelruntime.Part {
	out := make([]modelruntime.Part, len(parts))
	for i, part := range parts {
		out[i] = part
		if isImagePart(part.Type) {
			if out[i].URI != "" {
				out[i].URI = telemetryImageMarker
			}
			if len(out[i].Data) > 0 {
				out[i].Data = nil
				out[i].Text = telemetryImageMarker
			}
		}
		if out[i].Text != "" {
			out[i].Text = redactTraceText(out[i].Text)
		}
	}
	return out
}

func redactTraceText(text string) string {
	if strings.HasPrefix(text, "data:image/") {
		return telemetryImageMarker
	}
	return text
}

func isImagePart(partType modelruntime.PartType) bool {
	switch strings.ToLower(string(partType)) {
	case "image", "input_image":
		return true
	default:
		return false
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}
