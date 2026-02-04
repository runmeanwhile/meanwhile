package mcp

import sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

// ToolFilter decides whether a tool should be exposed.
type ToolFilter func(*sdkmcp.Tool) bool

// ToolIDFunc maps an MCP tool name to a local tool ID.
type ToolIDFunc func(serverName, toolName string) string
