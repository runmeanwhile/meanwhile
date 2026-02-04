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

	// Engine with workplace logging
	provider, _ := openai.FromEnv()
	eng, _ := engine.New(
		engine.WithProvider(provider),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)

	// Agent
	consultant := eng.Agent("Consultant").
		Prompt("You're a ISO consultant in late 1999. Everything is urgent. Everything is a potential disaster. Your billable hours depend on this panic.").
		Model("gpt-4o-mini").
		Build()

	// Session with metadata
	sess, _ := eng.Session("ISO Assessment").
		Participant(consultant).
		Protocol(protocol.Solo()).
		Tags("urgent", "billable").
		Metadata("client", "BigCorp").
		Start(ctx)

	// Run and inspect result
	result, err := eng.Run(ctx, sess.ID(), message.User("Should we be worried about our mainframe?"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Structured Result ===")
	fmt.Printf("Final: %s\n", result.Final)
	fmt.Printf("Transcript: %d messages\n", len(result.Transcript))
	fmt.Printf("Events: %d\n", len(result.Events))
	fmt.Printf("Tags: %v\n", result.Metadata["tags"])
}
