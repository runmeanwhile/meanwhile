package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

type timeoutProvider struct {
	calls int32
}

func (p *timeoutProvider) ID() string { return "timeout" }

func (p *timeoutProvider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	if req.Model == "" {
		return nil, errors.New("model required")
	}
	call := atomic.AddInt32(&p.calls, 1)
	if call == 1 {
		return &blockingStream{ctx: ctx}, nil
	}
	return &contextSingleMessageStream{message: runtimeFromAgentMessage(message.Assistant("ok"))}, nil
}

type blockingStream struct {
	ctx context.Context
}

func (s *blockingStream) Recv() (provider.Event, error) {
	<-s.ctx.Done()
	return provider.Event{}, s.ctx.Err()
}

func (s *blockingStream) Close() error { return nil }

func TestRunTimeoutsDoNotPoisonSession(t *testing.T) {
	prov := &timeoutProvider{}
	eng, err := New(WithProvider(prov), WithDefaultRunTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Protocol: protocol.Solo(),
		Participants: []protocol.Participant{agent.Agent{
			Name:  "agent",
			Model: "mock-model",
		}},
	})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}

	_, err = eng.Run(context.Background(), sess.ID(), message.User("hello"))
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout error, got %v", err)
	}

	result, err := eng.Run(context.Background(), sess.ID(), message.User("retry"))
	if err != nil {
		t.Fatalf("expected successful run after timeout, got %v", err)
	}
	if result.Final == "" {
		t.Fatalf("expected final response after retry")
	}
}
