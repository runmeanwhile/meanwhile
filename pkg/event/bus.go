package event

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Subscriber handles events delivered by the Bus.
type Subscriber func(Event)

// Unsubscribe removes a subscription.
type Unsubscribe func()

var (
	// ErrBusClosed indicates a publish or subscribe attempt on a closed bus.
	ErrBusClosed = errors.New("event bus closed")
)

type subscription struct {
	ch      chan Event
	handler Subscriber
	done    chan struct{}
}

// Bus is a simple pub-sub event bus with buffered delivery.
type Bus struct {
	mu            sync.RWMutex
	subs          map[int]subscription
	syncSubs      map[int]Subscriber
	nextID        int
	closed        bool
	defaultBuffer int
	onDrop        func(Event)
	dropped       uint64
}

// BusOption configures the bus.
type BusOption func(*Bus)

// WithBuffer sets the default subscriber buffer size.
func WithBuffer(size int) BusOption {
	return func(b *Bus) {
		if size > 0 {
			b.defaultBuffer = size
		}
	}
}

// WithDropHandler sets a handler for dropped events.
func WithDropHandler(fn func(Event)) BusOption {
	return func(b *Bus) {
		b.onDrop = fn
	}
}

// NewBus creates a new event bus.
func NewBus(opts ...BusOption) *Bus {
	b := &Bus{
		subs:          make(map[int]subscription),
		syncSubs:      make(map[int]Subscriber),
		defaultBuffer: 128,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Subscribe registers a subscriber and returns an unsubscribe function.
func (b *Bus) Subscribe(handler Subscriber) (Unsubscribe, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrBusClosed
	}

	id := b.nextID
	b.nextID++

	sub := subscription{
		ch:      make(chan Event, b.defaultBuffer),
		handler: handler,
		done:    make(chan struct{}),
	}
	b.subs[id] = sub

	go func(ch <-chan Event, done <-chan struct{}, h Subscriber) {
		for {
			select {
			case ev := <-ch:
				h(ev)
			case <-done:
				return
			}
		}
	}(sub.ch, sub.done, sub.handler)

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if sub, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(sub.done)
		}
	}, nil
}

// SubscribeSync registers a synchronous subscriber and returns an unsubscribe function.
// Sync subscribers are invoked inline during Publish.
func (b *Bus) SubscribeSync(handler Subscriber) (Unsubscribe, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrBusClosed
	}

	id := b.nextID
	b.nextID++
	b.syncSubs[id] = handler

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.syncSubs, id)
	}, nil
}

// Publish delivers an event to all subscribers.
func (b *Bus) Publish(ev Event) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrBusClosed
	}

	subs := make([]subscription, 0, len(b.subs))
	for _, sub := range b.subs {
		subs = append(subs, sub)
	}
	syncHandlers := make([]Subscriber, 0, len(b.syncSubs))
	for _, handler := range b.syncSubs {
		syncHandlers = append(syncHandlers, handler)
	}
	b.mu.RUnlock()

	for _, handler := range syncHandlers {
		handler(ev)
	}
	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		default:
			atomic.AddUint64(&b.dropped, 1)
			if b.onDrop != nil {
				b.onDrop(ev)
			}
		}
	}
	return nil
}

// Dropped returns the number of events dropped due to backpressure.
func (b *Bus) Dropped() uint64 {
	return atomic.LoadUint64(&b.dropped)
}

// Close closes the bus and all subscribers.
func (b *Bus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	subs := b.subs
	b.subs = make(map[int]subscription)
	syncSubs := b.syncSubs
	b.syncSubs = make(map[int]Subscriber)
	b.mu.Unlock()

	for _, sub := range subs {
		close(sub.done)
	}
	for id := range syncSubs {
		delete(syncSubs, id)
	}
}
