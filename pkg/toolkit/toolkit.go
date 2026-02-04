package toolkit

import (
	"context"

	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

// Toolkit represents a bundle of tools.
type Toolkit interface {
	ID() string
	Tools(ctx context.Context) ([]tool.Tool, error)
	DefaultToolIDs() []string
}

// RegistryBinder allows a toolkit to bind to a tool registry for refresh updates.
type RegistryBinder interface {
	BindRegistry(reg *tool.Registry)
}
