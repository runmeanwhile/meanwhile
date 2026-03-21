package engine

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/contextpolicy"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

const durableProtocolID = "protocol.durable_test"

type durableProtocol struct {
	history []agent.Message
}

func (p *durableProtocol) ID() string { return durableProtocolID }
func (p *durableProtocol) Participants() []protocol.Participant {
	return nil
}
func (p *durableProtocol) Init(ctx context.Context, sess protocol.Session) error {
	_ = ctx
	_ = sess
	return nil
}
func (p *durableProtocol) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	participants := sess.Participants()
	if len(participants) == 0 {
		return errors.New("durable protocol requires a participant")
	}
	agentParticipant, ok := participants[0].Agent()
	if !ok {
		return errors.New("durable protocol requires an agent participant")
	}
	p.history = append(p.history, msg)
	resp, err := sess.RunAgent(ctx, agentParticipant, protocol.RunRequest{
		Messages: append([]agent.Message(nil), p.history...),
	})
	if err != nil {
		return err
	}
	resp.Name = participants[0].DisplayName()
	p.history = append(p.history, resp)

	payload := map[string]any{"message": resp}
	return sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload))
}
func (p *durableProtocol) OnEvent(ctx context.Context, sess protocol.Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}
func (p *durableProtocol) Shutdown(ctx context.Context, sess protocol.Session) error {
	_ = ctx
	_ = sess
	return nil
}

func (p *durableProtocol) GetState() (map[string]any, error) {
	state := durableState{History: append([]agent.Message(nil), p.history...)}
	return protocol.EncodeState(state)
}

func (p *durableProtocol) SetState(state map[string]any) error {
	var snapshot durableState
	if err := protocol.DecodeState(state, &snapshot); err != nil {
		return err
	}
	p.history = append([]agent.Message(nil), snapshot.History...)
	return nil
}

type durableState struct {
	History []agent.Message `json:"history"`
}

type blipProvider struct {
	mu          sync.Mutex
	requests    []provider.Request
	streamCalls int
	failStreams int
}

func (p *blipProvider) ID() string { return "blip" }

func (p *blipProvider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	if req.Model == "" {
		return nil, errors.New("model required")
	}
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.streamCalls++
	call := p.streamCalls
	p.mu.Unlock()

	fail := call <= p.failStreams
	events := []provider.Event{
		{Type: provider.EventMessageDelta, Delta: "ok"},
		{Type: provider.EventMessageCompleted, Message: runtimeFromAgentMessage(message.Assistant("ok"))},
	}
	return &blipStream{events: events, fail: fail, failAfter: 1}, nil
}

func (p *blipProvider) LastRequest() provider.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.requests) == 0 {
		return provider.Request{}
	}
	return p.requests[len(p.requests)-1]
}

type blipStream struct {
	events    []provider.Event
	fail      bool
	failAfter int
	idx       int
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func (s *blipStream) Recv() (provider.Event, error) {
	if s.fail && s.idx == s.failAfter {
		s.idx++
		return provider.Event{}, timeoutErr{}
	}
	if s.idx >= len(s.events) {
		return provider.Event{}, io.EOF
	}
	ev := s.events[s.idx]
	s.idx++
	return ev, nil
}

func (s *blipStream) Close() error { return nil }

func TestDurableExecutionIntegration(t *testing.T) {
	store := NewInMemorySessionStore()
	prov := &blipProvider{failStreams: 2}

	cfg := config.Config{
		Global: config.GlobalConfig{
			RunTimeoutSeconds: 5,
			ProviderRetry: config.ProviderRetryConfig{
				MaxRetries:      3,
				InitialInterval: time.Millisecond,
				MaxInterval:     time.Millisecond,
				Multiplier:      1,
			},
			Context: config.ContextConfig{
				AutoSummarize: config.AutoSummarizeConfig{
					SummarizeAtTokens: 1,
					MinKeepMessages:   1,
				},
			},
		},
	}

	summarizer := contextpolicy.FuncSummarizer(func(ctx context.Context, messages []agent.Message) (string, error) {
		_ = ctx
		if len(messages) == 0 {
			return "", errors.New("no messages to summarize")
		}
		return "summary", nil
	})

	eng, err := New(
		WithProvider(prov),
		WithSessionStore(store),
		WithConfig(cfg),
		WithContextSummarizer(summarizer),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	registerDurableProtocolFactory(eng)

	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol: &durableProtocol{},
		Participants: []protocol.Participant{agent.Agent{
			Name:  "agent",
			Model: "test-model",
		}},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	msg1 := message.User(strings.Repeat("first message ", 8))
	if _, err := eng.Run(context.Background(), sess.ID(), msg1); err != nil {
		t.Fatalf("run 1: %v", err)
	}

	eng2, err := New(
		WithProvider(prov),
		WithSessionStore(store),
		WithConfig(cfg),
		WithContextSummarizer(summarizer),
	)
	if err != nil {
		t.Fatalf("new engine (rehydrate): %v", err)
	}
	registerDurableProtocolFactory(eng2)

	rehydrated, err := eng2.session(context.Background(), sess.ID())
	if err != nil {
		t.Fatalf("rehydrate session: %v", err)
	}
	durable, ok := rehydrated.protocol.(*durableProtocol)
	if !ok {
		t.Fatalf("expected durable protocol, got %T", rehydrated.protocol)
	}
	if len(durable.history) < 2 {
		t.Fatalf("expected history restored, got %d messages", len(durable.history))
	}

	msg2 := message.User(strings.Repeat("second message ", 8))
	if _, err := eng2.Run(context.Background(), rehydrated.ID(), msg2); err != nil {
		t.Fatalf("run 2: %v", err)
	}

	lastReq := prov.LastRequest()
	if !hasSummaryMessage(lastReq.Messages) {
		t.Fatalf("expected auto-summary message in provider request")
	}
}

func registerDurableProtocolFactory(eng *Engine) {
	if eng.protocols == nil {
		eng.protocols = protocol.NewRegistry()
	}
	eng.protocols.Register(durableProtocolID, func(cfg protocol.Config) protocol.Protocol {
		_ = cfg
		return &durableProtocol{}
	})
}

func hasSummaryMessage(messages []modelruntime.Message) bool {
	for _, msg := range messages {
		if msg.Metadata == nil {
			continue
		}
		if v, ok := msg.Metadata[contextpolicy.SummaryMetadataKey].(bool); ok && v {
			return true
		}
	}
	return false
}
