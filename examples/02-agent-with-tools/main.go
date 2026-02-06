package main

import (
	"context"
	"fmt"
	"log"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// TicketArgs defines parameters for ticket creation
type TicketArgs struct {
	Issue    string `json:"issue" description:"Description of the issue"`
	Priority string `json:"priority" description:"Priority level: low, medium, high, critical"`
}

func main() {
	// Provider and engine
	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("Failed to create OpenAI provider: %v", err)
	}
	eng, err := engine.New(engine.WithProvider(provider))
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// Create typed tool
	ticketTool, err := tool.New[TicketArgs, string]("create_ticket", func(_ context.Context, args TicketArgs) (string, error) {
		return fmt.Sprintf("Ticket #%d created: %s [%s]", 1337, args.Issue, args.Priority), nil
	})
	if err != nil {
		log.Fatalf("Failed to create tool: %v", err)
	}

	// Register and use tool in one step
	result, err := eng.Agent("Helpdesk").
		Prompt("You are tier-1 support. For any actual problem, create a ticket. You've learned not to promise quick fixes.").
		Model("gpt-4o-mini").
		Tool(ticketTool). // Registers with engine AND adds to agent
		Run(message.User("The entire network is down and the CEO is screaming"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Text())
}
