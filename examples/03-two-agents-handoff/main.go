package main

import (
	"context"
	"fmt"
	"log"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	// Setup
	provider, _ := openai.FromEnv()
	eng, _ := engine.New(engine.WithProvider(provider))

	// Two agents with distinct roles
	receptionist := eng.Agent("Reception").
		Prompt("You're front desk. Triage incoming requests and pass them along. No technical work—that's not your job description.").
		Model("gpt-4o-mini").
		Build()

	specialist := eng.Agent("Specialist").
		Prompt("You're the specialist who actually fixes things. You get escalations from reception. Provide technical solutions with weary expertise.").
		Model("gpt-4o-mini").
		Build()

	// Create session with handoff protocol
	sess, _ := eng.Session("Escalation").
		Protocol(protocol.Handoff(receptionist, specialist)).
		Start(ctx)

	// Run through the chain
	result, err := eng.Run(ctx, sess.ID(), message.User("I deleted the production database. Was that bad?"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Final Response ===")
	fmt.Println(result.Final)
}
