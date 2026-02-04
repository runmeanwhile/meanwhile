package engine

import (
	"context"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol/consensus"
)

func TestProtocolCheckpointing_Brainstorming(t *testing.T) {
	store := NewInMemorySessionStore()
	prov := &staticProvider{id: "mock", response: "idea"}

	eng, err := New(WithProvider(prov), WithSessionStore(store))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	registerProtocolFactories(eng, consensus.WithMaxRounds(1))

	participants := []protocol.Participant{
		agent.Agent{Name: "a1", Model: "mock-model"},
		agent.Agent{Name: "a2", Model: "mock-model"},
	}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     protocol.Brainstorming(),
		Participants: participants,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	if _, err := eng.Run(context.Background(), sess.ID(), message.User("topic")); err != nil {
		t.Fatalf("run session: %v", err)
	}

	record, err := store.Load(context.Background(), sess.ID())
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	state, ok := record.Metadata[protocolStateMetadataKey].(map[string]any)
	if !ok {
		t.Fatalf("expected protocol state in metadata")
	}
	if ideaCount := sliceLen(state["ideas"]); ideaCount != len(participants) {
		t.Fatalf("expected %d ideas, got %d", len(participants), ideaCount)
	}

	eng2, err := New(WithProvider(prov), WithSessionStore(store))
	if err != nil {
		t.Fatalf("new engine (rehydrate): %v", err)
	}
	registerProtocolFactories(eng2, consensus.WithMaxRounds(1))
	rehydrated, err := eng2.session(context.Background(), sess.ID())
	if err != nil {
		t.Fatalf("rehydrate session: %v", err)
	}
	stateful, ok := rehydrated.protocol.(protocol.StatefulProtocol)
	if !ok {
		t.Fatalf("expected stateful protocol")
	}
	restored, err := stateful.GetState()
	if err != nil {
		t.Fatalf("get restored state: %v", err)
	}
	if ideaCount := sliceLen(restored["ideas"]); ideaCount != len(participants) {
		t.Fatalf("expected %d ideas after restore, got %d", len(participants), ideaCount)
	}
}

func TestProtocolCheckpointing_Adversarial(t *testing.T) {
	store := NewInMemorySessionStore()
	prov := &staticProvider{id: "mock", response: "argument"}

	eng, err := New(WithProvider(prov), WithSessionStore(store))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	registerProtocolFactories(eng, consensus.WithMaxRounds(1))

	participants := []protocol.Participant{
		agent.Agent{Name: "pro", Model: "mock-model"},
		agent.Agent{Name: "con", Model: "mock-model"},
	}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     protocol.Adversarial(),
		Participants: participants,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	if _, err := eng.Run(context.Background(), sess.ID(), message.User("topic")); err != nil {
		t.Fatalf("run session: %v", err)
	}

	record, err := store.Load(context.Background(), sess.ID())
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	state, ok := record.Metadata[protocolStateMetadataKey].(map[string]any)
	if !ok {
		t.Fatalf("expected protocol state in metadata")
	}
	if state["pro"] == nil || state["con"] == nil {
		t.Fatalf("expected pro/con state")
	}
}

func TestProtocolCheckpointing_Consensus(t *testing.T) {
	store := NewInMemorySessionStore()
	prov := &staticProvider{id: "mock", response: "response"}

	eng, err := New(WithProvider(prov), WithSessionStore(store))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	registerProtocolFactories(eng, consensus.WithMaxRounds(1))

	participants := []protocol.Participant{
		agent.Agent{Name: "p1", Model: "mock-model"},
		agent.Agent{Name: "p2", Model: "mock-model"},
	}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     consensus.Consensus(consensus.WithMaxRounds(1)),
		Participants: participants,
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	if _, err := eng.Run(context.Background(), sess.ID(), message.User("topic")); err != nil {
		t.Fatalf("run session: %v", err)
	}

	record, err := store.Load(context.Background(), sess.ID())
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	state, ok := record.Metadata[protocolStateMetadataKey].(map[string]any)
	if !ok {
		t.Fatalf("expected protocol state in metadata")
	}
	pulseState, ok := state["pulse"].(map[string]any)
	if !ok {
		t.Fatalf("expected pulse state")
	}
	if got := sliceLen(pulseState["positions"]); got != len(participants) {
		t.Fatalf("expected %d pulse positions, got %d", len(participants), got)
	}
}

func sliceLen(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	case []map[string]any:
		return len(v)
	default:
		return 0
	}
}

func registerProtocolFactories(eng *Engine, consensusOpts ...consensus.Option) {
	if eng.protocols == nil {
		eng.protocols = protocol.NewRegistry()
	}
	eng.protocols.Register(protocol.Brainstorming().ID(), func(cfg protocol.Config) protocol.Protocol {
		_ = cfg
		return protocol.Brainstorming()
	})
	eng.protocols.Register(protocol.Adversarial().ID(), func(cfg protocol.Config) protocol.Protocol {
		_ = cfg
		return protocol.Adversarial()
	})
	eng.protocols.Register(consensus.Consensus(consensusOpts...).ID(), func(cfg protocol.Config) protocol.Protocol {
		_ = cfg
		return consensus.Consensus(consensusOpts...)
	})
}
