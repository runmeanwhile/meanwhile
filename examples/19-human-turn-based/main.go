// Package main demonstrates turn-based human participation in a protocol.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol/consensus"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("failed to create provider: %v", err)
	}

	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithDefaultModel("gpt-5-mini"),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)
	if err != nil {
		log.Fatalf("failed to create engine: %v", err)
	}

	moderator := eng.Agent("Moderator").
		Prompt("You refine briefs by asking concise, practical questions.").
		Build()

	analyst := eng.Agent("Analyst").
		Prompt("You surface edge cases and missing details.").
		Build()

	human := eng.Human("User").Build()

	sess, err := eng.Session("Brief Refinement").
		Participant(moderator).
		Participant(human).
		Participant(analyst).
		Participation(engine.TurnBased()).
		Protocol(consensus.Consensus()).
		Start(ctx)
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}

	result, err := sess.Run(ctx, message.User("Draft brief: build a dashboard for customer analytics."))
	if err != nil {
		log.Fatalf("run error: %v", err)
	}

	reader := bufio.NewReader(os.Stdin)
	for result.Status == engine.StatusAwaitingInput {
		fmt.Println("Awaiting your input.")
		if strings.TrimSpace(result.Context) != "" {
			fmt.Printf("\nContext:\n%s\n\n", result.Context)
		}
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("read input: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			line = "(no response)"
		}

		result, err = sess.Respond(ctx, result.RequestID, message.User(line))
		if err != nil {
			log.Fatalf("respond error: %v", err)
		}
	}

	fmt.Println("\nFinal:")
	fmt.Println(result.Final)
}
