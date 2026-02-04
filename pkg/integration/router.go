package integration

import (
	"context"
	"fmt"
	"sort"
)

// Router dispatches requests to registered integrations.
type Router struct {
	registry *Registry
}

// NewRouter creates a router for a registry.
func NewRouter(registry *Registry) *Router {
	return &Router{registry: registry}
}

// Dispatch sends a request using preferred/fallback channels.
func (r *Router) Dispatch(ctx context.Context, req Request, contacts map[string]string, preferred string) (DispatchResult, error) {
	if r == nil || r.registry == nil {
		return DispatchResult{}, fmt.Errorf("integration registry required")
	}
	if len(contacts) == 0 {
		return DispatchResult{}, fmt.Errorf("no contacts available for human request")
	}

	channels := orderChannels(contacts, preferred)
	var lastErr error
	for _, channel := range channels {
		contact := contacts[channel]
		if contact == "" {
			continue
		}
		req.Channel = channel
		req.Contact = contact
		for _, integration := range r.registry.Integrations(channel) {
			if integration == nil {
				continue
			}
			if ctx == nil {
				ctx = context.Background()
			}
			if err := integration.Send(ctx, req); err != nil {
				lastErr = err
				continue
			}
			return DispatchResult{
				IntegrationID: integration.ID(),
				Channel:       channel,
				Contact:       contact,
			}, nil
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no integration available for requested channels")
	}
	return DispatchResult{}, lastErr
}

func orderChannels(contacts map[string]string, preferred string) []string {
	if len(contacts) == 0 {
		return nil
	}

	keys := make([]string, 0, len(contacts))
	for channel := range contacts {
		if channel == "" {
			continue
		}
		if channel == preferred {
			continue
		}
		keys = append(keys, channel)
	}
	sort.Strings(keys)
	if preferred != "" {
		keys = append([]string{preferred}, keys...)
	}
	return keys
}
