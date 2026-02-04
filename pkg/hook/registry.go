package hook

import (
	"sort"
	"sync"
)

// Registry stores hooks by type.
type Registry struct {
	mu         sync.RWMutex
	preMessage []PreMessageHook
	preTurn    []PreTurnHook
	postTurn   []PostTurnHook
	preTool    []PreToolHook
	postTool   []PostToolHook
	stopHooks  []StopHook
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a hook to relevant lists.
func (r *Registry) Register(h Hook) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if hook, ok := h.(PreMessageHook); ok {
		r.preMessage = append(r.preMessage, hook)
	}
	if hook, ok := h.(PreTurnHook); ok {
		r.preTurn = append(r.preTurn, hook)
	}
	if hook, ok := h.(PostTurnHook); ok {
		r.postTurn = append(r.postTurn, hook)
	}
	if hook, ok := h.(PreToolHook); ok {
		r.preTool = append(r.preTool, hook)
	}
	if hook, ok := h.(PostToolHook); ok {
		r.postTool = append(r.postTool, hook)
	}
	if hook, ok := h.(StopHook); ok {
		r.stopHooks = append(r.stopHooks, hook)
	}

	sort.SliceStable(r.preMessage, func(i, j int) bool { return r.preMessage[i].Priority() < r.preMessage[j].Priority() })
	sort.SliceStable(r.preTurn, func(i, j int) bool { return r.preTurn[i].Priority() < r.preTurn[j].Priority() })
	sort.SliceStable(r.postTurn, func(i, j int) bool { return r.postTurn[i].Priority() < r.postTurn[j].Priority() })
	sort.SliceStable(r.preTool, func(i, j int) bool { return r.preTool[i].Priority() < r.preTool[j].Priority() })
	sort.SliceStable(r.postTool, func(i, j int) bool { return r.postTool[i].Priority() < r.postTool[j].Priority() })
	sort.SliceStable(r.stopHooks, func(i, j int) bool { return r.stopHooks[i].Priority() < r.stopHooks[j].Priority() })
}

// PreMessage returns registered pre-message hooks.
func (r *Registry) PreMessage() []PreMessageHook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]PreMessageHook(nil), r.preMessage...)
}

// PreTurn returns registered pre-turn hooks.
func (r *Registry) PreTurn() []PreTurnHook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]PreTurnHook(nil), r.preTurn...)
}

// PostTurn returns registered post-turn hooks.
func (r *Registry) PostTurn() []PostTurnHook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]PostTurnHook(nil), r.postTurn...)
}

// PreTool returns registered pre-tool hooks.
func (r *Registry) PreTool() []PreToolHook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]PreToolHook(nil), r.preTool...)
}

// PostTool returns registered post-tool hooks.
func (r *Registry) PostTool() []PostToolHook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]PostToolHook(nil), r.postTool...)
}

// StopHooks returns registered stop hooks.
func (r *Registry) StopHooks() []StopHook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]StopHook(nil), r.stopHooks...)
}
