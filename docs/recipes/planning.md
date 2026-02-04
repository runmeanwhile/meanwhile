# Planning

The planning primitive creates structured implementation plans using a planning agent.

## Basic Usage

```go
import "github.com/runmeanwhile/meanwhile/pkg/collab/planning"

// Create planner
planner := planning.New(planningAgent)

// Create plan
plan, err := planner.CreatePlan(ctx, sess, message.User("Build REST API"))
if err != nil {
    log.Fatal(err)
}

// Use the plan
fmt.Println(plan.Format())
for _, step := range plan.Steps {
    fmt.Printf("- %s: %s\n", step.Title, step.Description)
}
```

## Storage Strategies

### Return Only (Default)

```go
planner := planning.New(agent)
plan, _ := planner.CreatePlan(ctx, sess, msg)
// Plan is just returned, not stored
```

### Session Metadata

```go
planner := planning.New(
    agent,
    planning.WithStorage(planning.StorageSession, "plan"),
)
plan, _ := planner.CreatePlan(ctx, sess, msg)

// Later: retrieve from session
storedPlan, ok := planning.GetPlan(sess, "plan")
```

### Memory Store

```go
planner := planning.New(
    agent,
    planning.WithStorage(planning.StorageMemory, ""),
)
plan, _ := planner.CreatePlan(ctx, sess, msg)
// Plan persisted in memory store as event
```

### Custom Storage

```go
planner := planning.New(
    agent,
    planning.WithCustomStorage(func(ctx context.Context, sess protocol.Session, p *planning.Plan) error {
        // Store in database, file, etc.
        return db.SavePlan(ctx, p)
    }),
)
```

## Execution Patterns

### Pattern 1: Manual Step Execution

```go
plan, _ := planner.CreatePlan(ctx, sess, message.User("Build feature"))

for i, step := range plan.Steps {
    prompt := fmt.Sprintf("Execute step %d: %s\n\n%s", 
        i+1, step.Title, step.Description)
    
    result, _ := eng.Run(ctx, sess.ID(), message.User(prompt))
    fmt.Printf("Step %d complete: %s\n", i+1, result.Final)
}
```

### Pattern 2: Plan as Context

```go
plan, _ := planner.CreatePlan(ctx, sess, message.User("Task"))

// Give plan to execution agent as context
planContext := message.System(fmt.Sprintf(
    "Execute this implementation plan:\n\n%s",
    plan.Format(),
))

result, _ := eng.Run(ctx, sess.ID(), planContext)
```

### Pattern 3: Protocol with Planning

```go
type PlannedProtocol struct {
    planner *planning.Planner
    executor agent.Agent
}

func (p *PlannedProtocol) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
    // Create plan
    plan, _ := p.planner.CreatePlan(ctx, sess, msg)
    
    // Execute with plan in system context
    planMsg := message.System(plan.Format())
    resp, _ := sess.RunAgent(ctx, p.executor, protocol.RunRequest{
        Messages: []agent.Message{msg},
        SystemMessages: []agent.Message{planMsg},
    })
    
    return sess.Emit(event.New(event.ProtocolAction, sess.ID(), map[string]any{
        "plan": plan,
        "result": resp.Text(),
    }))
}
```

## Model Switching Pattern

Use different models for planning vs execution:

```go
// Planning agent with capable model
planner := eng.Agent("Architect").
    Prompt("Create detailed implementation plans.").
    Model("gpt-4o").  // or "claude-sonnet-4"
    Build()

// Execution agent with faster model
executor := eng.Agent("Developer").
    Prompt("Execute implementation steps.").
    Model("gpt-4o-mini").  // or "claude-haiku-4"
    Build()

// Create plan with architect
p := planning.New(planner)
plan, _ := p.CreatePlan(ctx, sess, message.User("Build API"))

// Execute with developer
planContext := message.System(plan.Format())
result, _ := executor.Run(planContext)
```

## Custom System Prompt

```go
customPrompt := `You are an expert software architect.
Create implementation plans with these requirements:
- Break complex tasks into 5-10 steps
- Include dependencies between steps
- Be specific and actionable
- Output JSON with: title, summary, steps[]`

planner := planning.New(
    agent,
    planning.WithSystemPrompt(customPrompt),
)
```

## Plan Structure

Plans are parsed from JSON output:

```json
{
    "title": "REST API Implementation",
    "summary": "Build a RESTful API with authentication",
    "steps": [
        {
            "id": "step-1",
            "title": "Setup project structure",
            "description": "Initialize Go project with required dependencies",
            "dependencies": []
        },
        {
            "id": "step-2",
            "title": "Create database schema",
            "description": "Design and implement user tables",
            "dependencies": ["step-1"]
        }
    ]
}
```

## Composing with Other Collab Kit Components

### With Agenda

```go
// Agenda refines scope, then create plan
agenda := agenda.New(agenda.WithScope("Feature implementation"))
scope, _ := agenda.RefineScope(ctx, sess, msg)

planner := planning.New(agent)
plan, _ := planner.CreatePlan(ctx, sess, message.User(scope))
```

### With Minutes

```go
mins := minutes.New()

// Create plan
plan, _ := planner.CreatePlan(ctx, sess, msg)
mins.Add("plan", plan.Format())

// Execute and capture results
for _, step := range plan.Steps {
    result, _ := executeStep(ctx, sess, step)
    mins.Add(step.ID, result)
}

return sess.Emit(event.New(event.ProtocolAction, sess.ID(), mins.Payload()))
```

## Events

Planning emits events for observability:

```go
// Emitted events:
- planning.started       // Planning begins
- planning.plan_created  // Plan successfully created
- planning.plan_stored   // Plan persisted (memory storage)
```

Subscribe to events:

```go
sess.bus.Subscribe(func(ev event.Event) {
    if ev.Type == "planning.plan_created" {
        plan := ev.Payload.(map[string]any)["plan"].(*planning.Plan)
        log.Printf("Plan created: %s with %d steps", plan.Title, len(plan.Steps))
    }
})
```

## Best Practices

1. **Use capable models for planning**: GPT-4o, Claude Sonnet for plan creation
2. **Use efficient models for execution**: GPT-4o-mini, Claude Haiku for following plans
3. **Store plans when needed**: Session metadata for short-term, memory for cross-session
4. **Keep plans focused**: 5-10 steps is ideal; break larger tasks into sub-plans
5. **Provide context**: Give execution agents the full plan, not just individual steps

## Anti-Patterns

❌ **Don't use planning for simple tasks**
```go
// Overkill for simple tasks
plan, _ := planner.CreatePlan(ctx, sess, message.User("Add a comment"))
```

❌ **Don't execute without plan context**
```go
// Agent doesn't know the bigger picture
for _, step := range plan.Steps {
    eng.Run(ctx, sess.ID(), message.User(step.Title)) // Missing context!
}
```

❌ **Don't mix planning and execution agents**
```go
// Use planner for planning, executor for executing
planner.CreatePlan(...)  // ✓
executor.Run(...)        // ✓
```

## Related

- [Build a Protocol](../guides/build-a-protocol.md) - Creating custom protocols with planning
- [Agenda](../concepts/collaboration-kit.md#agenda) - Scope refinement before planning
- [Example 17](../../examples/17-planning-mode/) - Full planning mode demonstration
