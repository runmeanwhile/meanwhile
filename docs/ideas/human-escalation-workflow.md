# Human Escalation Workflow (Ask Human + Pause/Resume)

## Goal

Define the end-to-end workflow for human-in-the-loop escalation where an agent calls an `ask_human` tool, the session pauses, an external message is sent (Slack/Email/Webhook), and the session resumes when a human responds or a timeout expires.

This doc also inventories the required elements and marks what exists today vs. what is missing or proposed.

---

## Workflow Outline

### 1) Agent decides to escalate
- Agent recognizes uncertainty or a policy requirement and calls `ask_human`.
- The tool payload includes at minimum: `question`, `target_human`, `context`, `timeout`, and `required`.

### 2) Tool handler sends outbound request
- Tool handler persists a **HumanRequest** record (with correlation ID).
- It emits events (e.g., `human.request.created`, `human.request.sent`).
- It sends the question to Slack/Email/Webhook (integration layer).

### 3) Session pauses awaiting human input
- The current run returns an **AwaitingHuman** status (not an error).
- Protocol halts at a stable boundary.
- UI/clients can show "Waiting for Anna on Slack (timeout: 6h)".

### 4) Timeout behavior (optional)
- If no response after `timeout`, a scheduler triggers a fallback:
  - Continue without input (and note uncertainty), or
  - Retry to a backup human, or
  - Mark the run as incomplete.

### 5) Human response arrives
- Integration posts to a webhook receiver with the correlation ID.
- The system validates/authenticates the response.
- The response is stored and emitted as `human.response.received`.

### 6) Session resumes
- The response is injected as a **human participant message**.
- The protocol continues from the paused point.
- The transcript preserves provenance: "Human (real) via Slack".

---

## Sequence Sketch (ASCII)

```
Agent -> ask_human tool
Tool -> persist HumanRequest
Tool -> emit human.request.created
Tool -> Slack/Email/Webhook send
Session -> AwaitingHuman (pause)

... time passes ...

Webhook -> receive response (correlation ID)
System -> emit human.response.received
Session -> inject human message
Protocol -> continue
```

---

## Required Elements and Current Status

Status legend:
- Present: implemented in code
- Partial: present but missing key behavior
- Proposed: documented idea only
- Missing: not present

| Element                            | Needed for                          | Status   | Notes / Where                                                                    |
| ---------------------------------- | ----------------------------------- | -------- | -------------------------------------------------------------------------------- |
| Tool registry + tool calls         | `ask_human` invocation              | Present  | `pkg/tool`, `pkg/engine/agent_run.go`                                            |
| Hook system                        | Pause/interrupt points              | Partial  | Hooks can block turns but no resume flow (`pkg/hook`, `pkg/engine/agent_run.go`) |
| Event bus + logger                 | Observability of requests/responses | Present  | `pkg/event`, `pkg/logger`                                                        |
| Session store interface            | Persist sessions while waiting      | Partial  | Interface exists; default is in-memory (`pkg/engine/session_store.go`)           |
| Human participant abstraction      | Join session as human               | Missing  | Protocols are agent-only today (`pkg/protocol/protocol.go`)                      |
| AwaitingHuman status + Respond API | Pause and resume                    | Missing  | Only proposed in `docs/ideas/human-participation.md`                             |
| Protocol compatibility with humans | Run turns with human participants   | Missing  | Proposed in `docs/ideas/human-participation.md`                                  |
| ask_human tool                     | Escalation trigger                  | Missing  | No tool implementation today                                                     |
| Integration adapters               | Slack/Email/Teams/Webhook           | Missing  | None in repo                                                                     |
| Webhook receiver                   | Inbound human responses             | Missing  | No HTTP server in core                                                           |
| Correlation ID + request tracking  | Match responses to requests         | Missing  | Would need HumanRequest store                                                    |
| Timeout scheduler                  | Resume or fallback after N hours    | Missing  | No job scheduler in core                                                         |
| Consent + preferences              | When/how humans are contacted       | Proposed | See `docs/ideas/connected-personas.md`                                           |
| Provenance in transcript           | "Human (real)" attribution          | Partial  | Event payloads can carry metadata                                                |
| UI/CLI affordances                 | Show pending human state            | Missing  | Only planned in `docs/strategic/studio-tui-implementation-plan.md`               |

---

## What We Do Have That Helps

- **Synchronous tool calls** and structured tool schema support for building `ask_human`.
- **Hooks** that can block or modify turns (useful for pausing at boundaries).
- **Event streaming** for observability and transcripts.
- **Session store interface** for future persistence beyond in-memory.
- **Planning and approvals docs** that describe hook-based blocking patterns.

These are good building blocks, but they do not yet provide the required pause/resume, human participant, or async response flow.

---

## What Is Missing to Enable This Flow End-to-End

Minimum missing pieces:
- Human participant type and protocol compatibility.
- AwaitingHuman state + `Respond` API to resume sessions.
- `ask_human` tool and a HumanRequest persistence model.
- Outbound integrations + inbound webhook receiver.
- Timeout scheduler / job runner.

Nice-to-have (but likely required for real use):
- Consent, preferences, and rate limits (connected personas).
- Security/auth for inbound responses.
- UI surfaces for "waiting for human" and "resume now".

---

## Related Docs

- `docs/ideas/human-participation.md` (human as participant, AwaitingInput/Respond API)
- `docs/ideas/connected-personas.md` (human escalation preferences and integrations)
- `docs/concepts/hooks.md` (interruptions via hooks)

---

**Status**: Proposed  
**Owner**: TBD  
**Last updated**: 2026-02-02
