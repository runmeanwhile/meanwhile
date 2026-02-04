package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

type configProtocol struct {
	cfg protocol.Config
}

func (p *configProtocol) ID() string { return "test.protocol" }

func (p *configProtocol) Participants() []protocol.Participant { return nil }

func (p *configProtocol) Init(ctx context.Context, sess protocol.Session) error {
	_ = ctx
	_ = sess
	return nil
}

func (p *configProtocol) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	_ = ctx
	_ = sess
	_ = msg
	if mode, _ := p.cfg["mode"].(string); mode != "persisted" {
		return fmt.Errorf("expected persisted config, got %v", p.cfg["mode"])
	}
	return nil
}

func (p *configProtocol) OnEvent(ctx context.Context, sess protocol.Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}

func (p *configProtocol) Shutdown(ctx context.Context, sess protocol.Session) error {
	_ = ctx
	_ = sess
	return nil
}

func (p *configProtocol) Config() protocol.Config { return p.cfg }

func TestSessionStoreRehydratesSession(t *testing.T) {
	store := NewInMemorySessionStore()

	engA, err := New(WithSessionStore(store))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	engB, err := New(WithSessionStore(store))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	factory := func(cfg protocol.Config) protocol.Protocol {
		return &configProtocol{cfg: cfg}
	}
	engA.protocols.Register("test.protocol", factory)
	engB.protocols.Register("test.protocol", factory)

	proto := &configProtocol{cfg: protocol.Config{"mode": "persisted"}}
	sess, err := engA.NewSession(context.Background(), SessionConfig{
		ID:           "sess-1",
		Name:         "persisted",
		Protocol:     proto,
		Participants: []protocol.Participant{agent.Agent{Name: "Alice"}},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	_, err = engB.Run(context.Background(), sess.ID(), message.User("hello"))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestSessionStoreGroupsUseAgentIDWhenPresent(t *testing.T) {
	store := NewInMemorySessionStore()

	engA, err := New(WithSessionStore(store))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	engB, err := New(WithSessionStore(store))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	factory := func(cfg protocol.Config) protocol.Protocol {
		return &configProtocol{cfg: cfg}
	}
	engA.protocols.Register("test.protocol", factory)
	engB.protocols.Register("test.protocol", factory)

	agentA := agent.Agent{ID: "agent-1", Name: "Agent", Model: "m1"}
	agentB := agent.Agent{ID: "agent-2", Name: "Agent", Model: "m1"}

	sess, err := engA.NewSession(context.Background(), SessionConfig{
		ID:           "sess-groups",
		Name:         "persisted",
		Protocol:     &configProtocol{cfg: protocol.Config{"mode": "persisted"}},
		Participants: []protocol.Participant{agentA, agentB},
		Groups: map[string][]protocol.Participant{
			"group-a": {agentA},
			"group-b": {agentB},
		},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	_, err = engB.Run(context.Background(), sess.ID(), message.User("hello"))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	rehydrated, err := engB.session(context.Background(), sess.ID())
	if err != nil {
		t.Fatalf("session lookup failed: %v", err)
	}

	groups := rehydrated.Groups()
	if len(groups["group-a"]) != 1 || groups["group-a"][0].Identifier() != "agent-1" {
		t.Fatalf("expected group-a to contain agent-1")
	}
	if len(groups["group-b"]) != 1 || groups["group-b"][0].Identifier() != "agent-2" {
		t.Fatalf("expected group-b to contain agent-2")
	}
}

func TestSessionStorePersistsState(t *testing.T) {
	store := NewInMemorySessionStore()

	eng, err := New(WithSessionStore(store))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	human := eng.Human("User").Build()
	proto := &awaitProtocol{participant: human}
	sess, err := eng.Session("Await State").
		Participant(human).
		Protocol(proto).
		Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result, err := sess.Run(context.Background(), message.User("start"))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Status != StatusAwaitingInput {
		t.Fatalf("expected awaiting status, got %s", result.Status)
	}

	state, err := store.LoadState(context.Background(), sess.ID())
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if len(state.Pending) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(state.Pending))
	}
	if state.Pending[0].RequestID != result.RequestID {
		t.Fatalf("expected request ID to match persisted state")
	}

	if _, err := sess.Respond(context.Background(), result.RequestID, message.User("ok")); err != nil {
		t.Fatalf("Respond failed: %v", err)
	}
	if _, err := store.LoadState(context.Background(), sess.ID()); err == nil {
		t.Fatalf("expected state to be cleared after respond")
	}
}

type stateAwaitProtocol struct{}

func (p *stateAwaitProtocol) ID() string                                       { return "test.await.state" }
func (p *stateAwaitProtocol) Participants() []protocol.Participant             { return nil }
func (p *stateAwaitProtocol) Init(_ context.Context, _ protocol.Session) error { return nil }
func (p *stateAwaitProtocol) OnMessage(ctx context.Context, sess protocol.Session, _ agent.Message) error {
	var human protocol.Participant
	for _, participant := range sess.Participants() {
		if participant != nil && participant.IsHuman() {
			human = participant
			break
		}
	}
	if human == nil {
		return fmt.Errorf("human required")
	}
	return sess.AwaitInput(ctx, human, "need input", func(context.Context, agent.Message) error {
		return nil
	})
}
func (p *stateAwaitProtocol) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}
func (p *stateAwaitProtocol) Shutdown(_ context.Context, _ protocol.Session) error { return nil }

func TestSessionStoreRehydratesPendingState(t *testing.T) {
	store := NewInMemorySessionStore()
	factory := func(protocol.Config) protocol.Protocol {
		return &stateAwaitProtocol{}
	}

	engA, err := New(WithSessionStore(store))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	engB, err := New(WithSessionStore(store))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	engA.protocols.Register("test.await.state", factory)
	engB.protocols.Register("test.await.state", factory)

	human := engA.Human("User").Build()
	sessA, err := engA.NewSession(context.Background(), SessionConfig{
		ID:           "sess-state",
		Name:         "persisted-state",
		Protocol:     &stateAwaitProtocol{},
		Participants: []protocol.Participant{human},
	})
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}

	result, err := sessA.Run(context.Background(), message.User("start"))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.Status != StatusAwaitingInput {
		t.Fatalf("expected awaiting status, got %s", result.Status)
	}

	sessB, err := engB.session(context.Background(), sessA.ID())
	if err != nil {
		t.Fatalf("session rehydrate failed: %v", err)
	}

	pending := sessB.PendingRequests()
	if len(pending) != 1 || pending[0].RequestID != result.RequestID {
		t.Fatalf("expected pending request to be restored")
	}

	if _, err := engB.SessionForRequest(context.Background(), result.RequestID); err != nil {
		t.Fatalf("expected request registry to resolve: %v", err)
	}

	if _, err := sessB.Respond(context.Background(), result.RequestID, message.User("ok")); err != ErrSessionNotResumable {
		t.Fatalf("expected ErrSessionNotResumable, got %v", err)
	}
}
