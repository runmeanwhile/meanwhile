package main

import (
	"context"
	"fmt"
	"log"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	// Setup
	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("Failed to create OpenAI provider: %v", err)
	}
	eng, err := engine.New(engine.WithProvider(provider))
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

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
	sess, err := eng.Session("Escalation").
		Protocol(protocol.Handoff(receptionist, specialist)).
		Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	// Run through the chain
	result, err := eng.Run(ctx, sess.ID(), message.User("I deleted the production database. Was that bad?"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Final Response ===")
	fmt.Println(result.Final)
}
