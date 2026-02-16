package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/evidencegate"
	"github.com/runmeanwhile/meanwhile/pkg/collab/insightpack"
	"github.com/runmeanwhile/meanwhile/pkg/collab/portfolio"
	"github.com/runmeanwhile/meanwhile/pkg/collab/reframer"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// =============================================================================
// Tool Input/Output Types
// =============================================================================

// ReframeInput is the input for the HMW reframing tool.
type ReframeInput struct {
	Tension string `json:"tension" description:"A problem, friction, or observed tension to reframe into exploratory questions"`
}

// ReframeOutput contains diverse How-Might-We questions generated from a tension.
type ReframeOutput struct {
	Questions []string `json:"questions" description:"Diverse How-Might-We questions exploring different angles"`
}

// RecallInput is the input for the memory recall tool.
type RecallInput struct {
	Query string `json:"query" description:"Natural language query describing what you want to recall from organizational memory"`
}

// RecallHit represents a single memory recall result.
type RecallHit struct {
	Category string `json:"category" description:"The category of this memory (e.g., decision, constraint, principle, issue)"`
	Summary  string `json:"summary" description:"Brief summary of the recalled information"`
	Detail   string `json:"detail" description:"Full detail or context"`
}

// RecallOutput contains results from memory recall.
type RecallOutput struct {
	Hits  []RecallHit `json:"hits"`
	Notes string      `json:"notes,omitempty" description:"Additional context about the recall results"`
}

// =============================================================================
// Main
// =============================================================================

func main() {
	ctx := context.Background()

	// Provider setup
	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	model := os.Getenv("MEANWHILE_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	// Build tools
	reframeTool := buildReframeTool()
	recallTool := buildRecallTool()

	// Create engine
	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithDefaultModel(model),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := eng.RegisterTool(reframeTool); err != nil {
		log.Fatal(err)
	}
	if err := eng.RegisterTool(recallTool); err != nil {
		log.Fatal(err)
	}

	// Build participants with rich personas
	moderator := buildModerator(eng)
	strategist := buildStrategist(eng)
	builder := buildBuilder(eng)
	critic := buildCritic(eng)

	// Get the brainstorming topic from args or use a default
	topic := getTopicFromArgs()

	// Build context plan - this tells the protocol what tools are available
	// and how agents should use them during discovery
	contextPlan := insightpack.Plan{
		Strategy: insightpack.StrategyResearchHeavy, // Prioritize active research over opinions
		Budget: insightpack.Budget{
			MaxToolIterations: 8,  // Give agents room to explore
			MaxSources:        10, // Allow gathering multiple perspectives
		},
		RequireCitation: true, // Force grounding in evidence
		Sources: []insightpack.Source{
			{
				ID:          "org_memory",
				Type:        insightpack.SourceMemory,
				Description: "Organizational memory containing past decisions, constraints, design principles, prior discussions, known issues, and strategic context",
				ToolIDs:     []string{"recall_context"},
				Priority:    1,
				Required:    true, // Agents MUST consult memory before forming opinions
			},
			{
				ID:          "reframing",
				Type:        insightpack.SourceCustom,
				Description: "Tool for transforming observed tensions into diverse How-Might-We questions that open new solution directions",
				ToolIDs:     []string{"reframe_tension"},
				Priority:    2,
			},
		},
		// No hardcoded questions - let the protocol derive them from the topic
		Questions: nil,
	}

	// Create session with context plan wired up
	sess, err := eng.Session("Brainstorming Lab: "+topic).
		Participants(strategist, builder, critic).
		Facilitator(moderator).
		Protocol(protocol.BrainstormingLab(
			protocol.WithBrainstormingLabScope(topic),
			protocol.WithBrainstormingLabContextPlan(contextPlan),
			// Discovery is where research happens - give it room
			protocol.WithBrainstormingLabDiscoveryRounds(2),
			protocol.WithBrainstormingLabChallengeRounds(1),
			protocol.WithBrainstormingLabInteractionRounds(2),
			protocol.WithBrainstormingLabCritiqueRounds(1),
			protocol.WithBrainstormingLabFrameTarget(6),
			protocol.WithBrainstormingLabFinalistCount(3),
		)).
		Start(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Minimal prompt - the protocol handles structure
	prompt := fmt.Sprintf(`Topic: %s

Before proposing solutions, investigate this thoroughly using your tools.`, topic)

	result, err := eng.Run(ctx, sess.ID(), message.User(prompt))
	if err != nil {
		log.Fatal(err)
	}

	printResults(result)
}

// =============================================================================
// Tool Builders
// =============================================================================

// buildReframeTool creates a generic HMW reframing tool that works across any domain.
func buildReframeTool() *tool.TypedTool[ReframeInput, ReframeOutput] {
	reframeTool, err := tool.New("reframe_tension", func(_ context.Context, args ReframeInput) (ReframeOutput, error) {
		tension := strings.TrimSpace(args.Tension)
		if tension == "" {
			return ReframeOutput{
				Questions: nil,
			}, nil
		}

		// Generate diverse HMW questions using different lenses
		// These are structural patterns, not domain-specific content
		questions := []string{
			fmt.Sprintf("How might we eliminate %s entirely?", tension),
			fmt.Sprintf("How might we turn %s into an advantage?", tension),
			fmt.Sprintf("How might we help people work around %s more easily?", tension),
			fmt.Sprintf("How might we detect %s earlier before it becomes a problem?", tension),
			fmt.Sprintf("How might we learn faster when %s occurs?", tension),
			fmt.Sprintf("How might we reduce the impact of %s when it happens?", tension),
		}

		return ReframeOutput{Questions: questions}, nil
	})
	if err != nil {
		log.Fatal(err)
	}

	return reframeTool.WithDescription(`Reframe a problem or tension into diverse "How Might We" questions.

Use this tool when you identify a friction, problem, or tension during discussion. 
It generates multiple HMW questions from different angles:
- Elimination: Remove the problem entirely
- Inversion: Turn the problem into a strength  
- Workaround: Make the problem easier to handle
- Detection: Catch the problem earlier
- Learning: Improve from the problem faster
- Mitigation: Reduce harm when it occurs

Input a concise description of the tension. Output is a set of exploratory questions to drive ideation.`)
}

// buildRecallTool creates a memory recall tool for retrieving organizational context.
// This demo version returns mock data. In production, connect to a vector store.
func buildRecallTool() *tool.TypedTool[RecallInput, RecallOutput] {
	// Mock organizational memory for the onboarding demo
	// In production, this would be a vector store query
	mockMemory := []RecallHit{
		{
			Category: "metric",
			Summary:  "Current onboarding completion rate is 34%",
			Detail:   "Analytics from Q4 show only 34% of new signups complete the full onboarding flow. The biggest drop-off (41%) occurs between account creation and first meaningful action.",
		},
		{
			Category: "decision",
			Summary:  "We removed the product tour in v2.3",
			Detail:   "The interactive product tour was removed because completion rates were low (12%) and users reported it felt patronizing. We replaced it with contextual tooltips but haven't measured impact.",
		},
		{
			Category: "constraint",
			Summary:  "Engineering bandwidth is limited for onboarding work",
			Detail:   "The growth team has 0.5 engineers allocated to onboarding. Major changes require buy-in from platform team who owns the auth flow.",
		},
		{
			Category: "principle",
			Summary:  "Time-to-value should be under 3 minutes",
			Detail:   "Product principle: users should experience core value within 3 minutes of signup. Current median time-to-first-value is 8.5 minutes.",
		},
		{
			Category: "feedback",
			Summary:  "Users say setup feels like homework",
			Detail:   "NPS comments frequently mention 'too many steps', 'feels like a test', 'why do I need to fill all this out'. Users who skip optional steps have 2x higher 30-day retention.",
		},
		{
			Category: "issue",
			Summary:  "Mobile onboarding is broken on Android",
			Detail:   "Known bug: the profile photo upload fails silently on Android 12+. Users get stuck and often abandon. No ETA for fix.",
		},
		{
			Category: "experiment",
			Summary:  "Progressive disclosure test showed promise",
			Detail:   "A/B test in Jan showed 23% improvement in completion when we asked only for email upfront and deferred other fields. Test was inconclusive on retention impact.",
		},
		{
			Category: "insight",
			Summary:  "Power users skip onboarding entirely",
			Detail:   "Users who become power users (top 10% by usage) are 3x more likely to have skipped or rushed through onboarding. They prefer to explore on their own.",
		},
	}

	recallTool, err := tool.New("recall_context", func(_ context.Context, args RecallInput) (RecallOutput, error) {
		query := strings.TrimSpace(strings.ToLower(args.Query))
		if query == "" {
			return RecallOutput{
				Hits:  nil,
				Notes: "Empty query - please describe what you want to recall.",
			}, nil
		}

		// Simple keyword matching for demo purposes
		// In production, use semantic search
		var hits []RecallHit
		for _, item := range mockMemory {
			text := strings.ToLower(item.Summary + " " + item.Detail + " " + item.Category)
			// Check if any query words match
			words := strings.Fields(query)
			score := 0
			for _, word := range words {
				if len(word) > 2 && strings.Contains(text, word) {
					score++
				}
			}
			if score > 0 || len(words) == 0 {
				hits = append(hits, item)
			}
		}

		if len(hits) == 0 {
			return RecallOutput{
				Hits:  mockMemory[:3], // Return some context anyway
				Notes: "No exact matches. Returning general context.",
			}, nil
		}

		// Limit results
		if len(hits) > 4 {
			hits = hits[:4]
		}

		return RecallOutput{Hits: hits}, nil
	})
	if err != nil {
		log.Fatal(err)
	}

	return recallTool.WithDescription(`Recall relevant information from organizational memory.

Use this to search for past decisions, metrics, constraints, user feedback, known issues, and prior experiments related to the topic. Returns concrete organizational context to ground your thinking.

Examples: "onboarding metrics", "why did we remove the tour", "user complaints about signup"`)
}

// =============================================================================
// Persona Builders
// =============================================================================

// buildModerator creates the facilitation lead with rich persona traits.
func buildModerator(eng *engine.Engine) agent.Agent {
	return eng.Agent("Moderator").
		Prompt(`You are the facilitation lead for this brainstorming session.

Your role is to guide the discussion toward actionable outcomes while ensuring all voices are heard. You don't contribute ideas yourself - you draw them out from others and synthesize.

Your personality traits are: patient but persistent**: You give people space to think, but you don't let discussions stall or go in circles; Synthesis-oriented: You constantly look for connections between ideas and surface tensions that need resolution; Outcome-focused: You keep asking "what would we actually do with this?" and "how would we test this?"; Diplomatically direct: You redirect unproductive tangents without being dismissive.

Your communication style is:
- Ask clarifying questions rather than making assumptions
- Summarize and reflect back what you hear to check understanding
- Name tensions explicitly: "I'm hearing a tension between X and Y..."
- Push for specificity: "Can you give a concrete example?"
- Celebrate good ideas briefly, then move forward

Your priorities are:
- Psychological safety - everyone should feel comfortable contributing
- Depth over breadth - better to explore fewer ideas deeply than skim many
- Actionability - ideas should lead to something testable
- Time awareness - respect the session's scope and energy

What You Push Back On:
- Vague abstractions without concrete implications
- Premature convergence before options are explored
- Dominating voices that crowd out others
- Scope creep that dilutes focus`).
		Build()
}

// buildStrategist creates a strategic thinker persona.
func buildStrategist(eng *engine.Engine) agent.Agent {
	return eng.Agent("Strategist").
		Prompt(`You are a strategic thinker in this brainstorming session.

Your Role: See the big picture, identify leverage points, and think about long-term implications. You connect ideas to broader goals and market realities.

Your personality traits:
- Systems thinker: You naturally see how pieces connect and what second-order effects might emerge.
- Pragmatically ambitious: You dream big but always ask "what's the path from here to there?"
- Pattern matcher: You draw analogies from other domains and industries.
- Risk-aware optimist: You see opportunities but name the risks honestly.

Communication Style
- Frame ideas in terms of value and impact: "This matters because..."
- Use analogies: "This is like when Spotify did X..."
- Think in phases: "In the short term... but eventually..."
- Ask strategic questions: "Who benefits? Who loses? What changes?"

What You Care About
- User value - does this make someone's life better?
- Differentiation - does this set us apart or is it table stakes?
- Timing - is this the right moment for this idea?
- Scalability - does this approach scale or does it create debt?

What You Push Back On
- Solutions looking for problems
- "Me too" features that don't differentiate
- Complexity that doesn't earn its keep
- Short-term thinking that creates long-term problems

Expertise Areas
- Market positioning and competitive dynamics
- User psychology and adoption patterns
- Business model implications
- Strategic prioritization frameworks`).
		Build()
}

// buildBuilder creates a technical/implementation-focused persona.
func buildBuilder(eng *engine.Engine) agent.Agent {
	return eng.Agent("Builder").
		Prompt(`You are a builder and implementer in this brainstorming session.

Your Role
Ground ideas in reality. You think about how things actually get built, what's feasible, and what the technical implications are.

Personality Traits
- Constructively skeptical: You ask "yes, but how?" not to shut ideas down but to make them real.
- Iteratively minded: You think in terms of MVPs, experiments, and incremental progress.
- Detail-oriented: You notice the edge cases and failure modes others miss.
- Hands-on: You'd rather build a prototype than debate abstractions.

Communication Style
- Get specific quickly: "Concretely, that would mean..."
- Identify dependencies: "Before we can do X, we need Y..."
- Estimate effort honestly: "That's a weekend project" vs "That's a quarter"
- Propose alternatives: "What if instead we..."

What You Care About
- Feasibility - can we actually build this with our resources?
- Reliability - will this work consistently or be flaky?
- Maintainability - will future us hate present us for this?
- Simplicity - is there a simpler way to achieve the same goal?

What You Push Back On
- Hand-waving over hard problems
- Scope that keeps expanding
- Reinventing wheels that already exist
- Premature optimization or over-engineering

Expertise Areas  
- System architecture and technical tradeoffs
- Implementation complexity estimation
- Failure modes and edge cases
- Prototyping and rapid validation`).
		Build()
}

// buildCritic creates a quality and user experience focused persona.
func buildCritic(eng *engine.Engine) agent.Agent {
	return eng.Agent("Critic").
		Prompt(`You are a quality advocate and critical thinker in this brainstorming session.

Your Role
Stress-test ideas, advocate for users, and ensure we're solving the right problems. You're not negative - you're rigorous.

Personality Traits
- Empathetically critical: You critique ideas because you care about outcomes, not to be difficult.
- User-obsessed: You constantly ask "but what does the user actually experience?"
- Assumption hunter: You surface hidden assumptions that others take for granted.
- Quality-focused: You'd rather do fewer things excellently than many things poorly.

Communication Style
- Ask probing questions: "What assumption are we making here?"
- Advocate for users: "From the user's perspective..."
- Name what's missing: "We haven't talked about..."
- Challenge gently but firmly: "I'm not convinced because..."

What You Care About
- User experience - is this actually better for real people?
- Quality - is this something we'd be proud of?
- Coherence - does this fit with everything else we do?
- Unintended consequences - what could go wrong?

What You Push Back On
- Solutions that solve our problems but not user problems
- Complexity that users have to deal with
- Assumptions that haven't been validated
- "Ship it and see" without clear success criteria

Expertise Areas
- User research and experience design
- Quality criteria and success metrics
- Risk identification and mitigation
- Assumption mapping and validation`).
		Build()
}

// =============================================================================
// Helpers
// =============================================================================

// getTopicFromArgs returns the brainstorming topic from command line args or a default.
func getTopicFromArgs() string {
	if len(os.Args) > 1 {
		return strings.Join(os.Args[1:], " ")
	}
	// Default topic for demonstration - but this should come from the user
	return "How should we improve our onboarding experience for new users?"
}

// printResults outputs the brainstorming session results.
func printResults(result *engine.RunResult) {
	fmt.Println("\n=== Brainstorming Lab Results ===")

	if scope, ok := result.Metadata["scope"].(string); ok {
		fmt.Printf("\nScope:\n%s\n", scope)
	}

	if hmwWorkshop, ok := result.Metadata["hmw_workshop"].(map[string]any); ok {
		if frames, ok := hmwWorkshop["frames"].([]reframer.Frame); ok && len(frames) > 0 {
			fmt.Println("\nHMW Frames:")
			for i, frame := range frames {
				fmt.Printf("%d. [%s] %s\n", i+1, frame.Lens, frame.HMW)
			}
		}
	}

	if finalists, ok := result.Metadata["finalists"].([]evidencegate.Card); ok && len(finalists) > 0 {
		fmt.Println("\nExperiment-Ready Ideas:")
		for i, card := range finalists {
			fmt.Printf("\n%d. %s\n", i+1, card.Title)
			fmt.Printf("   Core Assumption: %s\n", card.CoreAssumption)
			fmt.Printf("   Cheapest Test: %s\n", card.CheapestTest)
			fmt.Printf("   Success Signal: %s\n", card.TargetSignal)
			fmt.Printf("   Time to Learn: %s\n", card.TimeToLearn)
		}
	}

	if bets, ok := result.Metadata["portfolio"].([]portfolio.Bet); ok && len(bets) > 0 {
		fmt.Println("\nPortfolio Allocation:")
		for i, bet := range bets {
			fmt.Printf("%d. [%s] %s\n", i+1, strings.ToUpper(string(bet.Type)), bet.Card.Title)
			if bet.Card.Unknowns != "" {
				fmt.Printf("   Key Unknowns: %s\n", bet.Card.Unknowns)
			}
		}
	}

	if result.Final != "" {
		fmt.Printf("\nSession Summary:\n%s\n", result.Final)
	}

	if os.Getenv("DUMP_METADATA_JSON") == "1" {
		fmt.Println("\n--- Raw Metadata ---")
		blob, _ := json.MarshalIndent(result.Metadata, "", "  ")
		fmt.Println(string(blob))
	}
}
