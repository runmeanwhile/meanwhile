package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	eng, _ := engine.New(
		engine.WithProvider(provider),
		engine.WithDefaultModel("gpt-4o-mini"),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)

	moderator := eng.Agent("Moderator").
		Prompt("You are the brainstorm moderator. Keep the session lively, focused, and grounded in the user's question. Be crisp and energetic. No emojis.").
		Build()

	// Multiple agents with distinct voices
	marketing := eng.Agent("Marketing").
		Prompt("You're marketing. You think everything needs better positioning and stickiness. Circa 2001, when 'webinar' was still edgy terminology. You are enthusiastic, optimistic, and persuasive. Vary your length: sometimes a sharp one‑liner, sometimes a short paragraph.").
		Build()

	engineering := eng.Agent("Engineering").
		Prompt("You're engineering. You think everything needs better architecture. You're tired of marketing's buzzwords. You are grumpy, cynical, and sarcastic. Keep it terse: 1–3 sentences max, unless you have a real technical constraint to explain.").
		Build()

	design := eng.Agent("Design").
		Prompt("You're design. You think everything needs better UX. You're tired of both engineering's jargon and marketing's enthusiasm. You are calm, thoughtful, and opinionated. Use concrete user scenarios, and allow 2–5 sentences if needed.").
		Build()

	// Brainstorming protocol: diverge, interact, then vote
	sess, _ := eng.Session("Product Ideas").
		Participants(marketing, engineering, design).
		Facilitator(moderator).
		Protocol(protocol.Brainstorming(
			protocol.WithBrainstormingInteractionRounds(2),
			protocol.WithBrainstormingVotesPerAgent(2),
		)).
		Start(ctx)

	userPrompt := `We are "SignalThread", a B2B SaaS that helps mid‑market operations teams run weekly KPI reviews.

ICP: Ops managers at 200–2000 employee companies in retail logistics.
Current product: dashboards, weekly KPI snapshots, and task assignments.
Problem: adoption drops after week 3 because the review meetings feel repetitive and the follow‑up tasks aren't sticky.

We are NOT redesigning the whole product. We want ONE new feature to improve engagement and follow‑through.

Brainstorm that feature.`

	result, err := eng.Run(ctx, sess.ID(), message.User(userPrompt))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Divergent Ideas ===")
	divergent, _ := result.Metadata["divergent"].(map[string]any)
	ideas, _ := divergent["ideas"].([]agent.Message)
	for i, msg := range ideas {
		fmt.Printf("\nIdea %d: %s\n", i+1, msg.Text())
	}

	if shortlist, ok := result.Metadata["shortlist"].([]string); ok && len(shortlist) > 0 {
		fmt.Println("\n=== Shortlist ===")
		for i, idea := range shortlist {
			fmt.Printf("%d. %s\n", i+1, idea)
		}
	}

	if votes, ok := result.Metadata["votes"].(map[string]any); ok {
		if tally, ok := votes["tally"].([]protocol.VoteTally); ok && len(tally) > 0 {
			fmt.Println("\n=== Vote Tally ===")
			for _, item := range tally {
				fmt.Printf("%s (score %d, %d votes)\n", item.Idea, item.Score, item.Votes)
			}
		}
	}
}
