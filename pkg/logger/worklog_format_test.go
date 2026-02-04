package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

func TestWorklogFormatAgentStart(t *testing.T) {
	var buf bytes.Buffer
	w := Worklog(&buf, WithColor(false))

	ev := event.Event{
		Type:      event.AgentStarted,
		AgentID:   "agent-1",
		SessionID: "sess-123",
		Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	if err := w.Log(ev); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "agent-1") {
		t.Errorf("Expected output to contain agent name, got: %s", output)
	}
	if !strings.Contains(output, "composing response") {
		t.Errorf("Expected output to contain 'composing response', got: %s", output)
	}
}

func TestWorklogFormatAgentMessageComplete(t *testing.T) {
	var buf bytes.Buffer
	w := Worklog(&buf, WithColor(false))

	ev := event.Event{
		Type:      event.AgentMessageComplete,
		AgentID:   "agent-1",
		SessionID: "sess-123",
		Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Payload: map[string]any{
			"message": agent.Message{
				Role:  agent.RoleAssistant,
				Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Hello, world!"}},
			},
		},
	}

	if err := w.Log(ev); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "agent-1") {
		t.Errorf("Expected output to contain agent name, got: %s", output)
	}
	if !strings.Contains(output, "Hello, world!") {
		t.Errorf("Expected output to contain message content, got: %s", output)
	}
}

func TestWorklogFormatToolCall(t *testing.T) {
	var buf bytes.Buffer
	w := Worklog(&buf, WithColor(false))

	ev := event.Event{
		Type:      event.ToolCallStarted,
		AgentID:   "agent-1",
		ToolID:    "search_tool",
		SessionID: "sess-123",
		Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Payload: map[string]any{
			"call": tool.Call{
				ToolID:    "search_tool",
				Arguments: json.RawMessage(`{"query":"test"}`),
			},
		},
	}

	if err := w.Log(ev); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "search_tool") {
		t.Errorf("Expected output to contain tool name, got: %s", output)
	}
	if !strings.Contains(output, "calling") {
		t.Errorf("Expected output to contain 'calling', got: %s", output)
	}
}

func TestWorklogFormatToolResult(t *testing.T) {
	var buf bytes.Buffer
	w := Worklog(&buf, WithColor(false))

	ev := event.Event{
		Type:      event.ToolCallCompleted,
		AgentID:   "agent-1",
		ToolID:    "search_tool",
		SessionID: "sess-123",
		Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		Payload: map[string]any{
			"result": tool.Result{
				ToolID: "search_tool",
				Parts:  []agent.ContentPart{{Type: agent.ContentPartText, Text: "Search results found"}},
			},
		},
	}

	if err := w.Log(ev); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "search_tool") {
		t.Errorf("Expected output to contain tool name, got: %s", output)
	}
	if !strings.Contains(output, "Search results found") {
		t.Errorf("Expected output to contain result, got: %s", output)
	}
}

func TestWorklogShowTimestamps(t *testing.T) {
	var buf bytes.Buffer
	w := Worklog(&buf, WithColor(false), WithTimestamps(true))

	ev := event.Event{
		Type:      event.AgentStarted,
		AgentID:   "agent-1",
		SessionID: "sess-123",
		Time:      time.Date(2026, 1, 15, 10, 30, 45, 0, time.UTC),
	}

	if err := w.Log(ev); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	output := buf.String()
	// Should contain timestamp
	if !strings.Contains(output, "10:30:45") {
		t.Errorf("Expected timestamp in output, got: %s", output)
	}
}

func TestWorklogNoTimestamps(t *testing.T) {
	var buf bytes.Buffer
	w := Worklog(&buf, WithColor(false), WithTimestamps(false))

	ev := event.Event{
		Type:      event.AgentStarted,
		AgentID:   "agent-1",
		SessionID: "sess-123",
		Time:      time.Date(2026, 1, 15, 10, 30, 45, 0, time.UTC),
	}

	if err := w.Log(ev); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	output := buf.String()
	// Should not contain full timestamp
	if strings.Contains(output, "10:30:45") {
		t.Errorf("Expected no timestamp in output, got: %s", output)
	}
}

func TestWorklogWithColor(t *testing.T) {
	var buf bytes.Buffer
	w := Worklog(&buf, WithColor(true))

	ev := event.Event{
		Type:      event.AgentStarted,
		AgentID:   "agent-1",
		SessionID: "sess-123",
		Time:      time.Now(),
	}

	if err := w.Log(ev); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	output := buf.String()
	// Should contain ANSI color codes
	if !strings.Contains(output, "\x1b[") {
		t.Error("Expected color codes when color is enabled")
	}
}

func TestWorklogProtocolAction(t *testing.T) {
	var buf bytes.Buffer
	w := Worklog(&buf, WithColor(false))

	ev := event.Event{
		Type:      event.ProtocolAction,
		SessionID: "sess-123",
		Time:      time.Now(),
		Payload: map[string]any{
			"type":         "consensus.moderator",
			"intervention": "Let's focus on the main topic",
		},
	}

	if err := w.Log(ev); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Let's focus on the main topic") {
		t.Errorf("Expected intervention text in output, got: %s", output)
	}
}
