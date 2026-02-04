# Structured Output

This example demonstrates three patterns for getting type-safe, structured responses from agents.

## Patterns

### 1. Agent-Level Schema
Agent is configured with an output schema - **all responses must conform**:
```go
agent := eng.Agent("Extractor").
    OutputSchema(ContactInfo{}).
    Build()
```
Best for: Single-purpose extraction/classification agents.

### 2. Tool Pattern (Recommended)
Agent calls a tool that accepts structured input - Meanwhile **already infers the schema**:
```go
tool.New("submit_plan", func(ctx context.Context, plan Plan) (string, error) {
    // plan is type-safe! No parsing needed
    return "Plan submitted", nil
})
```
Best for: Meanwhile's collaboration model. Structured data flows as tool arguments.

### 3. RunRequest-Level Override
Flexible agent with schema specified per invocation:
```go
sess.RunAgent(ctx, agent, protocol.RunRequest{
    Messages:     []agent.Message{msg},
    OutputSchema: SimpleResponse{}, // Just this call
})
```
Best for: Agents that need mixed output types depending on context.

## Key Insight

**Tool pattern eliminates the customer's pain point:**
- ✅ No raw JSON in system prompts
- ✅ Schema inferred from Go structs (like Pydantic)
- ✅ Works today with existing Meanwhile infrastructure
- ✅ Natural for Meanwhile's action-oriented design

Agent-level and run-level schemas are available when tools don't fit the use case.

## Run
```bash
go run main.go
```
