package engine

import (
	"context"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

type testProvider struct{}

func (p *testProvider) ID() string { return "mock" }
func (p *testProvider) Stream(_ context.Context, _ provider.Request) (provider.Stream, error) {
	return nil, nil
}

func TestApplyConfigRegistersProviders(t *testing.T) {
	provider.RegisterFactory("mock", func(cfg config.ProviderConfig) (provider.Provider, error) {
		return &testProvider{}, nil
	})

	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cfg := config.Config{
		Global: config.GlobalConfig{
			Providers: map[string]config.ProviderConfig{
				"primary": {Type: "mock"},
			},
			Defaults: config.AgentConfig{ProviderID: "primary"},
		},
		Agents: map[string]config.AgentConfig{
			"agent1": {
				Name:       "Agent One",
				ProfileID:  "profile1",
				ProviderID: "primary",
				Tools:      []string{"t1"},
				Params:     map[string]any{"model": "m1"},
			},
		},
		Sessions: map[string]config.SessionConfig{
			"sess1": {
				Name:         "Session 1",
				ProtocolID:   "proto.test",
				Participants: []string{"agent1"},
			},
		},
	}

	eng.RegisterProfile(agent.Profile{ID: "profile1", Name: "Agent One", Prompt: "test"})
	eng.ProtocolRegistry().Register("proto.test", func(cfg protocol.Config) protocol.Protocol {
		return protocol.Solo()
	})

	if err := eng.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig error: %v", err)
	}

	if _, ok := eng.ProviderRegistry().Get("primary"); !ok {
		t.Fatalf("expected provider 'primary' to be registered")
	}
	if eng.defaultProvider != "primary" {
		t.Fatalf("expected default provider to be primary, got %q", eng.defaultProvider)
	}

	ag, err := eng.AgentFromConfig("agent1")
	if err != nil {
		t.Fatalf("AgentFromConfig error: %v", err)
	}
	if ag.Name != "Agent One" {
		t.Fatalf("unexpected agent name: %q", ag.Name)
	}
	if ag.ProviderID != "primary" {
		t.Fatalf("expected agent ProviderID to be primary, got %q", ag.ProviderID)
	}
	if ag.Profile == nil || ag.Profile.ID != "profile1" {
		t.Fatalf("expected profile to be resolved")
	}

	sb, err := eng.SessionFromConfig("sess1")
	if err != nil {
		t.Fatalf("SessionFromConfig error: %v", err)
	}
	sess, err := sb.Build(context.Background())
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if len(sess.Participants()) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(sess.Participants()))
	}
}

func TestApplyConfigMemoryStoreInMemory(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cfg := config.Config{
		Global: config.GlobalConfig{
			Memory: config.MemoryConfig{
				Store: "inmemory",
			},
		},
	}

	if err := eng.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig error: %v", err)
	}

	if _, ok := eng.memory.(*memory.InMemoryStore); !ok {
		t.Fatalf("expected in-memory store, got %T", eng.memory)
	}
}

func TestApplyConfigUnknownMemoryStore(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cfg := config.Config{
		Global: config.GlobalConfig{
			Memory: config.MemoryConfig{
				Store: "mystery",
			},
		},
	}

	if err := eng.ApplyConfig(cfg); err == nil {
		t.Fatalf("expected error for unknown memory store")
	}
}

func TestApplyConfigToolConfigMissingFactory(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cfg := config.Config{
		Tools: map[string]config.ToolConfig{
			"tool1": {Params: map[string]any{"x": 1}},
		},
	}

	if err := eng.ApplyConfig(cfg); err == nil {
		t.Fatalf("expected error for missing tool factory")
	}
}

func TestApplyConfigToolConfigRegistersTool(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	eng.RegisterToolFactory(testFactory{id: "tool1"})

	cfg := config.Config{
		Tools: map[string]config.ToolConfig{
			"tool1": {Params: map[string]any{"x": 1}},
		},
	}

	if err := eng.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig error: %v", err)
	}

	if _, ok := eng.ToolRegistry().Get("tool1"); !ok {
		t.Fatalf("expected tool to be registered")
	}
}

type testFactory struct{ id string }

func (t testFactory) ID() string { return t.id }

func (t testFactory) Build(_ map[string]any) (tool.Tool, error) {
	return tool.Func{IDValue: t.id, SchemaValue: tool.Schema{JSONSchema: []byte(`{}`)}}, nil
}

func TestApplyConfigFileStoreSyncEvery(t *testing.T) {
	dir := t.TempDir()
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cfg := config.Config{
		Global: config.GlobalConfig{
			Memory: config.MemoryConfig{
				Store: "file",
				Params: map[string]any{
					"path":       dir,
					"sync_every": 3,
				},
			},
		},
	}

	if err := eng.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig error: %v", err)
	}

	store, ok := eng.memory.(*memory.FileChatStore)
	if !ok {
		t.Fatalf("expected file store, got %T", eng.memory)
	}
	if store == nil || store.SyncEvery() != 3 {
		t.Fatalf("expected syncEvery=3")
	}
}
