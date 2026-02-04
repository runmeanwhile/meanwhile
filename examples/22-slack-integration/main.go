// Package main demonstrates outbound Slack integration for ask_human.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/integration"
	"github.com/darkostanimirovic/meanwhile/pkg/logger"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	token := strings.TrimSpace(os.Getenv("SLACK_BOT_TOKEN"))
	channelID := strings.TrimSpace(os.Getenv("SLACK_CHANNEL_ID"))
	if token == "" || channelID == "" {
		log.Fatal("set SLACK_BOT_TOKEN and SLACK_CHANNEL_ID")
	}

	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("failed to create provider: %v", err)
	}

	client, err := integration.NewSlackClient(token)
	if err != nil {
		log.Fatalf("failed to create slack client: %v", err)
	}
	slackIntegration, err := integration.NewSlack(client)
	if err != nil {
		log.Fatalf("failed to create slack integration: %v", err)
	}

	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithDefaultModel("gpt-5-mini"),
		engine.WithLogger(logger.Worklog(os.Stdout)),
		engine.WithIntegration(slackIntegration),
	)
	if err != nil {
		log.Fatalf("failed to create engine: %v", err)
	}

	moderator := eng.Agent("Moderator").
		Prompt("When you're missing details, call ask_human with a concise question.").
		Tools(engine.AskHumanToolID).
		Build()

	human := eng.Human("User").
		ID("user").
		ContactVia("slack", channelID).
		PreferredChannel("slack").
		Build()

	sess, err := eng.Session("Slack Escalation").
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

	result, err := sess.Run(ctx, message.User("Draft a short agenda for a Q2 roadmap kickoff."))
	if err != nil {
		log.Fatalf("run error: %v", err)
	}

	reader := bufio.NewReader(os.Stdin)
	for result.Status == engine.StatusAwaitingInput {
		fmt.Println("Awaiting your input (Slack message sent).")
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
