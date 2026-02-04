package mcp

import (
	"context"
	"fmt"

	"github.com/darkostanimirovic/meanwhile/pkg/tool"
	"github.com/darkostanimirovic/meanwhile/pkg/toolkit"
)

// Toolkit wraps an MCP server as a toolkit.
type Toolkit struct {
	id       string
	server   *Server
	autoInit bool
}

// NewToolkit creates a toolkit for an MCP server.
func NewToolkit(id string, server *Server) *Toolkit {
	if id == "" {
		id = "toolkit.mcp"
	}
	return &Toolkit{id: id, server: server, autoInit: true}
}

// ID returns the toolkit ID.
func (t *Toolkit) ID() string { return t.id }

// Tools returns MCP proxy tools.
func (t *Toolkit) Tools(ctx context.Context) ([]tool.Tool, error) {
	if t.server == nil {
		return nil, fmt.Errorf("mcp server required")
	}
	if t.autoInit {
		if err := t.server.Connect(ctx); err != nil {
			return nil, err
		}
		if err := t.server.Refresh(ctx); err != nil {
			return nil, err
		}
	}
	tools := make([]tool.Tool, 0)
	for _, proxy := range t.server.Tools() {
		tools = append(tools, proxy)
	}
	return tools, nil
}

// DefaultToolIDs returns MCP tool IDs.
func (t *Toolkit) DefaultToolIDs() []string {
	if t.server == nil {
		return nil
	}
	return t.server.ToolIDs()
}

// BindRegistry binds the toolkit to a registry for refresh updates.
func (t *Toolkit) BindRegistry(reg *tool.Registry) {
	if t.server == nil {
		return
	}
	t.server.RegisterTools(reg)
}

var _ toolkit.Toolkit = (*Toolkit)(nil)
var _ toolkit.RegistryBinder = (*Toolkit)(nil)
