package tool

import (
	"context"
	"encoding/json"
	"testing"
)

type dummyTool struct{ id string }

func (d dummyTool) ID() string     { return d.id }
func (d dummyTool) Schema() Schema { return Schema{JSONSchema: json.RawMessage(`{}`)} }
func (d dummyTool) Run(_ context.Context, call Call, _ Emitter) (Result, error) {
	return TextResult(call, "ok"), nil
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.Register(dummyTool{id: "t1"})

	if _, ok := reg.Get("t1"); !ok {
		t.Fatal("expected tool")
	}
}
