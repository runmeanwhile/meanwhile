package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/hook"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

type testProtocol struct {
	id     string
	mu     sync.Mutex
	msgs   []agent.Message
	events []event.Event
}

func (t *testProtocol) ID() string                                       { return t.id }
func (t *testProtocol) Participants() []protocol.Participant             { return nil }
func (t *testProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }

func (t *testProtocol) OnMessage(_ context.Context, _ protocol.Session, msg agent.Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.msgs = append(t.msgs, msg)
	return nil
}

func (t *testProtocol) OnEvent(_ context.Context, _ protocol.Session, ev event.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, ev)
	return nil
}

func (t *testProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func TestEngineRunWithHook(t *testing.T) {
	reg := hook.NewRegistry()
	reg.Register(modifyHook{})

	proto := &testProtocol{id: "test"}
	eng, err := New(WithHookRegistry(reg))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     proto,
		Participants: []protocol.Participant{agent.Agent{Name: "Agent 1"}},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.Run(context.Background(), message.User("hi"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	proto.mu.Lock()
	defer proto.mu.Unlock()
	if len(proto.msgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(proto.msgs))
	}
	if proto.msgs[0].Text() != "hi (modified)" {
		t.Fatalf("unexpected message content: %s", proto.msgs[0].Text())
	}
}

func TestEngineSubscribe(t *testing.T) {
	proto := &testProtocol{id: "test"}
	eng, err := New()
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     proto,
		Participants: []protocol.Participant{agent.Agent{Name: "Agent 1"}},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	done := make(chan struct{})
	_, err = eng.Subscribe(sess.ID(), func(ev event.Event) {
		if ev.Type == event.AgentStarted {
			close(done)
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := sess.Emit(event.New(event.AgentStarted, sess.ID(), nil)); err != nil {
		t.Fatalf("emit: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected event")
	}
}

type modifyHook struct{}

func (modifyHook) ID() string    { return "mod" }
func (modifyHook) Priority() int { return 0 }
func (modifyHook) OnPreMessage(_ context.Context, _ hook.SessionMeta, msg agent.Message) (hook.Decision, agent.Message, error) {
	text := msg.Text()
	msg.Parts = []agent.ContentPart{{Type: agent.ContentPartText, Text: text + " (modified)"}}
	return hook.Modify, msg, nil
}

type actionProtocol struct {
	id string
}

func (a *actionProtocol) ID() string                                       { return a.id }
func (a *actionProtocol) Participants() []protocol.Participant             { return nil }
func (a *actionProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (a *actionProtocol) OnMessage(_ context.Context, sess protocol.Session, _ agent.Message) error {
	return sess.Emit(event.New(event.ProtocolAction, sess.ID(), map[string]any{"source": "event"}))
}
func (a *actionProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (a *actionProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

type providerProtocol struct {
	actionProtocol
	result map[string]any
}

func (p *providerProtocol) Result() map[string]any {
	return p.result
}

func TestRunResultMetadataFromProtocolAction(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     &actionProtocol{id: "action"},
		Participants: []protocol.Participant{agent.Agent{Name: "Agent 1"}},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	result, err := sess.Run(context.Background(), message.User("hi"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Metadata["source"] != "event" {
		t.Fatalf("expected protocol action metadata, got %#v", result.Metadata)
	}
}

func TestRunResultMetadataFromResultProvider(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol: &providerProtocol{
			actionProtocol: actionProtocol{id: "provider"},
			result:         map[string]any{"source": "provider"},
		},
		Participants: []protocol.Participant{agent.Agent{Name: "Agent 1"}},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	result, err := sess.Run(context.Background(), message.User("hi"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Metadata["source"] != "provider" {
		t.Fatalf("expected provider metadata, got %#v", result.Metadata)
	}
}
