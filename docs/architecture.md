# Architecture

Meanwhile is organized as a minimal core runtime with extensible registries. The design philosophy prioritizes ergonomics, type safety, and workplace metaphors over enterprise abstraction.

## Design Philosophy

### Collaboration Over Orchestration

Most agent frameworks model task execution. Meanwhile models **human collaboration patterns**: brainstorming, debate, handoffs, consensus-building, facilitation. The question isn't "which agent runs next?" but "what kind of collaboration is happening?"

### Model-First, Not Provider-First

Agents specify **models** (e.g., "gpt-4o-mini"), not providers. The engine resolves which provider serves that model. This mirrors reality: developers think "I need GPT-4" not "I need the OpenAI provider."

```go
// Natural - agent needs a model
agent := eng.Agent("Dale").Model("gpt-4o-mini").Build()

// Not: agent needs a "provider ID" string reference
```

### Objects Over Strings

String-based references create fragile APIs. Meanwhile passes objects:

```go
// Good - compile-time safety
eng.Session("audit").Participant(manager).Start(ctx)

// Avoid - string references break easily  
eng.Session("audit").ParticipantID("manager-id").Start(ctx)
```

### Ergonomics by Default

Complex tasks should be simple; advanced use cases should be possible. The agent builder, session builder, and `Agent.Run()` shortcut eliminate boilerplate for common patterns while preserving full control when needed.

## Core Concepts

### Engine
The runtime that owns sessions, registries, and the event bus. Provides builder methods (`Agent()`, `Session()`) and generic helpers (`AsTool()` for protocol-to-tool conversion).

### Session  
A collaboration instance with participants, optional facilitator, protocol, and metadata (name, tags, groups). Sessions emit events that can be logged, monitored, or used for control flow. For multi-process continuity, provide a `SessionStore` so sessions can be persisted and rehydrated by ID. Stores can also implement `SessionStateStore` to persist pending human input requests. Pending requests are rehydrated for visibility and routing; resuming them requires in-process protocol callbacks.

### Protocol
Collaboration rules that determine how agents interact: turn-taking, parallel execution, debate structure, consensus mechanisms, etc. Protocols are first-class and composable—they can even be wrapped as tools.

### Collaboration Kit
Reusable building blocks (Agenda, Chair, Roundtable, PulseCheck, Minutes, Interrupts) that protocols compose. This keeps the core runtime small while making collaboration behavior consistent and recognizable.

### Agent
An AI participant with a name, model, prompt (profile), and optional tools. Created via the agent builder:

```go
agent := eng.Agent("Name").
    Prompt("System instructions").
    Model("gpt-4o-mini").
    Tools("tool1", "tool2").
    Build()
```

### Profile
Reusable agent persona and prompt instructions. Managed internally by the agent builder—users rarely interact with profiles directly.

### Tool
A callable capability with JSON schema and implementation. Can be created from Go structs using `tool.New[T]()` for automatic schema generation, or manually with `tool.Func` for dynamic cases.

### Provider  
LLM integration (OpenAI currently, with more planned). Providers are registered as objects, not strings:

```go
eng, _ := engine.New(
    engine.WithProvider(openai.New(apiKey)),
)
```

### Hook
Intercepts lifecycle points (pre-message, pre-turn, post-turn, pre-tool, post-tool, stop). Hooks can block, modify, or observe actions for control flow, guardrails, or interrupts.

### Memory
Store for event logs, retrieval, and summaries. Abstraction allows pluggable backends.

### Integration
Outbound delivery for human escalation requests (Slack, email, webhook) with routing based on contact preferences.

### Request Registry
Maps human request IDs to session IDs so inbound responders (webhook/Slack command) can locate the correct session. Drivers are pluggable (in-memory or Redis).

### Scheduler
Pluggable job scheduling drivers used for human request timeouts (in-memory or Redis).

### Skill
SKILL.md-based instruction packs that provide reusable capabilities and context to agents.

## Event Model

Everything is an event. Providers, tools, protocols, and hooks emit events. The event bus enables:

- **Logging** - Clean workplace-style logs via `logger.Worklog()`
- **Observability** - Real-time monitoring of agent behavior
- **Control flow** - Dynamic decisions based on runtime state
- **Telemetry** - Integration with platforms like Langfuse

Events flow through the bus to subscribers, which can render them for CLI output, SSE streams, or other transports.

## API Ergonomics

### Agent Builder

Create agents fluently without manual profile management:

```go
agent := eng.Agent("Name").
    Prompt("Instructions").
    Model("gpt-4o-mini").
    Tools("tool1", "tool2").
    Skills("skill1").
    Param("temperature", 0.7).
    Build()
```

The builder registers profiles automatically. No ID fields, no string references—just names.

### Session Builder

Set up multi-agent sessions with a fluent API:

```go
sess, _ := eng.Session("Planning").
    Participant(alice).
    Participant(bob).
    Protocol(protocol.Consensus()).
    Tags("planning", "q1").
    Facilitator(manager).
    Start(ctx)
```

Defaults to solo protocol if none specified.

### Agent.Run() Shortcut

For simple single-agent tasks:

```go
result, _ := agent.Run(agent.User("Task description"))
fmt.Println(result.Final)
```

Creates an ephemeral solo session internally, runs the agent, and returns structured results.

### Protocol as Tool

Convert any protocol into a callable tool:

```go
handoffTool := eng.AsTool(
    protocol.Handoff(manager, specialist),
    engine.WithToolName("escalate"),
    engine.WithToolDescription("Escalate to specialist"),
)

eng.ToolRegistry().Register(handoffTool)
```

Works with all protocols—no protocol-specific wrappers needed.

### Typed Tools

Define tools from Go structs with automatic schema generation:

```go
type Args struct {
    Task string `json:"task" description:"Task to perform"`
}

tool := tool.New("name", func(ctx context.Context, args Args) (string, error) {
    // args already unmarshaled and validated
    return "result", nil
})
```

### Toolkits + Policy

Toolkits bundle related tools (filesystem, system, MCP, internal APIs). Sessions can attach toolkits and enforce guardrails with allow/deny policies:

```go
sess, _ := eng.Session("Ops").
    Participant(agent).
    Protocol(protocol.Solo()).
    Toolkits("filesystem", "system").
    ToolPolicy(tool.Policy{
        Mode:      tool.PolicyAllowlist,
        AllowTags: []string{"filesystem", "read", "write"},
    }).
    Build(ctx)
```

Tools can also signal long-running work via `tool.Await(...)`, and sessions can resume later with `ResumeTool(...)`.

### Structured Results

`Run()` returns `*RunResult` with:

```go
type RunResult struct {
    Final      string              // Last assistant message
    Transcript []agent.Message     // Full conversation
    Events     []event.Event       // Raw events
    Metadata   map[string]any      // Protocol-specific data
}
```

## Package Layout

```
cmd/                  optional CLI (future)
internal/             internal helpers
pkg/
  agent/              agent identity, messages, profiles, builder
  config/             configuration loading
  engine/             core runtime, sessions, event bus
  event/              event types and bus
  hook/               hook interfaces and registry
  integration/        outbound human escalation integrations
  logger/             logging abstraction (Worklog formatter)
  memory/             memory store interfaces
  requestregistry/    request registry drivers (e.g., Redis)
  protocol/           protocol implementations and registry
  provider/           provider interfaces (OpenAI implementation)
  mcp/                MCP integration + proxy tools
  scheduler/          scheduling drivers for timeouts
  server/             inbound webhook/command handlers
  skill/              skill loader and registry
  telemetry/          telemetry abstraction (Langfuse adapter)
  tool/               tool interfaces, registry, typed tools
  toolkit/            tool bundles + policy tagging
```

## Control Flow

1. User message enters a session via `eng.Run(ctx, sessionID, message)`
2. Pre-message hooks can block or modify the message
3. The protocol determines which agents run and in what order
4. Each agent execution:
   - Resolves model → provider
   - Constructs messages (profile + conversation)
   - Streams from provider, emitting events
   - Handles tool calls if any
5. Tool execution:
   - Pre-tool hooks can intercept
   - Tool policy/allowlist enforced per session/run
   - Tool runs with emitter for streaming output
   - Tools can pause with `tool.Await` for long-running work
   - Results fed back to agent
   - Post-tool hooks observe
6. Protocol completes when collaboration finishes
7. `Run()` returns `RunResult` with transcript and metadata

All steps emit events to the bus for logging, observability, and control.

## Workplace Metaphors

The API uses workplace terminology:

- **Session** - A meeting or collaboration instance
- **Participants** - Team members (agents) in the session
- **Facilitator** - Optional session leader for consensus/synthesis protocols
- **Handoff** - Delegation from one agent to another
- **Protocol** - The type of collaboration happening
- **Worklog** - Clean, narrative-style event logging

This vocabulary makes the framework feel cohesive and domain-appropriate for multi-agent collaboration.
