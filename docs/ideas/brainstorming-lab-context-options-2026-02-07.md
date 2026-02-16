# Brainstorming Lab Context Options (2026-02-07)

This note explains how the new `protocol.brainstorming_lab` can balance memory and tool-based research without blurring protocol vs collab-kit boundaries.

## Boundary model

- **Protocol (`pkg/protocol/brainstorming_lab.go`)**: orchestrates phases and applies strategy.
- **Collab kits (`pkg/collab/*`)**: reusable primitives independent from protocol arc.
  - `insightpack`: context-intake planning and tool-policy shaping
  - `reframer`: HMW reframing helpers
  - `evidencegate`: experiment-readiness validation

## What is configurable today

### 1. Context strategy

Choose one strategy:

- `memory_first`: minimal tool calls, lean on prior transcript/memory context.
- `balanced`: memory + targeted tool calls.
- `research_heavy`: aggressive retrieval and search.

Set via:

- `WithBrainstormingLabContextStrategy(...)`
- or full plan: `WithBrainstormingLabContextPlan(...)`

### 2. Tool scope and policy

Set which tools are in-bounds by source:

- `WithBrainstormingLabContextSource(insightpack.Source{...ToolIDs: [...]})`

The protocol converts this into a per-run allowlist policy (`tool.PolicyAllowlist`) and passes it through `RunRequest.ToolPolicy` and `RunRequest.Tools`.

### 3. Budgets

Two explicit budgets are supported:

- Tool iteration budget: `WithBrainstormingLabToolBudget(int)`
- Source budget: `WithBrainstormingLabSourceBudget(int)`

You can also set both in `insightpack.Plan.Budget`.

## Recommended operating presets

### Preset A: Memory-first product team (fastest)

Use when you already have rich session memory and internal notes.

- Strategy: `memory_first`
- Tools: memory + one internal doc search tool only
- Budget: `max_tool_iterations=2`, `max_sources=4`

### Preset B: Balanced default (recommended)

Use for most teams.

- Strategy: `balanced`
- Tools: memory + internal docs + issue search
- Budget: `max_tool_iterations=6`, `max_sources=8`

### Preset C: Research-heavy discovery sprint

Use for greenfield or market-shift topics.

- Strategy: `research_heavy`
- Tools: memory + internal docs + web research
- Budget: `max_tool_iterations=10`, `max_sources=12`

## Suggested next extension (optional)

If you want protocol-agnostic indexing/retrieval governance, add a separate "context index builder" utility that:

1. builds curated memory/doc indexes,
2. tags trusted sources,
3. outputs source IDs + tool allowlists consumed by `insightpack.Plan`.

That keeps heavy indexing outside protocol logic while preserving protocol portability.
