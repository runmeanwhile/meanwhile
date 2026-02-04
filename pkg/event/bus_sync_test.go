package event

import (
	"sync/atomic"
	"testing"
)

func TestBusSubscribeSync(t *testing.T) {
	bus := NewBus()

	var called int32
	unsub, err := bus.SubscribeSync(func(_ Event) {
		atomic.AddInt32(&called, 1)
	})
	if err != nil {
		t.Fatalf("SubscribeSync error: %v", err)
	}

	if err := bus.Publish(New(AgentStarted, "sess", nil)); err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected sync subscriber to be called")
	}

	unsub()
}
