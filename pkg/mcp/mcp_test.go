package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type noopEmitter struct{}

func (noopEmitter) Emit(string, any) error { return nil }

func TestMCPBuilderRegistersTools(t *testing.T) {
	ctx := context.Background()

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test"}, nil)
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "add", Description: "add numbers"}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, input struct {
		A int `json:"a"`
		B int `json:"b"`
	}) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("%d", input.A+input.B)}}}, nil, nil
	})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() {
		_ = serverSession.Close()
	}()

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine new: %v", err)
	}

	_, err = eng.MCP("math").Transport(clientTransport).Register(ctx)
	if err != nil {
		t.Fatalf("mcp register: %v", err)
	}

	toolImpl, ok := eng.ToolRegistry().Get("math.add")
	if !ok {
		t.Fatalf("expected tool math.add to be registered")
	}

	args := json.RawMessage(`{"a":2,"b":3}`)
	res, err := toolImpl.Run(ctx, tool.Call{ID: "1", ToolID: "math.add", Arguments: args}, noopEmitter{})
	if err != nil {
		t.Fatalf("tool run: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("tool error: %s", res.Error.Message)
	}
	if res.Text() != "5" {
		t.Fatalf("expected 5, got %q", res.Text())
	}
}
