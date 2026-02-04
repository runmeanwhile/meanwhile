# Consensus Protocol Strategic Direction

This document captures the strategic direction for Consensus as the blueprint protocol for Meanwhile, with an emphasis on preserving brand differentiation, reducing protocol boilerplate, and elevating hooks and shared collaboration primitives.

## Positioning Impact

**Before:** Consensus is a large, feature-rich protocol that risks feeling like a one-off. It implies that the core is too thin, and future protocols may require copy-paste implementation patterns (moderator behavior, scope management, position signaling).

**After:** Consensus becomes the **Blueprint Protocol** that proves the framework’s differentiator: **collaboration, not orchestration**. The core remains minimal, but the **Collaboration Kit** makes the framework feel cohesive, opinionated, and distinctly Meanwhile. Hooks retain their power by supporting interjections at any point in the loop, without protocols re-implementing control flow.

**Result:** Meanwhile is not “a shim over a generic framework.” It becomes a recognizable collaboration engine with a vocabulary, patterns, and a public API that reads like a worklog.

## Vision

Meanwhile is the collaboration OS for AI teams. Protocols are meetings, agents are participants, and the system provides the structure humans expect: agendas, chairs, rounds, and minutes.

Consensus remains the highest-fidelity demonstration of that vision. But it should not be the only place where that structure lives. The Collaboration Kit makes the structure reusable, composable, and obvious — for both contributors and users.

## High-Level Architecture

### 1) Core Runtime (Minimal, Stable)
- Engine, Session, Event Bus, Provider, Tool Registry
- Message loop and tool execution
- Hook registry and event emission

### 2) Collaboration Kit (Shared, Opinionated)
Reusable components that encode human collaboration patterns while staying protocol-agnostic:
- **Agenda**: scope refinement + boundaries (drift handling is protocol-owned)
- **Chair**: facilitator interventions and tone
- **Roundtable**: turn order and context packaging
- **PulseCheck**: position/vote signaling tool
- **Minutes**: result aggregation and output typing
- **Interrupts**: hook-driven interjections (Ralph Wiggum moments)

### 3) Protocols (Composed)
Protocols become thin compositions of Collaboration Kit components:
- Consensus = Agenda + Chair + Roundtable + PulseCheck + Minutes
- Debate = Roundtable + Minutes + optional Chair
- Brainstorming = Scope refinement + moderator + Divergent caucus + Roundtable (interaction) + Minutes (+ optional voting)
- Breakout = Roundtable (grouped) + Minutes

### 4) Documentation + Examples
Docs teach the Collaboration Kit first, then show protocols as assemblies of those parts.

## Levels of Abstraction

1) **Runtime Primitives**  
Engine, Session, Event, Tool, Hook

2) **Collaboration Primitives**  
Agenda, Chair, Roundtable, PulseCheck, Minutes, Interrupts

3) **Protocol Assemblies**  
Consensus, Debate, Brainstorming, Breakout, Handoff

4) **Experience Layer**  
Examples, CLIs, integrations, and “recipes” that show how real teams collaborate

This layered model keeps the core minimal while delivering a distinctly Meanwhile UX.

## Taxonomy (Brand-Led Vocabulary)

Use workplace terms consistently so the mental model remains coherent.

**Core**
- Session = Meeting
- Protocol = Collaboration Pattern
- Participant = Team Member
- Facilitator = Chair
- Event = Worklog Entry

**Collaboration Kit**
- Agenda = Scope + boundaries + intended outcome
- Chair = Interventions + convergence nudges
- Roundtable = Turn-taking + discussion context
- PulseCheck = Position/vote signaling
- Minutes = Structured summary / result
- Interrupts = Hook-based interjections or guardrails

**Protocols**
- Consensus = Convergent alignment
- Debate = Constructive disagreement
- Brainstorming = Divergent + interactive ideation with moderator-guided convergence
- Breakout = Parallel group work

This taxonomy becomes part of the public API and documentation structure.

## Public API Improvements

### 1) Structured Results as First-Class Output
Protocols should return structured results via a supported path, not just events.
Options:
- **Engine captures the final ProtocolAction payload** into `RunResult.Metadata`.
- Or a small `ResultProvider` interface:
  - `Result() map[string]any`
  - Engine inspects and persists it.

This removes boilerplate and makes protocols feel native, not event hacks.

### 2) Turn Hooks (Interruption Layer)
Add a “turn hook” phase around `RunAgent` so hooks can inject or modify turns.
This preserves the framework’s promise: **hooks can interject anywhere**.

### 3) Collaboration Kit Builders
Expose fluent configuration for kit components to keep the API expressive:

```go
consensus := protocol.Consensus(
    consensus.WithAgenda(agenda.WithScope("Policy-level decision")),
    consensus.WithChair(chair.Interventions(0.4, 0.7, 0.9)),
    consensus.WithPulseCheck(pulse.MaxConditions(3)),
)
```

The code still reads like a meeting log.

### 4) Clean, Fluent Session API
Maintain the fluent ergonomics and the “worklog” feel:

```go
sess, _ := eng.Session("Friday Deploy Policy").
    Participants(dev, ops, sec).
    Facilitator(moderator).
    Protocol(consensus).
    Tags("policy", "release").
    Build(ctx)
```

The names and defaults should reinforce the meeting metaphor, not expose internal mechanics.

## Mental Model New Users Adopt

New users should quickly internalize:

1) **A session is a meeting.**  
You put people in a room, pick a collaboration pattern, and run it.

2) **Protocols are meeting formats.**  
Consensus, debate, brainstorm, breakout.

3) **The kit is the facilitator’s playbook.**  
Agenda keeps scope, Chair nudges, PulseCheck captures decisions, Minutes summarize.

4) **Hooks are interruptions.**  
They can step in at any phase to redirect, block, or annotate.

This keeps the learning curve shallow while preserving power.

## Implementation Roadmap

### Phase 1: Extract Reusable Primitives
- Extract scope refinement into `Agenda` (drift handling is protocol-owned).
- Extract moderator intervention logic into `Chair`.
- Extract turn orchestration into `Roundtable`.
- Extract signaling tool into `PulseCheck`.
- Extract result aggregation into `Minutes`.
- Add `ResultProvider` or metadata-capture support in Engine.

**Outcome:** Consensus becomes mostly composition; boilerplate shrinks.

### Phase 2: Turn Hooks and Interrupts
- Add turn-level hook surface (pre/post agent run).
- Add a standard “Interrupt” contract to inject messages.
- Ensure events are consistently emitted and logged (no special-case types).

**Outcome:** Hooks regain visibility and power; “Ralph Wiggum” is real.

### Phase 3: Protocol Refactors
- Rebuild Consensus on the Collaboration Kit.
- Migrate Debate/Brainstorming/Breakout to reuse shared primitives.
- Align outputs and event types across protocols.

**Outcome:** Protocols become small and consistent; new ones are cheap to build.

### Phase 4: Documentation & Examples
- Introduce Collaboration Kit docs and examples.
- Provide protocol recipes that show how to assemble new ones.

**Outcome:** Community contributions become realistic and attractive.

## Proposed Folder Structure

```
pkg/
  collab/
    agenda/        // scope refinement
    chair/         // interventions, tone, facilitation prompts
    roundtable/    // turn order, context packaging
    pulse/         // signaling/voting tools
    minutes/       // result aggregation, typed outputs
    interrupts/    // turn hooks and interjections
  protocol/
    consensus/     // assembly of collab components
    adversarial/
    brainstorming/
    breakout/
    handoff/
    solo/
docs/
  overview.md
  concepts/
    collaboration-kit.md
    protocols.md
    hooks.md
  guides/
    build-a-protocol.md
    facilitation.md
  recipes/
    consensus.md
    debate.md
    breakout.md
```

This structure makes collaboration primitives visible and encourages reuse.

## Documentation Information Architecture

### 1) Start Here
- “Meanwhile in 5 minutes” (mental model + quick example)
- “Why collaboration, not orchestration”

### 2) Concepts
- Engine, Session, Protocol
- Collaboration Kit (Agenda, Chair, Roundtable, PulseCheck, Minutes)
- Hooks as Interrupts

### 3) Protocol Library
- Consensus
- Debate
- Brainstorming
- Breakout
- Handoff

### 4) Guides
- Build a new protocol
- Customize facilitation
- Add a PulseCheck or Minutes
- Use hooks to interject

### 5) Recipes
Problem-oriented “meetings”:
- “Policy decision in 5 rounds”
- “Security review with a chair”
- “Breakouts + reconvene synthesis”

This IA keeps the high-level story coherent and avoids dumping users into low-level APIs too early.

## Why This Keeps the Brand Strong

Meanwhile is about **how people collaborate**. The Collaboration Kit becomes the codified version of the behaviors you want the brand to represent. Consensus stays the beacon, but it no longer monopolizes the “core logic.” Hooks remain powerful and visible. Protocols stay lightweight and composable.

The public API becomes more fluent, more consistent, and more “meeting-like.” That is the differentiation.

---

If you want this turned into a short RFC, a migration plan, or a public-facing blog-style narrative, say the word and I’ll draft it.
