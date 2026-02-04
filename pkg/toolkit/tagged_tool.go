package toolkit

import (
	"context"

	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

// Tagged wraps a tool with tags.
func Tagged(inner tool.Tool, tags ...string) tool.Tool {
	if inner == nil {
		return nil
	}
	return taggedTool{inner: inner, tags: append([]string(nil), tags...)}
}

type taggedTool struct {
	inner tool.Tool
	tags  []string
}

func (t taggedTool) ID() string { return t.inner.ID() }

func (t taggedTool) Schema() tool.Schema { return t.inner.Schema() }

func (t taggedTool) Run(ctx context.Context, call tool.Call, emit tool.Emitter) (tool.Result, error) {
	return t.inner.Run(ctx, call, emit)
}

func (t taggedTool) Tags() []string { return append([]string(nil), t.tags...) }

func (t taggedTool) Description() string {
	if d, ok := t.inner.(interface{ Description() string }); ok {
		return d.Description()
	}
	return ""
}
