package mcp

import "sync"

// Registry stores MCP server connections by name.
type Registry struct {
	mu      sync.RWMutex
	servers map[string]*Server
}

// NewRegistry creates a new MCP registry.
func NewRegistry() *Registry {
	return &Registry{servers: make(map[string]*Server)}
}

// Register stores a server by name.
func (r *Registry) Register(server *Server) {
	if server == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers[server.Name()] = server
}

// Get returns a server by name.
func (r *Registry) Get(name string) (*Server, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	server, ok := r.servers[name]
	return server, ok
}

// List returns registered server names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	return names
}

// CloseAll closes all registered servers.
func (r *Registry) CloseAll() error {
	r.mu.RLock()
	servers := make([]*Server, 0, len(r.servers))
	for _, server := range r.servers {
		servers = append(servers, server)
	}
	r.mu.RUnlock()

	var firstErr error
	for _, server := range servers {
		if err := server.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
