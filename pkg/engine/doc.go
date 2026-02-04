// Package engine provides the core session runtime for Meanwhile.
//
// The engine manages sessions, protocols, and the event bus. It owns registries
// for providers, tools, hooks, and agents (profiles). Think of the engine
// as the workplace itself—where sessions (meetings) happen, agents (team members)
// collaborate, and protocols (collaboration styles) orchestrate interactions.
//
// Create an engine with providers and optional logger:
//
//	eng, _ := engine.New(
//	    engine.WithProvider(openai.New(apiKey)),
//	    engine.WithLogger(logger.Worklog(os.Stdout)),
//	)
//
// Use builders for ergonomic agent and session creation:
//
//	agent := eng.Agent("Name").Prompt("...").Model("gpt-4o-mini").Build()
//	sess, _ := eng.Session("Meeting").Participant(agent).Start(ctx)
//
// Meanwhile... the engine orchestrates collaboration while staying out of your way.
package engine
