package mcp

import "errors"

var (
	// ErrNotConnected indicates the MCP server has no active session.
	ErrNotConnected = errors.New("mcp server not connected")
	// ErrTransportRequired indicates no transport was configured.
	ErrTransportRequired = errors.New("mcp transport required")
)
