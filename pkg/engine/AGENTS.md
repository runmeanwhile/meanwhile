# Engine Package Notes (for coding agents)

Focus: core runtime (sessions, runs, persistence, human escalation).

Key types/flows
- Engine owns registries (providers, protocols, tools, toolkits, integrations), session cache, and stores.
- Session handles run/resume, pending human requests, event emission, and persistence hooks.
- Toolkits bundle tool sets and register defaults; tool policy is enforced before tool execution.
- Human escalation: `ask_human` → `HumanRequestCreated` → integration dispatch → inbound `Respond()`.
- Human request inbox: `HumanRequestStore` captures lifecycle (pending/sent/failed/answered/timed_out); `ListHumanRequests` returns inbox views.
- Timeouts: `TimeoutPolicy` is per-session default; `HandleTimeout()` emits `human.request.timed_out`.
- Timeout scheduling is pluggable via `TimeoutScheduler`; `SessionStateStore` rehydrates pending requests.

Important invariants
- Pending requests are stored in-memory with a resume callback; rehydrated pending requests are **not resumable**.
- `Respond()` on rehydrated pending returns `ErrSessionNotResumable` and clears state.
- Pending tool requests store a continuation and can be resumed via `ResumeTool`.
- `RequestRegistry` maps requestID → sessionID for inbound handlers.
- `HumanRequestStore` is event-driven (subscribed on session creation); failures should not block session runs.
- `TimeoutScheduler` schedules by requestID; cancellation happens when pending is removed.

Concurrency + safety
- `runMu` guards session runs/resumes; `pendingMu` guards pending map.
- Avoid long blocking work inside event subscribers; use goroutines if needed.
- Use `context.Context` for IO and always propagate deadlines.

Extensibility pointers
- Add new runtime mechanics here (not protocols) if they benefit all sessions.
- Prefer emitting events instead of logging or side effects.
- Use builder APIs for ergonomic additions (`SessionBuilder`, `Agent.Builder`).
