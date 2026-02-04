package protocol

import (
	"context"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

func TestAdversarialProtocol_ID(t *testing.T) {
	p := Adversarial()
	if p.ID() != "protocol.adversarial_debate" {
		t.Errorf("expected ID 'protocol.adversarial_debate', got '%s'", p.ID())
	}
}

func TestDebateAlias(t *testing.T) {
	p := Debate()
	if p.ID() != "protocol.adversarial_debate" {
		t.Errorf("Debate() should create adversarial protocol, got ID '%s'", p.ID())
	}
}

func TestAdversarialProtocol_Participants(t *testing.T) {
	p := Adversarial()
	participants := p.Participants()
	if participants != nil {
		t.Errorf("expected nil participants, got %v", participants)
	}
}

func TestAdversarialProtocol_Init(t *testing.T) {
	p := Adversarial()
	sess := &mockSession{id: "test"}

	err := p.Init(context.Background(), sess)
	if err != nil {
		t.Errorf("Init() failed: %v", err)
	}
}

func TestAdversarialProtocol_OnMessage_Success(t *testing.T) {
	p := Adversarial()

	var capturedMessages []agent.Message
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Pro"},
			agent.Agent{Name: "Con"},
		},
		emittedEvents: []event.Event{},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if len(req.Messages) > 0 {
				capturedMessages = append(capturedMessages, req.Messages[0])
			}
			return agent.Message{Role: "assistant", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: ag.Name + " argument"}}}, nil
		},
	}

	msg := agent.Message{Role: "user", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "AI is beneficial"}}}
	err := p.OnMessage(context.Background(), sess, msg)

	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if len(capturedMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(capturedMessages))
	}

	// Check that pro message has the pro prefix
	if capturedMessages[0].Text() != "Argue in favor: AI is beneficial" {
		t.Errorf("unexpected pro message: %s", capturedMessages[0].Text())
	}

	// Check that con message has the con prefix
	if capturedMessages[1].Text() != "Argue against: AI is beneficial" {
		t.Errorf("unexpected con message: %s", capturedMessages[1].Text())
	}

	if len(sess.emittedEvents) < 1 {
		t.Errorf("expected at least 1 emitted event, got %d", len(sess.emittedEvents))
	}
}

func TestAdversarialProtocol_PropagatesImageParts(t *testing.T) {
	p := Adversarial()

	var capturedMessages []agent.Message
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Pro"},
			agent.Agent{Name: "Con"},
		},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if len(req.Messages) > 0 {
				capturedMessages = append(capturedMessages, req.Messages[0])
			}
			return agent.Message{Role: "assistant", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "response"}}}, nil
		},
	}

	msg := agent.Message{
		Role: agent.RoleUser,
		Parts: []agent.ContentPart{
			{Type: agent.ContentPartText, Text: "AI is beneficial"},
			{Type: agent.ContentPartImage, URI: "https://example.com/image.png"},
		},
	}
	err := p.OnMessage(context.Background(), sess, msg)
	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if len(capturedMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(capturedMessages))
	}

	for i, captured := range capturedMessages {
		if !hasImagePart(captured.Parts) {
			t.Fatalf("expected image part in message %d", i)
		}
	}
}

func TestAdversarialProtocol_SynthesisPropagatesImageParts(t *testing.T) {
	p := Adversarial()

	var capturedMessages []agent.Message
	facilitator := agent.Agent{Name: "Facilitator"}
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Pro"},
			agent.Agent{Name: "Con"},
		},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if len(req.Messages) > 0 {
				capturedMessages = append(capturedMessages, req.Messages[0])
			}
			return agent.Message{Role: "assistant", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "response"}}}, nil
		},
		facilitator: &facilitator,
	}

	msg := agent.Message{
		Role: agent.RoleUser,
		Parts: []agent.ContentPart{
			{Type: agent.ContentPartText, Text: "test topic"},
			{Type: agent.ContentPartImage, URI: "https://example.com/image.png"},
		},
	}
	err := p.OnMessage(context.Background(), sess, msg)
	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if len(capturedMessages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(capturedMessages))
	}

	if !hasImagePart(capturedMessages[2].Parts) {
		t.Fatal("expected synthesis prompt to include image part")
	}
}

func TestAdversarialProtocol_OnMessage_NotEnoughParticipants(t *testing.T) {
	p := Adversarial()

	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "OnlyOne"},
		},
	}

	msg := agent.Message{Role: "user", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "test"}}}
	err := p.OnMessage(context.Background(), sess, msg)

	if err == nil {
		t.Error("expected error for insufficient participants, got nil")
	}

	if err != errNotEnoughParticipants {
		t.Errorf("expected errNotEnoughParticipants, got %v", err)
	}
}

func TestAdversarialProtocol_OnMessage_WithFacilitator(t *testing.T) {
	p := Adversarial()

	facilitator := agent.Agent{Name: "Facilitator"}
	callCount := 0

	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Pro"},
			agent.Agent{Name: "Con"},
		},
		emittedEvents: []event.Event{},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			callCount++
			return agent.Message{Role: "assistant", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "response"}}}, nil
		},
	}
	sess.facilitator = &facilitator

	msg := agent.Message{Role: "user", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "test"}}}
	err := p.OnMessage(context.Background(), sess, msg)

	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	// Should call: Pro, Con, and Facilitator
	if callCount != 3 {
		t.Errorf("expected 3 agent calls (pro, con, facilitator), got %d", callCount)
	}
}

func TestAdversarialProtocol_WithCustomPrefixes(t *testing.T) {
	p := Adversarial(WithDebatePrefixes("Support: ", "Oppose: "))

	var capturedMessages []agent.Message
	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Pro"},
			agent.Agent{Name: "Con"},
		},
		emittedEvents: []event.Event{},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if len(req.Messages) > 0 {
				capturedMessages = append(capturedMessages, req.Messages[0])
			}
			return agent.Message{Role: "assistant", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "response"}}}, nil
		},
	}

	msg := agent.Message{Role: "user", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "topic"}}}
	err := p.OnMessage(context.Background(), sess, msg)

	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if capturedMessages[0].Text() != "Support: topic" {
		t.Errorf("unexpected pro prefix: %s", capturedMessages[0].Text())
	}

	if capturedMessages[1].Text() != "Oppose: topic" {
		t.Errorf("unexpected con prefix: %s", capturedMessages[1].Text())
	}
}

func TestAdversarialProtocol_WithCustomSynthesis(t *testing.T) {
	customSynthesis := func(topic, pro, con string) string {
		return "CUSTOM: " + topic
	}

	p := Adversarial(WithDebateSynthesis(customSynthesis))

	var synthesisPrompt string
	facilitator := agent.Agent{Name: "Facilitator"}

	sess := &mockSession{
		id: "test",
		participants: []Participant{
			agent.Agent{Name: "Pro"},
			agent.Agent{Name: "Con"},
		},
		emittedEvents: []event.Event{},
		runAgentFunc: func(ctx context.Context, ag agent.Agent, req RunRequest) (agent.Message, error) {
			if ag.Name == "Facilitator" && len(req.Messages) > 0 {
				synthesisPrompt = req.Messages[0].Text()
			}
			return agent.Message{Role: "assistant", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "response"}}}, nil
		},
	}
	sess.facilitator = &facilitator

	msg := agent.Message{Role: "user", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "test topic"}}}
	err := p.OnMessage(context.Background(), sess, msg)

	if err != nil {
		t.Fatalf("OnMessage() failed: %v", err)
	}

	if synthesisPrompt != "CUSTOM: test topic" {
		t.Errorf("expected custom synthesis prompt, got: %s", synthesisPrompt)
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

func TestAdversarialProtocol_OnEvent(t *testing.T) {
	p := Adversarial()
	sess := &mockSession{id: "test"}

	err := p.OnEvent(context.Background(), sess, event.Event{})
	if err != nil {
		t.Errorf("OnEvent() failed: %v", err)
	}
}

func TestAdversarialProtocol_Shutdown(t *testing.T) {
	p := Adversarial()
	sess := &mockSession{id: "test"}

	err := p.Shutdown(context.Background(), sess)
	if err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}
}

func TestWithDebatePrefixes_EmptyPrefixes(t *testing.T) {
	p := Adversarial(WithDebatePrefixes("", ""))
	adv := p.(*adversarial)

	// Empty prefixes should not override defaults
	if adv.cfg.ProPrefix == "" || adv.cfg.ConPrefix == "" {
		t.Error("empty prefixes should not override defaults")
	}
}

func TestWithDebateSynthesis_Nil(t *testing.T) {
	p := Adversarial(WithDebateSynthesis(nil))
	adv := p.(*adversarial)

	// Nil synthesis function should not override default
	if adv.cfg.SynthesisFn == nil {
		t.Error("nil synthesis function should not override default")
	}
}
