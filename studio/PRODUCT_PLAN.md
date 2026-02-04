# Meanwhile Studio: Product Plan

This plan defines the product layer that sits above the Meanwhile framework. Studio is a web app + Go backend that delivers a complete, local-first experience aligned with `VISION.md` and `docs/ideas/product-layer.md`.

Studio compiles product concepts down to existing framework primitives (Agents, Protocols, Sessions, Tools). The core engine remains unchanged.

---

## 1) Goals and non-goals

### Goals
- **End-to-end local experience**: spin up a server, open a browser, run collaboration sessions.
- **Stupid-simple UX**: minimal steps from intent to session start.
- **Config-first**: everything can be defined in config and re-used.
- **Human-in-the-loop**: native flow for asking humans, pausing, and resuming.
- **Extensible**: MCP servers, integrations, and new persona specs without core changes.

### Non-goals
- Becoming a general agent execution platform outside the Meanwhile vision.
- Re-implementing protocol logic in Studio (protocols live in framework).
- Overly tight coupling between Studio and engine internals.

---

## 2) Value proposition

Studio delivers "collaboration at the speed of software":
- **Auto-assembled teams** that cover multiple perspectives.
- **Protocol-driven sessions** that feel like real meetings.
- **Human grounding** when the model is uncertain.
- **Artifacts and outcomes** that can be saved, re-run, and improved.

---

## 3) Product capabilities

### A) Personas (create, edit, version)
- Structured PersonaSpec (expertise, behavior, communication style).
- Multiple templates per persona (critique, brainstorm, risk review).
- Versioning with changelogs.

### B) Protocol runs
- Protocol selection (gallery + descriptions).
- Inputs and parameters per protocol.
- Run history and outcomes.

### C) Auto team assembly
- LLM-based team selection with rationale.
- Constraints: team size, cost cap, required roles, diversity vs depth.

### D) MCP connections
- Connect and manage MCP servers.
- Tool discovery and access control per session.

### E) Human collaboration
- Connect human channels (Slack, email, webhook).
- Pending human questions and response routing.
- Pause/resume sessions with notifications.

### F) Session timeline
- Live event stream with message timeline.
- Participants list, roles, and outcomes.
- User interjections (commentary and directives).

### G) Session presets (recurring meetings)
- Save a session setup for one-click re-run.
- Optional scheduling (future).

---

## 4) UX overview

### Global navigation
- Dashboard
- Personas
- Protocols
- Sessions
- MCP
- Integrations (human channels)
- Settings

### First-time flow
1) Connect provider (OpenAI, etc).
2) Create or import personas (or use auto-generated starter set).
3) Pick a protocol.
4) Auto-assemble a team (default on).
5) Run session.

### Session view
- Left: participant list + roles + tools.
- Center: timeline (messages, tool calls, outcomes).
- Right: live status (pending human questions, progress, current turn).
- Controls: pause/resume, interject, end session, save as preset.

---

## 5) Architecture overview

### Frontend (Next.js)
- Next.js (App Router) + TypeScript.
- Tailwind + shadcn/ui + light/dark themes.
- Real-time updates via SSE or WebSockets.
- Pages are config-driven (schema-defined forms for protocol params).

**Key UI surfaces**
- Persona Studio (editor + versioning).
- Session Runner (live timeline).
- Session Presets.
- MCP Connection Manager.
- Integrations (channels, routing, preferences).

### Backend (Go)
Studio backend sits alongside the framework and orchestrates sessions.

Responsibilities:
- Run sessions via framework engine.
- Persist product models (Personas, Presets, Runs, Integrations).
- Manage auth (local-first; multi-user optional).
- Serve API for web app.
- Stream events to UI (SSE/WS).

**Backend packages (conceptual)**
- `studio/server`: HTTP API, auth, SSE/WS.
- `studio/runner`: session runner and lifecycle.
- `studio/store`: persistence layer.
- `studio/persona`: PersonaSpec compiler to framework Agents.
- `studio/mcp`: MCP registry and connection metadata.
- `studio/integration`: channel config and routing (Slack/email/webhook).
- `studio/presets`: SessionPreset compiler to SessionConfig.

### Persistence (local-first)
- Default: SQLite with a file in user home dir.
- Optional: Postgres via docker compose.
- Store: Personas, Persona versions, Sessions, Presets, Human requests, Integrations.

### Config-first
Studio bootstraps from config files that can be edited manually:
```
~/.meanwhile/studio.yaml
~/.meanwhile/personas/*.yaml
~/.meanwhile/presets/*.yaml
```
Config is the source of truth; UI edits update config + DB.

---

## 6) Data model (product layer)

### PersonaSpec
```
id, name, expertise, behavior, communication, templates, version
```

### SessionPreset
```
name, protocol_id, participants, params, context_sources, tags
```

### SessionRun
```
id, preset_id, protocol_id, started_at, status, outcomes
```

### HumanRequest
```
id, session_id, participant_id, status, question, response
```

### MCPConnection
```
id, name, endpoint, status, tools
```

---

## 7) Engine integration (no core changes)

**Flow**
1) Studio compiles PersonaSpec to framework `agent.Agent`.
2) Studio compiles SessionPreset to framework `config.SessionConfig`.
3) Studio uses Engine + Protocols to run sessions.
4) Session events are streamed to UI.

**Human flows**
- `ask_human` tool triggers pending request.
- Studio displays and routes responses.
- `Respond()` resumes session when possible.

---

## 8) Docker and local setup

### Default (local)
- Run Go backend.
- Run Next.js frontend.
- SQLite storage.

### Optional (docker compose)
- Postgres for persistence.
- Redis for request registry / scheduler (optional).
- MCP server containers.

---

## 9) Roadmap

1) **MVP**
   - Persona editor
   - Protocol runner
   - Session timeline
   - Basic persistence (SQLite)

2) **Auto mode**
   - Auto team assembly
   - Session presets

3) **Human collaboration**
   - Channel connections
   - Pending human questions
   - Pause/resume sessions

4) **Iteration layer**
   - Persona tests
   - Drift detection
   - Feedback loop tooling

---

## 10) Why this fits Meanwhile

- Protocols remain the central abstraction.
- Studio delivers the product experience, not the framework itself.
- Human collaboration is first-class, expanded through product UX.
- Everything compiles down to existing primitives, keeping core stable.
