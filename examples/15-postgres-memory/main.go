package main

import (
	"context"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	// Get PostgreSQL connection string from environment
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		connString = "postgresql://postgres:postgres@localhost:5432/meanwhile?sslmode=disable"
		fmt.Println("⚠️  Using default DATABASE_URL:", connString)
		fmt.Println("   Set DATABASE_URL environment variable to customize")
	}

	// Create PostgreSQL-backed memory store
	fmt.Println("📊 Creating PostgreSQL memory store...")
	memStore, err := memory.NewPostgresStore(
		connString,
		memory.WithSchema("meanwhile"),
		memory.WithTable("events"),
		memory.WithAutoMigrate(true),
	)
	if err != nil {
		log.Fatalf("Failed to create memory store: %v", err)
	}
	defer memStore.Close()

	fmt.Println("✅ Connected to PostgreSQL and initialized schema")

	// Create provider
	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Create engine with PostgreSQL memory
	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithMemoryStore(memStore),
		engine.WithLogger(logger.Worklog(os.Stdout)),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// Create support agent
	support := eng.Agent("support").
		Prompt("You are a helpful support agent. Remember the conversation history.").
		Model("gpt-4o-mini").
		Build()

	// === SESSION 1 ===
	sessionID := "ticket-001"
	fmt.Println("=== SESSION 1: First Contact ===")

	sess1, err := eng.Session(sessionID).
		Participant(support).
		Protocol(protocol.Solo()).
		Tags("support").
		Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	result1, err := eng.Run(ctx, sess1.ID(),
		message.User("Hello, I'm having issues with my account"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nSupport:", result1.Final)

	// === CHECK MEMORY ===
	fmt.Println("\n=== MEMORY STATS ===")
	stats, err := memStore.Stats(ctx, sessionID, memory.Policy{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Session ID: %s\n", stats.SessionID)
	fmt.Printf("Total events: %d\n", stats.TotalEvents)
	fmt.Printf("First event: %v\n", stats.FirstEvent)
	fmt.Printf("Last event: %v\n", stats.LastEvent)
	fmt.Println("\nEvent types breakdown:")
	for typ, count := range stats.EventCounts {
		fmt.Printf("  %s: %d\n", typ, count)
	}

	// === SESSION 2: Same conversation (resumed) ===
	fmt.Println("\n=== SESSION 1 CONTINUED: Follow-up ===")

	result2, err := eng.Run(ctx, sess1.ID(),
		message.User("Can you check my email settings?"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nSupport:", result2.Final)

	// === NEW SESSION ===
	sessionID2 := "ticket-002"
	fmt.Println("\n=== SESSION 2: Different Ticket ===")

	sess2, err := eng.Session(sessionID2).
		Participant(support).
		Protocol(protocol.Solo()).
		Tags("support").
		Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start session 2: %v", err)
	}

	result3, err := eng.Run(ctx, sess2.ID(),
		message.User("I need help resetting my password"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nSupport:", result3.Final)

	// === VERIFY SESSION ISOLATION ===
	fmt.Println("\n=== VERIFYING SESSION ISOLATION ===")

	sess1Items, err := memStore.Query(ctx, memory.Query{SessionID: sessionID})
	if err != nil {
		log.Fatal(err)
	}

	sess2Items, err := memStore.Query(ctx, memory.Query{SessionID: sessionID2})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Session 1 (%s) events: %d\n", sessionID, len(sess1Items))
	fmt.Printf("Session 2 (%s) events: %d\n", sessionID2, len(sess2Items))
	fmt.Println("✅ Sessions are properly isolated")

	fmt.Println("\n✅ PostgreSQL memory example completed!")
	fmt.Println("\n💡 Benefits of PostgreSQL:")
	fmt.Println("   - Memory persists across restarts")
	fmt.Println("   - Multi-process access (multiple instances)")
	fmt.Println("   - Scalable to millions of events")
	fmt.Println("   - Advanced querying with SQL")
	fmt.Println("\n💾 Check your database:")
	fmt.Println("   SELECT session_id, event_type, created_at FROM meanwhile.events;")
}
