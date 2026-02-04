package engine

import "github.com/runmeanwhile/meanwhile/pkg/tool"

func (s *Session) resolveToolPolicy(runPolicy tool.Policy) tool.Policy {
	base, ok := s.ToolPolicy()
	if !ok {
		return tool.MergePolicy(tool.Policy{}, runPolicy)
	}
	return tool.MergePolicy(base, runPolicy)
}
