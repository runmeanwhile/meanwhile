package integration

import (
	"context"
	"fmt"
	"sync"
)

// Integration delivers human escalation requests to an external channel.
type Integration interface {
	ID() string
	Channel() string
	Send(ctx context.Context, req Request) error
}

// Registry stores integrations by channel.
type Registry struct {
	mu        sync.RWMutex
	byChannel map[string][]Integration
	byID      map[string]Integration
}

// NewRegistry creates a registry for integrations.
func NewRegistry() *Registry {
	return &Registry{
		byChannel: make(map[string][]Integration),
		byID:      make(map[string]Integration),
	}
}

// Register adds an integration.
func (r *Registry) Register(integration Integration) error {
	if integration == nil {
		return fmt.Errorf("integration required")
	}
	channel := integration.Channel()
	if channel == "" {
		return fmt.Errorf("integration channel required")
	}
	id := integration.ID()
	if id == "" {
		return fmt.Errorf("integration id required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[id]; exists {
		return fmt.Errorf("integration already registered: %s", id)
	}
	r.byID[id] = integration
	r.byChannel[channel] = append(r.byChannel[channel], integration)
	return nil
}

// Integrations returns integrations registered for a channel.
func (r *Registry) Integrations(channel string) []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byChannel[channel]
	if len(list) == 0 {
		return nil
	}
	out := make([]Integration, len(list))
	copy(out, list)
	return out
}

// List returns all registered integration IDs.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	return ids
}
