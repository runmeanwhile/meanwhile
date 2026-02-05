# Meanwhile

[![CI](https://github.com/runmeanwhile/meanwhile/actions/workflows/ci.yml/badge.svg)](https://github.com/runmeanwhile/meanwhile/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/runmeanwhile/meanwhile.svg)](https://pkg.go.dev/github.com/runmeanwhile/meanwhile)
[![Go Report Card](https://goreportcard.com/badge/github.com/runmeanwhile/meanwhile)](https://goreportcard.com/report/github.com/runmeanwhile/meanwhile)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Meanwhile is a collaboration runtime for AI agents. It's not a task pipeline—it's a workplace. Instead of asking "which agent runs next?", it asks "what kind of collaboration is happening right now?".

The core is intentionally minimal. Collaboration modes live in **protocols** that can evolve independently of the engine, with hook points for dynamic control.

## Why Meanwhile

Most agent frameworks model execution flow. Meanwhile models human collaboration: brainstorming, debate, handoffs, consensus-building, and facilitation. It's built for open-ended reasoning—research, strategy, design critique, and sense-making—not rigid workflows.

**Meanwhile...** agents collaborate naturally, logs read like workplace memos, and protocols feel like office dynamics.

## Features

- **Ergonomic builder APIs** - Agent creation, session setup, and protocol configuration with fluent interfaces
- **Model-first design** - Agents specify models; the engine handles provider resolution automatically
- **Collaboration protocols** - Brainstorming, adversarial debate, consensus, breakouts, handoffs, and more
- **Typed tool construction** - Build tools from Go structs with automatic schema generation
- **Toolkits + guardrails** - Assign bundles of tools with allow/deny policies
- **Structured results** - `RunResult` with transcript, final message, events, and metadata
- **Durable tool calls** - Long-running tool execution can pause and resume safely
- **Clean logging** - Workplace-themed `Worklog` formatter turns events into readable narratives
- **Protocol-as-tool** - Convert any protocol into a callable tool for nested collaboration
- **Streaming events** - Real-time event bus for observability and dynamic control
- **Multi-provider** - OpenAI (with more coming)
- **Memory** - Built-in registries for context and event storage
- **Human escalation** - `ask_human` tool, human participants, and integration routing
- **Inbound responses** - Webhook and Slack command handlers for human replies
- **Timeout handling** - Default timeout policy + pluggable scheduler drivers
- **Request registry** - Map request IDs to sessions (in-memory or Redis)
- **Telemetry** - Integration hooks (Langfuse adapter included)

## Quickstart

### Single Agent (The Simple Path)

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/runmeanwhile/meanwhile/pkg/engine"
    "github.com/runmeanwhile/meanwhile/pkg/logger"
    "github.com/runmeanwhile/meanwhile/pkg/message"
    "github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
    ctx := context.Background()

    // Setup provider and engine with clean logging
    provider, _ := openai.FromEnv()
    eng, _ := engine.New(
        engine.WithProvider(provider),
        engine.WithLogger(logger.Worklog(os.Stdout)),
    )

    // Create agent with builder API
    dale := eng.Agent("Dale from IT").
        Prompt("You are Dale, an IT support tech in 2001. Keep it brief.").
        Model("gpt-4o-mini").
        Build()

    // Run agent with simple shortcut
    result, err := dale.Run(message.User("My printer says PC LOAD LETTER. Help!"))
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("\n=== Dale's Response ===")
    fmt.Println(result.Content)
}
```

**Output:**
```
09:23:41 [Dale from IT] thinking...
09:23:42 [Dale from IT] It means the printer is out of paper...

=== Dale's Response ===
It means the printer is out of paper in the letter-size tray. Load paper and hit continue.
```

### Multi-Agent Collaboration

```go
// Create multiple agents
manager := eng.Agent("Manager").
    Prompt("You are a project manager. Delegate when needed.").
    Model("gpt-4o-mini").
    Build()

engineer := eng.Agent("Engineer").
    Prompt("You are a senior engineer. Provide technical solutions.").
    Model("gpt-4o-mini").
    Build()

// Use session builder for protocols
sess, _ := eng.Session("ISO Audit").
    Participant(manager).
    Participant(engineer).
    Protocol(protocol.Handoff(manager, engineer)).
    Tags("audit", "compliance").
    Start(ctx)

// Run and get structured result
result, _ := eng.Run(ctx, sess.ID(), agent.User("Audit the payroll system"))

fmt.Println("Final:", result.Final)
fmt.Println("Transcript:", len(result.Transcript), "messages")
```

### Session Persistence

Sessions can be rehydrated across processes by providing a `SessionStore`:

```go
store := engine.NewInMemorySessionStore() // or your own implementation
eng, _ := engine.New(engine.WithSessionStore(store))

// Create session in one process
sess, _ := eng.NewSession(ctx, engine.SessionConfig{
    ID:           "ticket-123",
    Protocol:     protocol.Solo(),
    Participants: []protocol.Participant{manager},
})

// Later (another engine instance), the session can be rehydrated
_, _ = eng.Run(ctx, "ticket-123", agent.User("hello"))
```

Stores can also implement `engine.SessionStateStore` to persist pending human input requests and pending tool executions for pause/resume introspection. Pending requests are restored for visibility and routing; resuming them requires the original in-process protocol callbacks.

### Human Escalation & Timeouts

Meanwhile supports human-in-the-loop escalation via the `ask_human` tool, with outbound integrations and inbound response handlers:

```go
moderator := eng.Agent("Moderator").
    Prompt("Call ask_human when you need input.").
    Build()

human := eng.Human("User").
    ContactVia("slack", channelID).
    PreferredChannel("slack").
    Build()

sess, _ := eng.Session("Escalation").
    Participant(moderator).
    Participant(human).
    Protocol(protocol.Solo()).
    TimeoutPolicy(engine.ContinueWithNote("No response received; continuing.")).
    Build(ctx)

_, _ = sess.EnableAskHumanTool()
```

For scheduled timeout handling across processes, provide a timeout scheduler (in-memory or Redis driver) and a request registry (in-memory or Redis) for inbound responders.

Meanwhile also records human request lifecycle state (pending/sent/failed/answered/timed_out) in a `HumanRequestStore` so you can build inbox-style UIs or dashboards:

```go
requests, _ := eng.ListHumanRequests(ctx, engine.HumanRequestFilter{
    Statuses: []engine.HumanRequestStatus{engine.HumanRequestStatusPending},
    Limit:    50,
})

handler := &server.HumanRequestInboxHandler{Engine: eng}
http.Handle("/inbox/human-requests", handler)
```

### Context Management

Control prompt size and tool output growth per run:

```go
_, _ = sess.RunAgent(ctx, manager, protocol.RunRequest{
    Messages: []agent.Message{message.User("Summarize the latest status.")},
    Context: protocol.ContextConfig{
        MaxPromptTokens:    4000,
        RollingWindow:      12,
        MaxToolOutputChars: 1500,
        Summarization: protocol.SummarizationConfig{
            Enabled:         true,
            ThresholdTokens: 6000,
        },
    },
})
```

If the provider implements `provider.TokenEstimator`, the engine uses it for token budgets; otherwise it falls back to the built-in heuristic (4 chars ≈ 1 token).

### Durable Execution Defaults

Configure retries, auto-summarization, and run timeouts globally:

```go
cfg := config.Config{
    Global: config.GlobalConfig{
        RunTimeoutSeconds: 600,
        ProviderRetry: config.ProviderRetryConfig{
            MaxRetries:      5,
            InitialInterval: 1 * time.Second,
            MaxInterval:     10 * time.Second,
            Multiplier:      2,
        },
        Context: config.ContextConfig{
            AutoSummarize: config.AutoSummarizeConfig{
                SummarizeAtTokens: 4000,
                MinKeepMessages:   6,
            },
        },
    },
}

eng, _ := engine.New(
    engine.WithConfig(cfg),
    engine.WithContextSummarizer(mySummarizer),
)
```

### Typed Tools

```go
// Define tool arguments as a struct
type DelegateArgs struct {
    Task   string `json:"task" description:"Task to hand off"`
    Reason string `json:"reason" description:"Why specialist is needed"`
}

// Create typed tool - schema generated automatically
delegateTool := tool.New("delegate", func(ctx context.Context, args DelegateArgs) (string, error) {
    // args is already unmarshaled and validated
    specialist := eng.Agent("Specialist").Model("gpt-4o-mini").Build()
    result, _ := specialist.Run(agent.User(args.Task))
    return result.Final, nil
})

eng.ToolRegistry().Register(delegateTool)
```

### Toolkits + Tool Policy

Toolkits bundle related tools (filesystem, system, MCP, internal APIs) and can be attached to a session with a policy guardrail:

```go
// Register toolkits once on the engine.
fsToolkit, _ := filesystem.New("filesystem", filesystem.Config{
    Roots: []string{"/repo"},
})
_ = eng.RegisterToolkit(fsToolkit)
_ = eng.RegisterToolkit(system.New("system", system.Config{
    Allow: []string{"rg", "go", "git"},
}))

sess, _ := eng.Session("Ops").
    Participant(manager).
    Protocol(protocol.Solo()).
    Toolkits("filesystem", "system").
    ToolPolicy(tool.Policy{
        Mode:       tool.PolicyAllowlist,
        AllowTags:  []string{"filesystem", "read", "write", "system", "path"},
        EnforcedBy: "ops",
        Reason:     "limit to repo operations",
    }).
    Build(ctx)

_ = sess
```

### Durable Tool Calls

Tools can pause a run while waiting on external work (jobs, long-running tasks, human approval) and resume later:

```go
deploy := tool.New("deploy", func(ctx context.Context, call tool.Call) (tool.Result, error) {
    // enqueue job, return awaiting request
    return tool.Await(call, tool.WithContext("waiting for deploy job"))
})

// Later, resume the pending tool request (requestID from RunResult.AwaitingTool or PendingToolRequests)
result := tool.Result{ToolID: "deploy", Output: "deploy complete"}
_, _ = sess.ResumeTool(ctx, requestID, result)
```

### MCP Tools

```go
ctx := context.Background()

// Connect to an MCP server (stdio)
github, _ := eng.MCP("github").
    Command("github-mcp").
    Env("GITHUB_TOKEN", os.Getenv("GITHUB_TOKEN")).
    Register(ctx)

// Use MCP tools by ID (prefix defaults to server name)
agent := eng.Agent("Ops").
    Prompt("Use the GitHub tools when needed.").
    Model("gpt-4o-mini").
    Tools("github.search_repos", "github.create_issue").
    Build()

_ = github
_ = agent
```

### Protocol as Tool

```go
// Convert any protocol into a callable tool
handoffTool := eng.AsTool(
    protocol.Handoff(manager, specialist),
    engine.WithToolName("escalate"),
    engine.WithToolDescription("Escalate complex issues to specialist"),
)

eng.ToolRegistry().Register(handoffTool)

// Now agents can call it naturally
coordinator := eng.Agent("Coordinator").
    Prompt("You coordinate work. Use tools when needed.").
    Model("gpt-4o-mini").
    Tools("escalate").
    Build()
```

## Built-in Protocols

Meanwhile ships with collaboration protocols that mirror workplace dynamics:

- **Solo** - Single-agent execution (default for `Agent.Run()`)
- **Handoff** - Simple delegation from one agent to another  
- **Brainstorming** - Diverge, interact, and vote like a real brainstorm
- **Adversarial** - Debate with opposing positions and optional synthesis
- **Consensus** - Convergent collaboration to reach shared outcomes
- **Breakout** - Parallel group work with synthesis
- **Caucus** - Private per-participant prep before reconvening

All protocols support functional options:

```go
proto := protocol.Brainstorming(
    protocol.WithBrainstormingConcurrency(3),
    protocol.WithBrainstormingInteractionRounds(2),
    protocol.WithBrainstormingVotesPerAgent(3),
)

proto := consensus.Consensus(
    consensus.WithAgenda(agenda.WithScope("Policy-level decision")),
    consensus.WithChair(chair.WithInterventions(0.4, 0.7, 0.9)),
    consensus.WithPulseCheck(pulse.WithMaxConditions(3)),
)
```

## Examples

See [`examples/`](examples/) for complete working demos with 1999-2001 office scenarios. Highlights include:

- **01-single-agent** — "PC LOAD LETTER" troubleshooting
- **05-protocol-brainstorming** — Divergent ideation
- **06-protocol-consensus** — Consensus building
- **19-human-turn-based** — Human participants in protocols
- **21-ask-human-tool** — Agent-driven escalation
- **22-slack-integration** — Outbound Slack delivery
- **23-webhook-receiver** — Inbound webhook responses
- **24-timeout-handling** — Scheduled timeout handling

For the full list, see [`examples/README.md`](examples/README.md) and [`examples/OVERVIEW.md`](examples/OVERVIEW.md).

## Design Principles

- **Protocol-first** - Collaboration modes as first-class abstractions
- **Event-driven** - Streaming events for observability and control
- **Ergonomic by default** - Builder APIs, smart defaults, clean logs
- **Workplace metaphors** - Sessions, participants, facilitators, handoffs
- **Type-safe where it matters** - Object references over string IDs

## Documentation

- [`AGENTS.md`](AGENTS.md) - Quick orientation for coding agents
- [`docs/overview.md`](docs/overview.md) - Meanwhile in 5 minutes + positioning
- [`docs/concepts/collaboration-kit.md`](docs/concepts/collaboration-kit.md) - Shared primitives
- [`docs/concepts/protocols.md`](docs/concepts/protocols.md) - Protocol assemblies
- [`docs/concepts/tools.md`](docs/concepts/tools.md) - Toolkits, policy guardrails, durable tools
- [`docs/concepts/hooks.md`](docs/concepts/hooks.md) - Hooks as interrupts
- [`docs/guides/build-a-protocol.md`](docs/guides/build-a-protocol.md) - Compose a new protocol
- [`docs/guides/facilitation.md`](docs/guides/facilitation.md) - Agenda + chair tuning
- [`docs/observability.md`](docs/observability.md) - Events, logging, telemetry

## Go Version

This repository targets **Go 1.24+**. Ensure Go 1.24 is in your PATH if multiple versions are installed.

## License

Meanwhile is licensed under the Apache License 2.0. See `LICENSE`.

Meanwhile is a trademark of its owner. See `TRADEMARK.md` for usage guidance.

## Future: CLI and Studio

The roadmap includes a CLI for session management and a web-based Studio for visual collaboration. See [ROADMAP.md](ROADMAP.md) for details on planned features.

---

**Meanwhile...** agents collaborate, protocols orchestrate, and logs tell the story. ✨
