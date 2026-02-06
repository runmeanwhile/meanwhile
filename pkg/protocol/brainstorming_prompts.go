package protocol

import (
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

// DivergentPromptBuilder builds a participant system prompt for divergent ideation.
type DivergentPromptBuilder func(input DivergentPromptInput) string

// InteractionPromptBuilder builds a participant system prompt for interactive discussion.
type InteractionPromptBuilder func(input InteractionPromptInput) string

// VotePromptBuilder builds a participant vote prompt.
type VotePromptBuilder func(input VotePromptInput) string

// ContextMessageBuilder builds a context message for participants.
type ContextMessageBuilder func(thread []agent.Message, currentRound, maxRounds int) agent.Message

// ModeratorPrompt defines input for moderator runs.
type ModeratorPrompt struct {
	System            string
	User              string
	MaxToolIterations int
}

// ModeratorOpeningBuilder builds a moderator opening prompt.
type ModeratorOpeningBuilder func(input ModeratorOpeningInput) ModeratorPrompt

// ModeratorSynthesisBuilder builds a moderator synthesis prompt.
type ModeratorSynthesisBuilder func(input ModeratorSynthesisInput) ModeratorPrompt

// ModeratorInterjectionBuilder builds a moderator interjection prompt.
type ModeratorInterjectionBuilder func(input ModeratorInterjectionInput) ModeratorPrompt

// ModeratorShortlistBuilder builds a moderator shortlist prompt.
type ModeratorShortlistBuilder func(input ModeratorShortlistInput) ModeratorPrompt

// ModeratorClosingBuilder builds a moderator closing prompt.
type ModeratorClosingBuilder func(input ModeratorClosingInput) ModeratorPrompt

// DivergentPromptInput provides context for divergent prompts.
type DivergentPromptInput struct {
	BasePrompt string
	Scope      string
	Round      int
	MaxRounds  int
	IdeaTarget int
}

// InteractionPromptInput provides context for interaction prompts.
type InteractionPromptInput struct {
	BasePrompt string
	Scope      string
	Board      string
	Round      int
	MaxRounds  int
	TurnIndex  int
	Speakers   int
	Move       string
}

// VotePromptInput provides context for vote prompts.
type VotePromptInput struct {
	BasePrompt string
	Scope      string
	Shortlist  []string
	Picks      int
}

// ModeratorOpeningInput provides context for the opening message.
type ModeratorOpeningInput struct {
	Scope  string
	Agenda string
	Seed   string
}

// ModeratorSynthesisInput provides context for idea synthesis.
type ModeratorSynthesisInput struct {
	Scope          string
	DivergentBrief string
}

// ModeratorInterjectionInput provides context for interjections.
type ModeratorInterjectionInput struct {
	Scope        string
	Board        string
	Progress     float64
	CurrentRound int
	MaxRounds    int
	Recent       []string
}

// ModeratorShortlistInput provides context for shortlist building.
type ModeratorShortlistInput struct {
	Scope  string
	Board  string
	Thread []string
	Limit  int
}

// ModeratorClosingInput provides context for the closing summary.
type ModeratorClosingInput struct {
	Scope     string
	Board     string
	Shortlist []string
	Tally     []VoteTally
}

func defaultDivergentPrompt(input DivergentPromptInput) string {
	phase := ""
	if input.MaxRounds > 1 {
		phase = fmt.Sprintf(" (round %d of %d)", input.Round, input.MaxRounds)
	}

	ideaTarget := input.IdeaTarget
	if ideaTarget <= 0 {
		ideaTarget = defaultIdeaTarget
	}

	return fmt.Sprintf(`DIVERGENT IDEATION%s
Focus: %s

Generate %d distinct ideas. One per line. Short and specific.
Do not write paragraphs. No headings, no markdown, no extra commentary.`, phase, input.Scope, ideaTarget)
}

func defaultInteractionPrompt(input InteractionPromptInput) string {
	progress := 0.0
	if input.MaxRounds > 0 {
		progress = float64(input.Round) / float64(input.MaxRounds)
	}

	pacing := ""
	switch {
	case progress >= 0.8:
		pacing = "Final stretch: converge on the top 2-3 ideas and call out tradeoffs."
	case progress >= 0.5:
		pacing = "Midway: pressure-test and sharpen the best ideas."
	default:
		pacing = "Early: explore reactions, questions, and extensions."
	}

	board := strings.TrimSpace(input.Board)
	if board == "" {
		board = "(No idea board available yet.)"
	}
	move := strings.TrimSpace(input.Move)
	if move == "" {
		move = "build or challenge one specific point from the recent thread"
	}

	return fmt.Sprintf(`BRAINSTORMING DISCUSSION (round %d of %d)
Problem: %s

Current idea board:
%s

%s
Turn cue (%d/%d): %s

React to something specific someone just said. Keep it short (1-3 sentences).
Prefer the turn cue above, but stay natural if a different move fits better.
If you can, add one concrete detail, risk, metric, or example. Avoid generic praise.
Do not summarize the whole thread. Most turns should be declarative; use a question only when it unlocks a concrete next action.
Stay in your own voice. Keep it conversational (contractions are fine). No speaker labels or lists. If you see <agent:...> tags in the transcript, ignore them and never repeat them.`, input.Round, input.MaxRounds, input.Scope, board, pacing, input.TurnIndex, input.Speakers, move)
}

func defaultVotePrompt(input VotePromptInput) string {
	shortlist := ""
	for _, idea := range input.Shortlist {
		shortlist += fmt.Sprintf("- %s\n", idea)
	}
	shortlist = strings.TrimSpace(shortlist)

	return fmt.Sprintf(`%s

Pick your top %d ideas from the shortlist below.

Rules:
- Choose only from the shortlist
- Rank your picks in order of strength
- Output JSON ONLY (no markdown) in this exact shape:
  {"picks":["Idea A","Idea B"],"rationale":"short reason"}

Shortlist:
%s`, input.BasePrompt, input.Picks, shortlist)
}

func defaultBrainstormingContextMessage(thread []agent.Message, currentRound, maxRounds int) agent.Message {
	if len(thread) == 0 {
		return agent.Message{
			Role:  agent.RoleUser,
			Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: fmt.Sprintf("Round %d of %d - jump in with a short response", currentRound, maxRounds)}},
		}
	}

	// Just show who said what recently - let the agent figure out how to respond
	var sb strings.Builder

	// Show the immediate last 2-3 messages as context
	recentCount := 2
	if len(thread) < recentCount {
		recentCount = len(thread)
	}

	startIdx := len(thread) - recentCount
	for i := startIdx; i < len(thread); i++ {
		msg := thread[i]
		if msg.Name != "" {
			fmt.Fprintf(&sb, "%s: %s\n", msg.Name, truncateForContext(msg.Summary(), 150))
		}
	}

	prefix := fmt.Sprintf("Recent (%d/%d):\n", currentRound, maxRounds)
	return agent.Message{
		Role:  agent.RoleUser,
		Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: strings.TrimSpace(prefix + sb.String())}},
	}
}

func defaultBrainstormScopeRefinementPrompt(userQuestion, configuredScope string) (string, string) {
	system := `You are a brainstorming moderator. Your job is to define the scope clearly and tightly.

Output ONLY a concise scope statement (1-2 sentences, descriptive not imperative). No greeting, no bullet points, no extra commentary.
Avoid formulaic phrases like "the goal/objective/scope of this brainstorming session". Keep the total under 300 characters.`

	user := fmt.Sprintf(`User question: %s
Configured scope: %s

Write the scope statement for the brainstorm.`, userQuestion, configuredScope)

	return system, user
}

func defaultBrainstormScopeFallback(userQuestion, configuredScope string) string {
	if strings.TrimSpace(configuredScope) != "" {
		return configuredScope
	}
	return userQuestion
}

func defaultModeratorOpeningPrompt(input ModeratorOpeningInput) ModeratorPrompt {
	system := `You are the brainstorm moderator. You're a product lead in the room, not a scripted facilitator. Keep it natural and concise.`

	user := fmt.Sprintf(`Brief: %s

Kick off the brainstorm like a product lead in the room. Start directly (no formal welcome) with a concrete observation from the brief and ask a diagnostic question.
Invite quick reactions or questions and make it clear we are not jumping into solutions yet.
Don't restate the goal or scope. Avoid meta-praise or recaps. Keep it conversational, 2-3 sentences. No bullet points or markdown.`, input.Seed)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}

func defaultModeratorSynthesisPrompt(input ModeratorSynthesisInput) ModeratorPrompt {
	system := `You are the brainstorm moderator. You've been listening to the team explore ideas. Now you're helping them see what's emerging - patterns, tensions, promising threads. Talk to them like a colleague making sense of a messy whiteboard together.`

	user := fmt.Sprintf(`Scope: %s

Here's what the team has been exploring:
%s

Reflect back what you're hearing. What patterns or tensions do you notice? What threads seem worth pulling on? What questions should we dig into next?

Talk to the team conversationally - no bullet points, no numbered lists, no markdown. Just share your observations and set up the next phase of discussion.`, input.Scope, input.DivergentBrief)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}

func defaultModeratorInterjectionPrompt(input ModeratorInterjectionInput) ModeratorPrompt {
	urgency := "We're early - let's encourage building on ideas and getting specific reactions."
	switch {
	case input.Progress >= 0.85:
		urgency = "We're in the home stretch - converge on the top 2-3 directions and make tradeoffs explicit."
	case input.Progress >= 0.6:
		urgency = "We're about halfway - prune what is weak and sharpen what is strong."
	}

	recent := strings.Join(input.Recent, "\n")
	if strings.TrimSpace(recent) == "" {
		recent = "(No recent messages captured.)"
	}

	system := `You are the brainstorm moderator. Jump in to keep things moving productively. You're a colleague helping guide the conversation - warm but direct.`

	user := fmt.Sprintf(`Scope: %s
Progress: round %d of %d (%.0f%%)
%s

Idea board so far:
%s

What just happened:
%s

Jump in with a quick interjection (2-3 sentences) that:
- Calls out what is working or what needs push-back
- Adds one concrete push: either one focused question OR one explicit constraint for the next turns
- Keeps momentum going
- Sounds natural, like you are in the room
- No bullet points, no emojis, no "great discussion"/"love the energy" filler`, input.Scope, input.CurrentRound, input.MaxRounds, input.Progress*100, urgency, input.Board, recent)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}

func defaultModeratorShortlistPrompt(input ModeratorShortlistInput) ModeratorPrompt {
	thread := strings.Join(input.Thread, "\n")
	if strings.TrimSpace(thread) == "" {
		thread = "(No discussion thread captured.)"
	}

	system := `You are the brainstorm moderator. Select the most promising ideas for a shortlist and tee up a quick vote. Keep it crisp.`
	user := fmt.Sprintf(`Scope: %s
Idea board:
%s

Discussion highlights:
%s

Create a shortlist of up to %d idea titles. Output:
- One short sentence that signals we are converging and about to vote
- Then a numbered list (1., 2., 3.) with one idea per line
No extra commentary after the list.`, input.Scope, input.Board, thread, input.Limit)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}

func defaultModeratorClosingPrompt(input ModeratorClosingInput) ModeratorPrompt {
	var tallyLines []string
	for _, item := range input.Tally {
		tallyLines = append(tallyLines, fmt.Sprintf("- %s (score %d, %d votes)", item.Idea, item.Score, item.Votes))
	}
	tallyText := strings.Join(tallyLines, "\n")
	if strings.TrimSpace(tallyText) == "" {
		tallyText = "(No vote tally available.)"
	}

	shortlist := strings.Join(input.Shortlist, "\n- ")
	if strings.TrimSpace(shortlist) != "" {
		shortlist = "- " + shortlist
	}

	system := `You are the brainstorm moderator wrapping up the session. Produce a client-ready summary report.`

	user := fmt.Sprintf(`Scope: %s

Idea board:
%s

Shortlist:
%s

Vote results:
%s

Write a short report in markdown (no code fences) with these sections:
## Goal / Problem
## What We Explored (themes, not every idea)
## Finalists (top 2-3 with 1-2 sentence rationale each)
## Open Questions / Risks
## Recommended Next Step

Use the vote results if available. Keep it concise and client-facing.`, input.Scope, input.Board, shortlist, tallyText)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}
