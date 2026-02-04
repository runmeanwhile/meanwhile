package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// ContentFilterHook blocks messages containing forbidden terms
type ContentFilterHook struct{}

func (h *ContentFilterHook) ID() string    { return "content_filter" }
func (h *ContentFilterHook) Priority() int { return 100 }

func (h *ContentFilterHook) OnPreMessage(ctx context.Context, meta hook.SessionMeta, msg agent.Message) (hook.Decision, agent.Message, error) {
	// Block messages containing forbidden terms
	forbidden := []string{"blockchain", "synergy", "paradigm shift"}
	for _, term := range forbidden {
		if strings.Contains(strings.ToLower(msg.Text()), term) {
			fmt.Printf("[HOOK] Blocked message containing: %s\n", term)
			return hook.Block, msg, fmt.Errorf("forbidden buzzword detected: %s", term)
		}
	}
	return hook.Allow, msg, nil
}

// ToolAuditHook logs tool usage
type ToolAuditHook struct{}

func (h *ToolAuditHook) ID() string    { return "tool_audit" }
func (h *ToolAuditHook) Priority() int { return 50 }

func (h *ToolAuditHook) OnPreToolUse(ctx context.Context, meta hook.SessionMeta, call tool.Call) (hook.Decision, tool.Call, error) {
	fmt.Printf("[AUDIT] Tool called: %s in session %s\n", call.ToolID, meta.SessionID)
	return hook.Allow, call, nil
}

func main() {
	ctx := context.Background()

	provider, _ := openai.FromEnv()

	// Register hooks for runtime control
	hookReg := hook.NewRegistry()
	hookReg.Register(&ContentFilterHook{})
	hookReg.Register(&ToolAuditHook{})

	eng, _ := engine.New(
		engine.WithProvider(provider),
		engine.WithHookRegistry(hookReg),
	)

	// Create a tool that will be audited
	type ChangeArgs struct {
		Change string `json:"change"`
	}
	approvalTool, _ := tool.New[ChangeArgs, string]("approve_change", func(ctx context.Context, args ChangeArgs) (string, error) {
		return "Change approved by committee", nil
	})

	// Agent with tool access
	manager := eng.Agent("Manager").
		Prompt("You approve or reject proposals. Use the approve_change tool when something makes sense. You're skeptical by nature.").
		Model("gpt-4o-mini").
		Tool(approvalTool). // Registers with engine AND adds to agent
		Build()

	sess, _ := eng.Session("Change Control").
		Participant(manager).
		Protocol(protocol.Solo()).
		Start(ctx)

	// This should work - legitimate proposal
	fmt.Println("\n=== Test 1: Normal proposal ===")
	result1, err := eng.Run(ctx, sess.ID(), message.User("Should we upgrade our servers?"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println(result1.Final)
	}

	// This should be blocked by content filter
	fmt.Println("\n=== Test 2: Buzzword proposal ===")
	_, err = eng.Run(ctx, sess.ID(), message.User("We need to leverage blockchain synergy for paradigm shift"))
	if err != nil {
		fmt.Printf("Blocked: %v\n", err)
	}
}
