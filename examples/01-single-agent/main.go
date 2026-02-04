package main

import (
	"fmt"
	"log"

	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
	// Provider and engine setup
	provider, _ := openai.FromEnv()
	eng, _ := engine.New(engine.WithProvider(provider))

	// Run agent directly
	result, err := eng.Agent("Dale").
		Prompt("You are Dale, IT support circa 2001. Brief answers only. You've seen every obscure error code twice.").
		Model("gpt-4o-mini").
		Run(message.User("My screen says 'keyboard not found'. What do I do?"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result.Text())
}
