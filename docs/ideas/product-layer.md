# Product Layer on Top of Meanwhile Framework

This doc sketches a product layer that sits above the Meanwhile framework. The goal is to wow users with a complete experience (persona creation, matching, iteration, human collaboration) while keeping the core engine small and composable.

The framework stays "dumb" and stable: it accepts Agents, Protocols, Sessions, Tools, and emits events. The product adds UX, persistence, automation, and opinionated workflows that compile down to existing framework primitives.

---

## 1) Value proposition (product vs. framework)

**Framework promise**
- Clean primitives: Protocols define meeting logic; Sessions run participants; Agents are prompts + models + tools.
- Human collaboration is first-class via `ask_human` and Human participants.
- Extensible + composable; no product assumptions in core.

**Product promise**
- "Bring a problem, get a team and a meeting" in minutes.
- Auto-assembled multi-angle teams (LLM-based, not keyword matching).
- Personas that are editable, testable, and reusable.
- Human-in-the-loop as a differentiator (real humans connected to personas).

**Positioning**
The product is the delightful UX and automation layer. The framework is the reliable engine underneath.

---

## 2) Principles for keeping the core clean

- **No new core concepts**: use existing `Protocol`, `Session`, `Agent`, `HumanParticipant`.
- **Compile down**: all product artifacts compile to `Agent`/`SessionConfig`.
- **Isolation**: product packages live in product repo or `studio/` / `extensions/` with zero dependencies from core back up to product.

---

## 3) Product capabilities (what it should offer)

### A) Persona Studio
Create and manage personas with structured attributes that generate prompts.

**What users get**
- Persona editor: identity + expertise + behavior + communication style.
- Multiple prompt templates per persona (critique, brainstorm, risk review).
- Versioning and changelog: "Calm Marketer v2" vs v1.

**Why it matters**
Separates expertise from personality and creates consistent behavior across protocols.

---

### B) Auto Team Assembly ("Auto mode")
User states intent and desired coverage; system builds a team.

**What users get**
- Input: "I want this strategy critically evaluated from multiple angles."
- Output: recommended team of N personas with rationale and diversity notes.
- Constraints: team size, cost cap, required roles, adversarial pairs.

**Why it matters**
Removes manual selection friction and improves results via diversity/expertise coverage.

---

### C) Session Presets (recurring meetings)
A saved meeting configuration that re-runs easily.

**What users get**
- A saved "Team A Retro" preset with protocol + participants + context.
- One-click run or scheduled cadence.

**Why it matters**
Recurring meetings are common; saves time and preserves context.

**Note**
This is *not* a new framework concept. It compiles to `config.SessionConfig`.

---

### D) Connected Personas (human collaboration)
Link personas to real humans, with consent and preferences.

**What users get**
- Connect a persona to a real human (Slack/email/webhook).
- Persona can escalate uncertainty via `ask_human`.
- Human review and learning feedback loop.

**Why it matters**
This is the core differentiator: real human input where it matters most.

---

### E) Iteration + Evaluation
Product analytics for persona quality and team effectiveness.

**What users get**
- Test suites for personas (golden prompts).
- Drift detection and regressions on output quality.
- Post-session feedback loops to refine persona specs.

---

## 4) UX sketches (what it feels like)

### FTUE (first-time user)
1) "Describe your task" (single prompt input)
2) "Pick a meeting format" (Protocol gallery)
3) "Auto assemble team" (default on, editable)
4) "Review personas" (adjust tone, expertise sliders)
5) "Start session"

### Persona editor
- Tabs: Identity, Expertise, Behavior, Communication, Templates
- Live preview: "What this persona will say" sample
- Save as version (v1, v2, v3)

### Auto mode panel
- Input: intent + constraints
- Output: team list with rationale
- Button: "Generate alternative team" (diversity vs depth)

### Session preset
- Save current setup as preset
- Run immediately or schedule
- "Team A Retro" listed in presets

---

## 5) Implementation outline (product layer)

### Data models (product-owned)
**PersonaSpec** (stored file or DB)
```
id: marketer-calm-v1
name: Calm Marketer
expertise:
  domains: [positioning, go-to-market, messaging]
behavior:
  temperament: calm
  conflict_style: collaborative
communication:
  tone: warm
  brevity: medium
templates:
  critique: templates/critique.md
  brainstorm: templates/brainstorm.md
```

**SessionPreset** (maps 1:1 to SessionConfig)
```
name: Team A Retro
protocol_id: retrospective
participants: [marketer-calm-v1, eng-pragmatic-v2, human-alex]
context_sources: [project_docs, last_sprint_notes]
defaults:
  params: { temperature: 0.3 }
```

**TeamPlan**
```
goal: "Critically evaluate strategy"
team:
  - persona_id: strategist-skeptical-v1
    role: "Adversarial reviewer"
    rationale: "Risk and counterfactuals"
  - persona_id: marketer-calm-v1
    role: "Market positioning"
    rationale: "GTM expertise with balanced tone"
```

---

### Compiler layer (product → framework)
**PersonaSpec → Agent**
- Generates the final prompt from structured attributes + template.
- Produces `agent.Agent` or `config.AgentConfig`.

**SessionPreset → SessionConfig**
- Saved in product, compiled to `config.SessionConfig`.
- Protocol ID, participants, tools, params map directly.

**TeamPlan → SessionPreset**
- Auto-assembly outputs a plan, user can approve/edit.

---

### Runtime integration (no core changes)
1) Product loads PersonaSpecs
2) Product assembles Agents
3) Product creates SessionConfig
4) Framework runs the session normally

---

## 6) What stays out of core (explicitly)

- Persona storage, versioning, evaluation, matching logic
- Auto team assembly logic and LLM selection
- Session presets and scheduling UI
- Human persona consent and preference UX

All of these compile down to existing framework primitives.

---

## 7) Milestones (product roadmap)

1) Persona Studio + Compiler
2) Session Presets (save & rerun)
3) Auto Team Assembly
4) Iteration + Evaluation
5) Connected Personas

---

## 8) Why this is on-brand for Meanwhile

- Protocols stay pure (meeting logic only).
- Product creates the "meeting room" experience above the engine.
- Human participation remains core, amplified by product workflows.
- Clean separation reduces blast radius and preserves framework clarity.
