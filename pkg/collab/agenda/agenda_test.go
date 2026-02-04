package agenda

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

type mockAgendaSession struct {
	facilitator *agent.Agent
	runRequests []protocol.RunRequest
}

func (m *mockAgendaSession) ID() string                                { return "test-session" }
func (m *mockAgendaSession) Name() string                              { return "test-session" }
func (m *mockAgendaSession) Tags() []string                            { return nil }
func (m *mockAgendaSession) Metadata() map[string]any                  { return nil }
func (m *mockAgendaSession) ProtocolID() string                        { return "protocol.consensus" }
func (m *mockAgendaSession) Participants() []protocol.Participant      { return nil }
func (m *mockAgendaSession) Facilitator() *agent.Agent                 { return m.facilitator }
func (m *mockAgendaSession) Groups() map[string][]protocol.Participant { return nil }
func (m *mockAgendaSession) DefaultTools() []string                    { return nil }
func (m *mockAgendaSession) RegisterTool(t any) error                  { return nil }
func (m *mockAgendaSession) RegisterTools(tools ...any) error          { return nil }
func (m *mockAgendaSession) AddDefaultTools(ids ...string)             {}

func (m *mockAgendaSession) Emit(ev event.Event) error {
	_ = ev
	return nil
}

func (m *mockAgendaSession) EmitWithContext(ctx context.Context, ev event.Event) error {
	_ = ctx
	return m.Emit(ev)
}

func (m *mockAgendaSession) RunAgent(ctx context.Context, ag agent.Agent, req protocol.RunRequest) (agent.Message, error) {
	_ = ctx
	_ = ag
	m.runRequests = append(m.runRequests, req)
	return message.Assistant("refined scope"), nil
}

func (m *mockAgendaSession) RunTurn(ctx context.Context, participant protocol.Participant, req protocol.RunRequest, _ ...protocol.TurnOption) (agent.Message, error) {
	ag, ok := participant.Agent()
	if !ok {
		return agent.Message{}, fmt.Errorf("human turn not supported")
	}
	return m.RunAgent(ctx, ag, req)
}

func (m *mockAgendaSession) AwaitInput(ctx context.Context, participant protocol.Participant, context string, resume protocol.TurnResume, _ ...protocol.InputOption) error {
	_ = ctx
	_ = participant
	_ = context
	_ = resume
	return fmt.Errorf("human input not supported")
}

func TestRefineScopePropagatesImageParts(t *testing.T) {
	a := New(WithRefinementPrompt(func(userQuestion, configuredScope string) (string, string) {
		return "system", "refine: " + userQuestion + " (" + configuredScope + ")"
	}))

	facilitator := agent.Agent{Name: "Facilitator"}
	sess := &mockAgendaSession{facilitator: &facilitator}

	msg := agent.Message{
		Role: agent.RoleUser,
		Parts: []agent.ContentPart{
			{Type: agent.ContentPartText, Text: "Assess this."},
			{Type: agent.ContentPartImage, URI: "https://example.com/image.png"},
		},
	}

	_, err := a.RefineScope(context.Background(), sess, msg)
	if err != nil {
		t.Fatalf("RefineScope() failed: %v", err)
	}

	if len(sess.runRequests) != 1 {
		t.Fatalf("expected 1 RunAgent call, got %d", len(sess.runRequests))
	}

	req := sess.runRequests[0]
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}

	if !hasImagePart(req.Messages[0].Parts) {
		t.Fatal("expected image part in scope refinement prompt")
	}
}

func hasImagePart(parts []agent.ContentPart) bool {
	for _, part := range parts {
		if part.Type == agent.ContentPartImage {
			return true
		}
	}
	return false
}

func TestAgendaContextIncludesOutcomeBriefAndItems(t *testing.T) {
	a := New(
		WithScope("Scope"),
		WithOutcome("Outcome"),
		WithBrief("Brief one"),
		WithBrief("Brief two"),
		WithItem(Item{
			Title:   "Item A",
			Outcome: "Decide A",
			Timebox: 5 * time.Minute,
			Notes:   "Focus on impact",
		}),
	)

	context := a.Context()
	if context == "" {
		t.Fatal("expected context to be populated")
	}
	if !strings.Contains(context, "Scope: Scope") {
		t.Fatalf("expected scope in context, got %q", context)
	}
	if !strings.Contains(context, "Outcome: Outcome") {
		t.Fatalf("expected outcome in context, got %q", context)
	}
	if !strings.Contains(context, "Brief:") {
		t.Fatalf("expected brief in context, got %q", context)
	}
	if !strings.Contains(context, "Item A") || !strings.Contains(context, "Decide A") {
		t.Fatalf("expected item details in context, got %q", context)
	}
}

func TestAgendaAdvanceAndResolvedScope(t *testing.T) {
	a := New(
		WithScope("Configured"),
		WithFallback(func(userQuestion, configuredScope string) string {
			return "Refined " + configuredScope
		}),
		WithItem(Item{Title: "Item 1"}),
		WithItem(Item{Title: "Item 2"}),
	)

	item, ok := a.CurrentItem()
	if !ok || item.Title != "Item 1" {
		t.Fatalf("expected current item 1, got %#v", item)
	}

	item, ok = a.Advance()
	if !ok || item.Title != "Item 2" {
		t.Fatalf("expected advance to item 2, got %#v", item)
	}

	sess := &mockAgendaSession{}
	_, err := a.RefineScope(context.Background(), sess, message.User("Question"))
	if err != nil {
		t.Fatalf("RefineScope failed: %v", err)
	}

	if a.ResolvedScope() != "Refined Configured" {
		t.Fatalf("expected resolved scope to use refined scope, got %q", a.ResolvedScope())
	}
}
