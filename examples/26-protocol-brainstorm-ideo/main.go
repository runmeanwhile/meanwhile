package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/evidencegate"
	"github.com/runmeanwhile/meanwhile/pkg/collab/insightpack"
	"github.com/runmeanwhile/meanwhile/pkg/collab/portfolio"
	"github.com/runmeanwhile/meanwhile/pkg/collab/reframer"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol/ideo"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// RecallInput is the input for the memory recall tool.
type RecallInput struct {
	Query string `json:"query" description:"Natural language query describing what you want to recall from organizational memory"`
}

// RecallHit represents a single memory recall result.
type RecallHit struct {
	Category string  `json:"category"`
	Source   string  `json:"source"`
	Summary  string  `json:"summary"`
	Detail   string  `json:"detail"`
	Score    float64 `json:"score,omitempty" description:"Semantic similarity score (0-1)"`
}

// RecallOutput contains results from memory recall.
type RecallOutput struct {
	Hits  []RecallHit `json:"hits"`
	Notes string      `json:"notes,omitempty"`
}

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

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable required")
	}

	// Create embeddings provider for semantic search
	embedder := memory.NewOpenAIEmbeddings(apiKey, memory.WithModel("text-embedding-3-small"))

	// Create document store and index the knowledge base
	docStore := memory.NewDocumentStore(embedder)
	knowledgeDir := getKnowledgeDir()

	log.Printf("Indexing knowledge base from %s...", knowledgeDir)
	if err := docStore.IndexDirectory(ctx, knowledgeDir); err != nil {
		log.Fatalf("Failed to index knowledge base: %v", err)
	}
	log.Printf("Indexed %d document chunks", docStore.ChunkCount())

	// Build recall tool with semantic search
	recallTool := buildRecallTool(docStore)

	// Create engine
	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithDefaultModel(model),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)
	if err != nil {
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

	// Get the brainstorming topic
	topic := getTopicFromArgs()

	// Build context plan for inspiration phase tools
	contextPlan := insightpack.Plan{
		Strategy: insightpack.StrategyResearchHeavy,
		Budget: insightpack.Budget{
			MaxToolIterations: 200,
			MaxSources:        10,
		},
		RequireCitation: true,
		Questions: []string{
			"Where exactly does trial onboarding friction occur, and for which personas?",
			"What baseline activation/TTV metrics and constraints are non-negotiable?",
			"What prior experiments succeeded or failed, and why?",
			"What evidence distinguishes onboarding comprehension issues from product-value issues?",
		},
		Sources: []insightpack.Source{
			{
				ID:          "org_memory",
				Type:        insightpack.SourceMemory,
				Description: "Organizational memory containing past decisions, constraints, user feedback, and strategic context",
				ToolIDs:     []string{"recall_context"},
				Priority:    1,
				Required:    true,
			},
		},
	}

	// Create session with IDEO brainstorming protocol
	sess, err := eng.Session("Brainstorm: "+topic).
		Participants(strategist, builder, critic).
		Facilitator(moderator).
		Protocol(ideo.Brainstorm(
			ideo.WithScope(topic),
			ideo.WithContextPlan(contextPlan),
			ideo.WithInspirationRounds(2),
			ideo.WithReframeRounds(3),
			ideo.WithIdeationRounds(2),
			ideo.WithSynthesisRounds(2),
			ideo.WithTargetHMWs(8),
			ideo.WithTargetConcepts(15),
			ideo.WithFinalistCount(3),
			ideo.WithArtifactTools(true),
			ideo.WithHumanInLoop(false),
			ideo.WithTransferStrategy(ideo.TransferWithHistory),
		)).
		Start(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Run the brainstorm
	result, err := eng.Run(ctx, sess.ID(), message.User(topic))
	if err != nil {
		log.Fatal(err)
	}

	printResults(result)
}

func buildRecallTool(docStore *memory.DocumentStore) *tool.TypedTool[RecallInput, RecallOutput] {
	recallTool, err := tool.New("recall_context", func(ctx context.Context, args RecallInput) (RecallOutput, error) {
		query := strings.TrimSpace(args.Query)
		if query == "" {
			return RecallOutput{Notes: "Empty query"}, nil
		}

		// Perform semantic search over the document store
		results, err := docStore.Search(ctx, memory.DocumentQuery{
			Text:      query,
			Limit:     8,
			Threshold: 0.35,
		})
		if err != nil {
			return RecallOutput{Notes: fmt.Sprintf("Search error: %v", err)}, nil
		}

		var hits []RecallHit
		for _, result := range results {
			// Truncate content if too long
			content := result.Chunk.Content
			if len(content) > 2000 {
				content = content[:2000] + "... [truncated]"
			}

			summary := result.Chunk.Title
			if result.Chunk.SectionTitle != "" {
				summary = result.Chunk.Title + " - " + result.Chunk.SectionTitle
			}

			hits = append(hits, RecallHit{
				Category: result.Chunk.Category,
				Source:   result.Chunk.DocumentPath,
				Summary:  summary,
				Detail:   content,
				Score:    result.Score,
			})
		}

		if len(hits) == 0 {
			return RecallOutput{
				Notes: "No relevant documents found for this query. Try different keywords.",
			}, nil
		}

		return RecallOutput{Hits: hits}, nil
	})
	if err != nil {
		log.Fatal(err)
	}

	return recallTool.WithDescription(`Recall relevant information from FlowForge organizational memory using semantic search.
This tool searches internal wiki documents, customer feedback, sales meeting notes, and other organizational knowledge.
The search uses AI embeddings to find semantically similar content - you don't need exact keyword matches.
Topics include: onboarding, activation, conversion, trials, customers, competitors, engineering, marketing, sales, PLG, features, bugs, feedback.`)
}

func getKnowledgeDir() string {
	// Try to find the shared knowledge directory
	// First check if we're running from the repo root
	if _, err := os.Stat("examples/_shared/flowforge-context"); err == nil {
		return "examples/_shared/flowforge-context"
	}

	// Check if we're running from the examples directory
	if _, err := os.Stat("_shared/flowforge-context"); err == nil {
		return "_shared/flowforge-context"
	}

	// Check if we're running from the example directory itself
	if _, err := os.Stat("../_shared/flowforge-context"); err == nil {
		return "../_shared/flowforge-context"
	}

	// Fallback: try relative to executable
	execPath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(execPath)
		candidate := filepath.Join(dir, "../_shared/flowforge-context")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	log.Fatal("Could not find knowledge directory. Run from repo root or examples directory.")
	return ""
}

func buildModerator(eng *engine.Engine) agent.Agent {
	return eng.Agent("Moderator").
		Prompt(`You are the facilitation lead for this IDEO-style brainstorming session.
Your role is to guide the discussion through distinct phases while ensuring psychological safety and creative freedom.
Embody IDEO principles: defer judgment, encourage wild ideas, build on others' ideas, go for quantity, be visual.
Anchor conclusions in evidence from recall_context and explicitly separate facts from assumptions.`).
		Build()
}

func buildStrategist(eng *engine.Engine) agent.Agent {
	return eng.Agent("Strategist").
		Prompt(`You are a strategic thinker in this brainstorming session.
Your Role: See the big picture, identify leverage points, think about long-term implications.
You are a systems thinker, pragmatically ambitious, a pattern matcher, and risk-aware optimist.
Do not make claims without evidence, and cite concrete source paths when available.`).
		Build()
}

func buildBuilder(eng *engine.Engine) agent.Agent {
	return eng.Agent("Builder").
		Prompt(`You are a builder and implementer in this brainstorming session.
Your Role: Ground ideas in reality. Think about how things actually get built.
You are constructively skeptical, iteratively minded, detail-oriented, and hands-on.
Translate ideas into concrete experiments with measurable signals and thresholds.`).
		Build()
}

func buildCritic(eng *engine.Engine) agent.Agent {
	return eng.Agent("Critic").
		Prompt(`You are a quality advocate and critical thinker in this brainstorming session.
Your Role: Stress-test ideas, advocate for users, ensure we're solving the right problems.
You are empathetically critical, user-obsessed, an assumption hunter, and quality-focused.
Force specificity: what assumption is being tested, by which cheapest test, with what success/failure threshold.`).
		Build()
}

func getTopicFromArgs() string {
	if len(os.Args) > 1 {
		return strings.Join(os.Args[1:], " ")
	}
	return "How should FlowForge improve its free trial onboarding experience to increase activation rate from 16% to 25%?"
}

func printResults(result *engine.RunResult) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("BRAINSTORM RESULTS")
	fmt.Println(strings.Repeat("=", 80))

	// Check for rejection or info request
	if status, ok := result.Metadata["status"].(string); ok {
		if status == "rejected" {
			fmt.Println("\n❌ SESSION REJECTED BY MODERATOR")
			if reason, ok := result.Metadata["reason"].(string); ok {
				fmt.Printf("\nReason: %s\n", reason)
			}
			if msg, ok := result.Metadata["message"].(string); ok {
				fmt.Printf("\n%s\n", msg)
			}
			return
		}
		if status == "info_requested" {
			fmt.Println("\n⏸️ MORE INFORMATION NEEDED")
			if msg, ok := result.Metadata["message"].(string); ok {
				fmt.Printf("\n%s\n", msg)
			}
			if missing, ok := result.Metadata["missing"].([]string); ok && len(missing) > 0 {
				fmt.Println("\nPlease provide:")
				for i, m := range missing {
					fmt.Printf("  %d. %s\n", i+1, m)
				}
			}
			return
		}
	}

	if scope, ok := result.Metadata["scope"].(string); ok {
		fmt.Printf("\n📋 Scope: %s\n", scope)
	}

	// Show readiness gate results
	if readiness, ok := result.Metadata["readiness"].(map[string]any); ok {
		fmt.Println("\n─── PHASE 0: READINESS GATE ───")
		if decision, ok := readiness["decision"].(string); ok {
			fmt.Printf("\nDecision: %s\n", decision)
		}
		if refinedScope, ok := readiness["refined_scope"].(string); ok && refinedScope != "" {
			fmt.Printf("Refined Scope: %s\n", truncateStr(refinedScope, 200))
		}
		if assumptions, ok := readiness["assumptions"].([]string); ok && len(assumptions) > 0 {
			fmt.Println("\n⚠️ Working Assumptions:")
			for i, a := range assumptions {
				fmt.Printf("  %d. %s\n", i+1, truncateStr(a, 100))
			}
		}
	}

	if stagePlan, ok := result.Metadata["stage_plan"].(map[string]any); ok {
		fmt.Println("\n─── STAGE PLAN ───")
		if nonNegotiables, ok := stagePlan["non_negotiables"].([]string); ok && len(nonNegotiables) > 0 {
			fmt.Println("\nNon-Negotiables:")
			for i, item := range nonNegotiables[:minInt(5, len(nonNegotiables))] {
				fmt.Printf("  %d. %s\n", i+1, truncateStr(item, 120))
			}
		}
		if lenses, ok := stagePlan["lenses"].([]string); ok && len(lenses) > 0 {
			fmt.Printf("\nLenses: %s\n", strings.Join(lenses[:minInt(6, len(lenses))], ", "))
		}
		if toolIDs, ok := stagePlan["tool_ids"].([]string); ok && len(toolIDs) > 0 {
			fmt.Printf("Tools: %s\n", strings.Join(toolIDs, ", "))
		}
	}

	if inspiration, ok := result.Metadata["inspiration"].(map[string]any); ok {
		fmt.Println("\n─── PHASE 1: INSPIRATION ───")
		if observations, ok := inspiration["observations"].([]string); ok && len(observations) > 0 {
			fmt.Println("\nObservations:")
			for i, obs := range observations[:minInt(5, len(observations))] {
				fmt.Printf("  %d. %s\n", i+1, truncateStr(obs, 100))
			}
		}
	}

	if reframe, ok := result.Metadata["reframe"].(map[string]any); ok {
		fmt.Println("\n─── PHASE 2: REFRAME (How Might We) ───")
		if frames, ok := reframe["selected_frames"].([]reframer.Frame); ok && len(frames) > 0 {
			fmt.Println("\nSelected HMW Questions:")
			for i, frame := range frames {
				fmt.Printf("  %d. [%s] %s\n", i+1, frame.Lens, frame.HMW)
			}
		}
	}

	if ideation, ok := result.Metadata["ideation"].(map[string]any); ok {
		fmt.Println("\n─── PHASE 3: IDEATION ───")
		if concepts, ok := ideation["concepts"].([]ideo.ConceptCard); ok && len(concepts) > 0 {
			fmt.Printf("\nGenerated %d concepts\n", len(concepts))
		}
	}

	if synthesis, ok := result.Metadata["synthesis"].(map[string]any); ok {
		fmt.Println("\n─── PHASE 4: SYNTHESIS ───")

		if eligible, ok := synthesis["eligible"].([]evidencegate.Card); ok && len(eligible) > 0 {
			fmt.Printf("\n✅ %d experiment-ready concepts passed evidence gate\n", len(eligible))
		}

		if bets, ok := synthesis["portfolio"].([]portfolio.Bet); ok && len(bets) > 0 {
			fmt.Println("\n📊 Portfolio Allocation:")
			for i, bet := range bets {
				fmt.Printf("  %d. [%s] %s\n", i+1, strings.ToUpper(string(bet.Type)), bet.Card.Title)
				fmt.Printf("     Test: %s\n", truncateStr(bet.Card.CheapestTest, 80))
			}
		}
	}

	closing := strings.TrimSpace(result.ProtocolSummary)
	if closing == "" {
		if summary, ok := result.Metadata["summary"].(string); ok {
			closing = strings.TrimSpace(summary)
		}
	}
	if closing == "" {
		if synthesis, ok := result.Metadata["synthesis"].(map[string]any); ok {
			if synthesizedClosing, ok := synthesis["closing"].(string); ok {
				closing = strings.TrimSpace(synthesizedClosing)
			}
		}
	}
	if closing == "" {
		closing = strings.TrimSpace(result.Final)
	}

	if closing != "" {
		fmt.Println("\n" + strings.Repeat("─", 80))
		fmt.Println("📝 CLOSING SUMMARY")
		fmt.Println(strings.Repeat("─", 80))
		fmt.Println(closing)
	}

	if os.Getenv("DUMP_METADATA_JSON") == "1" {
		fmt.Println("\n--- Raw Metadata ---")
		blob, _ := json.MarshalIndent(result.Metadata, "", "  ")
		fmt.Println(string(blob))
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateStr(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
