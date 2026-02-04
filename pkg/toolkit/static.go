package toolkit

import (
	"context"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Static bundles tools that are already constructed.
type Static struct {
	id       string
	tools    []tool.Tool
	defaults []string
}

// NewStatic creates a static toolkit.
func NewStatic(id string, tools ...tool.Tool) *Static {
	return &Static{id: id, tools: append([]tool.Tool(nil), tools...)}
}

// ID returns the toolkit ID.
func (s *Static) ID() string { return s.id }

// Tools returns the toolkit tools.
func (s *Static) Tools(_ context.Context) ([]tool.Tool, error) {
	return append([]tool.Tool(nil), s.tools...), nil
}

// DefaultToolIDs returns the default tool IDs for this toolkit.
func (s *Static) DefaultToolIDs() []string {
	if len(s.defaults) == 0 {
		return toolIDsFromTools(s.tools)
	}
	return append([]string(nil), s.defaults...)
}

// Defaults sets the default tool IDs.
func (s *Static) Defaults(ids ...string) *Static {
	s.defaults = append([]string(nil), ids...)
	return s
}

func toolIDsFromTools(tools []tool.Tool) []string {
	if len(tools) == 0 {
		return nil
	}
	out := make([]string, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		id := t.ID()
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
