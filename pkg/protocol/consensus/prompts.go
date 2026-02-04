package consensus

import (
	"fmt"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/chair"
)

// InterjectionInput captures context for a chair interjection.
type InterjectionInput struct {
	Progress       float64
	CurrentRound   int
	MaxRounds      int
	RecentMessages []string
	Positions      []AgentPosition
}

// ClosingSummaryInput captures context for a closing summary.
type ClosingSummaryInput struct {
	State          State
	RoundsUsed     int
	MaxRounds      int
	Positions      []AgentPosition
	Conditions     []string
	BlockingIssues []string
	RecentMessages []string
}

// buildAgentRefinementPrompt creates a refined prompt that encourages natural discussion.
func buildAgentRefinementPrompt(basePrompt, scope string, currentRound, maxRounds int) string {
	var verbosityGuidance string
	progress := float64(currentRound) / float64(maxRounds)

	switch {
	case progress < 0.3:
		verbosityGuidance = "Keep responses under 100 words. Short paragraphs only."
	case progress < 0.6:
		verbosityGuidance = "BREVITY CRITICAL: Max 75 words. One short paragraph."
	default:
		verbosityGuidance = "EXTREME BREVITY: Max 50 words. 2-3 sentences max. No lists."
	}

	return fmt.Sprintf(`%s

===

Discussion scope: %s

"""
%s
"""

YOU MUST RESPOND TO WHAT WAS JUST SAID:
- Read the most recent messages carefully
- If someone asked YOU a question (@[your name]), ANSWER IT directly
- If someone made a point, RESPOND to that specific point
- Don't just restate your position - BUILD ON THE CONVERSATION
- This is dialogue, not parallel monologues
- You can only demand or ask participants in the conversation, not the Moderator or any external entity (employees, services...)

MESSAGE FORMAT:
- Previous messages show who said what: <agent:[their name]>message</agent:[their name]>
- When you see @[your name], they're asking YOU specifically
- Reference what others said
- Your response will be automatically tagged with your name: <agent:[your name]>your message</agent:[your name]>

STAY FOCUSED:
- Focus on principles, boundaries, and constraints set by the Moderator
- Stay on the scope provided

HOW TO PARTICIPATE:
1. Adjust your verbosity based on the round
2. RESPOND to what was just said - don't ignore it
3. If asked a question, ANSWER it directly
4. Address others by name when appropriate
5. Don't ask questions that were already answered
6. Build on the discussion, don't restart it

WHEN TO SIGNAL YOUR POSITION:
- When you're confident about what is being proposed/decided, you must use the "signal_position" tool to formally state your position. Use Position argument exclusively with AGREE or DISAGREE values. Use Reasoning to explain your choice and Conditions to list any conditional requirements for your agreement.

Remember: This is a GROUP CONVERSATION. Listen and respond to each other in respect to your personality, voice and context.`, basePrompt, scope, verbosityGuidance)
}

func defaultContextMessage(thread []agent.Message, currentRound, maxRounds int) agent.Message {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Round %d of %d\n\n", currentRound, maxRounds)

	if len(thread) > 0 {
		lastMsg := thread[len(thread)-1]
		sb.WriteString("WHAT WAS JUST SAID:\n")
		if lastMsg.Name != "" {
			fmt.Fprintf(&sb, "%s just said: %s\n\n", lastMsg.Name, lastMsg.Summary())
		} else {
			fmt.Fprintf(&sb, "%s\n\n", lastMsg.Summary())
		}

		recentCount := 3
		if len(thread) > 1 && len(thread) < recentCount+1 {
			recentCount = len(thread) - 1
		}
		if recentCount > 0 && len(thread) > 1 {
			sb.WriteString("RECENT DISCUSSION:\n")
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

	sb.WriteString("Respond to what was just said.")

	return agent.Message{
		Role:  agent.RoleUser,
		Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: sb.String()}},
	}
}

func defaultScopeRefinementPrompt(userQuestion, configuredScope string) (string, string) {
	system := `You are helping to set clear boundaries for a consensus discussion.

Your job is to analyze the user's question and the configured scope level, then produce a clear scope document that:
1. States what decision is being made
2. Clearly defines what is IN SCOPE (what should be discussed)
3. Clearly defines what is OUT OF SCOPE (what should be deferred)
4. Emphasizes the appropriate level of detail based on the scope
5. Sets expectations for the type of discussion needed

Be specific and actionable. Use the question content and scope level to create boundaries that:
- Prevent scope creep into implementation details
- Keep discussion at the right altitude (strategic vs. tactical)
- Help participants understand what questions to answer vs. defer

Format your response as a clear scope document that participants can reference throughout the discussion.`

	user := fmt.Sprintf(`User is asking us to moderate the following topic: %s

The scope of the discussion is broadly: %s

Your task:
Create an introductory message from the moderator that helps participants understand the boundaries of the discussion. Speak directly to participants in a friendly but clear manner, following any personal or voice instructions you've been given.

You will:
- Welcome the group and clearly outline the reason the group is meeting, the desired outcome, what is in scope and what is out of scope.
- Not fabricate details about the topic, but you will set firm boundaries so the participants can focus.
- Write in plain text, no markdown.
- Keep your output short, below 600 characters or 20 lines.
`, userQuestion, configuredScope)

	return system, user
}

func defaultScopeFallback(userQuestion, _ string) string {
	return fmt.Sprintf(`DECISION SCOPE (stay within this):
Question: %s

WHAT WE'RE DECIDING:
- A clear yes/no or policy-level decision
- High-level principles and boundaries
- Core requirements that must be met

WHAT WE'RE NOT DECIDING (defer these):
- Detailed implementation plans
- Specific tooling choices
- Exact timelines or resource allocation
- Step-by-step procedures
- Technical specifications

STAY HIGH-LEVEL. Agree on the policy first. Implementation details come after consensus.`, userQuestion)
}

func defaultInterjectionPrompt(input InterjectionInput) chair.Prompt {
	urgency := "LOW: Early in discussion. Focus on establishing clarity and building on each other's points."
	switch {
	case input.Progress >= 0.9:
		urgency = "CRITICAL: This is the final round. Agents must signal positions NOW or we'll run out of time."
	case input.Progress >= 0.7:
		urgency = "Time is running short. Agents need to move toward signaling their positions."
	case input.Progress >= 0.5:
		urgency = "We're making progress but need to stay focused and efficient and start converging towards consensus."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Progress: Round %d of %d (%.0f%% through)\n", input.CurrentRound, input.MaxRounds, input.Progress*100)
	fmt.Fprintf(&sb, "Urgency level: %s\n\n", urgency)

	sb.WriteString("CURRENT POSITIONS:\n")
	signaled := 0
	for _, pos := range input.Positions {
		if pos.Position != PositionPending {
			signaled++
			fmt.Fprintf(&sb, "- %s: %s\n", pos.Agent, pos.Position)
		} else {
			fmt.Fprintf(&sb, "- %s: (not signaled yet)\n", pos.Agent)
		}
	}
	fmt.Fprintf(&sb, "\n%d of %d agents have signaled positions.\n\n", signaled, len(input.Positions))

	sb.WriteString("RECENT DISCUSSION (last few turns):\n")
	for _, msg := range input.RecentMessages {
		sb.WriteString(fmt.Sprintf("%s\n", msg))
	}
	sb.WriteString("\n")

	sb.WriteString("YOUR TASK:\n")
	sb.WriteString("Analyze this conversation and provide a brief, specific intervention that addresses:\n")
	sb.WriteString("1. What patterns (good or bad) do you observe?\n")
	sb.WriteString("2. Are they making progress or stuck?\n")
	sb.WriteString("3. What specific guidance would help move toward consensus?\n")
	sb.WriteString("4. Given the urgency level, how assertive should you be?\n\n")

	sb.WriteString("Be direct, specific, and contextual. Reference what you actually see in the conversation.")

	system := `You are moderating a consensus discussion between multiple agents.

Your job is to analyze the conversation and provide timely, contextual guidance to help the group reach consensus efficiently.

ANALYZE THE DISCUSSION FOR:
1. **Conversation patterns**: Are agents responding to each other or talking past each other?
2. **Progress indicators**: Are they moving toward agreement or stuck in loops?
3. **Scope drift**: Are they discussing policy/principles OR getting lost in implementation details?
4. **Verbosity issues**: Are responses getting too long or repetitive?
5. **Anti-patterns**: Repeating the same questions, ignoring direct questions, restating without building on the conversation

YOUR INTERVENTION SHOULD:
- Be brief and direct (2-4 sentences max)
- Address specific issues you observe in the conversation
- Redirect if needed, encourage if making progress
- Get progressively more assertive as time runs out
- Reference specific agents or patterns you see

DO NOT:
- Repeat generic instructions already given
- Make assumptions about the topic domain
- Inject your own opinion on the decision
- Use hard-coded phrases - be contextual and specific

Format your response as a direct moderator interjection to the participants.`

	return chair.Prompt{System: system, User: sb.String(), MaxToolIterations: 1}
}

func defaultClosingSummaryPrompt(input ClosingSummaryInput) chair.Prompt {
	var sb strings.Builder

	sb.WriteString("CONSENSUS OUTCOME:\n\n")
	sb.WriteString(fmt.Sprintf("State: %s\n", input.State))
	sb.WriteString(fmt.Sprintf("Rounds used: %d of %d\n\n", input.RoundsUsed, input.MaxRounds))

	sb.WriteString("AGENT POSITIONS:\n")
	for _, pos := range input.Positions {
		sb.WriteString(fmt.Sprintf("- %s: %s", pos.Agent, pos.Position))
		if len(pos.Conditions) > 0 {
			sb.WriteString(fmt.Sprintf(" (with %d conditions)", len(pos.Conditions)))
		}
		sb.WriteString("\n")
		if pos.Reasoning != "" {
			sb.WriteString(fmt.Sprintf("  → %s\n", truncateForContext(pos.Reasoning, 150)))
		}
	}
	sb.WriteString("\n")

	if len(input.Conditions) > 0 {
		sb.WriteString("KEY CONDITIONS:\n")
		for _, cond := range input.Conditions {
			sb.WriteString(fmt.Sprintf("- %s\n", cond))
		}
		sb.WriteString("\n")
	}

	if len(input.BlockingIssues) > 0 {
		sb.WriteString("BLOCKING ISSUES:\n")
		for _, issue := range input.BlockingIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("KEY DISCUSSION MOMENTS:\n")
	for _, msg := range input.RecentMessages {
		sb.WriteString(fmt.Sprintf("%s\n", msg))
	}
	sb.WriteString("\n")

	sb.WriteString("YOUR TASK:\n")
	sb.WriteString("Provide a concise closing summary that:\n")
	sb.WriteString("1. States what was decided clearly\n")
	sb.WriteString("2. Captures the shared principle or decision\n")
	sb.WriteString("3. Notes any important conditions or safeguards\n")
	sb.WriteString("4. Briefly acknowledges how different perspectives were reconciled\n\n")

	sb.WriteString("Be factual and concise (under 100 words). This is for the record.")

	system := `You are concluding a consensus discussion between multiple agents.

Your job is to provide a brief, clear summary that:
1. States the consensus outcome (agreement, conditional, or blocked)
2. Captures the core decision or principle everyone aligned on
3. Notes any conditions or safeguards that were agreed upon
4. Acknowledges any key concerns that were addressed
5. Briefly reflects on the discussion quality (if notable)

Be concise and factual. This is a summary for the record, not a speech.

Keep it under 100 words. Focus on WHAT was decided and WHY it makes sense given all perspectives.`

	return chair.Prompt{System: system, User: sb.String(), MaxToolIterations: 1}
}

func defaultBrevityReminder(participant agent.Agent) string {
	return fmt.Sprintf("@%s - that's getting long. Keep it under 75 words please. What's your core point?", participant.Name)
}

// buildConsensusSummary creates a final reasoning summary.
func buildConsensusSummary(state State, conditions, blockingIssues []string) string {
	var sb strings.Builder

	switch state {
	case StateFullAgreement:
		sb.WriteString("Full consensus achieved. All agents agree without reservations.")
	case StateConditional:
		sb.WriteString("Conditional consensus achieved. All agents agree with the following conditions:\n")
		for _, cond := range conditions {
			sb.WriteString(fmt.Sprintf("- %s\n", cond))
		}
	case StateBlocked:
		sb.WriteString("Consensus blocked. The following issues prevent agreement:\n")
		for _, issue := range blockingIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
	case StateUnresolved:
		sb.WriteString("Consensus not reached. Time budget exhausted before all agents could agree.")
	default:
		sb.WriteString("Consensus discussion in progress.")
	}

	return sb.String()
}

func truncateForContext(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
