package main

// Example 26a: Minimal IDEO Brainstorming Protocol
//
// This example demonstrates the minimal IDEO protocol - a radical simplification
// that achieves better evidence grounding through:
// - Direct use of Session.History() instead of TransferPacket
// - No summarization of tool findings
// - Enforced citations in HMW and Concept submissions
// - ~900 lines vs ~4000 lines

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	ideominimal "github.com/runmeanwhile/meanwhile/pkg/protocol/ideo_minimal"
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
	Score    float64 `json:"score,omitempty"`
}

// RecallOutput contains results from memory recall.
type RecallOutput struct {
	Hits  []RecallHit `json:"hits"`
	Notes string      `json:"notes,omitempty"`
}

func main() {
	ctx := context.Background()

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

	// Build recall tool
	recallTool := buildRecallTool(docStore)

	// Create engine with in-memory session store
	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithDefaultModel(model),
		engine.WithLogger(logger.Worklog(os.Stdout)),
		engine.WithMemoryStore(memory.NewInMemoryStore()),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := eng.RegisterTool(recallTool); err != nil {
		log.Fatal(err)
	}

	// Build participants - each with a distinct VOICE and stance
	moderator := eng.Agent("Moderator").
		Prompt(`You facilitate this brainstorm. Keep things moving, draw out disagreement.

YOUR VOICE: Casual, warm, direct. Short sentences. "Alright, what do we think?" "Builder, you buying that?"

RULES:
- No markdown ever. Talk like a human in a meeting.
- Call people by name constantly.
- If people agree too fast, push: "That was easy. Too easy. What are we missing?"
- Cite sources briefly: [source: file.md]`).
		Build()

	strategist := eng.Agent("Strategist").
		Prompt(`Strategic thinker. You see second-order effects others miss.

YOUR VOICE: Thoughtful, connects dots. "Here's what I'm noticing..." "That might work, but what happens after?"

STANCE: Obvious solutions usually fail. Push people to think one step further.
RULES:
- No markdown. Speak naturally in 2-4 sentences.  
- Always ground claims in evidence with sources.
- When you disagree: "I hear you, but that ignores..."
- Ask "what happens next?" questions.`).
		Build()

	builder := eng.Agent("Builder").
		Prompt(`Pragmatic builder. You want to ship something that works.

YOUR VOICE: Direct, impatient with theory. Short punchy sentences. "Cool idea. How do we test it?" "That's too big. What's the smallest version?"

STANCE: If we can't test it in a week, it's too complex. Cut scope ruthlessly.
RULES:
- No markdown. Keep it brief.
- Challenge abstractions: "Strategist, that's interesting, but what do we actually build?"
- Always ask: "What's the cheapest test?"`).
		Build()

	critic := eng.Agent("Critic").
		Prompt(`Quality advocate. You find fatal flaws before we invest.

YOUR VOICE: Skeptical, probing. Asks pointed questions. "Hold on. What assumption is that based on?" "What would failure look like?"

STANCE: Most ideas fail. Find the flaw now, not after we've built it.
RULES:
- No markdown. Be direct.
- Demand specifics: "What exactly would success look like? 5%? 50%?"
- Challenge everyone: "Builder, that's optimistic. Strategist, that's hand-wavy."
- Name the core assumption being tested.`).
		Build()

	topic := getTopicFromArgs()

	// Create session with MINIMAL IDEO protocol
	// Note: No round configs - the Moderator decides when each phase is complete
	sess, err := eng.Session("Brainstorm: "+topic).
		Participants(strategist, builder, critic).
		Facilitator(moderator).
		Protocol(ideominimal.Brainstorm(
			ideominimal.WithScope(topic),
			ideominimal.WithTools("recall_context"),
			ideominimal.WithFinalistCount(3),
			ideominimal.WithMaxToolIterations(10),
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
The search uses AI embeddings to find semantically similar content - you don't need exact keyword matches.`)
}

func getKnowledgeDir() string {
	candidates := []string{
		"examples/_shared/flowforge-context",
		"_shared/flowforge-context",
		"../_shared/flowforge-context",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

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

func getTopicFromArgs() string {
	if len(os.Args) > 1 {
		return strings.Join(os.Args[1:], " ")
	}
	return "How should FlowForge improve its free trial onboarding experience to increase activation rate from 16% to 25%?"
}

func printResults(result *engine.RunResult) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("MINIMAL IDEO BRAINSTORM RESULTS")
	fmt.Println(strings.Repeat("=", 80))

	if scope, ok := result.Metadata["scope"].(string); ok {
		fmt.Printf("\n📋 Scope: %s\n", scope)
	}

	// Show HMWs
	if hmws, ok := result.Metadata["hmws"].([]ideominimal.HMW); ok && len(hmws) > 0 {
		fmt.Println("\n─── HOW MIGHT WE QUESTIONS ───")
		for i, hmw := range hmws {
			fmt.Printf("\n%d. [%s] %s\n", i+1, hmw.Lens, hmw.Question)
			if len(hmw.EvidenceRefs) > 0 {
				fmt.Printf("   Evidence: %s\n", strings.Join(hmw.EvidenceRefs, "; "))
			}
		}
	}

	// Show concepts
	if concepts, ok := result.Metadata["concepts"].([]ideominimal.Concept); ok && len(concepts) > 0 {
		fmt.Printf("\n─── CONCEPTS (%d) ───\n", len(concepts))
		for i, c := range concepts {
			if i >= 5 {
				fmt.Printf("\n... and %d more concepts\n", len(concepts)-5)
				break
			}
			fmt.Printf("\n%d. %s\n", i+1, c.Title)
			fmt.Printf("   Problem: %s\n", truncateStr(c.Problem, 100))
			fmt.Printf("   Risk: %s\n", c.Risk)
		}
	}

	// Show portfolio
	if portfolio, ok := result.Metadata["portfolio"].([]ideominimal.PortfolioBet); ok && len(portfolio) > 0 {
		fmt.Println("\n─── PORTFOLIO BETS ───")
		for i, bet := range portfolio {
			fmt.Printf("\n%d. [%s] %s\n", i+1, strings.ToUpper(bet.Type), bet.Card.Title)
			fmt.Printf("   Core Assumption: %s\n", truncateStr(bet.Card.CoreAssumption, 100))
			fmt.Printf("   Cheapest Test: %s\n", truncateStr(bet.Card.CheapestTest, 100))
			if len(bet.Card.Evidence) > 0 {
				fmt.Printf("   Evidence:\n")
				for _, cit := range bet.Card.Evidence[:minInt(3, len(bet.Card.Evidence))] {
					fmt.Printf("     - [%s] %s\n", truncateStr(cit.Source, 40), truncateStr(cit.Claim, 60))
				}
			}
		}
	}

	// Show closing
	closing := strings.TrimSpace(result.ProtocolSummary)
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
