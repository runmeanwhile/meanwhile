# Meanwhile Examples

Progressive examples demonstrating the Meanwhile framework, from simple to sophisticated. Each example is a standalone Go program showcasing specific capabilities.

## Prerequisites

```bash
export OPENAI_API_KEY="your-key-here"
```

## Basic Examples

### [01-single-agent](./01-single-agent)
The simplest possible example: one agent, one message, one response. No ceremony.

```bash
cd 01-single-agent && go run main.go
```

### [02-agent-with-tools](./02-agent-with-tools)
Agent with typed tool access. Tools are Go functions with auto-generated schemas.

```bash
cd 02-agent-with-tools && go run main.go
```

### [03-two-agents-handoff](./03-two-agents-handoff)
Two agents with a handoff protocol. Reception triages, specialist executes.

```bash
cd 03-two-agents-handoff && go run main.go
```

### [04-session-with-result](./04-session-with-result)
Sessions with logging, metadata, and structured results. See what happened.

```bash
cd 04-session-with-result && go run main.go
```

## Protocol Examples

### [05-protocol-brainstorming](./05-protocol-brainstorming)
Brainstorming protocol: diverge, interact, and vote with a moderator-led flow.

```bash
cd 05-protocol-brainstorming && go run main.go
```

### [06-protocol-consensus](./06-protocol-consensus)
Consensus protocol: gather proposals, facilitator synthesizes.

```bash
cd 06-protocol-consensus && go run main.go
```

### [07-protocol-adversarial](./07-protocol-adversarial)
Adversarial debate: two agents argue opposing positions, judge synthesizes.

```bash
cd 07-protocol-adversarial && go run main.go
```

### [08-protocol-breakout](./08-protocol-breakout)
Breakout protocol: groups work in parallel, facilitator reconvenes.

```bash
cd 08-protocol-breakout && go run main.go
```

### [09-protocol-as-tool](./09-protocol-as-tool)
Protocols as tools: wrap any protocol as a callable tool for nested collaboration.

```bash
cd 09-protocol-as-tool && go run main.go
```

## Advanced Examples

### [10-memory-store](./10-memory-store)
Memory store for persistent session history and context tracking.

```bash
cd 10-memory-store && go run main.go
```

### [11-hooks-control](./11-hooks-control)
Hooks for runtime control: content filtering, tool auditing, dynamic behavior.

```bash
cd 11-hooks-control && go run main.go
```

### [12-custom-protocol](./12-custom-protocol)
Custom protocol implementation: round-robin escalation workflow.

```bash
cd 12-custom-protocol && go run main.go
```

### [13-full-stack](./13-full-stack)
Everything together: tools, protocols, hooks, memory, logging.

```bash
cd 13-full-stack && go run main.go
```

### [14-semantic-memory](./14-semantic-memory)
Semantic memory with embeddings and relevance-based context building.

```bash
cd 14-semantic-memory && go run main.go
```

### [15-postgres-memory](./15-postgres-memory)
PostgreSQL-backed memory store for multi-process persistence.

```bash
cd 15-postgres-memory && go run main.go
```

### [16-memory-automation](./16-memory-automation)
Automatic memory capture on session close with a default or custom prompt.

```bash
cd 16-memory-automation && go run main.go
```

### [17-planning-mode](./17-planning-mode)
Planning primitive for structured implementation plans.

```bash
cd 17-planning-mode && go run main.go
```

### [18-structured-output](./18-structured-output)
Output schema enforcement for agents and tools.

```bash
cd 18-structured-output && go run main.go
```

### [19-human-turn-based](./19-human-turn-based)
Turn-based human participation in protocols.

```bash
cd 19-human-turn-based && go run main.go
```

### [21-ask-human-tool](./21-ask-human-tool)
Agent-driven human escalation via the ask_human tool.

```bash
cd 21-ask-human-tool && go run main.go
```

### [22-slack-integration](./22-slack-integration)
Outbound Slack delivery for ask_human requests.

```bash
cd 22-slack-integration && go run main.go
```

### [23-webhook-receiver](./23-webhook-receiver)
Inbound webhook receiver for human responses.

```bash
cd 23-webhook-receiver && go run main.go
```

### [24-timeout-handling](./24-timeout-handling)
Scheduled timeout handling for human requests.

```bash
cd 24-timeout-handling && go run main.go
```

## Notes

- Examples use `gpt-4o-mini` for cost efficiency
- Error handling is minimal for clarity—production code should be more defensive
- Prompts use dry, workplace humor circa 2001—adjust to your taste
- Each example builds on concepts from previous ones

## Running All Examples

```bash
for dir in */; do
    if [ -f "$dir/main.go" ]; then
        echo "=== Running $dir ==="
        (cd "$dir" && go run main.go)
        echo ""
    fi
done
```

## Next Steps

- Read [the main README](../README.md) for architecture details
- Check [docs/protocols.md](../docs/protocols.md) for protocol patterns
- Review [pkg/](../pkg/) for API reference

**Meanwhile...** your agents are getting work done.
