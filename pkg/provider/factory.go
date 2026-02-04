package provider

import (
	"errors"
	"strings"
	"sync"

	"github.com/darkostanimirovic/meanwhile/pkg/config"
)

// Factory builds a Provider from configuration.
type Factory func(cfg config.ProviderConfig) (Provider, error)

var (
	factoryMu sync.RWMutex
	factories = make(map[string]Factory)

	// ErrProviderTypeRequired indicates a missing provider type.
	ErrProviderTypeRequired = errors.New("provider type required")
	// ErrProviderFactoryNotFound indicates an unknown provider type.
	ErrProviderFactoryNotFound = errors.New("provider factory not found")
)

// RegisterFactory registers a provider factory for a given type.
func RegisterFactory(providerType string, factory Factory) {
	if providerType == "" || factory == nil {
		return
	}
	key := strings.ToLower(providerType)
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[key] = factory
}

// NewFromConfig constructs a provider from configuration.
func NewFromConfig(cfg config.ProviderConfig) (Provider, error) {
	if strings.TrimSpace(cfg.Type) == "" {
		return nil, ErrProviderTypeRequired
	}
	key := strings.ToLower(cfg.Type)
	factoryMu.RLock()
	factory, ok := factories[key]
	factoryMu.RUnlock()
	if !ok {
		return nil, ErrProviderFactoryNotFound
	}
	return factory(cfg)
}

// WithID wraps a provider with a custom ID.
func WithID(p Provider, id string) Provider {
	if p == nil || id == "" {
		return p
	}
	return providerAlias{Provider: p, id: id}
}

type providerAlias struct {
	Provider
	id string
}

func (p providerAlias) ID() string { return p.id }
