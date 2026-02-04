package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type transportFactory func() (sdkmcp.Transport, error)

// Server represents a connected MCP server and its exposed tools.
type Server struct {
	name            string
	prefix          string
	toolIDFunc      ToolIDFunc
	filter          ToolFilter
	allowlist       map[string]struct{}
	denylist        map[string]struct{}
	client          *sdkmcp.Client
	transport       sdkmcp.Transport
	transportSource transportFactory
	sessionOptions  *sdkmcp.ClientSessionOptions
	toolRegistry    *tool.Registry
	refreshMu       sync.Mutex

	mu        sync.RWMutex
	session   *sdkmcp.ClientSession
	tools     map[string]*ProxyTool
	toolNames map[string]*ProxyTool
}

// Name returns the server name.
func (s *Server) Name() string { return s.name }

// Prefix returns the tool ID prefix.
func (s *Server) Prefix() string { return s.prefix }

// Client returns the underlying MCP client.
func (s *Server) Client() *sdkmcp.Client { return s.client }

// Session returns the underlying MCP client session.
func (s *Server) Session() *sdkmcp.ClientSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session
}

// Connect establishes the MCP session if not already connected.
func (s *Server) Connect(ctx context.Context) error {
	s.mu.RLock()
	if s.session != nil {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	if s.transportSource == nil {
		return ErrTransportRequired
	}

	transport, err := s.transportSource()
	if err != nil {
		return err
	}

	sess, err := s.client.Connect(ctx, transport, s.sessionOptions)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.transport = transport
	s.session = sess
	s.mu.Unlock()
	return nil
}

// Close closes the MCP session.
func (s *Server) Close() error {
	s.mu.Lock()
	sess := s.session
	s.session = nil
	s.transport = nil
	s.mu.Unlock()

	if sess != nil {
		return sess.Close()
	}
	return nil
}

// ToolIDs returns the registered tool IDs.
func (s *Server) ToolIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.tools))
	for id := range s.tools {
		ids = append(ids, id)
	}
	return ids
}

// ToolID returns the local tool ID for an MCP tool name.
func (s *Server) ToolID(toolName string) string {
	return s.toolID(toolName)
}

// Tools returns the proxy tools.
func (s *Server) Tools() []*ProxyTool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tools := make([]*ProxyTool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}
	return tools
}

// RegisterTools registers the MCP tools into the provided registry and stores it for refresh.
func (s *Server) RegisterTools(reg *tool.Registry) {
	s.mu.Lock()
	s.toolRegistry = reg
	tools := make([]*ProxyTool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}
	s.mu.Unlock()

	if reg == nil {
		return
	}
	for _, t := range tools {
		reg.Register(t)
	}
}

// Refresh re-fetches tools from the MCP server and updates proxies.
func (s *Server) Refresh(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	s.mu.RLock()
	sess := s.session
	s.mu.RUnlock()
	if sess == nil {
		return ErrNotConnected
	}

	tools, err := listAllTools(ctx, sess)
	if err != nil {
		return err
	}

	s.updateTools(tools)
	return nil
}

// CallTool calls a remote tool by name.
func (s *Server) CallTool(ctx context.Context, name string, args json.RawMessage) (*sdkmcp.CallToolResult, error) {
	s.mu.RLock()
	sess := s.session
	s.mu.RUnlock()
	if sess == nil {
		return nil, ErrNotConnected
	}

	var payload any
	if len(args) > 0 {
		payload = json.RawMessage(args)
	}

	return sess.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: payload,
	})
}

func (s *Server) toolID(toolName string) string {
	if s.toolIDFunc != nil {
		return s.toolIDFunc(s.name, toolName)
	}
	prefix := s.prefix
	if prefix == "" {
		prefix = s.name
	}
	if prefix == "" {
		return toolName
	}
	return fmt.Sprintf("%s.%s", prefix, toolName)
}

func (s *Server) allows(tool *sdkmcp.Tool) bool {
	if tool == nil {
		return false
	}
	if len(s.allowlist) > 0 {
		if _, ok := s.allowlist[tool.Name]; !ok {
			return false
		}
	}
	if _, ok := s.denylist[tool.Name]; ok {
		return false
	}
	if s.filter != nil {
		return s.filter(tool)
	}
	return true
}

func (s *Server) updateTools(tools []*sdkmcp.Tool) {
	// Track existing tool IDs for stale cleanup.
	s.mu.RLock()
	existing := make(map[string]struct{}, len(s.tools))
	for id := range s.tools {
		existing[id] = struct{}{}
	}
	reg := s.toolRegistry
	s.mu.RUnlock()

	proxies := make(map[string]*ProxyTool)
	byName := make(map[string]*ProxyTool)

	for _, t := range tools {
		if !s.allows(t) {
			continue
		}
		toolID := s.toolID(t.Name)
		schema := schemaFromInput(t.InputSchema)
		description := t.Description
		if description == "" && t.Annotations != nil {
			description = t.Annotations.Title
		}
		proxy := &ProxyTool{
			server:      s,
			toolName:    t.Name,
			toolID:      toolID,
			schema:      schema,
			description: description,
		}
		proxies[toolID] = proxy
		byName[t.Name] = proxy
	}

	// Remove stale tools from registry.
	if reg != nil {
		for id := range existing {
			if _, ok := proxies[id]; !ok {
				reg.Unregister(id)
			}
		}
	}

	s.mu.Lock()
	s.tools = proxies
	s.toolNames = byName
	s.mu.Unlock()

	if reg != nil {
		for _, t := range proxies {
			reg.Register(t)
		}
	}
}

func listAllTools(ctx context.Context, sess *sdkmcp.ClientSession) ([]*sdkmcp.Tool, error) {
	var (
		cursor string
		all    []*sdkmcp.Tool
	)
	for {
		res, err := sess.ListTools(ctx, &sdkmcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		all = append(all, res.Tools...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return all, nil
}

func schemaFromInput(input any) tool.Schema {
	if input == nil {
		return tool.Schema{}
	}

	switch value := input.(type) {
	case json.RawMessage:
		return tool.Schema{JSONSchema: value}
	case []byte:
		return tool.Schema{JSONSchema: json.RawMessage(value)}
	default:
		if raw, err := json.Marshal(value); err == nil {
			return tool.Schema{JSONSchema: raw}
		}
	}
	return tool.Schema{}
}
