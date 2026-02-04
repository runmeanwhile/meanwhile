// Package agent defines agent identities, messages, and configuration.
//
// Agents are the participants in Meanwhile sessions—AI team members with names,
// models, prompts (profiles), and optional tools. Use the agent builder for
// ergonomic creation:
//
//	dale := eng.Agent("Dale from IT").
//	    Prompt("You are Dale, an IT support tech...").
//	    Model("gpt-4o-mini").
//	    Build()
//
// For quick one-off tasks, use Builder.Run():
//
//	result, _ := eng.Agent("Helper").
//	    Prompt("You are helpful").
//	    Run(message.User("Hello"))
//
// For structured output, set an optional output schema:
//
//	type ContactInfo struct {
//	    Name  string `json:"name"`
//	    Email string `json:"email"`
//	}
//
//	extractor := eng.Agent("Extractor").
//	    Prompt("Extract contact information").
//	    OutputSchema(ContactInfo{}).
//	    Build()
//
// The schema is automatically inferred from the struct and passed to the LLM provider.
//
// Tools can be added in multiple ways:
//
//	// Single tool
//	agent.Tool(submitPlan)
//
//	// Multiple tools
//	agent.Tools(ticketTool, restartTool, searchTool)
//
//	// Mix instances and IDs
//	agent.Tools(myTool, "existing_tool_id")
//
// Both .Tool() and .Tools() automatically register tools with the engine.
//
// Meanwhile... agents collaborate naturally, without ceremony.
package agent
