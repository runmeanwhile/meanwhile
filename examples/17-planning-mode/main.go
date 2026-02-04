package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/darkostanimirovic/meanwhile/pkg/collab/planning"
	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/logger"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	// Setup engine
	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// Planning agent (uses more capable model)
	architect := eng.Agent("Architect").
		Prompt("You are a software architect. Create detailed, structured implementation plans. Be specific about steps and break down complex tasks.").
		Model("gpt-4o").
		Build()

	// Execution agent (uses faster model)
	developer := eng.Agent("Developer").
		Prompt("You are a developer following an implementation plan. Execute steps methodically and report what you did.").
		Model("gpt-4o-mini").
		Build()

	// Create planner with session storage
	planner := planning.New(
		architect,
		planning.WithStorage(planning.StorageSession, "plan"),
	)

	// Create session
	sess, err := eng.Session("Feature Development").
		Participant(architect).
		Participant(developer).
		Protocol(protocol.Solo()).
		Tags("planning", "development").
		Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	fmt.Println("\n=== PHASE 1: PLANNING ===")
	fmt.Println("Creating implementation plan...")

	// Phase 1: Create plan
	plan, err := planner.CreatePlan(ctx, sess, message.User(
		"Build a user authentication system with email/password login, session management, and password reset functionality",
	))
	if err != nil {
		log.Fatalf("Failed to create plan: %v", err)
	}

	fmt.Println(plan.Format())

	fmt.Println("\n=== PHASE 2: EXECUTION ===")
	fmt.Println("Executing plan steps...")

	// Phase 2: Execute using plan as context
	// Build context message with the plan
	planContext := fmt.Sprintf(
		"You are executing the following implementation plan:\n\n%s\n\nExecute each step and report your progress.",
		plan.Format(),
	)

	result, err := eng.Run(ctx, sess.ID(), message.System(planContext))
	if err != nil {
		log.Fatalf("Failed to execute plan: %v", err)
	}

	fmt.Println("\n=== EXECUTION RESULT ===")
	fmt.Println(result.Final)

	// Demonstrate retrieving the plan from session
	fmt.Println("\n=== PLAN RETRIEVAL ===")
	storedPlan, ok := planning.GetPlan(sess, "plan")
	if ok {
		fmt.Printf("Plan ID: %s\n", storedPlan.ID)
		fmt.Printf("Plan Title: %s\n", storedPlan.Title)
		fmt.Printf("Steps: %d\n", len(storedPlan.Steps))
	}
}
