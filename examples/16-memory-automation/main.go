package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
	ctx := context.Background()

	sessionsDir := filepath.Join(os.TempDir(), "meanwhile-memory-automation")
	store, err := memory.NewFileChatStore(sessionsDir)
	if err != nil {
		log.Fatalf("Failed to create memory store: %v", err)
	}
	defer store.Close()

	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	eng, err := engine.New(
		engine.WithProvider(provider),
		engine.WithMemoryStore(store),
		engine.WithMemoryAutomation(config.MemoryAutomationConfig{
			Enabled:    true,
			ProviderID: provider.ID(),
			Model:      "gpt-5-mini",
			Context: config.MemoryAutomationContext{
				RecentMessages: 20,
				TokenLimit:     4000,
			},
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	agent := eng.Agent("Support").
		Prompt("You are a calm support engineer. Be concise and helpful.").
		Model("gpt-5-mini").
		Build()

	customPrompt := "Record durable memory as key: value lines. Avoid secrets."

	sess, err := eng.Session("memory-automation-demo").
		Participant(agent).
		Protocol(protocol.Solo()).
		Metadata(engine.MemoryAutomationPromptKey, customPrompt).
		Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	_, err = eng.Run(ctx, sess.ID(), message.User("Our support backlog spikes every Friday. We want to track the pattern."))
	if err != nil {
		log.Fatal(err)
	}
	_, err = eng.Run(ctx, sess.ID(), message.User("Decision: review load tests weekly and write a handoff runbook."))
	if err != nil {
		log.Fatal(err)
	}

	if err := eng.CloseSession(ctx, sess.ID()); err != nil {
		log.Fatalf("Failed to close session: %v", err)
	}

	items, err := store.Query(ctx, memory.Query{SessionID: sess.ID(), Types: []event.Type{event.MemorySummary}})
	if err != nil {
		log.Fatalf("Failed to query memory summary: %v", err)
	}

	if len(items) == 0 {
		fmt.Println("No memory summary found.")
		return
	}

	payload, ok := items[len(items)-1].Event.Payload.(string)
	if !ok {
		fmt.Printf("Memory summary payload type: %T\n", items[len(items)-1].Event.Payload)
		return
	}

	fmt.Println("=== Memory Summary ===")
	fmt.Println(payload)
}
