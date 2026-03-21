package modelruntime

import (
	"context"
	"sync"
)

// Stream receives provider events.
type Stream interface {
	Recv() (Event, error)
	Close() error
}

// Provider starts streams for normalized model requests.
type Provider interface {
	ID() string
	Stream(ctx context.Context, req Request) (Stream, error)
}

// Registry stores providers by ID.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a provider registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register registers a provider.
func (r *Registry) Register(p Provider) {
	if r == nil || p == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID()] = p
}

// Get retrieves a provider by ID.
func (r *Registry) Get(id string) (Provider, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	return p, ok
}

// List returns registered provider IDs.
func (r *Registry) List() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}
