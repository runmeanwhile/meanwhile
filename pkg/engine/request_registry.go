package engine

import (
	"context"
	"fmt"
	"sync"
)

// RequestRegistry maps human request IDs to session IDs.
type RequestRegistry interface {
	Register(ctx context.Context, requestID, sessionID string) error
	Resolve(ctx context.Context, requestID string) (string, error)
	Delete(ctx context.Context, requestID string) error
}

// InMemoryRequestRegistry stores request mappings in memory.
type InMemoryRequestRegistry struct {
	mu       sync.RWMutex
	requests map[string]string
}

// NewInMemoryRequestRegistry creates an in-memory request registry.
func NewInMemoryRequestRegistry() *InMemoryRequestRegistry {
	return &InMemoryRequestRegistry{
		requests: make(map[string]string),
	}
}

// SessionForRequest resolves a session for a request ID.
func (e *Engine) SessionForRequest(ctx context.Context, requestID string) (*Session, error) {
	if e == nil || e.requestRegistry == nil {
		return nil, ErrRequestRegistryRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessionID, err := e.requestRegistry.Resolve(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return e.session(ctx, sessionID)
}

// Register records a request -> session mapping.
func (r *InMemoryRequestRegistry) Register(_ context.Context, requestID, sessionID string) error {
	if requestID == "" {
		return fmt.Errorf("request id required")
	}
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.requests[requestID]; ok {
		if existing == sessionID {
			return nil
		}
		return fmt.Errorf("request already registered")
	}
	r.requests[requestID] = sessionID
	return nil
}

// Resolve returns the session ID for a request.
func (r *InMemoryRequestRegistry) Resolve(_ context.Context, requestID string) (string, error) {
	if requestID == "" {
		return "", fmt.Errorf("request id required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	sessionID, ok := r.requests[requestID]
	if !ok {
		return "", ErrRequestNotFound
	}
	return sessionID, nil
}

// Delete removes a request mapping.
func (r *InMemoryRequestRegistry) Delete(_ context.Context, requestID string) error {
	if requestID == "" {
		return fmt.Errorf("request id required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.requests[requestID]; !ok {
		return ErrRequestNotFound
	}
	delete(r.requests, requestID)
	return nil
}
