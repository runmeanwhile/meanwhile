# Meanwhile Examples - Quick Reference

## Overview
24 progressive examples demonstrating the Meanwhile framework from basic to advanced.

## Quick Start

```bash
# Set API key
export OPENAI_API_KEY="your-key"

# Run any example
cd examples/01-single-agent && go run main.go
```

## Example Categories

### 🎯 Basics (01-04)
- **01-single-agent**: Minimal API - one agent, one message
- **02-agent-with-tools**: Typed tools with auto-generated schemas
- **03-two-agents-handoff**: Two-agent escalation pattern
- **04-session-with-result**: Logging, metadata, structured results

### 🤝 Protocols (05-09)
- **05-protocol-brainstorming**: Parallel idea generation
- **06-protocol-consensus**: Proposal gathering + synthesis
- **07-protocol-adversarial**: Debate with judge synthesis
- **08-protocol-breakout**: Parallel groups + reconvene
- **09-protocol-as-tool**: Protocols as callable tools
- **25-protocol-brainstorming-lab**: Context intake + reframing + evidence-gated finalists

### ⚡ Advanced (10-19)
- **10-memory-store**: Persistent session history
- **11-hooks-control**: Runtime hooks for filtering/auditing
- **12-custom-protocol**: Custom round-robin protocol
- **13-full-stack**: Everything together
- **14-semantic-memory**: Embedding-backed memory + semantic search
- **15-postgres-memory**: PostgreSQL-backed memory store
- **16-memory-automation**: Automatic memory summary on session close
- **17-planning-mode**: Planning primitive for structured implementation plans
- **18-structured-output**: Output schema enforcement for agents/tools
- **19-human-turn-based**: Turn-based human participation in protocols
- **21-ask-human-tool**: Agent-driven human escalation via ask_human
- **22-slack-integration**: Outbound Slack delivery for human requests
- **23-webhook-receiver**: Inbound webhook receiver for human responses
- **24-timeout-handling**: Scheduled timeout handling for human requests

## Key Patterns

### Simplest Agent Run
```go
result, _ := eng.Agent("Name").
    Prompt("Instructions").
    Model("gpt-4o-mini").
    Run(message.User("Question"))
```

### Session with Protocol
```go
sess, _ := eng.Session("Meeting").
    Participants(agent1, agent2).
    Protocol(protocol.Handoff(agent1, agent2)).
    Start(ctx)

result, _ := eng.Run(ctx, sess.ID(), message.User("Task"))
```

### Typed Tool
```go
type Args struct {
    Field string `json:"field" description:"Description"`
}

tool, _ := tool.New[Args, string]("name", func(ctx context.Context, args Args) (string, error) {
    return "result", nil
})
```

## Building Blocks

| Component | Examples Using It |
|-----------|------------------|
| Agent Builder | All |
| Sessions | 03, 04, 05-13, 17, 19 |
| Protocols | 03, 05-09, 12, 13, 17, 19 |
| Tools | 02, 09, 11, 13 |
| Logging | 04, 05-08, 10, 12, 13, 17, 19 |
| Memory | 10, 13-17 |
| Hooks | 11, 13 |
| Custom Protocol | 12 |
| Planning | 17 |

## Prompts Theme
All examples use dry, workplace humor circa 2001:
- IT support with war stories
- ISO consultant paranoia
- Enterprise software fatalism
- Committee dynamics
- Incident response procedures

**Meanwhile...** your examples are compiling.
