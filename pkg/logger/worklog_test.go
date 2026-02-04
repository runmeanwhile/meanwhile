package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/event"
)

func TestWorklogBasicFormatting(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := Worklog(buf, WithTimestamps(false))

	ev := event.New(event.AgentMessageComplete, "sess-123", map[string]any{
		"content": "Test message",
	})
	ev.AgentID = "TestAgent"

	if err := logger.Log(ev); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[TestAgent]") {
		t.Errorf("Expected agent name in output, got: %q", output)
	}
}

func TestWorklogSkipsProviderEvents(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := Worklog(buf)

	ev := event.New(event.ProviderRawEvent, "sess-123", nil)

	if err := logger.Log(ev); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	if buf.Len() > 0 {
		t.Errorf("Expected provider event to be skipped, got output: %q", buf.String())
	}
}

func TestNoopLogger(t *testing.T) {
	logger := NoopLogger()

	ev := event.New(event.AgentMessageComplete, "sess-123", nil)

	if err := logger.Log(ev); err != nil {
		t.Errorf("NoopLogger.Log() error = %v", err)
	}
}

func TestWorklogOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := Worklog(buf,
		WithTimestamps(true),
		WithSessionID(true),
		WithColor(false),
	)

	ev := event.New(event.AgentMessageComplete, "sess-123", map[string]any{
		"content": "Test",
	})
	ev.AgentID = "TestAgent"

	if err := logger.Log(ev); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	output := buf.String()

	// Should have timestamp (HH:MM:SS format)
	if !strings.Contains(output, ":") {
		t.Errorf("Expected timestamp in output, got: %q", output)
	}

	// Should have session ID
	if !strings.Contains(output, "sess-123") {
		t.Errorf("Expected session ID in output, got: %q", output)
	}
}
