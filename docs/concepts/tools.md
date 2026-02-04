# Tools

Tools let agents act, not just talk. Meanwhile treats tools as first-class runtime resources with
explicit assignment, policy guardrails, and durable execution.

## Core ideas

- Tools are **opt-in**: an agent can only call tools that are explicitly assigned.
- Policies are **enforced**: allow/deny rules are applied before execution.
- Toolkits are **bundles**: register a set of tools (filesystem, system, MCP, internal APIs) once and attach to sessions.
- Tools can be **durable**: long-running tools can pause and resume later.

## Assignment model

Tools are only available if they are assigned at one of these layers:

1) Agent profile tools  
2) Agent-specific tools  
3) Run-level tools  
4) Session default tools (including toolkits)

All four are merged for a run; tool policy can further restrict them.

## Tool policy guardrails

Policies can be set at the session level and overridden per run. The effective policy is merged and
enforced before tool execution.

```go
sess, _ := eng.Session("Ops").
    Participant(manager).
    Protocol(protocol.Solo()).
    Toolkits("filesystem", "system").
    ToolPolicy(tool.Policy{
        Mode:      tool.PolicyAllowlist,
        AllowTags: []string{"filesystem", "read", "write", "system", "path"},
        EnforcedBy: "ops",
        Reason:     "limit to repo operations",
    }).
    Build(ctx)
```

## Toolkits

Toolkits bundle related tools and can declare default tool IDs:

```go
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
    Build(ctx)
```

Built-in toolkits:
- `toolkit/filesystem` for guarded file access
- `toolkit/system` for safe PATH lookup
- `mcp` toolkit wrapper for MCP servers
- `toolkit/agentcall` for delegating to a specialist agent

## Durable tool execution

Tools can pause execution for long-running work and resume later:

```go
deploy := tool.New("deploy", func(ctx context.Context, call tool.Call) (tool.Result, error) {
    // enqueue job, then pause
    return tool.Await(call, tool.WithContext("waiting for deploy job"))
})

// Later, resume the pending tool request
result := tool.Result{ToolID: "deploy", Output: "deploy complete"}
_, _ = sess.ResumeTool(ctx, requestID, result)
```

Pending tool requests are stored in `SessionStateStore` so sessions can be rehydrated. Resuming a
pending tool requires that the same tool IDs/toolkits are registered in the engine.

## Events and results

Tool execution emits events (`tool.call.started`, `tool.call.completed`, `tool.call.error`, and
`tool.call.awaiting`). When a run pauses for a tool, the `RunResult` includes
`AwaitingTool`, and you can inspect `Session.PendingToolRequests()` to surface the outstanding work.

## Where to implement

- **Reusable behavior** → `pkg/toolkit/`
- **Runtime mechanics** → `pkg/engine/`
- **External integrations** → `pkg/mcp/` or `pkg/integration/`

Keep tools small and composable, enforce guardrails in toolkits, and rely on session policies for
final enforcement.
