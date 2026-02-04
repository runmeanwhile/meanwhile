package agentcall

import (
	"context"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
	"github.com/runmeanwhile/meanwhile/pkg/toolkit"
)

// Config controls agent call tool behavior.
type Config struct {
	ToolkitID   string
	ToolID      string
	Description string
	Tags        []string
}

// Toolkit exposes an agent delegation tool.
type Toolkit struct {
	engine *engine.Engine
	agent  agent.Agent
	cfg    Config
}

// New creates a toolkit that delegates to a specific agent.
func New(eng *engine.Engine, agent agent.Agent, cfg Config) (*Toolkit, error) {
	if eng == nil {
		return nil, fmt.Errorf("engine required")
	}
	if err := agent.Validate(); err != nil {
		return nil, err
	}
	if cfg.ToolkitID == "" {
		cfg.ToolkitID = "toolkit.agentcall"
	}
	if cfg.ToolID == "" {
		cfg.ToolID = "call_agent"
	}
	if cfg.Description == "" {
		cfg.Description = "Delegate a task to a specialist agent"
	}
	return &Toolkit{engine: eng, agent: agent, cfg: cfg}, nil
}

// ID returns the toolkit ID.
func (t *Toolkit) ID() string { return t.cfg.ToolkitID }

// Tools returns the agent call tool.
func (t *Toolkit) Tools(_ context.Context) ([]tool.Tool, error) {
	toolImpl := t.engine.AsTool(
		protocol.Solo(),
		engine.WithToolName(t.cfg.ToolID),
		engine.WithToolDescription(t.cfg.Description),
		engine.WithToolParticipants(t.agent),
	)
	return []tool.Tool{toolkit.Tagged(toolImpl, t.cfg.Tags...)}, nil
}

// DefaultToolIDs returns the delegation tool ID.
func (t *Toolkit) DefaultToolIDs() []string {
	return []string{t.cfg.ToolID}
}
