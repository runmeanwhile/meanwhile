package toolkit

import "sync"

// Registry stores toolkits by ID.
type Registry struct {
	mu       sync.RWMutex
	toolkits map[string]Toolkit
}

// NewRegistry creates a new toolkit registry.
func NewRegistry() *Registry {
	return &Registry{toolkits: make(map[string]Toolkit)}
}

// Register stores a toolkit by ID.
func (r *Registry) Register(tk Toolkit) {
	if tk == nil || tk.ID() == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.toolkits[tk.ID()] = tk
}

// Get returns a toolkit by ID.
func (r *Registry) Get(id string) (Toolkit, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tk, ok := r.toolkits[id]
	return tk, ok
}

// List returns registered toolkit IDs.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.toolkits))
	for id := range r.toolkits {
		ids = append(ids, id)
	}
	return ids
}
