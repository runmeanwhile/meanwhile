package engine

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/memory"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/provider"
)

type recordingProvider struct {
	mu       sync.Mutex
	requests []provider.Request
}

func (p *recordingProvider) ID() string { return "mock" }

func (p *recordingProvider) Stream(_ context.Context, req provider.Request) (provider.Stream, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()

	return &qualityStream{events: []provider.Event{{
		Type:    provider.EventMessageCompleted,
		Message: message.Assistant("ok"),
	}}}, nil
}

func (p *recordingProvider) lastRequest() provider.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.requests) == 0 {
		return provider.Request{}
	}
	return p.requests[len(p.requests)-1]
}

type qualityStream struct {
	events []provider.Event
}

func (s *qualityStream) Recv() (provider.Event, error) {
	if len(s.events) == 0 {
		return provider.Event{}, io.EOF
	}
	ev := s.events[0]
	s.events = s.events[1:]
	return ev, nil
}

func (s *qualityStream) Close() error { return nil }

// --- Protocols for testing ---

type onEventProtocol struct {
	called chan event.Event
}

func (p *onEventProtocol) ID() string                                            { return "protocol.onevent" }
func (p *onEventProtocol) Participants() []protocol.Participant                  { return nil }
func (p *onEventProtocol) Init(ctx context.Context, sess protocol.Session) error { return nil }
func (p *onEventProtocol) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	return sess.Emit(event.New(event.AgentStarted, sess.ID(), map[string]any{"msg": msg.Text()}))
}
func (p *onEventProtocol) OnEvent(ctx context.Context, sess protocol.Session, ev event.Event) error {
	select {
	case p.called <- ev:
	default:
	}
	return nil
}
func (p *onEventProtocol) Shutdown(ctx context.Context, sess protocol.Session) error { return nil }

type directMessageProtocol struct{}

func (p *directMessageProtocol) ID() string                                            { return "protocol.directmessage" }
func (p *directMessageProtocol) Participants() []protocol.Participant                  { return nil }
func (p *directMessageProtocol) Init(ctx context.Context, sess protocol.Session) error { return nil }
func (p *directMessageProtocol) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	payload := message.Assistant("hello from protocol")
	return sess.Emit(event.New(event.AgentMessageComplete, sess.ID(), payload))
}
func (p *directMessageProtocol) OnEvent(ctx context.Context, sess protocol.Session, ev event.Event) error {
	return nil
}
func (p *directMessageProtocol) Shutdown(ctx context.Context, sess protocol.Session) error {
	return nil
}

func TestProtocolOnEventIsInvoked(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	proto := &onEventProtocol{called: make(chan event.Event, 1)}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Name:     "on-event",
		Protocol: proto,
	})
	if err != nil {
		t.Fatalf("NewSession error: %v", err)
	}

	if err := sess.Emit(event.New(event.AgentStarted, sess.ID(), nil)); err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	select {
	case <-proto.called:
	case <-time.After(1 * time.Second):
		t.Fatal("expected OnEvent to be called")
	}
}

func TestRunResultCapturesDirectMessagePayload(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	proto := &directMessageProtocol{}
	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Name:     "direct-message",
		Protocol: proto,
	})
	if err != nil {
		t.Fatalf("NewSession error: %v", err)
	}

	res, err := eng.Run(context.Background(), sess.ID(), message.User("hi"))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if res.Final == "" {
		t.Fatalf("expected Final to be populated")
	}
	if len(res.Transcript) != 1 {
		t.Fatalf("expected 1 transcript message, got %d", len(res.Transcript))
	}
	if res.Transcript[0].Text() != "hello from protocol" {
		t.Fatalf("unexpected transcript content: %q", res.Transcript[0].Text())
	}
}

func TestRunAgentClosesSession(t *testing.T) {
	prov := &recordingProvider{}
	eng, err := New(WithProvider(prov), WithDefaultProvider("mock"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	agent := agent.Agent{Name: "tester", Model: "mock:model"}
	_, err = eng.RunAgent(agent, message.User("hello"))
	if err != nil {
		t.Fatalf("RunAgent error: %v", err)
	}

	if len(eng.sessions) != 0 {
		t.Fatalf("expected sessions to be closed, found %d", len(eng.sessions))
	}
}

func TestResolveProviderStripsPrefix(t *testing.T) {
	prov := &recordingProvider{}
	eng, err := New(WithProvider(prov), WithDefaultProvider("mock"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	agent := agent.Agent{Name: "tester", Model: "mock:unit"}
	_, err = eng.RunAgent(agent, message.User("hello"))
	if err != nil {
		t.Fatalf("RunAgent error: %v", err)
	}

	req := prov.lastRequest()
	if req.Model != "unit" {
		t.Fatalf("expected provider to receive model 'unit', got %q", req.Model)
	}
}

func TestMemoryAutomationNotAutoEnabled(t *testing.T) {
	eng, err := New(WithMemoryStore(memory.NewInMemoryStore()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if eng.memoryAutomator != nil {
		t.Fatalf("expected memory automation to be disabled by default")
	}
}
