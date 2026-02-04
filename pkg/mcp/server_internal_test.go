package mcp

import (
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerRefreshRemovesStaleTools(t *testing.T) {
	reg := tool.NewRegistry()
	s := &Server{
		name:         "srv",
		prefix:       "srv",
		toolRegistry: reg,
		tools:        make(map[string]*ProxyTool),
		toolNames:    make(map[string]*ProxyTool),
	}

	s.updateTools([]*sdkmcp.Tool{{Name: "alpha", Description: "a"}})
	if _, ok := reg.Get("srv.alpha"); !ok {
		t.Fatalf("expected tool srv.alpha to be registered")
	}

	s.updateTools([]*sdkmcp.Tool{{Name: "beta", Description: "b"}})
	if _, ok := reg.Get("srv.alpha"); ok {
		t.Fatalf("expected stale tool srv.alpha to be removed")
	}
	if _, ok := reg.Get("srv.beta"); !ok {
		t.Fatalf("expected tool srv.beta to be registered")
	}
}
