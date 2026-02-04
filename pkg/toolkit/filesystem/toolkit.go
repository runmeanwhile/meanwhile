package filesystem

import (
	"context"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
	"github.com/runmeanwhile/meanwhile/pkg/toolkit"
)

// Config controls filesystem guardrails.
type Config struct {
	Roots          []string
	ReadOnly       bool
	MaxReadBytes   int
	MaxListEntries int
	MaxDepth       int
	FileMode       uint32
	DirMode        uint32
}

// Toolkit exposes filesystem tools with guardrails.
type Toolkit struct {
	id     string
	cfg    Config
	safeFS *SafeFS
}

// New creates a filesystem toolkit.
func New(id string, cfg Config) (*Toolkit, error) {
	safeFS, err := NewSafeFS(cfg)
	if err != nil {
		return nil, err
	}
	if id == "" {
		id = "toolkit.filesystem"
	}
	return &Toolkit{id: id, cfg: normalizeConfig(cfg), safeFS: safeFS}, nil
}

// ID returns the toolkit ID.
func (t *Toolkit) ID() string { return t.id }

// Tools returns the guarded filesystem tools.
func (t *Toolkit) Tools(_ context.Context) ([]tool.Tool, error) {
	if t.safeFS == nil {
		return nil, fmt.Errorf("filesystem toolkit not initialized")
	}
	readTool, err := newReadTool(t.safeFS, t.cfg)
	if err != nil {
		return nil, err
	}
	listTool, err := newListTool(t.safeFS, t.cfg)
	if err != nil {
		return nil, err
	}
	statTool, err := newStatTool(t.safeFS)
	if err != nil {
		return nil, err
	}

	tools := []tool.Tool{
		toolkit.Tagged(readTool, "filesystem", "read"),
		toolkit.Tagged(listTool, "filesystem", "list"),
		toolkit.Tagged(statTool, "filesystem", "stat"),
	}
	if !t.cfg.ReadOnly {
		writeTool, err := newWriteTool(t.safeFS, t.cfg)
		if err != nil {
			return nil, err
		}
		tools = append(tools, toolkit.Tagged(writeTool, "filesystem", "write"))
	}
	return tools, nil
}

// DefaultToolIDs returns all tools as defaults.
func (t *Toolkit) DefaultToolIDs() []string {
	return nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.MaxReadBytes <= 0 {
		cfg.MaxReadBytes = 64 * 1024
	}
	if cfg.MaxListEntries <= 0 {
		cfg.MaxListEntries = 200
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 1
	}
	if cfg.FileMode == 0 {
		cfg.FileMode = 0o640
	}
	if cfg.DirMode == 0 {
		cfg.DirMode = 0o750
	}
	return cfg
}
