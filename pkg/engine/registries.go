package engine

import (
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/integration"
	"github.com/runmeanwhile/meanwhile/pkg/mcp"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
	"github.com/runmeanwhile/meanwhile/pkg/toolkit"
)

// ProviderRegistry returns the provider registry.
func (e *Engine) ProviderRegistry() *provider.Registry { return e.providers }

// ProtocolRegistry returns the protocol registry.
func (e *Engine) ProtocolRegistry() *protocol.Registry { return e.protocols }

// HookRegistry returns the hook registry.
func (e *Engine) HookRegistry() *hook.Registry { return e.hooks }

// ToolRegistry returns the tool registry.
func (e *Engine) ToolRegistry() *tool.Registry { return e.tools }

// ToolFactoryRegistry returns the tool factory registry.
func (e *Engine) ToolFactoryRegistry() *tool.FactoryRegistry { return e.toolFactories }

// ToolkitRegistry returns the toolkit registry.
func (e *Engine) ToolkitRegistry() *toolkit.Registry { return e.toolkits }

// IntegrationRegistry returns the integration registry.
func (e *Engine) IntegrationRegistry() *integration.Registry { return e.integrations }

// RequestRegistry returns the request registry.
func (e *Engine) RequestRegistry() RequestRegistry { return e.requestRegistry }

// RegisterIntegration registers an integration with the engine.
func (e *Engine) RegisterIntegration(integration integration.Integration) error {
	return e.registerIntegration(integration)
}

// RegisterTool registers a tool with the engine.
// This is a convenience method equivalent to engine.ToolRegistry().Register(t).
func (e *Engine) RegisterTool(t any) error {
	toolInst, ok := t.(tool.Tool)
	if !ok {
		return fmt.Errorf("not a valid tool: %T", t)
	}
	e.tools.Register(toolInst)
	return nil
}

// RegisterToolFactory registers a tool factory with the engine.
func (e *Engine) RegisterToolFactory(factory tool.Factory) error {
	if factory == nil || factory.ID() == "" {
		return fmt.Errorf("tool factory id required")
	}
	if e.toolFactories == nil {
		e.toolFactories = tool.NewFactoryRegistry()
	}
	e.toolFactories.Register(factory)
	return nil
}

// RegisterToolkit registers a toolkit with the engine.
func (e *Engine) RegisterToolkit(tk toolkit.Toolkit) error {
	if tk == nil || tk.ID() == "" {
		return fmt.Errorf("toolkit id required")
	}
	if e.toolkits == nil {
		e.toolkits = toolkit.NewRegistry()
	}
	e.toolkits.Register(tk)
	return nil
}

// MCPRegistry returns the MCP registry.
func (e *Engine) MCPRegistry() *mcp.Registry { return e.mcp }

// ProfileRegistry returns the agent profile registry.
func (e *Engine) ProfileRegistry() *agent.Registry { return e.profiles }
