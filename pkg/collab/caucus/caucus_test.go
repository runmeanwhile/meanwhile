package caucus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

func TestRunCaucusIsolatedThreads(t *testing.T) {
	participants := []agent.Agent{
		{Name: "alice"},
		{Name: "bob"},
	}

	builder := func(ag agent.Agent, thread []agent.Message, round, maxRounds int) protocol.RunRequest {
		_ = round
		_ = maxRounds
		return protocol.RunRequest{Messages: append([]agent.Message(nil), thread...)}
	}

	run := func(_ context.Context, ag agent.Agent, req protocol.RunRequest) (agent.Message, error) {
		for _, msg := range req.Messages {
			if msg.Role != agent.RoleAssistant {
				continue
			}
			if msg.Name == "" {
				continue
			}
			if msg.Name != ag.Name {
				return agent.Message{}, errors.New("unexpected message from " + msg.Name + " in " + ag.Name + " thread")
			}
		}
		return message.Assistant("note from " + ag.Name), nil
	}

	seed := message.User("Discuss privately.")
	result, err := Run(context.Background(), run, participants, builder, WithSeedMessage(seed), WithRounds(2))
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if len(result.Threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(result.Threads))
	}

	brief := result.Brief()
	if !strings.Contains(brief, "[alice]") || !strings.Contains(brief, "[bob]") {
		t.Fatalf("expected brief to include both participants, got %q", brief)
	}
}

func TestRunCaucusValidationErrors(t *testing.T) {
	_, err := Run(context.Background(), nil, nil, func(agent.Agent, []agent.Message, int, int) protocol.RunRequest {
		return protocol.RunRequest{}
	})
	if err == nil {
		t.Fatal("expected error for missing participants")
	}

	_, err = Run(context.Background(), nil, []agent.Agent{{Name: "alice"}}, nil)
	if err == nil {
		t.Fatal("expected error for missing turn builder")
	}

	builder := func(agent.Agent, []agent.Message, int, int) protocol.RunRequest { return protocol.RunRequest{} }
	_, err = Run(context.Background(), func(context.Context, agent.Agent, protocol.RunRequest) (agent.Message, error) {
		return agent.Message{}, nil
	}, []agent.Agent{{Name: "alice"}}, builder, func(cfg *Config) { cfg.Rounds = 0 })
	if err == nil {
		t.Fatal("expected error for invalid rounds")
	}
}
