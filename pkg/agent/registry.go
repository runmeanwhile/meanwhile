package agent

import "sync"

// Profile defines a reusable agent persona.
type Profile struct {
	ID     string
	Name   string
	Prompt string
	Tools  []string
}

// Registry stores agent profiles.
type Registry struct {
	mu       sync.RWMutex
	profiles map[string]Profile
}

// NewRegistry creates a new registry.
func NewRegistry() *Registry {
	return &Registry{profiles: make(map[string]Profile)}
}

// Register adds a profile.
func (r *Registry) Register(profile Profile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles[profile.ID] = profile
}

// Get returns a profile by ID.
func (r *Registry) Get(id string) (Profile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	profile, ok := r.profiles[id]
	return profile, ok
}

// List returns all registered profiles.
func (r *Registry) List() []Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	profiles := make([]Profile, 0, len(r.profiles))
	for _, profile := range r.profiles {
		profiles = append(profiles, profile)
	}
	return profiles
}
