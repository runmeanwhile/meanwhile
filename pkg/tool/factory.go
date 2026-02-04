package tool

import "sync"

// Factory creates tools from configuration parameters.
type Factory interface {
	ID() string
	Build(params map[string]any) (Tool, error)
}

// FactoryRegistry stores tool factories by ID.
type FactoryRegistry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewFactoryRegistry creates an empty factory registry.
func NewFactoryRegistry() *FactoryRegistry {
	return &FactoryRegistry{factories: make(map[string]Factory)}
}

// Register stores a factory by ID.
func (r *FactoryRegistry) Register(factory Factory) {
	if factory == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[factory.ID()] = factory
}

// Get retrieves a factory by ID.
func (r *FactoryRegistry) Get(id string) (Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[id]
	return factory, ok
}

// List returns registered factory IDs.
func (r *FactoryRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.factories))
	for id := range r.factories {
		ids = append(ids, id)
	}
	return ids
}
