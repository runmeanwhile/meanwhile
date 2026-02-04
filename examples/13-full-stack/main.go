package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/protocol/consensus"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// ToolArgs for various workplace tools
type IncidentArgs struct {
	Severity    string `json:"severity" description:"Severity: low, medium, high, critical"`
	Description string `json:"description" description:"Incident description"`
}

// SecurityHook validates operations
type SecurityHook struct{}

func (h *SecurityHook) ID() string    { return "security" }
func (h *SecurityHook) Priority() int { return 100 }

func (h *SecurityHook) OnPreToolUse(ctx context.Context, meta hook.SessionMeta, call tool.Call) (hook.Decision, tool.Call, error) {
	fmt.Printf("[SECURITY] Validating tool use: %s\n", call.ToolID)
	return hook.Allow, call, nil
}

func main() {
	ctx := context.Background()

	// Full stack: provider, memory, hooks, logging
	provider, _ := openai.FromEnv()
	memStore := memory.NewInMemoryStore()
	hookReg := hook.NewRegistry()
	hookReg.Register(&SecurityHook{})

	eng, _ := engine.New(
		engine.WithProvider(provider),
		engine.WithMemoryStore(memStore),
		engine.WithHookRegistry(hookReg),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)

	// Create tool
	incidentTool, _ := tool.New[IncidentArgs, string]("log_incident", func(ctx context.Context, args IncidentArgs) (string, error) {
		return fmt.Sprintf("Incident logged: [%s] %s", args.Severity, args.Description), nil
	})

	// Create specialist agents
	oncall := eng.Agent("On-Call Engineer").
		Prompt("You're on-call. You respond to incidents. You log everything. You've been woken up at 3am too many times.").
		Model("gpt-4o-mini").
		Tool(incidentTool). // Registers with engine AND adds to agent
		Build()

	sre := eng.Agent("SRE").
		Prompt("You're SRE. You analyze patterns and suggest improvements. You remember the incident from Q2 that nobody wants to talk about.").
		Model("gpt-4o-mini").
		Build()

	manager := eng.Agent("Engineering Manager").
		Prompt("You synthesize technical findings into status updates. You translate 'the database is on fire' into 'experiencing elevated error rates.'").
		Model("gpt-4o-mini").
		Build()

	// Protocol tool: wrap consensus as a tool
	postmortemTool := eng.AsTool(
		consensus.Consensus(),
		engine.WithToolName("conduct_postmortem"),
		engine.WithToolDescription("Conduct incident postmortem with team"),
		engine.WithToolParticipants(oncall, sre),
		engine.WithToolFacilitator(manager),
	)
	eng.ToolRegistry().Register(postmortemTool)

	// Incident commander with access to all tools
	commander := eng.Agent("Incident Commander").
		Prompt("You coordinate incident response. You log incidents, coordinate team response, and run postmortems. You stay calm because panic doesn't fix production.").
		Model("gpt-4o-mini").
		Tools("log_incident", "conduct_postmortem").
		Build()

	// Run incident through the system
	sess, _ := eng.Session("INC-2001-001").
		Participant(commander).
		Protocol(protocol.Solo()).
		Tags("incident", "production").
		Metadata("severity", "high").
		Start(ctx)

	result, err := eng.Run(ctx, sess.ID(),
		message.User("Alert: API response time exceeded 30s. Error rate jumped to 15%. Database CPU at 95%. Customer reports are coming in."))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Incident Response ===")
	fmt.Println(result.Final)

	// Show memory persistence
	query := memory.Query{SessionID: sess.ID()}
	items, _ := memStore.Query(ctx, query)
	fmt.Printf("\n=== Audit Trail ===")
	fmt.Printf("\nRecorded %d events for session %s\n", len(items), sess.ID())
	fmt.Printf("Metadata: %v\n", result.Metadata)
}
