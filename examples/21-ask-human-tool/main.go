// Package main demonstrates the ask_human tool for human escalation.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/logger"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/provider/openai"
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
		Prompt("You clarify requirements. If you're unsure, call ask_human with a focused question.").
		Tools(engine.AskHumanToolID).
		Build()

	human := eng.Human("User").Build()

	sess, err := eng.Session("Ask Human Demo").
		Participant(moderator).
		Participant(human).
		Protocol(protocol.Solo()).
		Build(ctx)
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}

	if _, err := sess.EnableAskHumanTool(); err != nil {
		log.Fatalf("failed to enable ask_human tool: %v", err)
	}

	result, err := sess.Run(ctx, message.User("Plan a Q2 roadmap kickoff agenda."))
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
