package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// WorklogFormat formats events as a clean workplace narrative log.
// Emphasizes terse, readable output with optional timestamps and colors.
type WorklogFormat struct {
	out            io.Writer
	showTimestamps bool
	showSessionID  bool
	useColor       bool

	// State tracking for message aggregation
	currentMessage strings.Builder
	currentAgent   string
}

// WorklogOption configures a WorklogFormat instance.
type WorklogOption func(*WorklogFormat)

// WithTimestamps enables timestamp display (HH:MM:SS format).
func WithTimestamps(show bool) WorklogOption {
	return func(w *WorklogFormat) {
		w.showTimestamps = show
	}
}

// WithSessionID enables session ID display in logs.
func WithSessionID(show bool) WorklogOption {
	return func(w *WorklogFormat) {
		w.showSessionID = show
	}
}

// WithColor enables ANSI color codes in output.
func WithColor(use bool) WorklogOption {
	return func(w *WorklogFormat) {
		w.useColor = use
	}
}

// Worklog creates a new workplace-themed logger.
func Worklog(out io.Writer, opts ...WorklogOption) Logger {
	w := &WorklogFormat{
		out:            out,
		showTimestamps: true,
		showSessionID:  false,
		useColor:       false,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Log formats and writes a single event.
func (w *WorklogFormat) Log(ev event.Event) error {
	// Filter out noisy events
	if w.shouldSkip(ev) {
		return nil
	}

	var line string
	switch ev.Type {
	case event.AgentStarted:
		// Agent starts responding
		w.currentAgent = ev.AgentID
		w.currentMessage.Reset()
		line = w.formatAgentStart(ev)

	case event.AgentMessageDelta:
		// Accumulate message content
		if payload, ok := ev.Payload.(map[string]any); ok {
			if delta, ok := payload["delta"].(string); ok {
				w.currentMessage.WriteString(delta)
			}
		}
		return nil // Don't print deltas individually

	case event.AgentMessageComplete:
		// Print complete message
		// Use the message from the event payload (which contains the complete text),
		// not the accumulated deltas which may duplicate content
		content := ""
		if payload, ok := ev.Payload.(map[string]any); ok {
			if msg, ok := payload["message"].(agent.Message); ok {
				content = msg.Summary()
			}
		}
		// Fallback to accumulated deltas if no message in payload
		if content == "" {
			content = w.currentMessage.String()
		}
		line = w.formatAgentMessage(ev, content)
		w.currentMessage.Reset()

	case event.ToolCallStarted:
		line = w.formatToolCall(ev)

	case event.ToolCallCompleted, event.ToolCallError:
		line = w.formatToolResult(ev)

	case event.ToolCallAwaiting:
		line = w.formatToolAwaiting(ev)

	default:
		// Other events get basic formatting
		line = w.formatGeneric(ev)
	}

	if line != "" {
		if _, err := fmt.Fprintln(w.out, line); err != nil {
			return err
		}
	}
	return nil
}

func (w *WorklogFormat) shouldSkip(ev event.Event) bool {
	// Skip provider-level events (too noisy)
	if strings.HasPrefix(string(ev.Type), "provider.") {
		return true
	}
	// Skip thinking events
	if ev.Type == "agent.thinking" {
		return true
	}
	return false
}

func (w *WorklogFormat) formatAgentStart(ev event.Event) string {
	prefix := w.buildPrefix(ev)
	agent := w.colorize(ev.AgentID, colorCyan)
	return fmt.Sprintf("%s[%s] composing response...", prefix, agent)
}

func (w *WorklogFormat) formatAgentMessage(ev event.Event, content string) string {
	prefix := w.buildPrefix(ev)
	agent := w.colorize(ev.AgentID, colorCyan)

	// Clean up whitespace
	content = strings.TrimSpace(content)

	return fmt.Sprintf("%s[%s] %s", prefix, agent, content)
}

func (w *WorklogFormat) formatToolCall(ev event.Event) string {
	prefix := w.buildPrefix(ev)
	toolName := w.colorize(ev.ToolID, colorYellow)

	// Include agent name if available
	agentPart := ""
	if ev.AgentID != "" {
		agent := w.colorize(ev.AgentID, colorCyan)
		agentPart = fmt.Sprintf("[%s] ", agent)
	}

	// Extract tool arguments if present
	var argHint string
	if payload, ok := ev.Payload.(map[string]any); ok {
		if args, ok := payload["arguments"].(string); ok && len(args) > 0 {
			argHint = formatArgHint(args)
		}
		if argHint == "" {
			switch call := payload["call"].(type) {
			case tool.Call:
				argHint = formatArgHint(string(call.Arguments))
			case map[string]any:
				if raw, ok := call["arguments"]; ok {
					if hint := formatArgHint(stringifyRaw(raw)); hint != "" {
						argHint = hint
					}
				}
			}
		}
	}

	return fmt.Sprintf("%s%s→ calling [%s]%s", prefix, agentPart, toolName, argHint)
}

func (w *WorklogFormat) formatToolResult(ev event.Event) string {
	prefix := w.buildPrefix(ev)
	toolName := w.colorize(ev.ToolID, colorYellow)

	// Include agent name if available
	agentPart := ""
	if ev.AgentID != "" {
		agent := w.colorize(ev.AgentID, colorCyan)
		agentPart = fmt.Sprintf("[%s] ", agent)
	}

	result := "completed"
	if payload, ok := ev.Payload.(map[string]any); ok {
		if errStr, ok := payload["error"].(string); ok && errStr != "" {
			result = w.colorize("error: "+errStr, colorRed)
		} else if output, ok := payload["output"].(string); ok {
			result = truncateResult(output)
		} else if res, ok := payload["result"].(tool.Result); ok {
			if res.Error != nil && res.Error.Message != "" {
				result = w.colorize("error: "+res.Error.Message, colorRed)
			} else if text := res.Text(); text != "" {
				result = truncateResult(text)
			}
		} else if resMap, ok := payload["result"].(map[string]any); ok {
			if errMap, ok := resMap["error"].(map[string]any); ok {
				if msg, ok := errMap["message"].(string); ok && msg != "" {
					result = w.colorize("error: "+msg, colorRed)
				}
			}
			if result == "completed" {
				if output, ok := resMap["output"].(string); ok && output != "" {
					result = truncateResult(output)
				}
			}
		}
	}

	return fmt.Sprintf("%s%s← [%s] %s", prefix, agentPart, toolName, result)
}

func (w *WorklogFormat) formatToolAwaiting(ev event.Event) string {
	prefix := w.buildPrefix(ev)
	toolName := w.colorize(ev.ToolID, colorYellow)

	agentPart := ""
	if ev.AgentID != "" {
		agent := w.colorize(ev.AgentID, colorCyan)
		agentPart = fmt.Sprintf("[%s] ", agent)
	}

	status := "awaiting result"
	if payload, ok := ev.Payload.(map[string]any); ok {
		if req, ok := payload["request"].(tool.Request); ok {
			if req.RequestID != "" {
				status = fmt.Sprintf("awaiting result (%s)", req.RequestID)
			}
		} else if reqMap, ok := payload["request"].(map[string]any); ok {
			if id, ok := reqMap["request_id"].(string); ok && id != "" {
				status = fmt.Sprintf("awaiting result (%s)", id)
			}
		}
	}

	return fmt.Sprintf("%s%s← [%s] %s", prefix, agentPart, toolName, status)
}

func formatArgHint(args string) string {
	if strings.TrimSpace(args) == "" {
		return ""
	}
	return fmt.Sprintf(" with %s", truncateResult(args))
}

func truncateResult(value string) string {
	if len(value) > 60 {
		return value[:57] + "..."
	}
	return value
}

func stringifyRaw(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case json.RawMessage:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (w *WorklogFormat) formatSessionStart(ev event.Event) string {
	prefix := w.buildPrefix(ev)
	sessionName := "Unnamed"
	if payload, ok := ev.Payload.(map[string]any); ok {
		if name, ok := payload["session_name"].(string); ok {
			sessionName = name
		}
	}
	return fmt.Sprintf("%s=== Session: %s ===", prefix, sessionName)
}

func (w *WorklogFormat) formatSessionEnd(ev event.Event) string {
	prefix := w.buildPrefix(ev)
	return fmt.Sprintf("%s=== Session complete ===", prefix)
}

func (w *WorklogFormat) formatGeneric(ev event.Event) string {
	prefix := w.buildPrefix(ev)

	// Special handling for moderator interventions
	if ev.Type == event.ProtocolAction {
		if payload, ok := ev.Payload.(map[string]any); ok {
			if evType, ok := payload["type"].(string); ok && evType == "consensus.moderator" {
				if intervention, ok := payload["intervention"].(string); ok {
					// Truncate if needed
					if len(intervention) > 300 {
						intervention = intervention[:297] + "..."
					}
					return fmt.Sprintf("%s%s", prefix, w.colorize(intervention, colorMagenta))
				}
			}
		}
	}

	return fmt.Sprintf("%s[%s] %v", prefix, ev.Type, ev.Payload)
}

func (w *WorklogFormat) buildPrefix(ev event.Event) string {
	var parts []string

	if w.showTimestamps {
		ts := ev.Time.Format("15:04:05")
		parts = append(parts, w.colorize(ts, colorGray))
	}

	if w.showSessionID {
		sid := ev.SessionID
		if len(sid) > 8 {
			sid = sid[:8]
		}
		parts = append(parts, w.colorize(fmt.Sprintf("[%s]", sid), colorGray))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ") + " "
}

func (w *WorklogFormat) colorize(text string, color string) string {
	if !w.useColor {
		return text
	}
	return color + text + colorReset
}

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorGray    = "\033[90m"
	colorCyan    = "\033[36m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorMagenta = "\033[35m"
)

// Noop is a logger that discards all events.
type Noop struct{}

// Log does nothing.
func (n *Noop) Log(_ event.Event) error {
	return nil
}

// NoopLogger returns a logger that discards all output.
func NoopLogger() Logger {
	return &Noop{}
}
