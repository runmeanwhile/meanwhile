# Interruptions & Overlap in Multi‑Agent Collaboration

This document explores how “mid‑sentence interruptions” could *appear* in a streaming agent system, what it implies technically, and how it might map to different protocols and collaboration styles. It is intentionally exploratory: the goal is to understand what *could* work, not what we should ship now.

## Core Reality: You Can’t Inject Tokens Mid‑Stream

LLM providers stream one response at a time per request. You can’t truly inject tokens into a running generation. But you *can*:
- **Preempt** the stream (stop it early),
- **Gate** what gets rendered to the UI,
- **Interleave** multiple streams at the event/compositor layer.

That’s enough to feel like real interruptions.

## Two Fundamental Models

### Model A — Single‑Speaker With Preemption
Only one agent is “on the floor” at a time. Interruption means cutting off the current stream and giving the floor to another agent.

**How it works:**
1) Agent A streams.
2) Agent B triggers an interrupt.
3) Engine cancels A’s stream or stops rendering it.
4) Engine immediately starts B’s stream.

**Pros:** Simple, feels like real interruption.  
**Cons:** A’s generation is abandoned; loses continuity.

### Model B — Parallel Streams With a Compositor (Overlap)
Multiple agents stream simultaneously. A compositor merges output into a single transcript, with speaker boundaries or overlaps.

**How it works:**
1) Agents A and B stream in parallel.
2) The compositor interleaves tokens based on timing or priority.
3) The UI shows overlapping dialogue like a transcript.

**Pros:** Feels like real multi‑party talk with cross‑talk.  
**Cons:** Complex, can become noisy.

## Can Agents “Interrupt” Without Parallel Runs?

Yes — if the interrupt signal is out‑of‑band:
- A “chair” hook watches for trigger conditions (scope drift, time pressure, conflict).
- The chair emits an interrupt event.
- The engine stops Agent A’s stream and runs Agent B (or the chair).

This works even with a single speaker, because the interrupt is not part of the stream — it’s a control event.

## Does This Imply All Agents Are Running in Parallel?

Not necessarily.

There are three viable patterns:
- **Single‑speaker runtime (default):** only the current speaker is running; interruptions come from out‑of‑band signals (chair hooks, timers, policy engines).
- **Listener agents:** lightweight “watchers” run on a cadence (or are invoked by hooks) to decide if they should interrupt. They are not streaming constantly.
- **Full parallel streams:** multiple agents stream simultaneously and a compositor merges outputs. This is the most “live conversation” experience, but also the most complex.

So an interrupt does *not* require all agents to stream at once — it just requires a way to trigger a control‑plane event.

## Expanded Concept: Parallel Streams + Compositor (Model B)

This is the most “realistic” overlap model: it mimics people talking over each other.

### How the compositor could work
- Each agent stream emits timestamped deltas.
- A compositor chooses a merge policy:
  - **Round‑robin merge:** alternates every N tokens.
  - **Priority merge:** higher‑priority speaker gets more continuous tokens.
  - **Burst merge:** buffer tokens and release in short bursts (5–20 tokens) per agent.
  - **Yield merge:** if one agent mentions another (“@Alice”), give the floor to the addressed agent immediately.

### Rendering options
1) **Inline transcript**
   ```
   A: We should— 
   B: Sorry, but—
   A: —focus on—
   ```
2) **Overlap blocks**
   ```
   [A ⤵] We should focus on ...
   [B ⤴] Sorry, but the risk is ...
   ```
3) **Live document**
   - A’s unfinished sentence is dimmed or struck through.
   - B’s interruption is inserted above it.

### Tradeoffs
- Most authentic but hard to read.
- Requires UI conventions and tooling.
- Needs a “floor” abstraction even if multiple streams run.

## Is This “Streaming Input” to the Speaking Agent?

Not exactly. Most providers don’t allow incremental input injection during generation.

**What you *can* do:**
- Stop A’s stream and restart with updated context.
- Run B in parallel and let a compositor merge output.
- Maintain a shared “conversation buffer” and re‑prompt with updated context after each interrupt.

**What you *cannot* do:**
- Feed new tokens into A’s prompt mid‑generation and have it continue seamlessly.

So interruption is always a control‑plane event, not a true mid‑generation input update.

## How Interrupts Might Be Triggered

Possible interrupt signals:
- **Explicit tool call**: `interrupt()` (privileged tool)
- **Moderator policy**: scope drift, time pressure, conflict
- **Priority speaker**: “chair” or “safety” agent can always cut in
- **Urgency cues**: “stop,” “hold on,” “that’s wrong”

These could be hard‑coded or learned via another agent (a “referee”).

## Which Protocols Suit Interruptions?

### Strong Fit
- **Storming** (early‑stage conflict + friction)
- **Debate** (adversarial but structured)
- **Crisis Room** (high urgency, rapid overrides)
- **Design Crit** (short, sharp interjections to push clarity)
- **Town Hall** (participants jump in, chair enforces order)

### Moderate Fit
- **Consensus** (interruptions mainly for moderation)
- **Breakout reconvene** (brief overlaps to surface conflicts)

### Weak Fit
- **Solo / Handoff** (no need for interruption)
- **Brainstorming** (overlap is noisy unless tightly managed)

## Extreme / Fun Protocol Ideas

These can live as experimental protocols or “playground” patterns:

1) **Storming**
   - Everyone can interrupt.
   - Chair only intervenes to keep relevance.
   - Outcome is a shortlist of core disagreements.

2) **Pub Discussion**
   - Relaxed, overlapping, lots of digressions.
   - Chair occasionally calls “last call” to converge.
   - Output is a “sticky” summary of what actually mattered.

3) **Cross‑Examination**
   - One speaker, one interrupter, short bursts.
   - Each interruption must be a direct question.

4) **War Room**
   - Priority speakers can cut in instantly.
   - Output is a timeline of decisions.

5) **Open Mic**
   - Queue with interruptions allowed if someone says “blocker.”

6) **Heated Review**
   - Strong debate with strict time‑boxing.
   - Interruptions allowed only at timer boundaries.

7) **Kitchen Table**
   - Informal overlap, encourages personal framing.
   - Chair “summarizes” every few minutes.

These help express the brand in a playful, human way.

## Architectural Implications (If Pursued)

If overlap/interruptions were implemented, you’d likely need:
- **Turn‑level hooks** (pre/post stream)
- **Interrupt event types** (`interrupt.requested`, `agent.message.interrupted`)
- **Floor controller** (decides who is heard)
- **Compositor** (merges streams)
- **UI conventions** (overlap rendering rules)

## Summary

Mid‑sentence interruption is possible in effect, even if not literal. The best approximation is either:
- **Hard preemption** (stop one stream, start another), or
- **Parallel streaming + compositor** (overlap at transcript level).

The tradeoff is realism vs. complexity. But conceptually, this aligns perfectly with Meanwhile’s brand: it models **real collaboration**, including messiness, friction, and interruptions.
