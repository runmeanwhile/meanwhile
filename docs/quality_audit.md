# Framework Quality Investigation

Reviewed the docs and core code paths with an enterprise/agentic-framework lens. Below are the main blockers and systemic risks, ordered by severity.

Last updated: 2026-01-23. Status tags reflect current state on main.

## Critical
- (Resolved) Protocol `OnEvent` is invoked by the session event bus, enabling event-driven protocols/hooks.
- (Resolved) `Engine.Run` collects events synchronously to avoid data races and partial results.
- (Resolved) Event bus avoids send-on-closed-channel races when unsubscribing/closing.
- (Resolved) `RunAgent` closes ephemeral sessions and accepts caller context.
- (Resolved) `provider:model` syntax strips provider prefix before provider call.

## High
- (Resolved) Memory automation is opt-in only; memory store does not auto-enable it.
- (Resolved) `Session.Emit` now returns memory errors and supports `EmitWithContext`.
- (Resolved) Postgres store validates identifiers to avoid SQL injection.
- (Resolved) Conversation context ordering is normalized chronologically.
- (Resolved) Protocol-as-tool uses the SessionBuilder path to auto-fill participants.

## Medium
- (Resolved) `Run` captures `AgentMessageComplete` with direct `agent.Message` payloads.
- (Resolved) Config system is wired: ProviderID/ProfileID and session configs resolve at runtime.
- (Partial) Session persistence is supported via `SessionStore`, but clients must provide a durable store for multi-process usage.
- (Resolved) MCP tool refresh removes stale tool IDs.
- (Resolved) Typed tool schema generation supports nested structs.
- (Resolved) OpenAI tool schemas preserve JSON payloads and tool call arguments.
- (Resolved) Roundtable context preserves full text/JSON content in attribution blocks.
- (Resolved) Duplicate participant names are rejected during validation.
- (Open) Agent identity is still name-based; add stable agent IDs if multi-tenant routing requires it.

## Questions / assumptions to confirm
- Should agents have stable IDs beyond `Name` for routing/audit in multi-tenant environments?

## Notes
- Tests run: `go test ./...`
