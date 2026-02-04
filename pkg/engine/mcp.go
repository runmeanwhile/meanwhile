package engine

import "github.com/darkostanimirovic/meanwhile/pkg/mcp"

// MCP creates a builder for connecting to an MCP server.
func (e *Engine) MCP(name string) *mcp.Builder {
	return mcp.NewBuilder(e, name)
}
