package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Type represents an event type string.
type Type string

// Event type constants emitted by the engine.
const (
	AgentStarted               Type = "agent.started"
	AgentThinking              Type = "agent.thinking"
	AgentMessageDelta          Type = "agent.message.delta"
	AgentMessageComplete       Type = "agent.message.completed"
	AgentReasoningDelta        Type = "agent.reasoning.delta"
	AgentReasoningSummaryDelta Type = "agent.reasoning.summary.delta"

	ToolCallStarted   Type = "tool.call.started"
	ToolCallDelta     Type = "tool.call.delta"
	ToolCallCompleted Type = "tool.call.completed"
	ToolCallError     Type = "tool.call.error"
	ToolCallAwaiting  Type = "tool.call.awaiting"

	ProtocolStateChanged Type = "protocol.state.changed"
	ProtocolAction       Type = "protocol.action"

	MemorySummary Type = "memory.summary"

	AwaitingUserInput Type = "session.awaiting_user_input"
	SessionPaused     Type = "session.paused"
	SessionResumed    Type = "session.resumed"

	HumanRequestCreated               Type = "human.request.created"
	HumanResponseReceived             Type = "human.response.received"
	HumanRequestSent                  Type = "human.request.sent"
	HumanRequestFailed                Type = "human.request.failed"
	HumanRequestTimedOut              Type = "human.request.timed_out"
	HumanRequestRegistryFailed        Type = "human.request.registry.failed"
	HumanRequestTimeoutScheduleFailed Type = "human.request.timeout.schedule_failed"

	HookBlocked  Type = "hook.blocked"
	HookModified Type = "hook.modified"

	ProviderRawEvent Type = "provider.raw.event"
)

// Event is the core unit for streaming and persistence.
type Event struct {
	ID         string    `json:"id"`
	Type       Type      `json:"type"`
	Time       time.Time `json:"time"`
	SessionID  string    `json:"session_id,omitempty"`
	AgentID    string    `json:"agent_id,omitempty"`
	ToolID     string    `json:"tool_id,omitempty"`
	ProtocolID string    `json:"protocol_id,omitempty"`
	Payload    any       `json:"payload,omitempty"`
}

// New creates a new event with a generated ID and timestamp.
func New(eventType Type, sessionID string, payload any) Event {
	return Event{
		ID:        NewID(),
		Type:      eventType,
		Time:      time.Now().UTC(),
		SessionID: sessionID,
		Payload:   payload,
	}
}

// String renders a compact, log-friendly representation.
func (e Event) String() string {
	var sb strings.Builder
	ts := e.Time.UTC().Format(time.RFC3339)
	sb.WriteString(ts)
	sb.WriteString(" ")
	sb.WriteString(string(e.Type))

	if e.SessionID != "" {
		sb.WriteString(" session=")
		sb.WriteString(e.SessionID)
	}
	if e.ProtocolID != "" {
		sb.WriteString(" protocol=")
		sb.WriteString(e.ProtocolID)
	}
	if e.AgentID != "" {
		sb.WriteString(" agent=")
		sb.WriteString(e.AgentID)
	}
	if e.ToolID != "" {
		sb.WriteString(" tool=")
		sb.WriteString(e.ToolID)
	}

	if summary := Summary(e); summary != "" {
		sb.WriteString(" ")
		sb.WriteString(summary)
	}

	return sb.String()
}

// Summary extracts a short, human-readable summary from the payload.
func Summary(ev Event) string {
	switch ev.Type {
	case AgentMessageDelta:
		if payload, ok := ev.Payload.(map[string]any); ok {
			if delta, ok := payload["delta"].(string); ok {
				return fmt.Sprintf("delta=%q", truncate(delta, 120))
			}
		}
	case AgentReasoningDelta:
		if payload, ok := ev.Payload.(map[string]any); ok {
			if delta, ok := payload["delta"].(string); ok {
				return fmt.Sprintf("reasoning=%q", truncate(delta, 120))
			}
		}
	case AgentReasoningSummaryDelta:
		if payload, ok := ev.Payload.(map[string]any); ok {
			if delta, ok := payload["delta"].(string); ok {
				return fmt.Sprintf("reasoning_summary=%q", truncate(delta, 120))
			}
		}
	case AgentMessageComplete:
		if msg, ok := ev.Payload.(agent.Message); ok {
			if summary := msg.Summary(); summary != "" {
				return fmt.Sprintf("message=%q", truncate(summary, 160))
			}
		}
		if payload, ok := ev.Payload.(map[string]any); ok {
			if msg, ok := payload["message"].(agent.Message); ok {
				if summary := msg.Summary(); summary != "" {
					return fmt.Sprintf("message=%q", truncate(summary, 160))
				}
			}
			if msgMap, ok := payload["message"].(map[string]any); ok {
				msg := agent.MessageFromMap(msgMap)
				if summary := msg.Summary(); summary != "" {
					return fmt.Sprintf("message=%q", truncate(summary, 160))
				}
			}
		}
	case ToolCallError, ToolCallCompleted, ToolCallAwaiting:
		isError := ev.Type == ToolCallError
		if result, ok := ev.Payload.(tool.Result); ok {
			if isError && result.Error != nil && result.Error.Message != "" {
				return fmt.Sprintf("error=%q", truncate(result.Error.Message, 160))
			}
			if text := result.Text(); text != "" {
				return fmt.Sprintf("result=%q", truncate(text, 160))
			}
			if result.Error != nil && result.Error.Message != "" {
				return fmt.Sprintf("error=%q", truncate(result.Error.Message, 160))
			}
		}
		if payload, ok := ev.Payload.(map[string]any); ok {
			if result, ok := payload["result"].(tool.Result); ok {
				if isError && result.Error != nil && result.Error.Message != "" {
					return fmt.Sprintf("error=%q", truncate(result.Error.Message, 160))
				}
				if text := result.Text(); text != "" {
					return fmt.Sprintf("result=%q", truncate(text, 160))
				}
				if result.Error != nil && result.Error.Message != "" {
					return fmt.Sprintf("error=%q", truncate(result.Error.Message, 160))
				}
			}
			if ev.Type == ToolCallAwaiting {
				if req, ok := payload["request"].(tool.Request); ok {
					if req.RequestID != "" {
						return fmt.Sprintf("awaiting=%q", truncate(req.RequestID, 160))
					}
				}
				if reqMap, ok := payload["request"].(map[string]any); ok {
					if id, ok := reqMap["request_id"].(string); ok && id != "" {
						return fmt.Sprintf("awaiting=%q", truncate(id, 160))
					}
				}
				return "awaiting"
			}
			if resultMap, ok := payload["result"].(map[string]any); ok {
				if parts, ok := resultMap["parts"]; ok {
					msg := agent.MessageFromMap(map[string]any{"parts": parts})
					if summary := msg.Summary(); summary != "" {
						return fmt.Sprintf("result=%q", truncate(summary, 160))
					}
				}
				if output, ok := resultMap["output"]; ok && output != nil {
					if raw, err := json.Marshal(output); err == nil {
						return fmt.Sprintf("result=%q", truncate(string(raw), 160))
					}
				}
				if errMap, ok := resultMap["error"].(map[string]any); ok {
					if msg, ok := errMap["message"].(string); ok && msg != "" {
						return fmt.Sprintf("error=%q", truncate(msg, 160))
					}
				}
			}
		}
	case ProtocolAction:
		return "action"
	case MemorySummary:
		if text := summaryText(ev.Payload); text != "" {
			return fmt.Sprintf("memory=%q", truncate(text, 160))
		}
	}
	return ""
}

func summaryText(payload any) string {
	switch value := payload.(type) {
	case string:
		return value
	case map[string]any:
		if mem, ok := value["memory"].(string); ok {
			return mem
		}
		if mem, ok := value["text"].(string); ok {
			return mem
		}
		if mem, ok := value["value"].(string); ok {
			return mem
		}
	}
	return ""
}

func truncate(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}
