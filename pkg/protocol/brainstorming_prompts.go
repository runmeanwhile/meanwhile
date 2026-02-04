package protocol

import (
	"fmt"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
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
	ideaTarget := input.IdeaTarget
	if ideaTarget <= 0 {
		ideaTarget = defaultIdeaTarget
	}
	phase := fmt.Sprintf("Divergent ideation (round %d of %d)", input.Round, input.MaxRounds)
	if input.MaxRounds <= 1 {
		phase = "Divergent ideation"
	}

	return fmt.Sprintf(`%s

===

BRAINSTORMING: %s
Scope: %s

Generate %d distinct ideas. Quantity over quality. No judging yet.
- Keep each idea short: "Title: one-sentence description"
- One idea per line
- Stay inside the scope and answer the user's question
- If you already proposed something similar earlier, do NOT repeat it

Use your unique voice and point of view. Avoid generic corporate language.
Be lively and specific, but keep each idea under 20 words.`, input.BasePrompt, phase, input.Scope, ideaTarget)
}

func defaultInteractionPrompt(input InteractionPromptInput) string {
	verbosity := "Max 120 words. Short paragraphs."
	progress := 0.0
	if input.MaxRounds > 0 {
		progress = float64(input.Round) / float64(input.MaxRounds)
	}
	switch {
	case progress >= 0.8:
		verbosity = "Max 70 words. Tight responses."
	case progress >= 0.5:
		verbosity = "Max 90 words. Stay concise."
	}

	board := strings.TrimSpace(input.Board)
	if board == "" {
		board = "(No idea board available yet.)"
	}

	return fmt.Sprintf(`%s

===

BRAINSTORMING: INTERACTION (round %d of %d)
Scope: %s

Idea board:
%s

YOUR JOB:
- Respond directly to what others just said
- Build on strong ideas ("yes, and...") or challenge weak ones ("no, because...")
- Combine ideas into stronger hybrids
- Push toward 2-3 promising directions
- Keep it lively and real — no parallel monologues
- Keep your unique voice and perspective — don't mirror other participants
- Vary your tone and length; avoid templatey lists
- It's ok to answer with a single sentence if that's the right move
- If you have a strong take, be blunt
- Do NOT include <agent:...> tags (they are added automatically)

%s`, input.BasePrompt, input.Round, input.MaxRounds, input.Scope, board, verbosity)
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
	var sb strings.Builder

	fmt.Fprintf(&sb, "Round %d of %d\n\n", currentRound, maxRounds)
	if len(thread) > 0 {
		last := thread[len(thread)-1]
		sb.WriteString("WHAT WAS JUST SAID:\n")
		if last.Name != "" {
			fmt.Fprintf(&sb, "%s: %s\n\n", last.Name, last.Summary())
		} else {
			fmt.Fprintf(&sb, "%s\n\n", last.Summary())
		}

		recentCount := 3
		if len(thread) > 1 && len(thread) < recentCount+1 {
			recentCount = len(thread) - 1
		}
		if recentCount > 0 && len(thread) > 1 {
			sb.WriteString("RECENT CONTEXT:\n")
			startIdx := len(thread) - recentCount - 1
			if startIdx < 0 {
				startIdx = 0
			}
			for i := startIdx; i < len(thread)-1; i++ {
				msg := thread[i]
				if msg.Name != "" {
					fmt.Fprintf(&sb, "- %s: %s\n", msg.Name, truncateForContext(msg.Summary(), 100))
				}
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("Respond directly to the most recent points. Build or challenge them.")

	return agent.Message{
		Role:  agent.RoleUser,
		Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: sb.String()}},
	}
}

func defaultBrainstormScopeRefinementPrompt(userQuestion, configuredScope string) (string, string) {
	system := `You are a brainstorming moderator. Your job is to clarify the question and define the brainstorming scope so the group stays focused and productive.

Produce a short moderator message that:
1) Restates the core question in clear language
2) Defines what is in scope vs out of scope
3) Specifies the type of ideas needed (strategy, product, process, etc.)
4) Keeps the scope tight enough to be actionable

Write in plain text, no markdown. Keep it under 600 characters.`

	user := fmt.Sprintf(`User question: %s
Configured scope: %s

Write the moderator's scope-setting message for the brainstorm.`, userQuestion, configuredScope)

	return system, user
}

func defaultBrainstormScopeFallback(userQuestion, configuredScope string) string {
	if strings.TrimSpace(configuredScope) != "" {
		return configuredScope
	}
	return userQuestion
}

func defaultModeratorOpeningPrompt(input ModeratorOpeningInput) ModeratorPrompt {
	system := `You are the brainstorm moderator. Set a lively, focused tone and explain how the session will run.`

	user := fmt.Sprintf(`Scope: %s
Agenda: %s

Write a short opening message that:
- Welcomes the group
- Reminds them of the scope
- Explains the phases (diverge → interact → vote)
- Encourages "yes, and" plus healthy challenge
- Stays under 8 lines
- Plain text, no markdown headings, no emojis`, input.Scope, input.Agenda)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}

func defaultModeratorSynthesisPrompt(input ModeratorSynthesisInput) ModeratorPrompt {
	system := `You are the brainstorm moderator. Summarize the divergent ideas into a clean idea board with themes and candidate ideas.`

	user := fmt.Sprintf(`Scope: %s

Here are the raw ideas from participants:
%s

Create an idea board:
- Group ideas into 3-5 themes
- Under each theme list 1-3 concrete ideas
- End with a short "Candidates" section listing 5-8 ideas worth debating
- Keep it readable and punchy
- Use plain text labels, no markdown headings`, input.Scope, input.DivergentBrief)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}

func defaultModeratorInterjectionPrompt(input ModeratorInterjectionInput) ModeratorPrompt {
	urgency := "Early: encourage building and specific reactions."
	switch {
	case input.Progress >= 0.85:
		urgency = "Final stretch: push for top 2-3 directions and clear tradeoffs."
	case input.Progress >= 0.6:
		urgency = "Midway: prune weak ideas and merge strong ones."
	}

	recent := strings.Join(input.Recent, "\n")
	if strings.TrimSpace(recent) == "" {
		recent = "(No recent messages captured.)"
	}

	system := `You are the brainstorm moderator. Interject to keep energy high, focus tight, and progress moving.`
	user := fmt.Sprintf(`Scope: %s
Progress: round %d of %d (%.0f%%)
Urgency: %s

Idea board:
%s

Recent messages:
%s

Write a 2-4 sentence interjection that:
- Calls out promising ideas
- Challenges weak or vague ones
- Asks a focused question or sets a mini-goal
- Use natural language, vary tone, no emojis`, input.Scope, input.CurrentRound, input.MaxRounds, input.Progress*100, urgency, input.Board, recent)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}

func defaultModeratorShortlistPrompt(input ModeratorShortlistInput) ModeratorPrompt {
	thread := strings.Join(input.Thread, "\n")
	if strings.TrimSpace(thread) == "" {
		thread = "(No discussion thread captured.)"
	}

	system := `You are the brainstorm moderator. Select the most promising ideas for a shortlist.`
	user := fmt.Sprintf(`Scope: %s
Idea board:
%s

Discussion highlights:
%s

Create a shortlist of up to %d idea titles. Output one idea per line, no extra commentary.`, input.Scope, input.Board, thread, input.Limit)

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

	system := `You are the brainstorm moderator. Close the session with a decisive, energetic summary.`
	user := fmt.Sprintf(`Scope: %s
Idea board:
%s

Shortlist:
%s

Vote tally:
%s

Write a closing summary that:
- Names the top 2-3 ideas
- Gives a brief rationale
- Calls out any open questions
- Suggests a next step
Keep it under 10 lines. Plain text only, no emojis.`, input.Scope, input.Board, shortlist, tallyText)

	return ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}
