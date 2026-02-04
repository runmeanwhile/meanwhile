package event

import (
	"sync"
	"testing"
	"time"
)

func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus()
	t.Cleanup(bus.Close)

	var mu sync.Mutex
	received := make([]Event, 0, 2)
	done := make(chan struct{})

	_, err := bus.Subscribe(func(ev Event) {
		mu.Lock()
		received = append(received, ev)
		if len(received) == 2 {
			close(done)
		}
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("subscribe error: %v", err)
	}

	ev1 := New(AgentStarted, "sess-1", map[string]any{"n": 1})
	ev2 := New(AgentThinking, "sess-1", map[string]any{"n": 2})

	if err := bus.Publish(ev1); err != nil {
		t.Fatalf("publish error: %v", err)
	}
	if err := bus.Publish(ev2); err != nil {
		t.Fatalf("publish error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for events")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
}

func TestBusDropHandler(t *testing.T) {
	dropped := 0
	bus := NewBus(WithBuffer(1), WithDropHandler(func(Event) { dropped++ }))
	t.Cleanup(bus.Close)

	_, err := bus.Subscribe(func(Event) {
		time.Sleep(50 * time.Millisecond)
	})
	if err != nil {
		t.Fatalf("subscribe error: %v", err)
	}

	for i := 0; i < 10; i++ {
		if err := bus.Publish(New(AgentThinking, "sess", nil)); err != nil {
			t.Fatalf("publish error: %v", err)
		}
	}

	if dropped == 0 {
		t.Fatal("expected dropped events")
	}
}

func TestBusDroppedCounter(t *testing.T) {
	bus := NewBus(WithBuffer(1))
	t.Cleanup(bus.Close)

	started := make(chan struct{})
	block := make(chan struct{})

	_, err := bus.Subscribe(func(Event) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-block
	})
	if err != nil {
		t.Fatalf("subscribe error: %v", err)
	}

	if err := bus.Publish(New(AgentThinking, "sess", nil)); err != nil {
		t.Fatalf("publish error: %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for handler")
	}

	for i := 0; i < 3; i++ {
		if err := bus.Publish(New(AgentThinking, "sess", nil)); err != nil {
			t.Fatalf("publish error: %v", err)
		}
	}

	if bus.Dropped() == 0 {
		t.Fatal("expected dropped counter to increment")
	}

	close(block)
}
