package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

var randReader io.Reader = rand.Reader

// Builder provides a fluent API for creating agents.
// Use eng.Agent(name) to create a builder, then chain methods
// before calling Build() to register the agent.
//
// Example:
//
//	ticketTool, _ := tool.New("create_ticket", handler)
//
//	agent := eng.Agent("Dale from IT").
//	    Prompt("You are Dale, an IT support tech...").
//	    Model("gpt-4o-mini").
//	    Tools(ticketTool, restartTool).  // Pass instances directly
//	    Build()
//
// Builder manages profile creation and registration automatically.
// Meanwhile... no ceremony, just agents.
type Builder struct {
	engine       engineRef
	name         string
	prompt       string
	model        string
	tools        []string
	params       map[string]any
	profile      *Profile
	outputSchema any
}

// engineRef is an interface to avoid circular dependency with engine package.
type engineRef interface {
	RegisterProfile(Profile)
	RegisterTool(t any) error
	RunAgent(agent Agent, messages ...Message) (Message, error)
	RunAgentWithContext(ctx context.Context, agent Agent, messages ...Message) (Message, error)
}

// NewBuilder creates a new agent builder.
func NewBuilder(engine engineRef, name string) *Builder {
	return &Builder{
		engine: engine,
		name:   name,
		params: make(map[string]any),
	}
}

// Prompt sets the agent's system prompt.
func (b *Builder) Prompt(prompt string) *Builder {
	b.prompt = prompt
	return b
}

// Model sets the model for this agent.
func (b *Builder) Model(model string) *Builder {
	b.model = model
	return b
}

// Tools adds tools to this agent.
// Accepts either tool IDs (strings) or tool instances (any type with ID() method).
// Tool instances are automatically registered with the engine.
//
// Examples:
//
//	// By ID (tools already registered)
//	agent.Tools("tool1", "tool2")
//
//	// By instance (registers + adds)
//	agent.Tools(createTicket, updateTicket, closeTicket)
//
//	// Mixed
//	agent.Tools(myTool, "existing_tool_id")
func (b *Builder) Tools(tools ...any) *Builder {
	for _, t := range tools {
		// Check if it's a string ID
		if id, ok := t.(string); ok {
			b.tools = append(b.tools, id)
			continue
		}

		// Try to register as tool instance
		if err := b.engine.RegisterTool(t); err != nil {
			// Ignore registration errors - tool may already be registered
		}

		// Extract ID and add to agent
		if toolWithID, ok := t.(interface{ ID() string }); ok {
			b.tools = append(b.tools, toolWithID.ID())
		}
	}
	return b
}

// Tool registers a tool with the engine and adds it to this agent.
// This is a convenience method that combines tool registration and agent configuration.
//
// Example:
//
//	submitPlan, _ := tool.New("submit_plan", func(ctx context.Context, plan Plan) (string, error) {
//	    return "Plan submitted", nil
//	})
//
//	agent := eng.Agent("Planner").
//	    Tool(submitPlan).  // Registers AND adds to agent
//	    Build()
func (b *Builder) Tool(t any) *Builder {
	// Register with engine
	if err := b.engine.RegisterTool(t); err != nil {
		// For now, ignore errors - the tool may already be registered
		// In the future, we could add a way to surface this
	}

	// Extract tool ID and add to agent
	if toolWithID, ok := t.(interface{ ID() string }); ok {
		b.tools = append(b.tools, toolWithID.ID())
	}

	return b
}

// Param sets a parameter for this agent.
func (b *Builder) Param(key string, value any) *Builder {
	b.params[key] = value
	return b
}

// OutputSchema sets an optional output schema for this agent.
// When set, all agent responses will be constrained to match this type.
// The schema is automatically derived from the struct using reflection.
//
// Example:
//
//	type Plan struct {
//	    Title string `json:"title"`
//	    Steps []Step `json:"steps"`
//	}
//
//	agent := eng.Agent("Planner").
//	    Prompt("Create implementation plans").
//	    OutputSchema(Plan{}).
//	    Build()
func (b *Builder) OutputSchema(schema any) *Builder {
	b.outputSchema = schema
	return b
}

// Build creates and registers the agent.
func (b *Builder) Build() Agent {
	// Create internal profile if prompt is set
	if b.prompt != "" {
		profileID := generateProfileID(b.name)
		profile := Profile{
			ID:     profileID,
			Name:   b.name,
			Prompt: b.prompt,
			Tools:  b.tools,
		}
		b.engine.RegisterProfile(profile)
		b.profile = &profile
	}

	profileID := ""
	if b.profile != nil {
		profileID = b.profile.ID
	}

	return Agent{
		Name:         b.name,
		Model:        b.model,
		Tools:        b.tools,
		Params:       b.params,
		Profile:      b.profile,
		ProfileID:    profileID,
		OutputSchema: b.outputSchema,
	}
}

// Run is a convenience method that builds the agent and executes it
// in a one-off solo session with the given messages.
// This is equivalent to:
//
//	agent := builder.Build()
//	session := engine.Session("one-off").Participant(agent).Start(ctx)
//	result := session.Run(ctx, messages...)
//
// Example:
//
//	result, err := engine.Agent("Assistant").
//	    Prompt("You are a helpful assistant").
//	    Model("gpt-4").
//	    Run(User("What is 2+2?"))
func (b *Builder) Run(messages ...Message) (Message, error) {
	agent := b.Build()
	return b.engine.RunAgent(agent, messages...)
}

// RunWithContext is a convenience method that builds the agent and executes it
// with a caller-provided context.
func (b *Builder) RunWithContext(ctx context.Context, messages ...Message) (Message, error) {
	agent := b.Build()
	return b.engine.RunAgentWithContext(ctx, agent, messages...)
}

// generateProfileID creates a unique profile ID from the agent name.
func generateProfileID(name string) string {
	// Use random suffix to allow multiple agents with same name
	buf := make([]byte, 4)
	if _, err := io.ReadFull(randReader, buf); err != nil {
		fallback := fmt.Sprintf("%x", time.Now().UnixNano())
		return fmt.Sprintf("profile-%s-%s", sanitizeName(name), fallback)
	}
	suffix := hex.EncodeToString(buf)
	return fmt.Sprintf("profile-%s-%s", sanitizeName(name), suffix)
}

// sanitizeName converts name to a safe ID component.
func sanitizeName(name string) string {
	// Simple sanitization: lowercase, replace spaces with dashes
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result += string(r)
		} else if r == ' ' || r == '-' {
			result += "-"
		}
	}
	return result
}
