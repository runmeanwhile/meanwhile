package protocol

import (
	"context"
	"errors"
	"reflect"
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
	expectedCalls := 12 // 3 participants * (exploration + ideation + presentation + interaction)
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

	exploration, ok := payload["exploration"].(map[string]any)
	if !ok {
		t.Fatal("expected exploration payload")
	}
	thread, ok := exploration["thread"].([]agent.Message)
	if !ok {
		t.Fatal("expected exploration thread to be []agent.Message")
	}
	if len(thread) == 0 {
		t.Errorf("expected exploration thread to be non-empty")
	}

	ideation, ok := payload["ideation"].(map[string]any)
	if !ok {
		t.Fatal("expected ideation payload")
	}
	results, ok := ideation["results"].([]ideationResult)
	if !ok {
		t.Fatal("expected ideation results to be []ideationResult")
	}
	if len(results) != 3 {
		t.Errorf("expected 3 ideation results, got %d", len(results))
	}
	if board, ok := ideation["idea_board"].(string); !ok || board == "" {
		t.Errorf("expected ideation idea_board to be non-empty")
	}

	presentation, ok := payload["presentation"].(map[string]any)
	if !ok {
		t.Fatal("expected presentation payload")
	}
	presentationThread, ok := presentation["thread"].([]agent.Message)
	if !ok {
		t.Fatal("expected presentation thread to be []agent.Message")
	}
	if len(presentationThread) == 0 {
		t.Errorf("expected presentation thread to be non-empty")
	}

	discussion, ok := payload["discussion"].(map[string]any)
	if !ok {
		t.Fatal("expected discussion payload")
	}
	discussionThread, ok := discussion["thread"].([]agent.Message)
	if !ok {
		t.Fatal("expected discussion thread to be []agent.Message")
	}
	if len(discussionThread) == 0 {
		t.Errorf("expected discussion thread to be non-empty")
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

func TestBrainstormingProtocol_RespectsInteractionRounds(t *testing.T) {
	interactionRounds := 5
	p := Brainstorming(
		WithBrainstormingInteractionRounds(interactionRounds),
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

	err := p.OnMessage(context.Background(), sess, message.User("generate ideas"))
	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	mu.Lock()
	expectedCalls := 3 * (1 + 1 + 1 + interactionRounds) // exploration + ideation + presentation + interaction
	if callCount != expectedCalls {
		t.Errorf("expected %d agent calls, got %d", expectedCalls, callCount)
	}
	mu.Unlock()
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
	if tally[0].Idea != "Idea Alpha" {
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

func TestRotateAgentOrder(t *testing.T) {
	participants := []agent.Agent{
		{Name: "Marketing"},
		{Name: "Engineering"},
		{Name: "Design"},
	}

	round1 := rotateAgentOrder(participants, 1)
	if got := []string{round1[0].Name, round1[1].Name, round1[2].Name}; !reflect.DeepEqual(got, []string{"Marketing", "Engineering", "Design"}) {
		t.Fatalf("round 1 order mismatch: %v", got)
	}

	round2 := rotateAgentOrder(participants, 2)
	if got := []string{round2[0].Name, round2[1].Name, round2[2].Name}; !reflect.DeepEqual(got, []string{"Engineering", "Design", "Marketing"}) {
		t.Fatalf("round 2 order mismatch: %v", got)
	}

	round3 := rotateAgentOrder(participants, 3)
	if got := []string{round3[0].Name, round3[1].Name, round3[2].Name}; !reflect.DeepEqual(got, []string{"Design", "Marketing", "Engineering"}) {
		t.Fatalf("round 3 order mismatch: %v", got)
	}
}

func TestRotateParticipantOrder(t *testing.T) {
	participants := []Participant{
		agent.Agent{Name: "Marketing"},
		agent.Agent{Name: "Engineering"},
		agent.Agent{Name: "Design"},
	}

	round2 := rotateParticipantOrder(participants, 2)
	got := []string{
		round2[0].DisplayName(),
		round2[1].DisplayName(),
		round2[2].DisplayName(),
	}
	want := []string{"Engineering", "Design", "Marketing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round 2 participant order mismatch: got %v want %v", got, want)
	}
}

func TestInteractionTurnMoveCycles(t *testing.T) {
	cases := []struct {
		round     int
		turnIndex int
		want      string
	}{
		{round: 1, turnIndex: 1, want: "build on one specific point from another speaker"},
		{round: 1, turnIndex: 2, want: "pressure-test one assumption"},
		{round: 1, turnIndex: 3, want: "add one concrete workflow example"},
		{round: 1, turnIndex: 4, want: "name one tradeoff or delivery risk"},
		{round: 2, turnIndex: 1, want: "pressure-test one assumption"},
	}

	for _, tc := range cases {
		if got := interactionTurnMove(tc.round, tc.turnIndex); got != tc.want {
			t.Fatalf("interactionTurnMove(%d,%d) = %q, want %q", tc.round, tc.turnIndex, got, tc.want)
		}
	}
}

func TestCleanListItem_PreservesNumericTitles(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "- 3-minute delta reel of KPI movers and drivers", want: "3-minute delta reel of KPI movers and drivers"},
		{in: "3. 3-minute delta reel of KPI movers and drivers", want: "3-minute delta reel of KPI movers and drivers"},
		{in: "- 1. Exception-only agenda", want: "Exception-only agenda"},
		{in: "- 2024. roadmap refresh", want: "2024. roadmap refresh"},
		{in: "2) Stale-action spotlight digest", want: "Stale-action spotlight digest"},
	}

	for _, tt := range tests {
		if got := cleanListItem(tt.in); got != tt.want {
			t.Fatalf("cleanListItem(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractShortlist_PreservesLeadingNumericTitle(t *testing.T) {
	text := `1. Exception-only agenda with capped anomalies + watchlist
2. Stale-action spotlight digest (pre-meeting Slack/email)
3. 3-minute delta reel of KPI movers and drivers`

	got := extractShortlist(text, 3)
	want := []string{
		"Exception-only agenda with capped anomalies + watchlist",
		"Stale-action spotlight digest (pre-meeting Slack/email)",
		"3-minute delta reel of KPI movers and drivers",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractShortlist() = %v, want %v", got, want)
	}
}
