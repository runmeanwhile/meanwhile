package ideo

import (
	"context"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/ideationops"
	"github.com/runmeanwhile/meanwhile/pkg/collab/reframer"
	"github.com/runmeanwhile/meanwhile/pkg/collab/roundtable"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

// IdeationResult contains outputs from the ideation phase.
type IdeationResult struct {
	// Concepts are the rough ideas generated
	Concepts []ConceptCard `json:"concepts"`

	// Artifacts created during ideation (diagrams, journeys, etc.)
	Artifacts []Artifact `json:"artifacts"`

	// Raw thread for debugging
	Thread []agent.Message `json:"-"`
}

// runIdeation executes the ideation phase.
// Goal: Generate divergent concepts with wild ideas and artifact-based thinking.
func (p *brainstormIDEO) runIdeation(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, reframeTransfer *TransferPacket) (*IdeationResult, error) {
	rounds := p.cfg.IdeationRounds
	if rounds <= 0 {
		rounds = 2
	}

	// Register artifact tools if enabled
	if p.cfg.ArtifactTools {
		if err := p.registerArtifactTools(sess); err != nil {
			return nil, err
		}
	}

	// Kick off with transition from reframe
	if err := p.runIdeationKickoff(ctx, sess, agents, scope, reframeTransfer); err != nil {
		return nil, err
	}

	// Extract selected frames from transfer
	var selectedFrames []reframer.Frame
	if reframeTransfer != nil {
		if frames, ok := reframeTransfer.Data["selected_frames"].([]reframer.Frame); ok {
			selectedFrames = frames
		}
	}

	// Run ideation rounds with different operators
	rt := roundtable.New(roundtable.WithMaxRounds(rounds))

	// Seed with reframe context
	if reframeTransfer != nil && reframeTransfer.Summary != "" {
		rt.Record(message.User(fmt.Sprintf("Context from reframe:\n%s", reframeTransfer.Summary)))
	}
	rt.Record(message.User("Ideation phase begins. Generate wild, creative concepts. Use artifact tools to sketch your ideas."))

	for rt.CurrentRound() < rt.MaxRounds() {
		currentRound := rt.IncrementRound()
		ordered := rotateAgentOrder(agents, currentRound)

		// Get ideation operators for this round
		ops := ideationops.ForRound(currentRound-1, len(ordered))

		for idx, participant := range ordered {
			thread := recentThread(rt.Thread(), 10)
			messages := append([]agent.Message(nil), thread...)

			// Find someone to build on
			buildTarget := recentPeer(thread, participant.Name)
			if buildTarget == "" {
				buildTarget = "the prior discussion"
			}

			messages = append(messages, message.User(fmt.Sprintf(
				"Your ideation turn (%d/%d). Generate a wild concept and sketch it using tools.",
				idx+1, len(ordered),
			)))

			// Select operator for this agent
			op := ideationops.Operator{Name: "Free Association", Prompt: "Let your mind wander and make unexpected connections."}
			if idx < len(ops) {
				op = ops[idx]
			}

			system := p.buildIdeationPrompt(participant, scope, selectedFrames, currentRound, rounds, op, buildTarget)

			// Allow more tool iterations for artifact creation
			toolBudget := 2
			if p.cfg.ArtifactTools {
				toolBudget = 4
			}

			resp, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
				Messages:          messages,
				SystemMessages:    []agent.Message{message.System(system)},
				Params:            p.runParamsFor(participant),
				MaxToolIterations: toolBudget,
				Tools:             p.ideationToolIDs(),
			})
			if err != nil {
				return nil, fmt.Errorf("ideation round %d (%s): %w", currentRound, participant.Name, err)
			}

			resp.Name = participant.Name
			rt.Record(resp)
		}

		// Between rounds, moderator encourages wilder ideas
		if currentRound < rounds {
			if err := p.runIdeationRoundBridge(ctx, sess, agents, currentRound, rounds); err != nil {
				return nil, err
			}
		}
	}

	// Extract concepts and artifacts from thread
	result := p.extractIdeationResults(rt.Thread())
	result.Thread = rt.Thread()

	// Moderator synthesis
	if err := p.runIdeationSynthesis(ctx, sess, agents, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (p *brainstormIDEO) runIdeationKickoff(ctx context.Context, sess protocol.Session, agents []agent.Agent, scope string, transfer *TransferPacket) error {
	runner := p.selectRunner(sess, agents)

	system := `You are the moderator beginning the IDEATION phase.

This is where we go wild. Write 2-3 sentences that:
1. Remind the team of our reframed HMWs (mention specific ones)
2. ENCOURAGE WILD IDEAS—the crazier the better
3. Tell them to build on each other's ideas, not critique yet
4. Invite them to use tools to sketch diagrams, concept cards, journeys

Channel IDEO's rule: "Defer judgment, go for quantity, encourage wild ideas."
Sound energized and playful. This is the fun part.`

	var userPrompt strings.Builder
	userPrompt.WriteString(fmt.Sprintf("Scope: %s\n\n", scope))
	if transfer != nil && transfer.Summary != "" {
		userPrompt.WriteString("From Reframe:\n")
		userPrompt.WriteString(transfer.Summary)
	}
	userPrompt.WriteString("\nKick off ideation!")

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(userPrompt.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	return err
}

func (p *brainstormIDEO) buildIdeationPrompt(participant agent.Agent, scope string, frames []reframer.Frame, round, totalRounds int, op ideationops.Operator, buildTarget string) string {
	var sb strings.Builder

	// Include persona if available
	if participant.Profile != nil && strings.TrimSpace(participant.Profile.Prompt) != "" {
		sb.WriteString("YOUR PERSONA:\n")
		sb.WriteString(strings.TrimSpace(participant.Profile.Prompt))
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf(`IDEO BRAINSTORM: IDEATION PHASE (Round %d/%d)

PROBLEM SCOPE:
"""
%s
"""

`, round, totalRounds, scope))

	// Include HMW frames
	if len(frames) > 0 {
		sb.WriteString("HOW MIGHT WE:\n")
		for _, frame := range frames {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", frame.Lens, frame.HMW))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`IDEO BRAINSTORMING RULES:
1. DEFER JUDGMENT - No "yes, but..." allowed. Say "yes, and..."
2. ENCOURAGE WILD IDEAS - The crazier the better. We'll filter later.
3. BUILD ON OTHERS - Use what ` + buildTarget + ` said as a springboard
4. STAY FOCUSED - Keep ideas connected to our HMWs
5. GO FOR QUANTITY - Multiple rough ideas beat one polished idea
6. BE VISUAL - Use sketch tools to make ideas concrete

`)

	// Add ideation operator
	sb.WriteString(fmt.Sprintf("YOUR CREATIVE OPERATOR: **%s**\n", op.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", op.Prompt))

	// Add artifact tools guidance if enabled
	if p.cfg.ArtifactTools {
		sb.WriteString(`SKETCH TOOLS AVAILABLE:
- sketch_diagram: Create mermaid diagrams (flows, architectures, journeys)
- sketch_concept_card: Create structured concept cards
- sketch_journey: Map user journeys with emotions and touchpoints

Use at least one sketch tool to make your concept concrete.

`)
	}

	sb.WriteString(fmt.Sprintf(`YOUR TASK:
1. Apply your creative operator to generate a concept
2. Build on what %s contributed
3. Sketch your idea using a tool (diagram, concept card, or journey)
4. Explain the wild bet you're making

3-5 sentences in your own voice, plus your sketched artifact.`, buildTarget))

	return sb.String()
}

func (p *brainstormIDEO) runIdeationRoundBridge(ctx context.Context, sess protocol.Session, agents []agent.Agent, round, totalRounds int) error {
	runner := p.selectRunner(sess, agents)

	system := `You are the moderator between ideation rounds.

Write 2-3 sentences that:
1. Celebrate the energy and wild ideas so far (mention something specific)
2. Push for even wilder ideas—"what if we..."
3. Remind them to build on each other, not start from scratch

Keep the momentum high. This is brainstorming at its best.`

	user := fmt.Sprintf("Round %d/%d complete. Push for wilder ideas in round %d.", round, totalRounds, round+1)

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user)},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	return err
}

func (p *brainstormIDEO) extractIdeationResults(thread []agent.Message) *IdeationResult {
	result := &IdeationResult{
		Concepts:  make([]ConceptCard, 0),
		Artifacts: make([]Artifact, 0),
	}

	// Extract concepts from messages
	for _, msg := range thread {
		if msg.Role != agent.RoleAssistant || msg.Name == "" {
			continue
		}

		text := strings.TrimSpace(msg.Text())
		if text == "" {
			continue
		}

		// Create a concept card from the message
		concept := ConceptCard{
			Title:     truncate(text, 80),
			Problem:   "Extracted from ideation discussion",
			Mechanism: text,
			Value:     "To be validated",
			Risk:      "To be assessed",
		}
		result.Concepts = append(result.Concepts, concept)
	}

	// Limit concepts
	if len(result.Concepts) > p.cfg.TargetConcepts {
		result.Concepts = result.Concepts[:p.cfg.TargetConcepts]
	}

	return result
}

func (p *brainstormIDEO) runIdeationSynthesis(ctx context.Context, sess protocol.Session, agents []agent.Agent, result *IdeationResult) error {
	runner := p.selectRunner(sess, agents)

	system := `You are the moderator concluding the IDEATION phase.

Write 2-3 sentences that:
1. Celebrate the creative output (mention specific wild ideas)
2. Group concepts into themes or clusters
3. Bridge to synthesis—explain we'll now evaluate and build evidence

Sound appreciative and forward-looking. The fun part is done, now we get rigorous.`

	var user strings.Builder
	user.WriteString(fmt.Sprintf("Generated %d concepts and %d artifacts.\n", len(result.Concepts), len(result.Artifacts)))
	user.WriteString("Synthesize and bridge to evidence phase.")

	_, err := sess.RunAgent(ctx, runner, protocol.RunRequest{
		Messages:          []agent.Message{message.User(user.String())},
		SystemMessages:    []agent.Message{message.System(system)},
		Params:            p.runParamsFor(runner),
		MaxToolIterations: 1,
	})
	return err
}

func (p *brainstormIDEO) ideationToolIDs() []string {
	tools := make([]string, 0)
	if p.cfg.ArtifactTools {
		tools = append(tools, "sketch_diagram", "sketch_concept_card", "sketch_journey")
	}
	return tools
}

// buildIdeationTransfer creates a transfer packet from ideation results.
func (p *brainstormIDEO) buildIdeationTransfer(result *IdeationResult, reframeTransfer *TransferPacket, scope string) *TransferPacket {
	var summary strings.Builder
	summary.WriteString("## Concepts from Ideation\n\n")

	for i, concept := range result.Concepts {
		if i >= 10 {
			summary.WriteString(fmt.Sprintf("... and %d more concepts\n", len(result.Concepts)-10))
			break
		}
		summary.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, concept.Title))
		if concept.Mechanism != "" && concept.Mechanism != concept.Title {
			summary.WriteString(fmt.Sprintf("   %s\n", truncate(concept.Mechanism, 150)))
		}
	}
	summary.WriteString("\n")

	if len(result.Artifacts) > 0 {
		summary.WriteString(fmt.Sprintf("**Artifacts:** %d diagrams/cards/journeys created\n\n", len(result.Artifacts)))
	}

	// Include HMW context from reframe
	if reframeTransfer != nil {
		if frames, ok := reframeTransfer.Data["selected_frames"].([]reframer.Frame); ok && len(frames) > 0 {
			summary.WriteString("**Original HMWs:**\n")
			for _, frame := range frames[:min(4, len(frames))] {
				summary.WriteString(fmt.Sprintf("- [%s] %s\n", frame.Lens, frame.HMW))
			}
		}
	}

	var priorMessages []agent.Message
	if p.cfg.TransferStrategy == TransferWithHistory || p.cfg.TransferStrategy == TransferFull {
		for _, msg := range result.Thread {
			if msg.Role == agent.RoleAssistant && msg.Name != "" {
				priorMessages = append(priorMessages, msg)
			}
		}
		if len(priorMessages) > 6 {
			priorMessages = priorMessages[len(priorMessages)-6:]
		}
	}

	return &TransferPacket{
		FromPhase: PhaseIdeation,
		Data: map[string]any{
			"scope":     scope,
			"concepts":  result.Concepts,
			"artifacts": result.Artifacts,
		},
		Summary:       summary.String(),
		PriorMessages: priorMessages,
	}
}
