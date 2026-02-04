package planning

import (
	"context"
	"fmt"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

func TestNew(t *testing.T) {
	ag := agent.Agent{Name: "Planner"}

	planner := New(ag)

	if planner.agent.Name != "Planner" {
		t.Errorf("expected agent name 'Planner', got %q", planner.agent.Name)
	}
	if planner.storage.Strategy != StorageReturn {
		t.Errorf("expected default storage strategy 'return', got %q", planner.storage.Strategy)
	}
	if planner.State() != StateIdle {
		t.Errorf("expected initial state 'idle', got %q", planner.State())
	}
}

func TestNew_WithOptions(t *testing.T) {
	ag := agent.Agent{Name: "Planner"}

	planner := New(ag,
		WithSystemPrompt("Custom prompt"),
		WithStorage(StorageSession, "my_plan"),
	)

	if planner.systemPrompt != "Custom prompt" {
		t.Errorf("expected custom prompt, got %q", planner.systemPrompt)
	}
	if planner.storage.Strategy != StorageSession {
		t.Errorf("expected session storage, got %q", planner.storage.Strategy)
	}
	if planner.storage.Key != "my_plan" {
		t.Errorf("expected key 'my_plan', got %q", planner.storage.Key)
	}
}

func TestPlanner_CreatePlan(t *testing.T) {
	ag := agent.Agent{Name: "Planner", Model: "test"}
	planner := New(ag)

	sess := &mockSession{
		id:       "test-session",
		metadata: make(map[string]any),
		runAgent: func(ctx context.Context, a agent.Agent, req protocol.RunRequest) (agent.Message, error) {
			// Return a plan in JSON format
			planText := `{
	"title": "Test Plan",
	"summary": "A test implementation plan",
	"steps": [
		{
			"title": "Step 1",
			"description": "First step"
		},
		{
			"title": "Step 2",
			"description": "Second step"
		}
	]
}`
			return agent.Message{
				Role: agent.RoleAssistant,
				Parts: []agent.ContentPart{
					{Type: "text", Text: planText},
				},
			}, nil
		},
	}

	plan, err := planner.CreatePlan(context.Background(), sess, agent.Message{
		Role:  agent.RoleUser,
		Parts: []agent.ContentPart{{Text: "Create a test plan"}},
	})

	if err != nil {
		t.Fatalf("CreatePlan() error: %v", err)
	}

	if plan == nil {
		t.Fatal("expected plan, got nil")
	}

	if plan.Title != "Test Plan" {
		t.Errorf("expected title 'Test Plan', got %q", plan.Title)
	}

	if len(plan.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(plan.Steps))
	}

	if plan.Steps[0].Title != "Step 1" {
		t.Errorf("expected step 1 title 'Step 1', got %q", plan.Steps[0].Title)
	}

	if planner.State() != StateComplete {
		t.Errorf("expected state 'complete', got %q", planner.State())
	}

	// Check events
	if len(sess.events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(sess.events))
	}

	var foundStart, foundCreated bool
	for _, ev := range sess.events {
		if ev.Type == "planning.started" {
			foundStart = true
		}
		if ev.Type == "planning.plan_created" {
			foundCreated = true
		}
	}

	if !foundStart {
		t.Error("expected 'planning.started' event")
	}
	if !foundCreated {
		t.Error("expected 'planning.plan_created' event")
	}
}

func TestPlanner_CreatePlan_SessionStorage(t *testing.T) {
	ag := agent.Agent{Name: "Planner"}
	planner := New(ag, WithStorage(StorageSession, "my_plan"))

	sess := &mockSession{
		id:       "test-session",
		metadata: make(map[string]any),
		runAgent: func(ctx context.Context, a agent.Agent, req protocol.RunRequest) (agent.Message, error) {
			return agent.Message{
				Role: agent.RoleAssistant,
				Parts: []agent.ContentPart{
					{Type: "text", Text: `{"title": "Plan", "summary": "Summary", "steps": []}`},
				},
			}, nil
		},
	}

	plan, err := planner.CreatePlan(context.Background(), sess, agent.Message{
		Role:  agent.RoleUser,
		Parts: []agent.ContentPart{{Text: "Test"}},
	})

	if err != nil {
		t.Fatalf("CreatePlan() error: %v", err)
	}

	// Check that plan is stored in session metadata
	stored, ok := sess.Metadata()["my_plan"].(*Plan)
	if !ok {
		t.Fatal("expected plan in session metadata")
	}

	if stored.ID != plan.ID {
		t.Errorf("expected stored plan ID %q, got %q", plan.ID, stored.ID)
	}
}

func TestGetPlan(t *testing.T) {
	sess := &mockSession{
		metadata: map[string]any{
			"plan": &Plan{
				ID:    "test-id",
				Title: "Test",
			},
		},
	}

	plan, ok := GetPlan(sess, "plan")
	if !ok {
		t.Fatal("expected plan to be found")
	}

	if plan.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %q", plan.ID)
	}

	// Test default key
	sess.metadata["plan"] = &Plan{ID: "default"}
	plan, ok = GetPlan(sess, "")
	if !ok {
		t.Fatal("expected plan with default key")
	}
	if plan.ID != "default" {
		t.Errorf("expected default plan ID, got %q", plan.ID)
	}
}

// mockSession implements protocol.Session for testing
type mockSession struct {
	id       string
	metadata map[string]any
	events   []event.Event
	runAgent func(context.Context, agent.Agent, protocol.RunRequest) (agent.Message, error)
}

func (m *mockSession) ID() string                                { return m.id }
func (m *mockSession) Name() string                              { return "test-session" }
func (m *mockSession) Tags() []string                            { return nil }
func (m *mockSession) Metadata() map[string]any                  { return m.metadata }
func (m *mockSession) ProtocolID() string                        { return "test.protocol" }
func (m *mockSession) Participants() []protocol.Participant      { return nil }
func (m *mockSession) Facilitator() *agent.Agent                 { return nil }
func (m *mockSession) Groups() map[string][]protocol.Participant { return nil }
func (m *mockSession) RegisterTool(t any) error                  { return nil }
func (m *mockSession) RegisterTools(tools ...any) error          { return nil }
func (m *mockSession) AddDefaultTools(ids ...string)             {}
func (m *mockSession) DefaultTools() []string                    { return nil }

func (m *mockSession) Emit(ev event.Event) error {
	m.events = append(m.events, ev)
	return nil
}

func (m *mockSession) EmitWithContext(ctx context.Context, ev event.Event) error {
	return m.Emit(ev)
}

func (m *mockSession) RunAgent(ctx context.Context, ag agent.Agent, req protocol.RunRequest) (agent.Message, error) {
	if m.runAgent != nil {
		return m.runAgent(ctx, ag, req)
	}
	return agent.Message{}, nil
}

func (m *mockSession) RunTurn(ctx context.Context, participant protocol.Participant, req protocol.RunRequest, _ ...protocol.TurnOption) (agent.Message, error) {
	ag, ok := participant.Agent()
	if !ok {
		return agent.Message{}, fmt.Errorf("human turn not supported")
	}
	return m.RunAgent(ctx, ag, req)
}

func (m *mockSession) AwaitInput(ctx context.Context, participant protocol.Participant, context string, resume protocol.TurnResume, _ ...protocol.InputOption) error {
	_ = ctx
	_ = participant
	_ = context
	_ = resume
	return fmt.Errorf("human input not supported")
}
