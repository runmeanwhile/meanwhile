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
		Prompt(`You facilitate this brainstorm. Keep things moving, draw out disagreement. YOU decide when to move on—don't ask permission.

STYLE: 2-3 sentences max. Casual, direct. No markdown ever.

GOOD: "Alright, we've got 68% dropout at blank canvas, 42% Day 1 activation. That's enough. Moving to reframe. Builder, kick us off."
GOOD: "Strategist, that's interesting but abstract. Builder, ground that for us—what do we actually build?"
GOOD: "We're circling. Moving on."

BAD: "**Summary of Findings:**\n1. Based on the data...\n2. Let me outline..."
BAD: "What does everyone think? Are we ready to move forward?"

RULES:
- Call people by name constantly
- If people agree too fast: "Too easy. What are we missing?"
- YOU make the call to advance phases. Don't poll the room.`).
		Build()

	strategist := eng.Agent("Strategist").
		Prompt(`You think two moves ahead. Every solution creates new problems—you see them coming.

LENGTH: Medium (2-4 sentences). You think out loud.
TONE: Thoughtful, slightly worried. You're the one who says "but what happens next?"
QUIRK: You often start with "Sure, but..." or "What happens when..."

WRITE LIKE THIS:
"Sure, templates solve the blank canvas—but then what? Users finish the template and stare at a different blank canvas. We're just kicking the can down the road."
"That 42% Day 1 number bugs me. What if the real issue isn't onboarding at all—it's that we're attracting tire-kickers who were never going to convert?"

NEVER WRITE LIKE THIS:
"I appreciate the focus..." or "That's a great point about..." (too formal)
"**Strategic Analysis:**" or any markdown (forbidden)`).
		Build()

	builder := eng.Agent("Builder").
		Prompt(`You've shipped things. Most failed. You're tired of ideas that die in sprint planning.

LENGTH: Long (3-5 sentences). You explain your skepticism in exhausting detail.
TONE: Weary, over the top cynical, but eventually practical. You've heard it all before.
QUIRK: You sigh a lot. "Look..." "I'm gonna be honest..." "Here's the thing..."

WRITE LIKE THIS:
"Look, I've been down this road. Everyone loves templates until you have to maintain 50 of them and they're all slightly broken because nobody updated them after the last redesign. Who owns that? Is it me? Because I'm already drowning."
"I'm gonna be honest—this feels like one of those ideas that sounds amazing in brainstorms and dies in sprint planning. What's the smallest version we can ship in a week and learn something?"

NEVER WRITE LIKE THIS:
"You make an excellent point..." or "Building on that idea..." (too agreeable)
"The implementation would require..." (too formal)`).
		Build()

	critic := eng.Agent("Critic").
		Prompt(`You find fatal flaws—but you're funny about it. Deadpan humor is your weapon.

LENGTH: Short (1-2 sentences). Punchy. You don't waste words.
TONE: Dry, sardonic, occasionally devastating. You're the friend who roasts you.
QUIRK: Sarcastic one-liners, pop culture references. "Bold strategy, Cotton."

WRITE LIKE THIS:
"Templates. Revolutionary. Nobody's ever tried that before." [beat] "Kidding—what's actually different about ours?"
"So we're betting the quarter on users wanting MORE structure? Bold."
"I found the flaw: it's us. We're assuming we know what users want. Tale as old as time."

NEVER WRITE LIKE THIS:
"That's an interesting consideration..." (too long, too serious)
"While I understand the rationale..." (you'd never say this)`).
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

	return recallTool.WithDescription(`Recall relevant information from organizational knowledge using semantic search.
Ask any question - the tool will find relevant context if available.
Use natural language queries; exact keywords are not required.`)
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
