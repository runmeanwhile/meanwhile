# Meanwhile Framework Investigation

Date: 2026-01-23

## Findings

- Critical: Postgres memory storage drops core event metadata (agent/tool/protocol/session IDs) by only persisting payload, and Query never rehydrates SessionID, so audit trails and memory context lose identity and provenance. (pkg/memory/postgres.go)
- High: Worklog logging expects tool payloads (arguments, output, error) that are never emitted, so tool calls/results are effectively blank or misleading in logs. (pkg/engine/tool_exec.go, pkg/logger/worklog.go)
- High: Session runs are not serialized, yet protocols hold mutable state with no locks (e.g., consensus), so concurrent Run calls can race and corrupt protocol state. (pkg/engine/engine.go, pkg/protocol/consensus/consensus.go)
- High: Tool-iteration limit executes the final tool call and then errors, so side effects occur but results are never fed back to the model, producing spurious failures and wasted actions. (pkg/engine/agent_run.go)
- High: Config surface area is much larger than what gets applied (memory store, telemetry, tool configs are defined but ignored), so enterprise configuration isn’t enforceable. (pkg/config/config.go, pkg/engine/config_apply.go)
- High: Typed tool creation can panic for pointer/nullable input types because reflect.TypeOf(zero) may be nil and Kind() is called unguarded. (pkg/tool/typed.go)
- Medium: Provider API defines EventToolResult, but the engine never handles it, so providers streaming tool results or partial outputs will be ignored. (pkg/provider/provider.go, pkg/engine/agent_run.go)
- Medium: FileChatStore claims concurrent appends per session but uses a single global lock and fsyncs every event, creating a major throughput bottleneck in production. (pkg/memory/filestore.go)
- Medium: Session persistence uses participant names as identifiers in groups, so renames or duplicate names break rehydration and multi-tenant identity management. (pkg/engine/session_store.go)
- Medium: OpenAI provider silently drops malformed tool messages instead of surfacing errors, so missing tool_call_id loses tool outputs without visibility. (pkg/provider/openai/client.go)
- Medium: Event bus drops events when subscriber buffers fill, with no backpressure or default error path, creating silent observability gaps. (pkg/event/bus.go)
- Medium: Memory automation failures are swallowed on session close, so automated memory capture can fail silently. (pkg/engine/session_close.go)
- Low: Agent builder carries unused skills field and panics on random ID failure; profile IDs are non-deterministic, making cross-process reuse brittle. (pkg/agent/builder.go)
