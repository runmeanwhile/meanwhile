package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

type serialProtocol struct {
	entered  chan struct{}
	release  chan struct{}
	active   int32
	shutdown int32
}

func (p *serialProtocol) ID() string                           { return "protocol.serial_test" }
func (p *serialProtocol) Participants() []protocol.Participant { return nil }
func (p *serialProtocol) Init(ctx context.Context, sess protocol.Session) error {
	_ = ctx
	_ = sess
	return nil
}
func (p *serialProtocol) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	_ = ctx
	_ = sess
	_ = msg
	atomic.AddInt32(&p.active, 1)
	p.entered <- struct{}{}
	<-p.release
	atomic.AddInt32(&p.active, -1)
	return nil
}
func (p *serialProtocol) OnEvent(ctx context.Context, sess protocol.Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}
func (p *serialProtocol) Shutdown(ctx context.Context, sess protocol.Session) error {
	_ = ctx
	_ = sess
	atomic.StoreInt32(&p.shutdown, 1)
	return nil
}

func TestEngineRunSerializesSessionRuns(t *testing.T) {
	eng, err := New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	proto := &serialProtocol{
		entered: make(chan struct{}, 2),
		release: make(chan struct{}, 2),
	}

	sess, err := eng.NewSession(context.Background(), SessionConfig{
		Name:     "serial-test",
		Protocol: proto,
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	run := func() {
		_, _ = eng.Run(context.Background(), sess.ID(), agent.Message{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "hi"}}})
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		run()
	}()
	go func() {
		defer wg.Done()
		run()
	}()

	// First run should enter immediately.
	select {
	case <-proto.entered:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for first run to start")
	}

	// Second run should not enter until the first is released.
	select {
	case <-proto.entered:
		t.Fatal("expected session runs to be serialized, but saw concurrent entry")
	case <-time.After(200 * time.Millisecond):
	}

	// Release first run and expect the second to enter.
	proto.release <- struct{}{}
	select {
	case <-proto.entered:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for second run to start after release")
	}
	proto.release <- struct{}{}

	wg.Wait()
	_ = eng.CloseSession(context.Background(), sess.ID())
}
