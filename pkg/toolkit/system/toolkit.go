package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
	"github.com/runmeanwhile/meanwhile/pkg/toolkit"
)

// Config controls system tool guardrails.
type Config struct {
	Allow []string
	Deny  []string
}

// Toolkit exposes system tools with guardrails.
type Toolkit struct {
	id  string
	cfg Config
}

// New creates a system toolkit.
func New(id string, cfg Config) *Toolkit {
	if id == "" {
		id = "toolkit.system"
	}
	return &Toolkit{id: id, cfg: normalizeConfig(cfg)}
}

// ID returns the toolkit ID.
func (t *Toolkit) ID() string { return t.id }

// Tools returns system tools.
func (t *Toolkit) Tools(_ context.Context) ([]tool.Tool, error) {
	lookupTool, err := newLookupTool(t.cfg)
	if err != nil {
		return nil, err
	}
	tools := []tool.Tool{toolkit.Tagged(lookupTool, "system", "path")}
	return tools, nil
}

// DefaultToolIDs returns all tools as defaults.
func (t *Toolkit) DefaultToolIDs() []string { return nil }

type lookupArgs struct {
	Binary string `json:"binary" description:"Executable name to locate in PATH"`
}

type lookupOutput struct {
	Binary string `json:"binary"`
	Path   string `json:"path,omitempty"`
	Found  bool   `json:"found"`
}

func newLookupTool(cfg Config) (tool.Tool, error) {
	lookupTool, err := tool.New[lookupArgs, lookupOutput]("path_lookup", func(_ context.Context, args lookupArgs) (lookupOutput, error) {
		binary := strings.TrimSpace(args.Binary)
		if binary == "" {
			return lookupOutput{}, fmt.Errorf("binary required")
		}
		if !allowed(binary, cfg) {
			return lookupOutput{}, fmt.Errorf("binary not allowed: %s", binary)
		}
		path, err := exec.LookPath(binary)
		if err != nil {
			return lookupOutput{Binary: binary, Found: false}, nil
		}
		return lookupOutput{Binary: binary, Path: path, Found: true}, nil
	})
	if err != nil {
		return nil, err
	}
	return lookupTool.WithDescription("Look up an executable on PATH"), nil
}

func normalizeConfig(cfg Config) Config {
	cfg.Allow = unique(cfg.Allow)
	cfg.Deny = unique(cfg.Deny)
	return cfg
}

func allowed(value string, cfg Config) bool {
	if contains(cfg.Deny, value) {
		return false
	}
	if len(cfg.Allow) == 0 {
		return true
	}
	return contains(cfg.Allow, value)
}

func unique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
