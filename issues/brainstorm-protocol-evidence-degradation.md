# Brainstorm Protocol: Evidence Degradation Analysis

**Date:** February 16, 2026  
**Branch:** feat/brainstorm-v3  
**Scope:** `pkg/protocol/ideo/` + `pkg/collab/`

---

## Executive Summary

The IDEO brainstorm protocol produces generic output despite having access to rich organizational knowledge. The root cause: **evidence degrades at every phase boundary** through a chain of unnecessary summarizations.

The codebase is significantly overengineered:
- **4,037 lines** in `pkg/protocol/ideo/`
- **4,088 lines** in `pkg/collab/`
- **64% of collab kit is unused** by the IDEO protocol (2,634 lines dead to this use case)

**Recommendation:** Cut 40-50% of the code through radical simplification.

---

## The Evidence Degradation Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 0: READINESS                                              │
│ ┌──────────────┐     ┌──────────────────┐     ┌───────────────┐ │
│ │ Tool returns │ ──▶ │ LLM summarizes   │ ──▶ │ Prose context │ │
│ │ "42% Day 1"  │     │ into categories  │     │ loses numbers │ │
│ └──────────────┘     └──────────────────┘     └───────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 1: INSPIRATION                                            │
│ ┌──────────────┐     ┌──────────────────┐     ┌───────────────┐ │
│ │ More tools   │ ──▶ │ Thread snippets  │ ──▶ │ []string      │ │
│ │ (truncated)  │     │ 220 char max     │     │ no source ref │ │
│ └──────────────┘     └──────────────────┘     └───────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 2: REFRAME                                                │
│ ┌──────────────┐     ┌──────────────────┐     ┌───────────────┐ │
│ │ Summary only │ ──▶ │ No metrics/data  │ ──▶ │ Generic HMWs  │ │
│ │ in prompt    │     │ in context       │     │               │ │
│ └──────────────┘     └──────────────────┘     └───────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│ PHASE 3-4: IDEATION + SYNTHESIS                                 │
│ ┌──────────────┐     ┌──────────────────┐     ┌───────────────┐ │
│ │ No evidence  │ ──▶ │ Concepts w/o     │ ──▶ │ Cards can't   │ │
│ │ available    │     │ grounding        │     │ cite sources  │ │
│ └──────────────┘     └──────────────────┘     └───────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## Root Cause Analysis

### 🔴 Problem 1: Readiness Gate Summarizes Away Specificity

**Location:** `session_readiness.go:131-185`

The `readinessContextSummary` struct forces the LLM to convert quantitative data into prose:

```go
type readinessContextSummary struct {
    Product     string   `json:"product"`
    Users       string   `json:"users"`
    Metrics     string   `json:"metrics"`  // Numbers become narrative
    // ...
}
```

**Effect:** "42% of activations happen Day 1" becomes "Many activations happen early in the trial period."

### 🔴 Problem 2: Tool Budget Burned Early, Then Divided

**Location:** `session_inspiration.go:62-63`

```go
turnToolBudget := max(p.cfg.ContextPlan.MaxToolIterations()/max(1, rounds*len(agents)), 2)
```

The readiness gate consumes the full tool budget (200 iterations) for the moderator alone. Then inspiration divides the same budget across participants.

### 🔴 Problem 3: Tool Findings Extracted But Not Propagated

**Location:** `session_inspiration.go:200-260`

Tool findings are extracted via `extractToolCallsFromHistory()` but only used for deduplication prompting. The actual findings get:
1. Truncated to 150-160 chars
2. Injected as prompt text
3. **Never stored structurally** for downstream phases

### 🔴 Problem 4: Transfer Packets Lose Tool Call Data

**Location:** `session_inspiration.go:638-696`

```go
func (p *brainstormIDEO) buildInspirationTransfer(...) *TransferPacket {
    return &TransferPacket{
        Data: map[string]any{
            "tensions":     result.Tensions,      // []string - no source refs
            "observations": result.Observations,  // []string - no source refs  
        },
        Summary: summary.String(),  // Markdown prose - THIS GETS USED
    }
}
```

Downstream phases use `transfer.Summary` (prose) instead of `transfer.Data` (structured but already degraded).

### 🔴 Problem 5: HMW Generation Without Evidence Context

**Location:** `session_reframe.go:185-250`

The reframe prompt builder includes inspiration summary but **no tool-level evidence**:

```go
func (p *brainstormIDEO) buildReframePrompt(...) string {
    // Includes: persona, scope, inspiration summary, lenses
    // Does NOT include: tool findings, source refs, specific metrics
}
```

### 🔴 Problem 6: Evidence Gate Lacks Citation Enforcement

**Location:** `session_synthesis.go:259-350`

```go
type Card struct {
    EvidenceRefs string `json:"evidence_refs,omitempty"`  // Optional!
}
```

Citation is a soft instruction, not enforcement. Cards pass with empty `EvidenceRefs`.

### 🔴 Problem 7: Default Transfer Strategy Discards Tool Messages

**Location:** `config.go:100-105`

```go
func DefaultConfig() Config {
    return Config{
        TransferStrategy: TransferSummaryOnly,  // Discards everything
    }
}
```

Even `TransferWithHistory` only transfers assistant messages, not tool role messages.

---

## Codebase Size Analysis

### IDEO Protocol (`pkg/protocol/ideo/`)

| File | Lines | Purpose |
|------|-------|---------|
| brainstorm_ideo.go | 335 | Main orchestration |
| config.go | 343 | Config structs + options |
| helpers.go | 336 | Utilities |
| session_readiness.go | 267 | Phase 0 |
| session_stage_plan.go | 200 | Stage planning |
| session_inspiration.go | 696 | Phase 1 |
| session_reframe.go | 493 | Phase 2 |
| session_ideation.go | 609 | Phase 3 |
| session_synthesis.go | 692 | Phase 4 |
| structured_output.go | 66 | Schema parsing |
| **Total** | **4,037** | |

### Collab Kit (`pkg/collab/`)

| Package | Lines | Used by IDEO? |
|---------|-------|---------------|
| agenda | 531 | ❌ No |
| caucus | 273 | ❌ No |
| chair | 130 | ❌ No |
| discovery | 299 | ❌ No |
| evidencegate | 122 | ✅ Yes |
| ideationops | 110 | ✅ Yes |
| insightpack | 317 | ✅ Yes |
| interrupts | 20 | ❌ No |
| minutes | 324 | ✅ Yes |
| planning | 786 | ❌ No |
| portfolio | 188 | ✅ Yes |
| pulse | 595 | ❌ No |
| reframer | 118 | ✅ Yes |
| roundtable | 275 | ✅ Yes |
| **Total** | **4,088** | **1,454 used / 2,634 unused** |

**64% of collab kit is dead code for this protocol.**

---

## Radical Simplification Proposal

### Core Insight

The protocol tries to be clever about "curating" context between phases. This is the fundamental mistake.

The Session already has `sess.History(ctx)` which returns **all messages including tool calls**. Each phase should read from history directly instead of building elaborate transfer packets.

### What To Delete (Target: -3,000+ lines)

#### 1. Delete These Structs Entirely

| Struct | File | Why |
|--------|------|-----|
| `TransferPacket` | config.go | Session.History() replaces this |
| `InspirationResult` | session_inspiration.go | Just use history |
| `ReframeResult` | session_reframe.go | Just use history |
| `IdeationResult` | session_ideation.go | Just use history |
| `StagePlan` | config.go | Overengineered; inline in prompts |
| `readinessContextSummary` | session_readiness.go | Causes summarization |
| `readinessAssessment` | session_readiness.go | Merge into simpler gate |

#### 2. Collapse Phase Files

| Before | After | Savings |
|--------|-------|---------|
| session_readiness.go (267) | | |
| session_stage_plan.go (200) | readiness.go (~150) | -317 |
| session_inspiration.go (696) | inspiration.go (~300) | -396 |
| session_reframe.go (493) | reframe.go (~200) | -293 |
| session_ideation.go (609) | ideation.go (~250) | -359 |
| session_synthesis.go (692) | synthesis.go (~300) | -392 |

#### 3. Delete/Move Unused Collab Packages

**Used by other protocols (keep for now):**
- `agenda/` (531) - consensus protocol
- `chair/` (130) - consensus protocol
- `pulse/` (595) - consensus protocol
- `planning/` (786) - example 17

**Truly dead code (592 lines):**
- `caucus/` (273) - no imports found
- `discovery/` (299) - no imports found
- `interrupts/` (20) - no imports found

Either delete the dead packages or move to `pkg/collab/experimental/`.

#### 4. Simplify Used Collab Packages

| Package | Current | Target | Change |
|---------|---------|--------|--------|
| minutes | 324 | ~100 | -224 (remove elaborate formatting) |
| roundtable | 275 | ~100 | -175 (just track turns) |
| insightpack | 317 | ~80 | -237 (inline into protocol) |

### New Architecture

```
pkg/protocol/ideo/
├── brainstorm.go       (~400 lines) - Main + phases inline
├── config.go           (~100 lines) - Minimal config
├── output.go           (~100 lines) - Final output structs only
└── prompts.go          (~200 lines) - All prompt templates

pkg/collab/
├── evidencegate/       (~100 lines) - Keep as-is
├── portfolio/          (~100 lines) - Simplify
└── roundtable/         (~100 lines) - Just turn tracking
```

### Key Design Changes

#### 1. No More Transfer Packets

```go
// OLD (each phase builds transfer)
inspirationTransfer := p.buildInspirationTransfer(inspirationResult, scope)
reframeResult, err := p.runReframe(ctx, sess, agents, scope, inspirationTransfer)

// NEW (each phase reads history)
p.runInspiration(ctx, sess, agents, scope)
p.runReframe(ctx, sess, agents, scope)  // reads sess.History()
```

#### 2. No More Result Structs Per Phase

```go
// OLD
type InspirationResult struct {
    Tensions     []string
    Observations []string
    Constraints  []string
    KeyQuotes    []string
    Artifacts    []Artifact
    Thread       []agent.Message
}

// NEW: Just emit to session, read from history
sess.Emit(event.New(event.PhaseComplete, sess.ID(), map[string]any{
    "phase": "inspiration",
    // minimal metadata
}))
```

#### 3. Tool Findings Stay Intact

```go
// OLD: Truncate and summarize
toolCalls := extractToolCallsFromHistory(history)
summary := formatToolCallSummaryFromHistory(toolCalls)  // 150 char truncation

// NEW: Pass raw history to prompts, let LLM read tool results directly
history, _ := sess.History(ctx)
// Include last N messages (including tool role messages) in prompt
```

#### 4. Evidence Gate Enforces Citations

```go
// OLD
type Card struct {
    EvidenceRefs string `json:"evidence_refs,omitempty"`  // Optional
}

// NEW
type Card struct {
    Evidence []Citation `json:"evidence"`  // Required, validated
}

type Citation struct {
    Source string `json:"source"`  // Must match a source from history
    Claim  string `json:"claim"`   // What fact this supports
}
```

---

## Estimated Impact

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| IDEO protocol lines | 4,037 | ~1,500 | **-63%** |
| Collab kit (IDEO-used portion) | 1,454 | ~400 | **-72%** |
| Collab kit (truly dead) | 592 | 0 | **-100%** |
| **IDEO + related collab** | **6,083** | **~1,900** | **-69%** |

*Note: Other collab packages (agenda, chair, pulse, planning = 2,042 lines) are used by consensus protocol and examples.*

### Quality Impact

| Issue | Before | After |
|-------|--------|-------|
| Evidence reaches final output | No (summarized away) | Yes (raw tool results in history) |
| Citations enforceable | No (optional string) | Yes (validated struct) |
| Specific metrics preserved | No (prose conversion) | Yes (LLM reads raw JSON) |
| Redundant tool calls | Yes (can't see prior findings) | No (full history visible) |

---

## Implementation Plan

### Phase 1: Prove The Concept (1 day)

1. Create `pkg/protocol/ideo_minimal/` with single-file protocol
2. No transfer packets, no result structs
3. Each phase reads `sess.History()` directly
4. Test with same example, compare output quality

### Phase 2: Delete Unused Collab (0.5 day)

1. Move `agenda/`, `caucus/`, `chair/`, `discovery/`, `interrupts/`, `planning/`, `pulse/` to `pkg/collab/experimental/`
2. Or delete entirely if no other protocols use them

### Phase 3: Simplify Used Collab (1 day)

1. Reduce `roundtable` to ~100 lines (just turn tracking)
2. Reduce `minutes` to ~100 lines (remove elaborate formatting)
3. Inline `insightpack` into protocol (it's just config)

### Phase 4: Replace IDEO Protocol (2 days)

1. Replace `pkg/protocol/ideo/` with simplified version
2. Update example
3. Run evals to verify quality improvement

### Phase 5: Documentation (0.5 day)

1. Update `docs/guides/build-a-protocol.md`
2. Remove references to deleted packages

---

## Open Questions

1. **Do we apply this same simplification to consensus protocol?** It likely has similar issues with overengineered transfers.
2. **Should we keep TransferStrategy as an option?** Probably not - it's a footgun that encourages bad patterns.
3. **How do we handle very long histories?** Keep rolling window but ensure tool messages are preserved.
4. **Should collab packages be protocol-specific?** Current "reusable" abstraction adds complexity without clear benefit.

---

## The Radical Diagnosis

The codebase is overengineered. Here's the core problem:

**The protocol doesn't trust the Session.**

The Session already has `sess.History(ctx)` which returns all messages including tool calls. But instead of using it, the protocol:

1. Builds elaborate `TransferPacket` structs with `Summary` fields
2. Each phase creates its own `*Result` struct (InspirationResult, ReframeResult, etc.)
3. Summaries are always more accessible than data, so downstream code uses summaries
4. Evidence degrades at every boundary

### The Structs That Should Die

| Struct | Why It's Harmful |
|--------|------------------|
| `TransferPacket` | Session.History() replaces it |
| `InspirationResult` | Just use history |
| `ReframeResult` | Just use history |
| `IdeationResult` | Just use history |
| `StagePlan` | Overengineered; inline in prompts |
| `readinessContextSummary` | This is where quantitative data dies |

### The Fix

Each phase should:

```go
// NEW: Just read history directly
history, _ := sess.History(ctx)
// Include tool messages in prompt - LLM sees raw JSON
// No transfer packets, no summaries, no fidelity loss
```

Instead of:

```go
// OLD: Build transfer, extract findings, truncate, summarize, lose data
inspirationResult := extractAndSummarizeEverything(...)
transfer := buildTransferPacketWithSummary(inspirationResult)
// Next phase reads transfer.Summary (prose), ignores transfer.Data
```

### The Validation Test

Before committing to the full rewrite: Create `pkg/protocol/ideo_minimal/` as a single ~800 line file. No transfer packets. Each phase reads `sess.History()`. Compare output quality on the same FlowForge example.

If the minimal version produces better output with 80% less code, the radical simplification is validated.
