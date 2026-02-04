# Planning Mode

Demonstrates the planning primitive from the Collaboration Kit.

## What it shows

- Creating structured implementation plans with a planning agent
- Storing plans in session metadata
- Using plans as context for execution
- Two-phase workflow: planning → execution
- Model switching (GPT-4o for planning, GPT-4o-mini for execution)

## Key concepts

**Planning primitive**: Creates structured Plan documents, doesn't execute them.  
**Storage strategies**: Return, session metadata, memory store, or custom.  
**Separation of concerns**: Planner creates plan and exits. Main agent/protocol handles execution.

## Run

```bash
export OPENAI_API_KEY="your-key"
go run main.go
```

## Pattern

```go
// 1. Create planner
planner := planning.New(
    planningAgent,
    planning.WithStorage(planning.StorageSession, "plan"),
)

// 2. Create plan
plan, _ := planner.CreatePlan(ctx, sess, message.User("Task"))

// 3. Execute using plan as context
planContext := message.System(fmt.Sprintf("Follow this plan:\n%s", plan.Format()))
result, _ := eng.Run(ctx, sess.ID(), planContext)
```

## Inspired by Claude Code

This implements a planning pattern similar to Claude Code's `/plan` command:
- Planning agent (Sonnet-class) creates the plan
- Main agent (Haiku-class) executes using plan as context
- Plan is a structured artifact, not just conversation
- Model switching for cost efficiency

Meanwhile's approach:
- Planning is a Collab Kit primitive, not protocol
- Reuses existing session/memory/event infrastructure
- Flexible storage (session, memory, custom)
- Composable with other collab kit components
