# AGENTS

This project welcomes coding agents. This file is a short orientation so automated helpers can be productive and safe.

## What this repo is
Meanwhile is a Go framework for multi‑agent collaboration. The core engine is small; collaboration behavior lives in **protocols** built from the collaboration kit.

## Key concepts (very short)
- **Engine**: registers providers, tools, sessions, logging, hooks, and context policy.
- **Agent**: prompt + model + params + tools; can run directly or within sessions.
- **Session**: a collaboration instance with participants + protocol + event stream.
- **Protocol**: defines how agents collaborate (brainstorm, consensus, handoff, adversarial, etc.).
- **Collaboration kit**: primitives used to assemble protocols (roundtable, consensus kit, etc.).
- **Events**: streaming events for observability and control; Worklog renders readable transcripts.
- **Toolkits**: bundles of tools (filesystem/system/MCP/internal) assignable per session with policy guardrails.
- **Integrations**: outbound delivery for human escalation requests (Slack/email/webhook).
- **Request registry**: maps human request IDs to sessions for inbound responses.
- **Scheduler**: pluggable timeout scheduling for human requests.

## Architecture layers: where does new code go?

### 🎯 Protocols (`pkg/protocol/`)
**What:** Unique, encapsulated collaboration behavior patterns.

**When to add a protocol:**
- It's a complete, self-contained collaboration pattern (e.g., brainstorming, handoff, debate)
- It combines Collab Kit components in a specific workflow
- Users will configure and use it as a whole (not decompose it)
- It's ready for end-users to adopt directly

**Examples:** `Solo`, `Handoff`, `Brainstorming`, `Adversarial`, `Consensus`

**Think:** "This is a meeting format people would recognize and use."

### 🧩 Collaboration Kit (`pkg/collab/`)
**What:** Reusable, composable primitives for building protocols.

**When to add to Collab Kit:**
- The behavior is meant to be **reused across multiple protocols**
- It's a building block, not a complete pattern
- Multiple users/protocols will need this capability
- It's a general collaboration primitive (turn-taking, facilitation, minutes, etc.)

**Examples:** `roundtable` (turn management), `chair` (facilitation), `minutes` (structured results), `agenda` (scope setting), `pulse` (consensus checking)

**Think:** "Multiple protocols need this piece; it shouldn't be duplicated."

### ⚙️ Engine Core (`pkg/engine/`)
**What:** Framework infrastructure that all protocols rely on.

**When to add to core:**
- It's fundamental to how the engine operates (sessions, agent execution, events)
- Every protocol or client would benefit from this capability
- It's about runtime mechanics, not collaboration patterns

**Examples:** Agent execution, session lifecycle, memory integration, hooks, context policy

**Think:** "This is infrastructure, not collaboration behavior."

### 🔌 Integrations + Drivers (`pkg/integration/`, `pkg/requestregistry/`, `pkg/scheduler/`, `pkg/server/`)
**What:** Pluggable delivery, persistence, and scheduling infrastructure.

**When to add here:**
- It's a driver for an external system (Redis, Slack, SMTP, webhooks)
- It supports human escalation flows (routing, inbound responses, timeouts)
- It should be swappable without changing engine or protocol behavior

**Think:** "Replaceable infrastructure, not collaboration logic."

## Quick decision tree

```
Is it a complete collaboration pattern users adopt directly?
  └─ YES → Protocol (pkg/protocol/)
  └─ NO ↓

Will multiple protocols need this building block?
  └─ YES → Collab Kit (pkg/collab/)
  └─ NO ↓

Is it fundamental runtime infrastructure?
  └─ YES → Engine Core (pkg/engine/)
  └─ NO → Consider if it belongs in this framework at all
```

## Design philosophy: Listen to user friction

This codebase evolves by **removing friction from real user code**:

### Example 1: Tool registration was verbose
**User feedback:** "It's annoying to register tools AND specify them by ID"
```go
// Before: Two steps, error-prone
eng.ToolRegistry().Register(myTool)
agent.Tools("my_tool")  // String ID - easy to typo
```

**Fix:** Added `.Tool(instance)` and `.Tools(instances...)`
```go
// After: One step, type-safe
agent.Tool(myTool)               // Single tool
agent.Tools(tool1, tool2, tool3) // Multiple tools
```

### Example 2: Structured output required manual JSON parsing
**User feedback:** "Why doesn't the framework infer the type like Pydantic AI?"
```go
// Before: Manual schema in prompt + parsing
agent.Prompt("Return JSON: {\"title\": string, \"steps\": []}")
json.Unmarshal(resp.Text(), &plan)
```

**Fix:** Added `.OutputSchema()` and fluent tool creation
```go
// After: Type-safe, auto-generated schema
agent.OutputSchema(Plan{})  // Or use tool pattern (recommended)
tool.New("submit", handler).WithDescription("Submit plan")
```

### Pattern for API improvements:
1. **Spot the pain:** User code has ceremony, duplication, or brittleness
2. **Fix in the builder:** Fluent APIs absorb complexity
3. **Maintain backward compatibility:** Old patterns still work
4. **Test both paths:** Ensure new and old approaches coexist
5. **Update examples:** Show the better way, but don't break existing code

**Rule:** If users write the same 3 lines in every project, those 3 lines belong in the framework.

## Where to learn fast
- `examples/README.md` and `examples/OVERVIEW.md` list all demos and what they show.
- `docs/overview.md` and `docs/concepts/protocols.md` explain the architecture.
- `docs/guides/build-a-protocol.md` is the quickest path to custom protocols.
- `pkg/collab/` contains the reusable primitives that protocols compose.

## Working style
- Prefer small, composable changes in protocols and collaboration-kit components.
- Keep APIs fluent and consistent with existing builder patterns.
- Stream events when adding new behaviors (so logs/transcripts stay coherent).
- Keep infra pluggable: add drivers and interfaces instead of hard-coded vendor logic.
- When building a new protocol, check if pieces belong in Collab Kit first.

## Commands
- `go test ./...` (or run the smallest relevant package tests)
- `gofmt -w <files>` for any Go edits

If you’re unsure where to implement something, start in `docs/guides/build-a-protocol.md` and follow the patterns used in `examples/`.

## Durability Learnings (session notes)
### What we struggled with
- **Rehydration tests failed until protocol factories were registered.** `sessionFromRecord` resolves protocols by registry ID, so tests that load sessions must register the protocol factory on *both* engines.
- **Auto-summarize got trimmed by the base policy.** The default policy’s rolling window can drop the summary unless we bypass or reset `RollingWindow` after inserting a summary.
- **Retry config needed a global off switch.** Some tests or environments need to disable resiliency; wiring a boolean enable flag avoids forcing retries everywhere.
- **Memory automation bypassed engine retry config.** Helper functions must honor the same retry settings as `RunAgent` to keep behavior consistent.

### How the codebase works (useful mental model)
- **Durability state lives in session metadata.** Engine persists `_protocol_state`; stateful protocols should serialize into `map[string]any` and rehydrate in `sessionFromRecord`.
- **Protocols are runtime-registered.** Sessions can only rehydrate if the protocol factory is in `Engine.protocols` registry.
- **Context policy is layered.** `AutoSummarizePolicy` wraps another policy; overrides must preserve the base policy so existing selection logic still applies.
- **Retries are stream-level.** The resilient wrapper sits around provider streams and must treat EOF as terminal (no retry).
- **Pending tool calls are first-class.** Pending tool requests + continuations are stored in `SessionState` and resumed via `ResumeTool`.

### What to follow next time
- **Register protocol factories in tests that load sessions.** Don’t assume `New()` auto-registers protocol factories.
- **Keep durability knobs in config + builder.** Defaults come from config; options should override but not diverge from config behavior.
- **Use deterministic tests for long-running behavior.** Simulate blips, crash/rehydrate, and summarization in a few seconds instead of running an hour.

### What to avoid
- **Don’t retry on EOF.** EOF should remain terminal to prevent infinite loops.
- **Don’t let summarization silently no-op.** Ensure summaries aren’t trimmed by the rolling window.
- **Don’t bypass tool policy.** Tool allow/deny enforcement must be applied before execution.
