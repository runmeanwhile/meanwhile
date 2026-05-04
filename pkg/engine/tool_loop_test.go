package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

type noopProtocol struct{}

func (noopProtocol) ID() string                                       { return "noop" }
func (noopProtocol) Participants() []protocol.Participant             { return nil }
func (noopProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (noopProtocol) OnMessage(_ context.Context, _ protocol.Session, _ agent.Message) error {
	return nil
}
func (noopProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (noopProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func TestRunAgentToolLoopExecutesTools(t *testing.T) {
	prov := &scriptedProvider{}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	toolRun := &echoTool{}
	eng.ToolRegistry().Register(toolRun)

	preCalled := false
	postCalled := false
	eng.HookRegistry().Register(toolHook{pre: &preCalled, post: &postCalled})

	participant := agent.Agent{Name: "agent", Model: "test", Tools: []string{"echo"}}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	msg, err := sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages:          []agent.Message{{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "do"}}}},
		MaxToolIterations: 2,
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if msg.Text() != "done" {
		t.Fatalf("unexpected message: %s", msg.Text())
	}

	if !preCalled || !postCalled {
		t.Fatalf("expected hooks called pre=%v post=%v", preCalled, postCalled)
	}

	if len(prov.requests) != 2 {
		t.Fatalf("expected 2 provider calls, got %d", len(prov.requests))
	}
	if len(prov.requests[1].Messages) < 2 {
		t.Fatalf("expected tool message in second call")
	}
	last := prov.requests[1].Messages[len(prov.requests[1].Messages)-1]
	if last.Role != modelruntime.RoleTool {
		t.Fatalf("expected last message to be tool, got %s", last.Role)
	}
	if last.Text() != "hello!" {
		t.Fatalf("unexpected tool message: %s", last.Text())
	}
	if toolRun.count == 0 {
		t.Fatal("expected tool to run")
	}
}

func TestRunAgentToolIterationsExceeded(t *testing.T) {
	prov := &scriptedProvider{alwaysToolCall: true}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	eng.ToolRegistry().Register(&echoTool{})

	participant := agent.Agent{Name: "agent", Model: "test", Tools: []string{"echo"}}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages:          []agent.Message{{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "do"}}}},
		MaxToolIterations: 1,
	})
	if !errors.Is(err, ErrToolIterationsExceeded) {
		t.Fatalf("expected ErrToolIterationsExceeded, got %v", err)
	}
}

func TestRunAgentToolIterationsExceededStopsBeforeFinalTool(t *testing.T) {
	prov := &scriptedProvider{alwaysToolCall: true}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	toolRun := &echoTool{}
	eng.ToolRegistry().Register(toolRun)

	participant := agent.Agent{Name: "agent", Model: "test", Tools: []string{"echo"}}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages:          []agent.Message{{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "do"}}}},
		MaxToolIterations: 1,
	})
	if !errors.Is(err, ErrToolIterationsExceeded) {
		t.Fatalf("expected ErrToolIterationsExceeded, got %v", err)
	}
	if toolRun.count != 1 {
		t.Fatalf("expected tool to run once before limit, got %d", toolRun.count)
	}
}

func TestRunAgentProviderToolResultUnsupported(t *testing.T) {
	prov := &toolResultProvider{}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	participant := agent.Agent{Name: "agent", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages: []agent.Message{{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "do"}}}},
	})
	if err == nil {
		t.Fatalf("expected error for provider tool result events")
	}
}

// scriptedProvider returns tool call on first request, message on second.
type scriptedProvider struct {
	mu             sync.Mutex
	calls          int
	requests       []provider.Request
	alwaysToolCall bool
}

func (s *scriptedProvider) ID() string { return "scripted" }

func (s *scriptedProvider) Stream(_ context.Context, req provider.Request) (provider.Stream, error) {
	s.mu.Lock()
	s.calls++
	s.requests = append(s.requests, req)
	call := s.calls
	always := s.alwaysToolCall
	s.mu.Unlock()

	if always || call == 1 {
		return &scriptedStream{events: []provider.Event{{
			Type: provider.EventToolCall,
			ToolCalls: []provider.ToolCall{{
				ID:        "call-1",
				ToolID:    "echo",
				Arguments: json.RawMessage(`{"text":"hello"}`),
			}},
		}}}, nil
	}

	return &scriptedStream{events: []provider.Event{{
		Type:    provider.EventMessageCompleted,
		Message: runtimeTextMessage(modelruntime.RoleAssistant, "done"),
	}}}, nil
}

type scriptedStream struct {
	index  int
	events []provider.Event
}

func (s *scriptedStream) Recv() (provider.Event, error) {
	if s.index >= len(s.events) {
		return provider.Event{}, io.EOF
	}
	ev := s.events[s.index]
	s.index++
	return ev, nil
}

func (s *scriptedStream) Close() error { return nil }

type toolResultProvider struct{}

func (p *toolResultProvider) ID() string { return "toolresult" }
func (p *toolResultProvider) Stream(_ context.Context, _ provider.Request) (provider.Stream, error) {
	return &scriptedStream{events: []provider.Event{{
		Type: provider.EventToolResult,
	}}}, nil
}

// echoTool returns a fixed response and counts executions.
type echoTool struct{ count int }

func (e *echoTool) ID() string { return "echo" }
func (e *echoTool) Schema() tool.Schema {
	return tool.Schema{JSONSchema: json.RawMessage(`{"type":"object"}`)}
}
func (e *echoTool) Run(_ context.Context, call tool.Call, emit tool.Emitter) (tool.Result, error) {
	e.count++
	_ = emit.Emit("progress", map[string]any{"step": "run"})
	return tool.TextResult(call, "hello"), nil
}

// toolHook modifies tool results.
type toolHook struct {
	pre  *bool
	post *bool
}

func (toolHook) ID() string    { return "toolhook" }
func (toolHook) Priority() int { return 0 }

func (h toolHook) OnPreToolUse(_ context.Context, _ hook.SessionMeta, call tool.Call) (hook.Decision, tool.Call, error) {
	*h.pre = true
	return hook.Modify, call, nil
}

func (h toolHook) OnPostToolUse(_ context.Context, _ hook.SessionMeta, res tool.Result) (hook.Decision, tool.Result, error) {
	*h.post = true
	text := res.Text()
	res.Parts = []agent.ContentPart{{Type: agent.ContentPartText, Text: text + "!"}}
	return hook.Modify, res, nil
}
