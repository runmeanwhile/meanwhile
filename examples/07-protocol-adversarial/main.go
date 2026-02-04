package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	// Two agents taking opposing positions
	optimist := eng.Agent("Optimist").
		Prompt("You argue the positive case with enthusiasm. You believe in synergy, paradigm shifts, and that this quarter will be different.").
		Model("gpt-4o-mini").
		Build()

	pessimist := eng.Agent("Pessimist").
		Prompt("You argue the negative case with data. You've seen three enterprise software rollouts fail. You know how this ends.").
		Model("gpt-4o-mini").
		Build()

	// Optional: facilitator to synthesize
	judge := eng.Agent("Judge").
		Prompt("You synthesize both arguments into a balanced assessment. You've learned that the truth is usually somewhere in the middle, leaning pessimist.").
		Model("gpt-4o-mini").
		Build()

	// Adversarial debate protocol
	sess, _ := eng.Session("Architecture Review").
		Participants(optimist, pessimist).
		Facilitator(judge).
		Protocol(protocol.Debate()).
		Start(ctx)

	result, err := eng.Run(ctx, sess.ID(), message.User("Should we migrate everything to microservices?"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Synthesis ===")
	fmt.Println(result.Final)
}
