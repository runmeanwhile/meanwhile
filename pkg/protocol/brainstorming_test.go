package protocol

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
)

func TestBrainstormingProtocol_ID(t *testing.T) {
	p := Brainstorming()
	if p.ID() != "protocol.brainstorming" {
		t.Errorf("expected ID 'protocol.brainstorming', got '%s'", p.ID())
	}
}

func TestBrainstormingProtocol_Participants(t *testing.T) {
	p := Brainstorming()
	participants := p.Participants()
	if participants != nil {
		t.Errorf("expected nil participants, got %v", participants)
	}
}

func TestBrainstormingProtocol_Init(t *testing.T) {
	p := Brainstorming()
	sess := &mockSession{id: "test"}

	err := p.Init(context.Background(), sess)
	if err != nil {
		t.Errorf("Init() failed: %v", err)
	}
}

func TestBrainstormingProtocol_OnMessage_Success(t *testing.T) {
	p := Brainstorming(
		WithBrainstormingInteractionRounds(1),
		WithBrainstormingShortlistSize(0),
		WithBrainstormingVoting(false),
	)

	callCount := 0
	var mu sync.Mutex
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
			agent.Agent{Name: "Agent2"},
			agent.Agent{Name: "Agent3"},
		},
		emittedEvents: []event.Event{},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return message.Assistant("idea from " + ag.Name), nil
		},
	}

	msg := message.User("generate ideas")
	err := p.OnMessage(context.Background(), sess, msg)

	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	mu.Lock()
	expectedCalls := 6 // 3 participants * (1 divergent + 1 interaction)
	if callCount != expectedCalls {
		t.Errorf("expected %d agent calls, got %d", expectedCalls, callCount)
	}
	mu.Unlock()

	if len(sess.emittedEvents) != 1 {
		t.Errorf("expected 1 emitted event, got %d", len(sess.emittedEvents))
	}

	if sess.emittedEvents[0].Type != event.ProtocolAction {
		t.Errorf("expected event type ProtocolAction, got %v", sess.emittedEvents[0].Type)
	}

	payload, ok := sess.emittedEvents[0].Payload.(map[string]any)
	if !ok {
		t.Fatal("expected payload to be map[string]any")
	}

	divergent, ok := payload["divergent"].(map[string]any)
	if !ok {
		t.Fatal("expected divergent payload")
	}
	ideas, ok := divergent["ideas"].([]agent.Message)
	if !ok {
		t.Fatal("expected divergent ideas to be []agent.Message")
	}
	if len(ideas) != 3 {
		t.Errorf("expected 3 divergent ideas, got %d", len(ideas))
	}

	interaction, ok := payload["interaction"].(map[string]any)
	if !ok {
		t.Fatal("expected interaction payload")
	}
	thread, ok := interaction["thread"].([]agent.Message)
	if !ok {
		t.Fatal("expected interaction thread to be []agent.Message")
	}
	if len(thread) == 0 {
		t.Errorf("expected interaction thread to be non-empty")
	}
}

func TestBrainstormingProtocol_OnMessage_WithConcurrency(t *testing.T) {
	p := Brainstorming(
		WithBrainstormingConcurrency(2),
		WithBrainstormingInteractionRounds(0),
		WithBrainstormingShortlistSize(0),
		WithBrainstormingVoting(false),
	)

	maxConcurrent := 0
	currentConcurrent := 0
	var mu sync.Mutex

	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
			agent.Agent{Name: "Agent2"},
			agent.Agent{Name: "Agent3"},
			agent.Agent{Name: "Agent4"},
		},
		emittedEvents: []event.Event{},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			mu.Lock()
			currentConcurrent++
			if currentConcurrent > maxConcurrent {
				maxConcurrent = currentConcurrent
			}
			mu.Unlock()

			<-ctx.Done()

			mu.Lock()
			currentConcurrent--
			mu.Unlock()

			return message.Assistant("idea"), nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for i := 0; i < 10; i++ {
			mu.Lock()
			current := currentConcurrent
			mu.Unlock()
			if current > 0 {
				break
			}
		}
		cancel()
	}()

	_ = p.OnMessage(ctx, sess, message.User("test"))

	mu.Lock()
	defer mu.Unlock()
	if maxConcurrent > 2 {
		t.Errorf("expected max 2 concurrent executions, got %d", maxConcurrent)
	}
}

func TestBrainstormingProtocol_OnMessage_AgentError(t *testing.T) {
	p := Brainstorming(
		WithBrainstormingInteractionRounds(0),
		WithBrainstormingShortlistSize(0),
		WithBrainstormingVoting(false),
	)

	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
			agent.Agent{Name: "Agent2"},
		},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if ag.Name == "Agent2" {
				return agent.Message{}, errors.New("agent2 failed")
			}
			return message.Assistant("idea"), nil
		},
	}

	msg := message.User("test")
	err := p.OnMessage(context.Background(), sess, msg)

	if err == nil {
		t.Error("expected error from failing agent, got nil")
	}
}

func TestBrainstormingProtocol_OnMessage_Voting(t *testing.T) {
	p := Brainstorming(
		WithBrainstormingInteractionRounds(0),
		WithBrainstormingShortlistSize(2),
		WithBrainstormingVotesPerAgent(1),
		WithBrainstormingVoteWeights(1),
	)

	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Agent1"},
			agent.Agent{Name: "Agent2"},
		},
		emittedEvents: []event.Event{},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if req.OutputSchema != nil {
				return message.Assistant(`{"picks":["[Agent1] Idea Alpha"],"rationale":"best"}`), nil
			}
			if ag.Name == "Agent1" {
				return message.Assistant("Idea Alpha"), nil
			}
			return message.Assistant("Idea Beta"), nil
		},
	}

	err := p.OnMessage(context.Background(), sess, message.User("test"))
	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if len(sess.emittedEvents) != 1 {
		t.Fatalf("expected 1 emitted event, got %d", len(sess.emittedEvents))
	}

	payload, ok := sess.emittedEvents[0].Payload.(map[string]any)
	if !ok {
		t.Fatal("expected payload to be map[string]any")
	}

	votesPayload, ok := payload["votes"].(map[string]any)
	if !ok {
		t.Fatal("expected votes payload")
	}

	tally, ok := votesPayload["tally"].([]VoteTally)
	if !ok {
		t.Fatal("expected tally to be []VoteTally")
	}
	if len(tally) != 1 {
		t.Fatalf("expected 1 tally entry, got %d", len(tally))
	}
	if tally[0].Idea != "[Agent1] Idea Alpha" {
		t.Errorf("unexpected top idea: %s", tally[0].Idea)
	}
	if tally[0].Score != 2 {
		t.Errorf("expected score 2, got %d", tally[0].Score)
	}
}

func TestBrainstormingProtocol_OnEvent(t *testing.T) {
	p := Brainstorming()
	sess := &mockSession{id: "test"}

	err := p.OnEvent(context.Background(), sess, event.Event{})
	if err != nil {
		t.Errorf("OnEvent() failed: %v", err)
	}
}

func TestBrainstormingProtocol_Shutdown(t *testing.T) {
	p := Brainstorming()
	sess := &mockSession{id: "test"}

	err := p.Shutdown(context.Background(), sess)
	if err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

func TestWithBrainstormingConcurrency(t *testing.T) {
	tests := []struct {
		name          string
		maxConcurrent int
		wantApplied   bool
	}{
		{
			name:          "positive value",
			maxConcurrent: 5,
			wantApplied:   true,
		},
		{
			name:          "zero value",
			maxConcurrent: 0,
			wantApplied:   false,
		},
		{
			name:          "negative value",
			maxConcurrent: -1,
			wantApplied:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Brainstorming(WithBrainstormingConcurrency(tt.maxConcurrent))
			bs := p.(*brainstorming)

			if tt.wantApplied {
				if bs.cfg.MaxConcurrent != tt.maxConcurrent {
					t.Errorf("expected MaxConcurrent %d, got %d", tt.maxConcurrent, bs.cfg.MaxConcurrent)
				}
			} else {
				if bs.cfg.MaxConcurrent != 0 {
					t.Errorf("expected MaxConcurrent to remain 0, got %d", bs.cfg.MaxConcurrent)
				}
			}
		})
	}
}
