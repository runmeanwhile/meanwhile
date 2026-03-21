package engine

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/contextpolicy"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

type contextRecordingProvider struct {
	requests []provider.Request
}

func (p *contextRecordingProvider) ID() string { return "recording" }
func (p *contextRecordingProvider) Stream(_ context.Context, req provider.Request) (provider.Stream, error) {
	p.requests = append(p.requests, req)
	return &contextSingleMessageStream{message: runtimeTextMessage(modelruntime.RoleAssistant, "ok")}, nil
}

type contextEstimatorProvider struct {
	contextRecordingProvider
}

func (p *contextEstimatorProvider) EstimateMessageTokens(ctx context.Context, model string, msg agent.Message) (int, error) {
	if msg.Text() == "drop" {
		return 100, nil
	}
	return 1, nil
}

type contextSingleMessageStream struct {
	sent    bool
	message modelruntime.Message
}

func (s *contextSingleMessageStream) Recv() (provider.Event, error) {
	if s.sent {
		return provider.Event{}, io.EOF
	}
	s.sent = true
	return provider.Event{Type: provider.EventMessageCompleted, Message: s.message}, nil
}

func (s *contextSingleMessageStream) Close() error { return nil }

func TestRunAgentAppliesRollingWindow(t *testing.T) {
	prov := &contextRecordingProvider{}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	participant := agent.Agent{Name: "agent", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		SystemMessages: []agent.Message{{Role: agent.RoleSystem, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "sys"}}}},
		Messages: []agent.Message{
			{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "m1"}}},
			{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "m2"}}},
			{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "m3"}}},
		},
		Context: protocol.ContextConfig{
			RollingWindow: 2,
		},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if len(prov.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(prov.requests))
	}
	req := prov.requests[0]
	if len(req.Messages) != 3 {
		t.Fatalf("expected system + last 2 messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != modelruntime.RoleSystem || req.Messages[2].Text() != "m3" {
		t.Fatalf("unexpected message selection")
	}
}

func TestRunAgentAppliesAgentPerspectiveToNamedAssistantMessages(t *testing.T) {
	prov := &contextRecordingProvider{}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	participant := agent.Agent{Name: "Alice", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages: []agent.Message{
			{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Start"}}},
			{Role: agent.RoleAssistant, Name: "Bob", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Peer idea"}}},
			{Role: agent.RoleAssistant, Name: "Alice", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "My prior idea"}}},
		},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if len(prov.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(prov.requests))
	}
	req := prov.requests[0]
	if len(req.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(req.Messages))
	}
	if req.Messages[1].Role != modelruntime.RoleUser {
		t.Fatalf("expected peer assistant message to be rewritten as user, got %s", req.Messages[1].Role)
	}
	if req.Messages[1].Name != "Bob" {
		t.Fatalf("expected peer speaker name to be preserved, got %q", req.Messages[1].Name)
	}
	if req.Messages[2].Role != modelruntime.RoleAssistant {
		t.Fatalf("expected current agent prior message to remain assistant, got %s", req.Messages[2].Role)
	}
}

func TestRunAgentPreservesNamedAssistantMessagesInLegacyPerspectiveMode(t *testing.T) {
	prov := &contextRecordingProvider{}
	eng, err := New(WithProvider(prov), WithAgentPerspectiveMode(AgentPerspectiveLegacy))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	participant := agent.Agent{Name: "Alice", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages: []agent.Message{
			{Role: agent.RoleAssistant, Name: "Bob", Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "Peer idea"}}},
		},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if len(prov.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(prov.requests))
	}
	req := prov.requests[0]
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != modelruntime.RoleAssistant {
		t.Fatalf("expected legacy mode to preserve assistant role, got %s", req.Messages[0].Role)
	}
}

func TestRunAgentTruncatesToolOutput(t *testing.T) {
	prov := &contextRecordingProvider{}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	participant := agent.Agent{Name: "agent", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages: []agent.Message{
			{Role: agent.RoleTool, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "0123456789"}}},
		},
		Context: protocol.ContextConfig{
			MaxToolOutputChars: 4,
		},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	req := prov.requests[0]
	if req.Messages[0].Text() == "0123456789" {
		t.Fatalf("expected truncated tool output")
	}
}

func TestRunAgentSummarizesContext(t *testing.T) {
	prov := &contextRecordingProvider{}
	eng, err := New(WithProvider(prov), WithContextSummarizer(contextpolicy.FuncSummarizer(func(ctx context.Context, messages []agent.Message) (string, error) {
		return "summary", nil
	})))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	participant := agent.Agent{Name: "agent", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages: []agent.Message{
			{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("m1", 12)}}},
			{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("m2", 12)}}},
			{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.Repeat("m3", 12)}}},
		},
		Context: protocol.ContextConfig{
			RollingWindow: 1,
			Summarization: protocol.SummarizationConfig{
				Enabled:         true,
				ThresholdTokens: 1,
			},
		},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	req := prov.requests[0]
	if len(req.Messages) < 2 {
		t.Fatalf("expected summary + recent message")
	}
	if req.Messages[0].Metadata[contextpolicy.SummaryMetadataKey] != true {
		t.Fatalf("expected summary metadata")
	}
	if req.Messages[0].Text() != "summary" {
		t.Fatalf("unexpected summary content")
	}
}

func TestRunAgentUsesProviderTokenEstimator(t *testing.T) {
	prov := &contextEstimatorProvider{}
	eng, err := New(WithProvider(prov))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	participant := agent.Agent{Name: "agent", Model: "test"}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol:     noopProtocol{},
		Participants: []protocol.Participant{participant},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), participant, protocol.RunRequest{
		Messages: []agent.Message{
			{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "drop"}}},
			{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "keep"}}},
		},
		Context: protocol.ContextConfig{
			MaxPromptTokens: 2,
		},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	req := prov.requests[0]
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message after token budgeting, got %d", len(req.Messages))
	}
	if req.Messages[0].Text() != "keep" {
		t.Fatalf("expected provider estimator to influence selection")
	}
}
