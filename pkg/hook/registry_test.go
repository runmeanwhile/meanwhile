package hook

import (
	"context"
	"testing"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

type testHook struct {
	id       string
	priority int
}

func (t testHook) ID() string    { return t.id }
func (t testHook) Priority() int { return t.priority }
func (t testHook) OnPreMessage(_ context.Context, _ SessionMeta, msg agent.Message) (Decision, agent.Message, error) {
	return Allow, msg, nil
}
func (t testHook) OnPreToolUse(_ context.Context, _ SessionMeta, call tool.Call) (Decision, tool.Call, error) {
	return Allow, call, nil
}

func TestRegistryOrder(t *testing.T) {
	reg := NewRegistry()
	reg.Register(testHook{id: "b", priority: 10})
	reg.Register(testHook{id: "a", priority: 1})

	hooks := reg.PreMessage()
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}
	if hooks[0].ID() != "a" {
		t.Fatalf("expected hook a first, got %s", hooks[0].ID())
	}
}
