package protocol

import "sync"

// Factory constructs a protocol with config.
type Factory func(cfg Config) Protocol

// Registry stores protocol factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates a new protocol registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register registers a protocol factory.
func (r *Registry) Register(id string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[id] = factory
}

// Get returns a protocol factory by ID.
func (r *Registry) Get(id string) (Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[id]
	return factory, ok
}

// List returns registered protocol IDs.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.factories))
	for id := range r.factories {
		ids = append(ids, id)
	}
	return ids
}
