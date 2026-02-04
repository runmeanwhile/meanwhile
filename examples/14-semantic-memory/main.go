// Example 14: Semantic Memory - Intelligent Context Retrieval
//
// This example demonstrates semantic memory capabilities:
// 1. Embedding-based event storage
// 2. Semantic search by meaning (not just keywords)
// 3. Smart context building (recent + relevant)
// 4. Relevance scoring and filtering
//
// Use Case: Building a chatbot that remembers past conversations
// and can retrieve relevant context even when exact keywords don't match.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
)

func main() {
	ctx := context.Background()

	// Check for API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable required")
	}

	// Set up embeddings provider
	embedder := memory.NewOpenAIEmbeddings(
		apiKey,
		memory.WithModel("text-embedding-3-small"),
	)

	// Create semantic memory store
	memStore := memory.NewSemanticStore(embedder)

	sessionID := "semantic-demo"

	// Simulate a conversation history
	fmt.Println("=== Building Conversation History ===")

	conversations := []struct {
		user      string
		assistant string
	}{
		{
			user:      "What is OAuth 2.0?",
			assistant: "OAuth 2.0 is an authorization framework that enables applications to obtain limited access to user accounts on an HTTP service.",
		},
		{
			user:      "Tell me about JWT tokens",
			assistant: "JWT (JSON Web Token) is a compact, URL-safe means of representing claims between two parties. It's commonly used for authentication.",
		},
		{
			user:      "What's the difference between authentication and authorization?",
			assistant: "Authentication verifies WHO you are (identity), while authorization determines WHAT you can access (permissions).",
		},
		{
			user:      "How does HTTPS work?",
			assistant: "HTTPS uses TLS/SSL to encrypt data in transit between client and server, ensuring confidentiality and integrity.",
		},
		{
			user:      "What are the best practices for password storage?",
			assistant: "Best practices include: using bcrypt or Argon2 for hashing, adding salt, never storing plaintext passwords, and using key stretching.",
		},
	}

	// Store conversation history
	for i, conv := range conversations {
		// Store user message
		userEv := event.New(event.AgentMessageComplete, sessionID, map[string]any{
			"message": map[string]any{
				"role":    "user",
				"content": conv.user,
			},
		})
		if err := memStore.Append(ctx, sessionID, userEv); err != nil {
			log.Fatalf("Failed to append user event: %v", err)
		}

		// Store assistant response
		assistantEv := event.New(event.AgentMessageComplete, sessionID, map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": conv.assistant,
			},
		})
		if err := memStore.Append(ctx, sessionID, assistantEv); err != nil {
			log.Fatalf("Failed to append assistant event: %v", err)
		}

		fmt.Printf("%d. User: %s\n", i+1, conv.user)
		fmt.Printf("   Assistant: %s\n\n", conv.assistant)
	}

	// Demonstrate semantic search
	fmt.Println("\n=== Semantic Search Examples ===")

	// Example 1: Search by concept (not exact keywords)
	queryText := "security credentials"
	fmt.Printf("\n1. Searching for '%s' (semantic, not keyword match):\n", queryText)
	results, err := memStore.QuerySemantic(ctx, memory.SemanticQuery{
		SessionID: sessionID,
		Text:      queryText,
		Limit:     3,
		Threshold: 0.3, // Lower threshold to see more results
	})
	if err != nil {
		log.Fatalf("Semantic query failed: %v", err)
	}

	if len(results) == 0 {
		fmt.Println("   (No results found - try lowering the similarity threshold)")
	} else {
		for _, result := range results {
			msg, _ := extractMessageFromEvent(result.Item.Event)
			fmt.Printf("   [Score: %.3f] %s: %s\n", result.Score, msg.Role, msg.Text())
		}
	}

	// Example 2: Search for authentication-related discussions
	fmt.Println("\n2. Searching for 'token-based authentication':")
	results, err = memStore.QuerySemantic(ctx, memory.SemanticQuery{
		SessionID: sessionID,
		Text:      "token-based authentication",
		Limit:     3,
		Threshold: 0.3,
	})
	if err != nil {
		log.Fatalf("Semantic query failed: %v", err)
	}

	if len(results) == 0 {
		fmt.Println("   (No results found - try lowering the similarity threshold)")
	} else {
		for _, result := range results {
			msg, _ := extractMessageFromEvent(result.Item.Event)
			fmt.Printf("   [Score: %.3f] %s: %s\n", result.Score, msg.Role, msg.Text())
		}
	}

	// Example 3: Smart context building
	fmt.Println("\n=== Smart Context Building ===")

	builder := memory.NewSemanticContextBuilder(memStore)

	query := "Tell me about authentication methods"
	fmt.Printf("New Question: %s\n\n", query)

	// Build context combining recent + semantically relevant messages
	contextMessages, err := builder.BuildSemanticContext(
		ctx, sessionID,
		memory.WithQuery("authentication security tokens"),
		memory.WithRecentMessages(2),        // Include 2 most recent
		memory.WithRelevantMessages(3),      // Plus 3 most relevant
		memory.WithSimilarityThreshold(0.3), // Lower threshold for demo
		memory.WithDeduplication(true),      // Remove duplicates
	)
	if err != nil {
		log.Fatalf("Failed to build semantic context: %v", err)
	}

	fmt.Println("Smart Context Retrieved:")
	fmt.Printf("  Total messages: %d\n", len(contextMessages))
	fmt.Println("\n  Context includes:")
	for i, msg := range contextMessages {
		preview := msg.Text()
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		fmt.Printf("    %d. [%s] %s\n", i+1, msg.Role, preview)
	}

	// Demonstrate semantic context usage
	fmt.Println("\n=== Using Semantic Context in Agent Prompts ===")

	// Build semantic context for a new query
	newQuery := "What are the main security concerns when implementing authentication?"
	fmt.Printf("New Query: %s\n\n", newQuery)

	semanticContext, err := builder.BuildSemanticContext(
		ctx, sessionID,
		memory.WithQuery("authentication security best practices"),
		memory.WithRecentMessages(3),
		memory.WithRelevantMessages(5),
		memory.WithSimilarityThreshold(0.3),
	)
	if err != nil {
		log.Fatalf("Failed to build semantic context: %v", err)
	}

	fmt.Printf("Context for Agent (%d messages):\n", len(semanticContext))
	for i, msg := range semanticContext {
		preview := msg.Text()
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		fmt.Printf("  %d. [%s] %s\n", i+1, msg.Role, preview)
	}

	fmt.Println("\nNote: In a real application, these messages would be prepended to the agent's prompt.")
	fmt.Println("The agent would then have access to relevant historical context when answering.")

	// Show statistics
	fmt.Println("\n=== Memory Statistics ===")
	stats, err := memStore.Stats(ctx, sessionID, memory.Policy{})
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	fmt.Printf("Total Events: %d\n", stats.TotalEvents)
	fmt.Println("Event Types:")
	for eventType, count := range stats.EventCounts {
		fmt.Printf("  %s: %d\n", eventType, count)
	}
	fmt.Printf("First Event: %s\n", stats.FirstEvent.Format("15:04:05"))
	fmt.Printf("Last Event: %s\n", stats.LastEvent.Format("15:04:05"))
}

// extractMessageFromEvent extracts a message from an event payload.
func extractMessageFromEvent(ev event.Event) (agent.Message, bool) {
	if payload, ok := ev.Payload.(map[string]any); ok {
		if msgData, ok := payload["message"].(map[string]any); ok {
			return agent.MessageFromMap(msgData), true
		}
	}
	return agent.Message{}, false
}
