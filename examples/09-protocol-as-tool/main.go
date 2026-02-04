package main

import (
	"fmt"
	"log"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/provider/openai"
)

func main() {
	provider, _ := openai.FromEnv()
	eng, _ := engine.New(engine.WithProvider(provider))

	// Specialist agents for escalation
	legal := eng.Agent("Legal").
		Prompt("You're legal counsel. You spot liability issues. You remember the lawsuit from '99 and won't let it happen again.").
		Model("gpt-4o-mini").
		Build()

	compliance := eng.Agent("Compliance").
		Prompt("You're compliance officer. You know every regulation. You have a binder full of audit findings from last quarter.").
		Model("gpt-4o-mini").
		Build()

	// Convert handoff protocol into callable tools
	legalEscalation := eng.AsTool(
		protocol.Handoff(legal, legal),
		engine.WithToolName("escalate_legal"),
		engine.WithToolDescription("Escalate legal concerns to counsel"),
		engine.WithToolParticipants(legal),
	)

	complianceReview := eng.AsTool(
		protocol.Handoff(compliance, compliance),
		engine.WithToolName("escalate_compliance"),
		engine.WithToolDescription("Escalate compliance issues"),
		engine.WithToolParticipants(compliance),
	)

	// Coordinator agent with access to protocol tools
	result, err := eng.Agent("Coordinator").
		Prompt("You're project coordinator. When you spot legal or compliance concerns, you escalate using the appropriate tool. You've learned not to wing it.").
		Model("gpt-4o-mini").
		Tool(legalEscalation).      // Registers AND adds to agent
		Tool(complianceReview).     // Registers AND adds to agent
		Run(message.User("We're planning to collect user data and share it with third parties to improve our services"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Coordinator Response ===")
	fmt.Println(result.Text())
}
