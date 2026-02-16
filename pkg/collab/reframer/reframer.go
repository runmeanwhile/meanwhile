package reframer

import (
	"context"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Frame is a reframed view of the problem.
type Frame struct {
	Lens      string `json:"lens" description:"The perspective or angle: operational, behavioral, emotional, economic, trust, workflow, adoption, or another lens that fits"`
	HMW       string `json:"hmw" description:"The How Might We question - specific enough to guide ideation, broad enough to allow creativity, free of embedded solutions"`
	Rationale string `json:"rationale,omitempty" description:"Brief explanation of why this framing matters or what direction it opens"`
	Title     string `json:"title,omitempty" description:"Optional short label for this frame"`
}

// Registry collects HMW frames submitted by agents.
type Registry struct {
	mu     sync.Mutex
	frames []Frame
}

// NewRegistry creates an empty frame registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Frames returns all submitted frames.
func (r *Registry) Frames() []Frame {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Frame(nil), r.frames...)
}

// Clear removes all frames.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = nil
}

type submitResult struct {
	Status string `json:"status"`
	Frame  Frame  `json:"frame"`
}

// Tool returns a Meanwhile tool that registers HMW frames.
func (r *Registry) Tool() (tool.Tool, error) {
	t, err := tool.New("submit_hmw", func(_ context.Context, input Frame) (submitResult, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.frames = append(r.frames, input)
		return submitResult{
			Status: "registered",
			Frame:  input,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return t.WithDescription(`Submit a "How Might We" reframing of the problem.

Call this tool when you've identified a meaningful way to reframe the problem. Good HMW questions:
- Are specific enough to guide ideation, broad enough to allow creativity
- Are free of embedded solutions or assumptions  
- Explore a distinct lens (operational, behavioral, emotional, economic, trust, workflow, adoption)
- Open new design directions the team hasn't considered

Submit one HMW at a time. You can call this multiple times if you have multiple distinct framings.`), nil
}

