// Package main demonstrates the consensus protocol with multiple agents
// reaching agreement through structured round-robin discussion.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/logger"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol/consensus"
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

	// Three agents with different perspectives
	development := eng.Agent("Development").
		Prompt(`You're a senior software developer with years of experience building and shipping products.

What you value:
- Team velocity and avoiding unnecessary friction
- Developer experience and morale
- Pragmatic solutions over perfect ones
- Balancing speed with quality

What frustrates you:
- Excessive process and bureaucracy
- Being blocked by approval chains
- Solutions that sound good on paper but don't work in practice
- Shipping features that no one uses

You're direct, practical, and protective of your team's ability to ship.`).
		Build()

	operations := eng.Agent("Operations").
		Prompt(`You're an operations lead responsible for keeping systems running and reliable.

What you value:
- System stability and uptime
- Predictability and risk management
- Having proper support and coverage
- Sustainable on-call practices

What frustrates you:
- Avoidable incidents and firefighting
- Decisions made without considering operational impact
- Being understaffed or under-supported
- Plans that ignore production realities

You're direct, experienced, and focused on what actually works in production.`).
		Build()

	security := eng.Agent("Security").
		Prompt(`You're a security professional who believes in practical, effective security.

What you value:
- Protecting users and data
- Security measures that actually get followed
- Being collaborative rather than obstructionist
- Real risk assessment over checkbox compliance

What frustrates you:
- Being bypassed because you're seen as "the no person"
- Security theater that doesn't actually protect anything
- Being blamed when preventable issues occur
- Vague appeals to "security" without specific reasoning

You've learned that being pragmatic and specific gets better results than being rigid.
You explain your concerns clearly and work to find solutions that balance security with practicality.`).
		Build()

	// Moderator facilitates discussion
	moderator := eng.Agent("Moderator").
		Prompt(`You're facilitating this discussion. Your job:
- Keep things moving without being pushy
- Highlight when people are talking past each other
- Ask clarifying questions when positions are vague
- Point out when someone made a concrete ask of someone else
- Nudge toward decisions when it's time

Don't be robotic. Say things like "I'm hearing X from Ops and Y from Dev - can we bridge that?"
Or "Security, you mentioned needing Z - Dev, can you work with that?"`).
		Build()

	// Create consensus protocol session
	sess, err := eng.Session("Friday Deployment Policy").
		Participants(development, operations, security).
		Facilitator(moderator).
		Protocol(consensus.Consensus(
			consensus.WithMaxRounds(5), // Short budget forces brevity
			consensus.WithScope("Arrive at a draft of a policy we can add to our Engineering Handbook. Doesn't need the smallest detail but should demonstrate direction and cover major concerns from all sides."),
			consensus.WithModeratorInterventions(0.4, 0.7, 0.9),
		)).
		Start(ctx)
	if err != nil {
		log.Fatalf("failed to start session: %v", err)
	}

	result, err := eng.Run(ctx, sess.ID(), message.User("Should we allow prod releases on Fridays, or make Friday a no-deploy day?"))
	if err != nil {
		log.Fatal(err)
	}

	// Extract structured consensus result
	consensusResult := extractConsensusResult(result)

	// Display results
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CONSENSUS OUTCOME")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("\nState: %s\n", consensusResult.State)
	fmt.Printf("Rounds Used: %d/%d\n", consensusResult.RoundsUsed, consensusResult.MaxRounds)

	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("QUICK REASONING")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println(consensusResult.Reasoning)

	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("AGENT POSITIONS")
	fmt.Println(strings.Repeat("-", 60))
	for _, pos := range consensusResult.Positions {
		fmt.Printf("\n%s: %s\n", pos.Agent, pos.Position)
		if pos.Reasoning != "" {
			fmt.Printf("  Reasoning: %s\n", truncate(pos.Reasoning, 200))
		}
		if len(pos.Conditions) > 0 {
			fmt.Println("  Conditions:")
			for _, cond := range pos.Conditions {
				fmt.Printf("    - %s\n", cond)
			}
		}
	}

	if len(consensusResult.BlockingIssues) > 0 {
		fmt.Println("\n" + strings.Repeat("-", 60))
		fmt.Println("BLOCKING ISSUES")
		fmt.Println(strings.Repeat("-", 60))
		for _, issue := range consensusResult.BlockingIssues {
			fmt.Printf("- %s\n", issue)
		}
	}

	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("FULL DISCUSSION THREAD")
	fmt.Println(strings.Repeat("-", 60))
	for i, msg := range consensusResult.MessageThread {
		if msg.Role == "user" {
			fmt.Printf("\n[%d] USER: %s\n", i+1, msg.Text())
		} else {
			fmt.Printf("\n[%d] %s: %s\n", i+1, msg.Name, truncate(msg.Text(), 300))
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
}

func extractConsensusResult(result *engine.RunResult) consensus.Result {
	// Extract from result.Metadata["consensus"]
	if result.Metadata == nil {
		return consensus.Result{}
	}

	consensusData, ok := result.Metadata["consensus"]
	if !ok {
		return consensus.Result{}
	}

	data, err := json.Marshal(consensusData)
	if err != nil {
		return consensus.Result{}
	}

	var cr consensus.Result
	if err := json.Unmarshal(data, &cr); err != nil {
		return consensus.Result{}
	}

	return cr
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
