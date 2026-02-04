package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	provider, _ := openai.FromEnv()
	eng, _ := engine.New(
		engine.WithProvider(provider),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)

	// Create agents for breakout groups
	frontendLead := eng.Agent("Frontend Lead").
		Prompt("You lead frontend. You worry about browser compatibility and that IE6 issue nobody wants to fix.").
		Model("gpt-4o-mini").
		Build()

	backendLead := eng.Agent("Backend Lead").
		Prompt("You lead backend. You worry about database locks and that stored procedure from 1997 that nobody understands.").
		Model("gpt-4o-mini").
		Build()

	qaLead := eng.Agent("QA Lead").
		Prompt("You lead QA. You've found 47 critical bugs this sprint. Nobody listens. You document everything anyway.").
		Model("gpt-4o-mini").
		Build()

	// Project manager synthesizes findings
	pm := eng.Agent("PM").
		Prompt("You're the PM. You collect updates from breakout groups and synthesize them into status nobody wants to hear.").
		Model("gpt-4o-mini").
		Build()

	// Breakout protocol: groups work in parallel, then reconvene
	sess, _ := eng.Session("Sprint Retrospective").
		Protocol(protocol.Breakout(
			protocol.WithBreakoutGroups(map[string][]agent.Agent{
				"Frontend": {frontendLead},
				"Backend":  {backendLead},
				"QA":       {qaLead},
			}),
		)).
		Facilitator(pm).
		Start(ctx)

	result, err := eng.Run(ctx, sess.ID(), message.User("What went wrong this sprint and what are we doing about it?"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Retrospective Summary ===")
	fmt.Println(result.Final)
}
