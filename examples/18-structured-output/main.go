package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Plan represents a structured planning output.
type Plan struct {
	Title   string `json:"title" description:"Title of the plan"`
	Summary string `json:"summary" description:"Brief summary"`
	Steps   []Step `json:"steps" description:"Implementation steps"`
}

type Step struct {
	ID          string `json:"id" description:"Step identifier"`
	Title       string `json:"title" description:"Step title"`
	Description string `json:"description,omitempty" description:"Detailed description"`
}

// ContactInfo represents structured contact extraction.
type ContactInfo struct {
	Name  string `json:"name" description:"Full name"`
	Email string `json:"email" description:"Email address"`
	Phone string `json:"phone,omitempty" description:"Phone number"`
}

func main() {
	eng, err := engine.New()
	if err != nil {
		log.Fatalf("engine: %v", err)
	}

	fmt.Println("=== Meanwhile Structured Output Demo ===")
	fmt.Println()

	// Pattern 1: Agent-level output schema (all responses must conform)
	fmt.Println("1. AGENT-LEVEL SCHEMA: Extraction agent always returns ContactInfo")
	result, err := eng.Agent("Contact Extractor").
		Prompt("Extract contact information from the text.").
		Model("gpt-4o-mini").
		OutputSchema(ContactInfo{}). // Every response must be ContactInfo
		Run(message.User("Hi, I'm Alice Smith. Reach me at alice@example.com or 555-1234"))
	if err != nil {
		log.Fatalf("extraction: %v", err)
	}

	// Response is JSON text - parse it
	var contact ContactInfo
	if err := json.Unmarshal([]byte(result.Text()), &contact); err != nil {
		log.Printf("Warning: Could not parse as JSON: %v\n", err)
	} else {
		fmt.Printf("   Extracted: %s <%s> %s\n\n", contact.Name, contact.Email, contact.Phone)
	}

	// Pattern 2: Tool-based structured output (recommended for Meanwhile)
	fmt.Println("2. TOOL PATTERN: Agent calls submit_plan tool with structured data")

	submitPlanTool, err := tool.New("submit_plan", func(ctx context.Context, plan Plan) (string, error) {
		fmt.Printf("   📋 Plan received: %s\n", plan.Title)
		fmt.Printf("   📝 Summary: %s\n", plan.Summary)
		fmt.Printf("   🔢 Steps: %d\n", len(plan.Steps))
		for i, step := range plan.Steps {
			fmt.Printf("      %d. %s\n", i+1, step.Title)
		}
		return fmt.Sprintf("Plan '%s' with %d steps submitted successfully", plan.Title, len(plan.Steps)), nil
	})
	if err != nil {
		log.Fatalf("create tool: %v", err)
	}

	planningAgent := eng.Agent("Planner").
		Prompt("You are a planning specialist. When asked to create a plan, think through the requirements, then call submit_plan with a structured plan.").
		Model("gpt-4o-mini").
		Tool(submitPlanTool). // Registers with engine AND adds to agent
		Build()

	sess, err := eng.Session("planning-demo").
		Participant(planningAgent).
		Protocol(protocol.Solo()).
		Start(context.Background())
	if err != nil {
		log.Fatalf("session: %v", err)
	}

	result, err = sess.RunAgent(context.Background(), planningAgent, protocol.RunRequest{
		Messages: []agent.Message{message.User("Create a plan for building a REST API in Go")},
	})
	if err != nil {
		log.Fatalf("planning: %v", err)
	}

	fmt.Printf("\n   Agent response: %s\n\n", result.Text())

	// Pattern 3: RunRequest-level override
	fmt.Println("3. RUN-LEVEL OVERRIDE: Same agent, different output schema per call")

	type SimpleResponse struct {
		Answer string `json:"answer" description:"A brief answer"`
	}

	flexibleAgent := eng.Agent("Assistant").
		Prompt("You are a helpful assistant.").
		Model("gpt-4o-mini").
		Build()

	// First call: structured output
	sess2, err := eng.Session("flex-demo").
		Participant(flexibleAgent).
		Protocol(protocol.Solo()).
		Start(context.Background())
	if err != nil {
		log.Fatalf("session: %v", err)
	}

	result, err = sess2.RunAgent(context.Background(), flexibleAgent, protocol.RunRequest{
		Messages:     []agent.Message{message.User("What is 2+2?")},
		OutputSchema: SimpleResponse{}, // This call must return SimpleResponse
	})
	if err != nil {
		log.Fatalf("flexible run: %v", err)
	}

	var simple SimpleResponse
	if err := json.Unmarshal([]byte(result.Text()), &simple); err == nil {
		fmt.Printf("   Structured: %s\n", simple.Answer)
	}

	// Second call: free-form (no schema)
	result, err = sess2.RunAgent(context.Background(), flexibleAgent, protocol.RunRequest{
		Messages: []agent.Message{message.User("Tell me a joke")},
		// No OutputSchema - free-form response
	})
	if err != nil {
		log.Fatalf("flexible run: %v", err)
	}
	fmt.Printf("   Free-form: %s\n\n", result.Text())

	fmt.Println("=== Summary ===")
	fmt.Println("• Agent.OutputSchema: Best for single-purpose agents (extraction, classification)")
	fmt.Println("• Tool pattern: Best for Meanwhile (structured actions with type safety)")
	fmt.Println("• RunRequest.OutputSchema: Best for flexible agents with mixed output needs")
}
