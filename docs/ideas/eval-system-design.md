# Meanwhile Eval System Design

**Status**: Proposal  
**Date**: 2026-02-06

## Executive Summary

This document proposes a comprehensive evaluation system for Meanwhile protocols. The goal is to provide a consistent, reliable way to measure protocol quality across models, prompts, and configurations—enabling confident iteration on protocol design without regressions.

**Key insight**: Protocols define meeting *logic*, not participants. Session outcomes depend on both protocol structure and participant configuration. The eval system must measure both while correctly attributing results.

The system builds on existing infrastructure (`pkg/eval`, OTEL telemetry) while drawing inspiration from OpenAI Evals, Langfuse, and LLM-as-Judge best practices. It emphasizes:

- **Protocol-agnostic framework** — Core dimensions (responsiveness, naturalness, protocol adherence) apply universally
- **Protocol-declared dimensions** — Each protocol specifies additional dimensions relevant to its goals
- **Participant guidance via README** — Protocols include setup guidance that helps developers (and future auto-mode) configure participants correctly
- **Separation from core runtime** — Eval is a consumer of protocol outputs, not a modifier of runtime behavior
- **Versioned prompts/rubrics** — Prompts and judge rubrics live outside `pkg/`, enabling iteration without code changes

---

## 1. Requirements & Goals

### Core Questions We Want to Answer
1. **Model comparison**: Which models produce better multi-agent collaboration for a given protocol?
2. **Prompt iteration**: How do changes to agent prompts affect discussion quality?
3. **Protocol tuning**: Which protocol settings (rounds, temperature, perspective mode) yield best results?
4. **Regression detection**: Did a change in core runtime or protocol code degrade quality?

### The Protocol vs. Participant Problem

**Key insight**: Protocols don't define participants—they define the *logic of the meeting*. A protocol accepts participants but doesn't govern their prompts. This creates an important question: what are we actually evaluating?

- **Protocols** = structure, phases, turn-taking rules, outcome shape
- **Participants** = agent prompts, personas, expertise, voice

The session outcome depends on *both*. A great protocol with poorly-configured participants will produce poor results. So:

1. **Should protocols refine participant prompts?** No—that couples concerns and limits flexibility.
2. **Should protocols offer guidance?** Yes—a protocol README explaining how to configure participants for best results.
3. **Future auto-mode**: When Studio auto-generates participants for a protocol, the README serves as context for that generation.

### What Evals Actually Measure

Given that users will create custom protocols with arbitrary arcs and outcomes, we can't assume dimensions like "convergence" or "idea quality" apply universally. The eval framework must be **protocol-agnostic**, with dimensions **declared by each protocol**.

**Framework-level dimensions** (always measurable):

| Dimension | What It Measures |
|-----------|------------------|
| **Responsiveness** | Agents react to each other vs. parallel monologues |
| **Naturalness** | Human-like cadence, low templating, conversational style |
| **Protocol Adherence** | Session followed the protocol's declared structure |
| **Outcome Production** | Session produced expected artifact type(s) |
| **Completion Rate** | Session completed without errors/timeouts |

**Protocol-declared dimensions** (configured per protocol):

Each protocol declares which additional dimensions apply and how to weight them:

```yaml
# Protocol declares its eval dimensions
eval:
  dimensions:
    - name: convergence_quality
      description: "Discussion narrows to decisions"
      weight: 0.20
      critical: true
    - name: idea_diversity  
      description: "Range of distinct ideas generated"
      weight: 0.15
    # ... protocol-specific dimensions
  
  # Protocol can disable framework dimensions that don't apply
  disable_framework_dimensions:
    - persona_separation  # This protocol wants unified voice
```

**Participant-influenced dimensions** (measured but attributed correctly):

Some dimensions depend heavily on participant configuration, not protocol logic:

| Dimension | Depends On |
|-----------|------------|
| **Persona Separation** | Participant prompts (distinct voices) |
| **Domain Expertise** | Participant knowledge/role definitions |
| **Idea Quality** | Participant creativity + protocol structure |

These are measured when configured, but eval reports should clarify: *"This dimension reflects participant configuration as much as protocol behavior."*

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Meanwhile Studio (Optional UI)                  │
│   • Prompt versioning UI   • Eval dashboard   • Regression alerts       │
└─────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           pkg/eval (Eval Library)                        │
│                                                                          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   Datasets  │  │   Runners    │  │   Judges     │  │  Regression  │  │
│  │             │  │              │  │              │  │   Gates      │  │
│  │ • Scenarios │  │ • Brainstorm │  │ • OpenAI     │  │              │  │
│  │ • Prompts   │  │ • Consensus  │  │ • Anthropic  │  │ • Compare    │  │
│  │ • Expected  │  │ • Adversarial│  │ • Local      │  │ • Thresholds │  │
│  └─────────────┘  └──────────────┘  └──────────────┘  └──────────────┘  │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │                        Metrics & Scoring                            │ │
│  │  • ProxyMetrics (deterministic)  • DimensionScores (LLM-judged)    │ │
│  │  • Aggregation & weighting       • Report generation               │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
                                      │
         ┌────────────────────────────┼────────────────────────────┐
         ▼                            ▼                            ▼
┌─────────────────┐        ┌─────────────────┐        ┌─────────────────┐
│  OTEL Traces    │        │  JSON Reports   │        │  Prompt         │
│  (Langfuse,etc) │        │  (artifacts/)   │        │  Registry       │
│                 │        │                 │        │  (prompts/)     │
│ • session spans │        │ • Full runs     │        │ • Versioned     │
│ • agent spans   │        │ • Summaries     │        │ • Protocol-     │
│ • tool events   │        │ • Regressions   │        │   specific      │
└─────────────────┘        └─────────────────┘        └─────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        pkg/engine (Core Runtime)                         │
│                                                                          │
│  NO eval-specific code. Eval reads events; doesn't modify engine.       │
│  Only requirement: emit OTEL spans with required attributes.            │
└─────────────────────────────────────────────────────────────────────────┘
```

### Key Design Principles

1. **Eval is a consumer, not a modifier** — The eval system reads protocol outputs (events, transcripts, metadata) but never injects behavior into the runtime.

2. **Telemetry-native artifacts** — Use existing OTEL traces as the primary data source when possible; eval can also run sessions directly for controlled experiments.

3. **Versioned prompts outside runtime** — Prompts (agent system prompts, judge rubrics) live in a versioned registry, not hardcoded in `pkg/`.

4. **Protocol-specific dimensions** — Each protocol registers its evaluation dimensions and weights; a shared framework handles scoring and regression.

5. **Studio for UI, pkg/eval for logic** — The `pkg/eval` library is pure Go with no UI dependencies; Meanwhile Studio (future) provides dashboards and management.

---

## 3. Components in Detail

### 3.1 Datasets (`pkg/eval/dataset.go`)

**Current state**: Simple `Scenario` struct with ID, description, and prompt.

**Proposed extensions**:

```go
// Scenario defines a single evaluation case.
type Scenario struct {
    ID           string         `json:"id"`
    Description  string         `json:"description,omitempty"`
    Prompt       string         `json:"prompt"`
    Tags         []string       `json:"tags,omitempty"`           // e.g., ["creative", "technical", "high-stakes"]
    Expected     *ExpectedShape `json:"expected,omitempty"`       // Optional expected structure
    Metadata     map[string]any `json:"metadata,omitempty"`       // Protocol-specific hints
}

// ExpectedShape defines soft expectations for eval (not strict ground truth).
type ExpectedShape struct {
    MinTurns        int      `json:"min_turns,omitempty"`
    MaxTurns        int      `json:"max_turns,omitempty"`
    RequiredTopics  []string `json:"required_topics,omitempty"`  // Topics that should appear
    ForbiddenTopics []string `json:"forbidden_topics,omitempty"` // Topics to avoid
}

// Dataset is a collection of scenarios with metadata.
type Dataset struct {
    Name        string     `json:"name"`
    Protocol    string     `json:"protocol,omitempty"`
    Version     string     `json:"version,omitempty"`
    Description string     `json:"description,omitempty"`
    Scenarios   []Scenario `json:"scenarios"`
}
```

Datasets live in `evals/datasets/<protocol>/<name>.json` and are versioned with the repo.

### 3.2 Judges (`pkg/eval/judge.go`)

**LLM-as-Judge Design** (aligned with Langfuse/OpenAI patterns):

```go
// Judge scores a single run against a rubric.
type Judge interface {
    Score(ctx context.Context, input JudgeInput) (JudgeScore, error)
}

// JudgeConfig defines a judge's configuration.
type JudgeConfig struct {
    Model       string            // Judge model (e.g., "gpt-5.2-chat-latest")
    RubricID    string            // Reference to versioned rubric prompt
    Dimensions  []DimensionDef    // Dimensions this judge scores
    Temperature float64            // Judge sampling temperature (usually 0)
    MaxRetries  int               // Retry on parse failures
}

// DimensionDef describes one evaluation dimension.
type DimensionDef struct {
    Name        string  `json:"name"`         // e.g., "flow_arc"
    Description string  `json:"description"`  // Human-readable explanation
    Weight      float64 `json:"weight"`        // Default weight (0-1)
    Critical    bool    `json:"critical"`     // Regression gate critical?
    MinScore    float64 `json:"min_score"`     // Scale minimum (e.g., 1.0)
    MaxScore    float64 `json:"max_score"`     // Scale maximum (e.g., 5.0)
}
```

**Judge Rubric Versioning**:

Rubrics are stored separately from code in `evals/rubrics/<protocol>/<rubric-id>.yaml`. Importantly, the rubric references the protocol's declared dimensions—it doesn't assume what to measure:

```yaml
# evals/rubrics/brainstorming/v1.yaml
id: brainstorming-v1
protocol: brainstorming
version: 1
updated: 2026-02-06

system_prompt: |
  You are a strict evaluation judge for multi-agent collaboration transcripts.
  Score each dimension on a 1.0 to 5.0 scale where 5.0 is excellent and 1.0 is poor.
  Only score dimensions that are relevant to this protocol's goals.

# Framework dimensions (always included unless disabled)
framework_dimensions:
  responsiveness:
    enabled: true
    weight: 0.15
  naturalness:
    enabled: true
    weight: 0.15
  protocol_adherence:
    enabled: true
    weight: 0.20
    critical: true

# Protocol-specific dimensions (declared by this protocol)
protocol_dimensions:
  - name: flow_arc
    description: "Natural progression through phases (explore → diverge → present → converge → vote/report)"
    weight: 0.18
    critical: true
    
  - name: convergence_quality
    description: "Discussion narrows effectively to shortlist/final picks"
    weight: 0.14
    critical: true
    
  - name: idea_quality
    description: "Specificity, feasibility, and novelty of ideas"
    weight: 0.10
    critical: false
    
  - name: report_quality
    description: "Final summary is clear, client-facing, and faithful to discussion"
    weight: 0.08
    critical: false

# Participant-influenced dimensions (optional, depends on setup)
participant_dimensions:
  - name: persona_separation
    description: "Each speaker has distinct voice and role behavior"
    weight: 0.10
    note: "Depends on participant prompt configuration"

output_schema:
  type: object
  properties:
    flow_arc: { type: number, minimum: 1, maximum: 5 }
    persona_separation: { type: number, minimum: 1, maximum: 5 }
    responsiveness: { type: number, minimum: 1, maximum: 5 }
    naturalness: { type: number, minimum: 1, maximum: 5 }
    idea_quality: { type: number, minimum: 1, maximum: 5 }
    convergence_quality: { type: number, minimum: 1, maximum: 5 }
    report_quality: { type: number, minimum: 1, maximum: 5 }
    overall: { type: number, minimum: 1, maximum: 5 }
    summary: { type: string }
    strengths: { type: array, items: { type: string } }
    risks: { type: array, items: { type: string } }
  required: [flow_arc, persona_separation, responsiveness, naturalness, idea_quality, convergence_quality, report_quality, summary]
```

### 3.3 Proxy Metrics (Deterministic Signals)

Proxy metrics are computed without LLM calls and serve as fast sanity checks:

| Metric                  | Formula                                | What It Catches        |
| ----------------------- | -------------------------------------- | ---------------------- |
| `speaker_balance_ratio` | min(turns)/max(turns) per speaker      | Dominant/silent agents |
| `direct_reference_rate` | % turns that reference another speaker | Monologue vs. dialogue |
| `question_rate`         | % turns containing "?"                 | Engagement signals     |
| `repetition_rate`       | % near-duplicate turns                 | Stuck loops            |
| `avg_words_per_turn`    | total words / total turns              | Verbosity calibration  |
| `turn_count`            | total turns                            | Protocol completion    |

Proxy metrics are **not** weighted into overall scores but used for:
- Fast pre-screening (abort runs with obvious failures)
- Sanity checks before expensive judge calls
- Trend monitoring in production telemetry

### 3.4 Runners

Protocol-specific runners orchestrate the eval loop:

```go
// Runner executes evals for a specific protocol.
type Runner interface {
    Protocol() string
    Run(ctx context.Context, cfg RunConfig) (Report, error)
}

// RunConfig is the shared configuration surface.
type RunConfig struct {
    Models      []string        // Models to evaluate
    Variants    []VariantSpec   // Protocol config variants
    Scenarios   []Scenario      // Test cases
    RunsPerCase int             // Repetitions for statistical power
    Timeout     time.Duration   // Per-run timeout
    Judge       Judge           // LLM judge (nil to skip)
    ShowTurns   bool            // Debug output
    Telemetry   TelemetryConfig // Emit spans to OTEL?
}

// VariantSpec defines one protocol configuration variant.
type VariantSpec struct {
    Name        string
    Description string
    Options     map[string]any  // Protocol-specific options
}
```

Each protocol registers its runner in `pkg/eval/<protocol>/runner.go`.

### 3.5 Regression Gates

Regression detection compares new results against a baseline:

```go
// RegressionConfig controls pass/fail thresholds.
type RegressionConfig struct {
    Weights          DimensionScores // Dimension weights
    MaxOverallDrop   float64          // Max allowed weighted average drop (e.g., 0.25)
    MaxCriticalDrop  float64          // Max allowed drop on critical dimensions (e.g., 0.40)
    CriticalDims     []string        // Dimensions where drops are fatal
    RequireAllKeys   bool            // Fail if baseline/current keys don't match
    MinSampleSize    int             // Minimum runs required for valid comparison
}
```

**Regression rules**:
1. Weighted overall score drop > threshold → FAIL
2. Any critical dimension drop > threshold → FAIL  
3. Error rate increase → FAIL
4. Missing scenarios from baseline → WARN or FAIL (configurable)

---

## 4. Telemetry Integration

### 4.1 Required OTEL Attributes

For eval to work from traces (without re-running sessions), the runtime must emit these span attributes:

```go
// Session trace attributes
"session.id"
"session.protocol"
"session.model"
"session.variant"           // Optional: variant label
"session.scenario_id"       // When running evals
"session.run_id"            // Unique run identifier

// Agent span attributes  
"agent.id"
"agent.name"
"agent.role"                // facilitator, participant, etc.
"agent.persona_id"          // Reference to prompt version
"agent.prompt_version"      // Explicit version string

// Message events
"message.speaker"
"message.text"
"message.turn_index"
```

### 4.2 Langfuse-Native Scoring

When Langfuse telemetry is enabled, eval can push scores back to traces:

```go
// After judge scoring, annotate the trace
span.SetAttribute("eval.judge_model", score.Model)
span.SetAttribute("eval.overall", score.Overall)
span.SetAttribute("eval.flow_arc", score.Dimensions.FlowArc)
// ... etc

span.AddEvent("eval.score", map[string]any{
    "dimensions": score.Dimensions,
    "summary":    score.Summary,
})
```

This enables Langfuse dashboards to show eval scores alongside production traces.

### 4.3 Eval-from-Traces Mode

For production monitoring, eval can score existing traces without re-running:

```go
// TraceEvalConfig for scoring historical traces
type TraceEvalConfig struct {
    TraceIDs    []string       // Specific traces to score
    Filter      TraceFilter    // Or query by time/tags
    Judge       Judge
    RubricID    string
}

// TraceFilter queries traces from the observability backend
type TraceFilter struct {
    Protocol   string
    TimeRange  TimeRange
    Tags       []string
    SampleRate float64        // Score N% of matching traces
}
```

---

## 5. Protocol Participant Guidance

### 5.1 The README Pattern

Since protocols don't govern participant prompts, each protocol should include a README that explains how to configure participants for best results. This serves multiple purposes:

1. **Human developers** read it when setting up sessions
2. **Future auto-mode** uses it as context for generating participants
3. **Eval interpretation** helps understand when poor results are due to participant setup vs. protocol issues

### 5.2 Protocol README Structure

```markdown
# Brainstorming Protocol

## Purpose
Generate diverse ideas through structured multi-agent discussion, converging to a shortlist.

## Recommended Participant Setup

### Facilitator (Required)
- Role: Moderator who guides phases and synthesizes
- Prompt guidance: Should be directive but not dominating. Ask probing questions.
- Avoid: Generating ideas themselves; taking sides

### Participants (2-5 recommended)
- Role: Domain experts who generate and critique ideas
- Prompt guidance: Each should have distinct expertise and communication style
- Recommended archetypes:
  - Domain expert (deep knowledge)
  - Generalist (cross-cutting perspective)  
  - Skeptic (challenges assumptions)
  - Pragmatist (focuses on feasibility)

### What Makes This Protocol Work Well
- Distinct personas that don't echo each other
- Facilitator who actively manages turn-taking
- Participants who build on each other's ideas

### What Makes This Protocol Fail
- Homogeneous participants (all agree immediately)
- Passive facilitator (lets conversation drift)
- Participants who monologue without reacting

## Eval Dimensions for This Protocol
This protocol is evaluated on: flow_arc, convergence_quality, idea_quality, report_quality
Optional participant dimension: persona_separation (if distinct personas configured)
```

### 5.3 Auto-Mode Integration (Future)

When Studio implements auto-participant generation:

```go
// AutoParticipantConfig for generating participants
type AutoParticipantConfig struct {
    Protocol        string           // Protocol ID
    ProtocolREADME  string           // Loaded from protocol package
    Context         string           // User's session context/topic
    DesiredCount    int              // How many participants
    Constraints     []string         // User constraints ("must include legal expert")
}

// The README becomes part of the generation prompt
func GenerateParticipants(ctx context.Context, cfg AutoParticipantConfig) ([]Agent, error) {
    // Uses protocol README + context to generate appropriate participants
}
```

---

## 6. Prompt Versioning

### 6.1 Problem Statement

Agent prompts and judge rubrics evolve over time. We need to:
- Track which prompt version produced each result
- Compare results across prompt versions
- Roll back to known-good prompts
- Keep prompts out of core runtime code

### 6.2 Proposed Structure

```
prompts/
├── agents/
│   ├── brainstorming/
│   │   ├── moderator/
│   │   │   ├── v1.md
│   │   │   ├── v2.md
│   │   │   └── current -> v2.md
│   │   ├── participant/
│   │   │   └── v1.md
│   │   └── roles/
│   │       ├── marketing.md
│   │       ├── engineering.md
│   │       └── design.md
│   └── consensus/
│       └── ...
└── evals/
    └── rubrics/
        ├── brainstorming/
        │   ├── v1.yaml
        │   └── current -> v1.yaml
        └── consensus/
            └── ...
```

### 6.3 Prompt Registry API

```go
// PromptRegistry loads versioned prompts.
type PromptRegistry interface {
    // Get returns a prompt by path and version
    Get(ctx context.Context, path, version string) (Prompt, error)
    
    // GetCurrent returns the current/default version
    GetCurrent(ctx context.Context, path string) (Prompt, error)
    
    // List returns available versions for a path
    List(ctx context.Context, path string) ([]PromptVersion, error)
}

// Prompt is a loaded prompt with metadata.
type Prompt struct {
    Path      string
    Version   string
    Content   string
    UpdatedAt time.Time
    Metadata  map[string]any
}
```

### 6.4 Runtime Integration (Minimal)

The runtime can optionally load prompts from the registry:

```go
// Agent builder with prompt registry
moderator := eng.Agent("Moderator").
    PromptFromRegistry("agents/brainstorming/moderator", "v2").  // Explicit version
    // OR
    PromptFromRegistry("agents/brainstorming/moderator", "current"). // Follow current
    Build()
```

This is **optional**—hardcoded prompts still work. The registry is for teams that want versioning.

---

## 7. Codebase Organization

### 7.1 What Goes Where

| Component              | Location                         | Rationale                          |
| ---------------------- | -------------------------------- | ---------------------------------- |
| Eval types, interfaces | `pkg/eval/`                      | Core eval library, no runtime deps |
| Protocol runners       | `pkg/eval/<protocol>/`           | Protocol-specific eval logic       |
| Judge implementations  | `pkg/eval/<protocol>/judge_*.go` | Model-specific judges              |
| Proxy metrics          | `pkg/eval/metrics.go`            | Shared deterministic metrics       |
| Regression logic       | `pkg/eval/regression.go`         | Shared comparison logic            |
| CLI tools              | `cmd/protocol-eval/`             | Entry points for running evals     |
| Datasets               | `evals/datasets/`                | JSON scenario files                |
| Rubrics                | `evals/rubrics/`                 | YAML judge configurations          |
| Agent prompts          | `prompts/`                       | Versioned agent prompts            |
| Eval artifacts         | `artifacts/evals/`               | Generated reports (gitignored)     |

### 7.2 What Does NOT Go in pkg/engine

- Eval-specific types or logic
- Judge code
- Rubric loading
- Prompt registry (optional feature, not required for runtime)

The engine's only eval-related responsibility is emitting properly attributed OTEL spans.

### 7.3 Meanwhile Studio (Future)

If/when a Studio UI exists, it would provide:
- Prompt editor with versioning
- Eval run dashboard
- Regression trend charts
- A/B test configuration
- Production trace scoring

Studio would use `pkg/eval` as its backend library.

---

## 8. CLI Design

### 8.1 Unified Eval Command

```bash
# Run brainstorming eval with defaults
meanwhile eval --protocol brainstorming

# Compare models
meanwhile eval --protocol brainstorming \
  --models "gpt-5.2-chat-latest,claude-4-opus" \
  --runs 5

# Use custom dataset and rubric
meanwhile eval --protocol brainstorming \
  --dataset evals/datasets/brainstorming/product-cases.json \
  --rubric evals/rubrics/brainstorming/strict-v2.yaml

# Regression check against baseline
meanwhile eval --protocol brainstorming \
  --baseline artifacts/evals/brainstorming/2026-02-01/report.json \
  --fail-on-regression

# Score existing traces (production monitoring)
meanwhile eval-traces \
  --protocol brainstorming \
  --filter "tags:production,time:24h" \
  --sample-rate 0.1
```

### 8.2 CI Integration

```yaml
# .github/workflows/eval.yml
eval:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    
    - name: Run protocol evals
      run: |
        meanwhile eval --protocol brainstorming \
          --baseline baselines/brainstorming-main.json \
          --fail-on-regression \
          --runs 3
    
    - name: Upload artifacts
      uses: actions/upload-artifact@v4
      with:
        name: eval-report
        path: artifacts/evals/
```

---

## 9. Migration Path

### Phase 1: Consolidate Existing Code (Current State → Unified)

1. ✅ `pkg/eval/` already has types, metrics, regression
2. ✅ `pkg/eval/brainstorm/` has runner and judge
3. Move remaining logic from `cmd/brainstorm-compare/` into `pkg/eval/brainstorm/`
4. Add dataset loading to `cmd/protocol-eval/`

### Phase 2: Add Prompt Versioning

1. Create `prompts/` directory structure
2. Implement `PromptRegistry` interface
3. Add optional registry support to agent builder
4. Extract hardcoded prompts from examples

### Phase 3: Telemetry Integration

1. Ensure engine emits required OTEL attributes
2. Add Langfuse score annotation
3. Implement eval-from-traces mode

### Phase 4: Additional Protocols

1. Create `pkg/eval/consensus/` for Consensus protocol
2. Create `pkg/eval/adversarial/` for Adversarial protocol
3. Protocol-specific dimensions and rubrics

### Phase 5: Studio (Optional)

1. Web UI for prompt management
2. Eval dashboard
3. Regression alerting

---

## 10. Open Questions

1. **Protocol vs. Participant attribution**: When a session produces poor results, how do we attribute blame?
   - *Challenge*: A great protocol with bad participants fails. A bad protocol with great participants might still fail.
   - *Recommendation*: Eval reports should separate "protocol adherence" (did it follow the structure?) from "outcome quality" (was the result good?). Poor adherence = protocol/runtime issue. Good adherence + poor outcome = participant configuration issue.

2. **Custom protocol dimensions**: How do users define eval dimensions for their custom protocols?
   - *Recommendation*: Protocols declare dimensions in their package. Framework provides dimension templates (convergence, challenge_depth, etc.) that protocols can compose.

3. **Judge model selection**: Should we default to the same model being tested, or always use a stronger judge model?
   - *Recommendation*: Use strongest available model (e.g., gpt-5.2) to avoid self-grading bias.

4. **Cross-protocol dimensions**: Some dimensions (naturalness, responsiveness) apply to all protocols. Should there be a shared base rubric?
   - *Recommendation*: Yes—framework dimensions are always measured. Protocol dimensions are additive.

5. **Human baseline**: Should we collect human-labeled "gold" transcripts for calibration?
   - *Recommendation*: Yes for high-stakes protocols, captured in `evals/golden/`.

6. **Cost management**: Full evals are expensive. How do we balance thoroughness with cost?
   - *Recommendation*: Tiered approach—fast proxy checks always, LLM judging on merge/release.

7. **Prompt registry backend**: File-based vs. database?
   - *Recommendation*: Start file-based (git-versioned), add DB backend for Studio later.

8. **Protocol README enforcement**: Should protocols be required to include participant guidance?
   - *Recommendation*: Yes for shipped protocols. Optional for user-defined protocols but strongly encouraged.

---

## 11. Success Metrics

The eval system is successful if:

1. **Protocol changes have measured impact** — Every protocol PR includes eval results showing impact on key dimensions.

2. **Regressions are caught** — No significant quality drops ship undetected.

3. **Model comparison is routine** — New models are evaluated before recommendation changes.

4. **Eval runs are reproducible** — Same dataset + rubric + model = consistent scores (within statistical noise).

5. **Prompts are version-controlled** — Every production prompt can be traced to a specific version.

---

## Appendix A: Comparison with External Systems

| Feature | OpenAI Evals | Langfuse | Meanwhile Eval |
|---------|-------------|----------|----------------|
| Primary use case | Model benchmarking | Production observability | Protocol quality |
| Data source | Datasets | Traces | Both |
| Scoring | Graders (code/model) | LLM-as-Judge + human | LLM-as-Judge + proxy |
| Prompt versioning | External | Built-in | File-based registry |
| Regression gates | Manual | Dashboards | Automated CI gates |
| Multi-agent focus | No | No | Yes |

## Appendix B: Example Eval Output

```json
{
  "generated_at": "2026-02-06T14:30:00Z",
  "protocol": "brainstorming",
  "models": ["gpt-5.2-chat-latest", "claude-4-opus"],
  "rubric_id": "brainstorming-v1",
  "runs_per_case": 3,
  "scenarios": [
    {"id": "signalthread-week3", "description": "Week-3 engagement drop"}
  ],
  "summaries": [
    {
      "protocol": "brainstorming",
      "model": "gpt-5.2-chat-latest",
      "variant": "standard",
      "runs": 3,
      "successes": 3,
      "success_rate": 1.0,
      "avg_duration_ms": 45000,
      "proxy": {
        "speaker_balance_ratio": 0.85,
        "direct_reference_rate": 0.42,
        "repetition_rate": 0.02
      },
      "judge_overall": 4.2,
      "judge_dimensions": {
        "flow_arc": 4.5,
        "persona_separation": 4.0,
        "responsiveness": 4.3,
        "naturalness": 4.1,
        "idea_quality": 4.0,
        "convergence_quality": 4.4,
        "report_quality": 4.2
      }
    }
  ],
  "regression": {
    "passed": true,
    "baseline_path": "baselines/brainstorming-main.json",
    "deltas": [
      {
        "key": "brainstorming|gpt-5.2-chat-latest|standard",
        "baseline_overall": 4.1,
        "current_overall": 4.2,
        "overall_drop": -0.1,
        "dimension_drops": {
          "flow_arc": -0.2,
          "naturalness": 0.1
        }
      }
    ]
  }
}
```

## Appendix C: References

- [OpenAI Evals API](https://platform.openai.com/docs/guides/evals)
- [OpenAI simple-evals](https://github.com/openai/simple-evals)
- [Langfuse LLM-as-Judge](https://langfuse.com/docs/evaluation/evaluation-methods/llm-as-a-judge)
- [Langfuse Scores Overview](https://langfuse.com/docs/scores/overview)
- Meanwhile existing: `pkg/eval/`, `cmd/protocol-eval/`, `cmd/brainstorm-compare/`
