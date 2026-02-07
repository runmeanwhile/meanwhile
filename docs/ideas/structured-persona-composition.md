# Structured Persona Composition

**Status**: Exploration  
**Date**: 2026-02-07

## The Question

Should personas be more than a single prompt blob?

Right now, a persona is essentially one text string—a prompt that describes who the agent is. This works, but it has limitations:

1. **Repetition**: Many personas share traits (data-driven, skeptical, defensive). Each prompt rewrites these from scratch.
2. **No evolution**: A persona at minute 1 behaves identically to minute 60. Humans don't work that way.
3. **No composition**: Users can't mix-and-match traits to create new personas.
4. **No context adaptation**: A "cautious" persona is equally cautious about low-stakes and high-stakes decisions.

The hypothesis: **structured, composable persona definitions** could produce more realistic, adaptable agents while reducing prompt duplication and enabling richer tools like auto-assembly.

But is this overengineering? Let's explore from first principles.

---

## Part 1: First Principles

### What makes a person "a person" in conversation?

Drawing from simplified psychology and organizational behavior:

1. **Identity** — Who they are, their role, how they see themselves
2. **Expertise** — What they know deeply, their domain lens
3. **Temperament** — Baseline emotional disposition (calm, anxious, enthusiastic)
4. **Cognitive style** — How they process information (intuitive vs. analytical, fast vs. deliberate)
5. **Communication style** — How they express themselves (verbose, terse, formal, casual)
6. **Values** — What they prioritize and protect
7. **State** — Current mood, fatigue, stakes perception (changes over time)

Most persona prompts today conflate all of these into one blob:

```
You are Sarah, a cautious engineering lead who values stability.
You speak concisely and push back on risky proposals.
```

This works! But it doesn't distinguish between:
- Sarah's **baseline** (cautious, values stability)
- Sarah's **style** (concise)
- Sarah's **behavior** (pushes back on risk)
- Sarah's **state** (how does she change when tired or excited?)

### Why might separation matter?

**Reuse**: The trait "pushes back on risky proposals" could apply to many personas. Define it once, compose it into many.

**Evolution**: "Patience" could be a resource that depletes. After 30 minutes of debate, even calm personas might get terse.

**Context-sensitivity**: A data-driven persona might rely more on intuition when data is unavailable, or become more insistent when stakes are high.

**Auto-assembly**: A system building a team could ensure trait diversity: "We have two optimists, let's add a skeptic."

**Debugging**: When a persona behaves unexpectedly, you can ask: was it the temperament trait? The cognitive style? The depleted patience?

---

## Part 2: A Trait-Based Model

### Layers of Persona Definition

```yaml
# persona: engineering-lead-sarah-v1

identity:
  name: Sarah
  role: Engineering Lead
  self_concept: "I'm the person who keeps the train on the rails"

expertise:
  domains: [distributed-systems, incident-response, technical-debt]
  depth: deep
  lens: "I see systems, dependencies, and failure modes"

temperament:
  baseline: calm
  anxiety_trigger: "unclear ownership"
  enthusiasm_trigger: "elegant technical solutions"

cognitive_style:
  information_processing: analytical    # vs. intuitive
  decision_speed: deliberate            # vs. rapid
  uncertainty_response: seek_data       # vs. trust_gut

communication:
  verbosity: terse
  formality: casual_professional
  directness: high

values:
  prioritizes: [reliability, clarity, team-autonomy]
  protects: [on-call health, technical standards]
  
behavioral_traits:
  - trait: risk_skeptic
    weight: 0.8
  - trait: consensus_builder
    weight: 0.6
  - trait: detail_oriented
    weight: 0.7
```

### Reusable Trait Definitions

Traits would be defined once and referenced:

```yaml
# traits/risk_skeptic.yaml
id: risk_skeptic
category: decision_making
description: "Actively seeks out failure modes and downside risks"

behaviors:
  - "When hearing a proposal, first identify what could go wrong"
  - "Ask for rollback plans before approving changes"
  - "Weight negative outcomes higher than positive ones"

expressions:
  mild: "What's our fallback if this doesn't work?"
  moderate: "I'm worried about the failure modes here"
  strong: "I can't support this without a clear rollback plan"

activation:
  increases_with: [high_stakes, past_failures, time_pressure]
  decreases_with: [low_stakes, team_confidence, clear_data]
```

### State and Evolution

Personas could have mutable state during a session:

```yaml
state:
  patience:
    initial: 1.0
    decay_per_turn: 0.02
    effects:
      low: "Responses become shorter, more direct"
      depleted: "May express frustration explicitly"
  
  conviction:
    initial: 0.5
    increases_when: "Arguments align with values"
    decreases_when: "Faced with strong counterarguments"
    effects:
      high: "Speaks with more certainty, harder to move"
      low: "More willing to defer, asks more questions"
  
  engagement:
    initial: 0.8
    increases_when: "Topic relates to expertise domain"
    decreases_when: "Discussion becomes abstract/theoretical"
```

---

## Part 3: Practical Considerations

### The "Just One Prompt" Argument

**Counter-argument**: Maybe all this structure just compiles to one prompt anyway. Why not write that prompt directly?

**Response**: True—at runtime, traits get flattened into a system prompt. But the value isn't in runtime; it's in:

1. **Authoring**: Easier to compose and adjust
2. **Reuse**: Trait library grows over time
3. **Auto-assembly**: "Give me a team with at least one risk_skeptic and one optimist"
4. **Analysis**: "This persona has high risk_skeptic but low detail_oriented—is that inconsistent?"
5. **Evolution**: State changes during session without rewriting the whole prompt

**Risk**: If the compilation is bad, structured traits could produce worse prompts than hand-crafted ones. Quality of the "trait → prompt" compilation matters a lot.

### The "Overengineering" Risk

Not every persona needs this. Quick one-off agents can remain simple prompts:

```go
eng.Agent("Helper").Prompt("You are helpful and concise.").Build()
```

Structured personas would be for:
- OOTB personas that Meanwhile ships
- Users who want to build persona libraries
- Auto-assembly features
- Long-running sessions where evolution matters

### What Would Need to Change?

**Runtime (minimal)**:
- Agent could optionally hold structured persona data
- Protocol could update agent state between turns (already possible via metadata)
- State-dependent prompt injection (persona_prompt + state_modifier)

**Product/Studio**:
- Persona editor with trait browser
- Trait library management
- State visualization during sessions

**Framework**:
- `PersonaSpec` type in product layer (not core)
- Compilation function: `PersonaSpec → string` (prompt)
- Optional: State evolution hooks in protocols

---

## Part 4: Evolution During Sessions

### The Vision

A persona doesn't just have static traits—they have **resources** that deplete and **triggers** that activate.

```
Turn 1-5:   Sarah is patient, asks clarifying questions
Turn 6-10:  Sarah's patience is lower, responses are terser
Turn 11-15: Sarah is visibly frustrated, speaks more directly
Turn 16+:   Sarah might say "We've been over this—let's decide"
```

### Implementation Approaches

**A) Protocol-managed state**

The protocol tracks persona state and modifies prompts:

```go
// In protocol's turn logic
state := session.GetAgentState(agent.ID)
state.Patience -= 0.02
if state.Patience < 0.3 {
    agent.InjectStateModifier("You are growing impatient. Speak more tersely.")
}
```

**B) Self-managed state (prompt-only)**

Include state awareness in the prompt itself:

```
You are Sarah. You value efficiency.
As discussions extend, you become more direct.
If you've made a point multiple times without progress, express mild frustration.
```

This is simpler but less controllable.

**C) Memory-based state**

Use memory to track and recall state:

```go
memory.Add("sarah_state", map[string]any{
    "patience": 0.7,
    "last_frustrated_at": turn12,
})
```

Persona prompt reads from memory:
```
Your current patience level: {{memory.sarah_state.patience}}
```

### State Dimensions to Consider

| Dimension   | Starts At | Depletes When                    | Replenishes When           |
| ----------- | --------- | -------------------------------- | -------------------------- |
| Patience    | High      | Repeated arguments, no progress  | Agreement, new information |
| Energy      | High      | Long discussions, complex topics | Breaks, topic changes      |
| Confidence   | Medium    | Counterarguments, uncertainty    | Data, validation           |
| Engagement  | Medium    | Off-topic, boring                | Relevant to expertise      |
| Frustration | Low       | Being ignored, blocked           | Being heard, progress      |

---

## Part 5: The Eval Question

### Can we measure if this helps?

The whole point of evals (see `eval-system-design.md`) is to answer: does this make sessions better?

**Testable hypotheses**:

1. **Persona separation improves with trait composition**
   - Measure: `persona_separation` dimension score
   - Test: Same scenario, hand-crafted prompts vs. trait-composed prompts

2. **Evolution produces more realistic discussions**
   - Measure: `naturalness` dimension score
   - Test: Static personas vs. state-evolving personas over long sessions

3. **Trait reuse doesn't degrade quality**
   - Measure: Overall quality scores
   - Test: Unique prompts vs. trait-composed prompts

4. **Auto-assembly produces good teams**
   - Measure: Outcome quality, diversity metrics
   - Test: Human-selected teams vs. auto-assembled teams

### Baseline First

Before building trait composition, we should:
1. Establish eval baselines with current hand-crafted prompts
2. Define what "better" means for each protocol
3. Only then test if structured composition improves results

---

## Part 6: Integration with Meanwhile

### Where This Fits

```
┌─────────────────────────────────────────────────────────────┐
│                     Studio / Product Layer                   │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ PersonaSpec │  │ Trait       │  │ State Evolution     │  │
│  │ (YAML/DB)   │  │ Library     │  │ Rules               │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
│         │                │                     │            │
│         └────────────────┼─────────────────────┘            │
│                          │                                  │
│                          ▼                                  │
│                  ┌───────────────┐                          │
│                  │ Compiler      │                          │
│                  │ (Spec→Prompt) │                          │
│                  └───────┬───────┘                          │
│                          │                                  │
└──────────────────────────┼──────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│                     Framework (pkg/engine)                    │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Agent                                                 │   │
│  │ - ID, Name, Model                                     │   │
│  │ - Profile (prompt string)     ← compiled from above   │   │
│  │ - Tools                                               │   │
│  │ - Params                                              │   │
│  │ - Metadata (optional state)   ← updated by protocol   │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  Protocol can update agent metadata between turns            │
│  (already supported, no changes needed)                      │
└──────────────────────────────────────────────────────────────┘
```

### No Core Changes Required

The framework doesn't need to know about traits. It just sees:
- An agent with a prompt (compiled from traits)
- Optional metadata that protocols can update
- The protocol decides when/how to modify state

This keeps the core clean while enabling rich persona behavior in the product layer.

---

## Part 7: Alternative: Simpler Approaches

### Option A: Just Better Prompts

Maybe the answer is simply better prompt engineering:

```
You are Sarah, an engineering lead.
Early in discussions: Ask clarifying questions, explore options.
Mid-discussion: Start forming opinions, probe weaknesses.
Late in discussions: Push for decisions, express urgency if stuck.
If you've repeated yourself: Show mild frustration.
```

**Pros**: Simple, no new systems  
**Cons**: No reuse, no composition, relies on LLM following instructions

### Option B: Protocol-Level Mood

Protocols could manage "mood" generically:

```go
protocol.WithMoodEvolution(MoodConfig{
    PatienceDecay: 0.02,
    FrustrationThreshold: 0.3,
})
```

This injects state modifiers to all participants based on protocol rules.

**Pros**: No persona-level changes, works with any prompt  
**Cons**: All personas evolve the same way, less realistic

### Option C: Memory-Only State

Use the existing memory system to track state:

```go
// Agent's prompt includes:
// "Check your current state in memory before responding."

session.Memory().Set("sarah.patience", 0.7)
```

**Pros**: Uses existing primitives  
**Cons**: Relies on LLM reading from memory correctly, fragile

---

## Part 8: Recommendation

### What to build now: Nothing (in core)

The core framework doesn't need changes. The existing primitives (Agent, Profile, Metadata) are sufficient.

### What to explore in product layer:

1. **PersonaSpec format** — Define a YAML/JSON schema for structured personas
2. **Trait library** — Start with 10-15 reusable traits (risk_skeptic, consensus_builder, data_driven, etc.)
3. **Compiler** — Simple function that flattens PersonaSpec into a prompt string
4. **Baseline evals** — Measure current hand-crafted prompts
5. **A/B test** — Compare trait-composed vs. hand-crafted on same scenarios

### What to defer:

- State evolution (complex, uncertain value)
- Protocol-managed mood (needs more use cases)
- Memory-based state (fragile)

### The test question:

> "Does trait composition produce prompts that perform as well or better than hand-crafted ones, while enabling reuse and auto-assembly?"

If yes → invest more in structured personas  
If no → stick with prompt templates and good examples

---

## Open Questions

1. **Trait granularity**: How fine-grained should traits be? "skeptical" vs. "risk_skeptical" vs. "skeptical_about_timelines"?

2. **Trait conflicts**: What if a persona has both "decisive" and "deliberate"? How does compilation resolve conflicts?

3. **State evolution timing**: When do state changes take effect? Every turn? On phase transitions? On explicit triggers?

4. **LLM adherence**: Will LLMs actually follow state modifiers like "you are growing impatient"? Need to test.

5. **User desire**: Do users actually want to compose personas from traits, or do they prefer writing prompts directly?

6. **Auto-assembly criteria**: What makes a "good" team? Diversity of what dimensions?

---

## Related Documents

- [Product Layer](product-layer.md) — PersonaSpec concept, Persona Studio
- [Connected Personas](connected-personas.md) — Human escalation from personas
- [Eval System Design](eval-system-design.md) — How to measure if this helps
- [Observability](../observability.md) — persona_id, persona_version attributes

---

## Appendix: Example Trait Library (Starter)

### Decision-Making Traits

| Trait           | Description                    | Behavioral Expression        |
| --------------- | ------------------------------ | ---------------------------- |
| `risk_skeptic`  | Actively seeks failure modes   | "What could go wrong?"       |
| `risk_taker`    | Comfortable with uncertainty   | "Let's try it and learn"     |
| `data_driven`   | Needs evidence before deciding | "What does the data say?"    |
| `intuition_led` | Trusts gut feelings            | "My instinct says..."        |
| `decisive`      | Makes quick decisions          | "Let's just decide and move" |
| `deliberate`    | Takes time to consider         | "Let me think about this..." |

### Interaction Traits

| Trait               | Description             | Behavioral Expression              |
| ------------------- | ----------------------- | ---------------------------------- |
| `consensus_builder` | Seeks alignment         | "Can we find middle ground?"        |
| `challenger`        | Pushes back on ideas    | "I disagree—here's why"            |
| `supportive`        | Validates others        | "That's a good point"              |
| `independent`       | States own view clearly | "Here's what I think"              |
| `collaborative`     | Builds on others' ideas | "Building on what X said..."       |
| `competitive`       | Wants their idea to win | "My approach is better because..." |

### Communication Traits

| Trait        | Description                | Behavioral Expression       |
| ------------ | -------------------------- | --------------------------- |
| `terse`      | Brief responses            | Short sentences, few words  |
| `verbose`    | Detailed explanations      | Long, thorough responses    |
| `direct`     | Says things plainly        | "No—that won't work"        |
| `diplomatic` | Softens difficult messages  | "I have some concerns..."   |
| `formal`     | Professional tone          | Structured, proper language |
| `casual`     | Conversational tone        | Relaxed, friendly language  |

### Expertise Application

| Trait        | Description                  | Behavioral Expression          |
| ------------ | ---------------------------- | ------------------------------ |
| `specialist` | Deep in one area             | Always references their domain |
| `generalist` | Broad perspective            | Connects across domains        |
| `mentor`     | Teaches and explains         | "Let me explain why..."        |
| `executor`   | Focus on getting things done | "How do we ship this?"         |
| `strategist` | Long-term thinking           | "In the long run..."           |
| `tactician`  | Short-term optimization      | "Right now, we should..."      |
