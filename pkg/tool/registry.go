package tool

import "sync"

// Registry stores tool implementations.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates a registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.ID()] = t
}

// Unregister removes a tool by ID.
func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, id)
}

// Get retrieves a tool.
func (r *Registry) Get(id string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[id]
	return tool, ok
}

// List returns registered tool IDs.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.tools))
	for id := range r.tools {
		ids = append(ids, id)
	}
	return ids
}
