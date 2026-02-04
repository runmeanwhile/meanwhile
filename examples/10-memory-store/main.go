package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/logger"
	"github.com/darkostanimirovic/meanwhile/pkg/memory"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	// Create persistent file-based memory store
	// This stores conversations to disk so they persist across restarts
	sessionsDir := filepath.Join(os.TempDir(), "meanwhile-sessions")
	memStore, err := memory.NewFileChatStore(sessionsDir)
	if err != nil {
		log.Fatalf("Failed to create memory store: %v", err)
	}
	defer memStore.Close()

	fmt.Printf("Memory store location: %s\n\n", sessionsDir)

	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithMemoryStore(memStore),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// Support agent that can reference past conversations
	support := eng.Agent("Support").
		Prompt("You are a senior technical support engineer. " +
			"You track issues across multiple conversations and reference " +
			"what users told you previously. Be concise and helpful.").
		Model("gpt-4o-mini").
		Build()

	sessionID := "ticket-1234"

	// === FIRST INTERACTION ===
	fmt.Println("=== FIRST INTERACTION ===")
	sess1, err := eng.Session(sessionID).
		Participant(support).
		Protocol(protocol.Solo()).
		Tags("support", "ongoing").
		Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	result1, err := eng.Run(ctx, sess1.ID(),
		message.User("I'm getting intermittent connection drops on port 8080. "+
			"It happens about every 5 minutes."))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nAssistant:", result1.Final)

	// === CHECK MEMORY STATS ===
	stats, err := memStore.Stats(ctx, sessionID, memory.Policy{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n=== MEMORY STATS ===\n")
	fmt.Printf("Total events stored: %d\n", stats.TotalEvents)
	fmt.Printf("Event types breakdown:\n")
	for eventType, count := range stats.EventCounts {
		fmt.Printf("  - %s: %d\n", eventType, count)
	}

	// === BUILD CONTEXT FROM MEMORY ===
	fmt.Printf("\n=== BUILDING CONVERSATION CONTEXT ===\n")
	history, err := memory.BuildConversationContext(ctx, memStore, sessionID,
		memory.WithRecent(10),       // Last 10 messages
		memory.WithTokenLimit(2000), // Stay under 2000 tokens
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Retrieved %d messages from memory\n", len(history))
	for i, msg := range history {
		fmt.Printf("  %d. [%s] %s\n", i+1, msg.Role, truncate(msg.Text(), 60))
	}

	// === SECOND INTERACTION (simulating conversation continuation) ===
	fmt.Println("\n=== SECOND INTERACTION ===")
	fmt.Println("User: I tried restarting the service but drops still occur")

	result2, err := eng.Run(ctx, sess1.ID(),
		message.User("I tried restarting the service but the drops still occur"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nAssistant:", result2.Final)

	// === DEMONSTRATE SESSION PERSISTENCE ===
	fmt.Println("\n=== DEMONSTRATING PERSISTENCE ===")
	fmt.Println("In a real scenario, you could:")
	fmt.Println("1. Close this program")
	fmt.Println("2. Restart it later")
	fmt.Println("3. Load history with BuildConversationContext")
	fmt.Println("4. Continue the conversation with full context")

	// Show how to load context for a resumed conversation
	fmt.Println("\nExample code to resume a conversation:")
	fmt.Println(`
    // Create store with same path
    store, _ := memory.NewFileChatStore("./sessions")
    
    // Load conversation history
    history, _ := memory.BuildConversationContext(
        ctx, store, "ticket-1234",
        memory.WithRecent(20),
    )
    
    // Continue conversation with context
    // The agent automatically has access to previous messages
	`)

	// === THIRD INTERACTION (testing agent's memory) ===
	fmt.Println("\n=== THIRD INTERACTION (Testing Agent Memory) ===")
	fmt.Println("User: What was the frequency I mentioned earlier?")

	result3, err := eng.Run(ctx, sess1.ID(),
		message.User("What was the frequency I mentioned earlier?"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nAssistant:", result3.Final)

	// === FINAL STATS ===
	finalStats, _ := memStore.Stats(ctx, sessionID, memory.Policy{})
	fmt.Printf("\n=== FINAL SESSION STATS ===\n")
	fmt.Printf("Total events: %d\n", finalStats.TotalEvents)
	fmt.Printf("Session duration: %s\n",
		finalStats.LastEvent.Sub(finalStats.FirstEvent).Round(1))
	fmt.Printf("\nMemory persisted to: %s/%s.jsonl\n",
		sessionsDir, sessionID)
}

// truncate shortens a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
