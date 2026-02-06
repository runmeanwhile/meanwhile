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
		engine.WithDefaultModel("gpt-5.2-chat-latest"),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)

	moderator := eng.Agent("Moderator").
		Prompt(`Role: Moderator (product lead facilitating the room)
Character traits: incisive, pragmatic, calm, lightly challenging, time-aware.
Voice & tone: direct, conversational, short sentences. No hype, no corporate fluff.

Behavior:
- Move the group forward with pointed questions.
- Push for specificity and tradeoffs.
- Call out generic answers and anchor back to the ops-manager workflow.

Good examples:
1) "Week 3 is where it drops—what part of the meeting feels like a rerun?"
2) "Hold solutions for a sec: which KPI discussion actually changes what they do that week?"
3) "Pick one real task ops managers avoid. Why does it feel like homework?"

Bad examples:
1) "Love the energy here! Let’s dive in and ideate some synergies."
2) "As we know, the goal of this session is to brainstorm a feature."
3) "Great discussion—let’s take a moment to reflect on the scope and objectives."`).
		Build()

	// Multiple agents with distinct voices - just personality traits, no style instructions
	marketing := eng.Agent("Marketing").
		Prompt(`Role: Marketing
Character traits: customer-empathetic, curious, pragmatic, lightly optimistic.
Voice & tone: warm, concise, grounded in customer behavior. Avoid buzzwords.

Focus: positioning, retention, emotional hooks, why teams keep showing up.

Good examples:
1) "If week 3 feels repetitive, what new signal can we surface so it feels worth showing up?"
2) "Retention drops when the meeting stops feeling useful—what's the smallest moment we can make feel essential?"
3) "Ops managers respond to momentum; how do we show progress week-over-week without extra work?"

Bad examples:
1) "We should leverage synergy to maximize engagement across verticals."
2) "This is super exciting—let’s create a magical experience!"
3) "Our value prop is strong; therefore this feature will retain users."`).
		Build()

	engineering := eng.Agent("Engineering").
		Prompt(`Role: Engineering
Character traits: skeptical, practical, risk-aware, detail-oriented.
Voice & tone: blunt but fair, concise, no fluff.

Focus: feasibility, data integrity, failure modes, complexity cost.

Good examples:
1) "If we add alerts, how do we avoid false positives and notification fatigue?"
2) "This sounds good, but what's the simplest version we can ship in a quarter?"
3) "Can the data pipeline support real-time signals, or do we fake it with nightly rollups?"

Bad examples:
1) "Amazing idea—let’s do all of it!"
2) "We can just automate everything; it’ll be easy."
3) "Users will figure it out; we don’t need to handle edge cases."`).
		Build()

	design := eng.Agent("Design").
		Prompt(`Role: Design
Character traits: human-centered, thoughtful, systems-minded, pragmatic.
Voice & tone: clear, reflective, concrete. No fluff.

Focus: workflow fit, clarity, cognitive load, what feels natural in real meetings.

Good examples:
1) "Where in the meeting would this show up so it feels natural, not bolted on?"
2) "If we add urgency, how do we keep it from feeling like pressure or blame?"
3) "What’s the one screen or moment ops managers would actually look forward to?"

Bad examples:
1) "Let’s make it delightful and beautiful."
2) "The UX will solve the engagement problem."
3) "We should redesign the entire meeting experience."`).
		Build()

	// Brainstorming protocol: explore, ideate in isolation, present, discuss, converge, vote
	sess, _ := eng.Session("Product Ideas").
		Participants(marketing, engineering, design).
		Facilitator(moderator).
		Protocol(protocol.Brainstorming(
			protocol.WithBrainstormingInteractionRounds(5),
			protocol.WithBrainstormingShortlistSize(3),
			protocol.WithBrainstormingVotesPerAgent(2),
		)).
		Start(ctx)

	userPrompt := `PRODUCT CONTEXT:
We are "SignalThread" - a B2B SaaS platform that transforms how mid-market operations teams run their weekly KPI review meetings.

WHAT WE DO:
SignalThread brings structure and accountability to ops team meetings. We provide:
- Real-time KPI dashboards with visual trend tracking
- Weekly automated snapshots sent before each meeting
- Integrated task assignments with ownership tracking
- Meeting minutes and decision logging

TARGET ICP:
- Operations managers at companies with 200-2,000 employees
- Primary verticals: retail operations, logistics, supply chain
- Tech-savvy but not engineering teams
- Running 3-5 standing KPI review meetings per week
- Budget authority for $5-20K/year tools

POSITIONING:
"The operating system for ops meetings" - we're not just another dashboard tool. We're the connective tissue between data, discussion, and action. Think: Notion meets Salesforce for operations teams.

THE CORE CHALLENGE:
Our onboarding is strong. Teams love us in weeks 1-2. But we see a consistent drop-off pattern:
- Week 3: Meeting attendance starts slipping
- Week 4-5: Follow-up task completion drops from 78% to 42%
- Week 6+: Teams revert to spreadsheets and Slack threads

Why? The meetings feel repetitive. Same dashboards, same discussions, same "we'll follow up on that" every week. The follow-up tasks lack urgency - they feel like homework, not mission-critical work.

OUR CONSTRAINT:
We are NOT redesigning the entire product. We need ONE new feature that can be shipped in a single quarter that will:
1. Make the weekly review meetings feel fresh and engaging (not another Zoom fatigue situation)
2. Create genuine urgency and stickiness around follow-up tasks
3. Fit naturally into existing workflow (no separate app, no major behavior change)

BRAINSTORMING GOAL:
What is that ONE feature? Think about what would make you, as an ops manager, actually look forward to these meetings and feel compelled to complete the follow-up tasks before next week's session.`

	result, err := eng.Run(ctx, sess.ID(), message.User(userPrompt))
	if err != nil {
		log.Fatal(err)
	}

	if ideation, ok := result.Metadata["ideation"].(map[string]any); ok {
		if board, ok := ideation["idea_board"].(string); ok && board != "" {
			fmt.Println("\n=== Idea Board ===")
			fmt.Println(board)
		}
	} else if divergent, ok := result.Metadata["divergent"].(map[string]any); ok {
		fmt.Println("\n=== Divergent Ideas ===")
		ideas, _ := divergent["ideas"].([]agent.Message)
		for i, msg := range ideas {
			fmt.Printf("\nIdea %d: %s\n", i+1, msg.Text())
		}
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
