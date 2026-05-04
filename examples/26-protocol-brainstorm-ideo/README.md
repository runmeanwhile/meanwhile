# Example 26: IDEO-Inspired Brainstorming Protocol

This example demonstrates the IDEO-inspired multi-session brainstorming protocol that applies first principles from design agency practice.

## Knowledge Base: FlowForge Context

This example uses a **real semantic search** over organizational documents stored in `examples/_shared/flowforge-context/`. The knowledge base simulates a B2B SaaS company (FlowForge - workflow automation) facing PLG onboarding challenges.

**Documents include:**
- **Wiki/Product**: Product overview, activation metrics, onboarding state, roadmap, competitors
- **Wiki/Marketing**: PLG strategy, trial funnel analysis, persona definitions, messaging
- **Wiki/Engineering**: Technical architecture, onboarding tech debt, API guide
- **Customer Feedback**: Feature requests, bug reports, support tickets, NPS responses
- **Sales**: Meeting notes with prospects, lost deal analysis, win/loss patterns

The recall tool uses OpenAI embeddings (`text-embedding-3-small`) to vectorize documents and perform semantic search—you don't need exact keyword matches.

## Key Features

1. **Distinct Phases with Distinct Mindsets**
   - **Inspiration**: Empathize, observe, gather tensions before jumping to solutions
   - **Reframe**: Generate diverse HMW (How Might We) questions across multiple lenses
   - **Ideation**: Generate wild, divergent concepts with artifact-based thinking
   - **Synthesis**: Converge to experiment-ready portfolio with evidence gates

2. **Deliberate Context Transfer**
   - Each phase receives *curated* context from prior phases, not full transcripts
   - Prevents anchoring bias while preserving essential insights
   - Configurable: `TransferSummaryOnly`, `TransferWithHistory`, or `TransferFull`

3. **Semantic Stage Planning (Tool-Driven)**
   - Moderator calls `set_stage_plan` before phase work begins
   - Plan captures non-negotiables, relevant lenses, prioritized tools, and key questions
   - Lenses are selected semantically by the model (not hard-coded per problem pattern)

4. **IDEO Brainstorming Rules Built-In**
   - Defer judgment (no "yes, but..." during ideation)
   - Encourage wild ideas
   - Build on others' ideas ("yes, and...")
   - Go for quantity
   - Be visual (artifact tools)

5. **Diversity Injection**
   - Moderator nudges agents toward different disciplines/lenses each round
   - Mental model prompts rotate to encourage different thinking styles
   - User vantage points ensure we consider multiple user types

6. **Artifact-Based Thinking**
   - `sketch_diagram`: Create mermaid diagrams (flows, journeys)
   - `sketch_concept_card`: Structured concept cards (problem, mechanism, value, risk)
   - `sketch_journey`: User journey maps with emotional arcs

7. **Human Validation Hook** (optional)
   - Protocol exposes a synthesis hook for stakeholder validation
   - This example run keeps it disabled by default to avoid placeholder feedback

8. **Metric-Linked Evidence Gate**
   - Experiment cards now require baseline/target/delta/segment/time-to-impact fields
   - Cards lacking concrete evidence references are rejected

## Running

```bash
# Basic run (requires OPENAI_API_KEY for embeddings and LLM)
go run main.go

# With custom topic
go run main.go "How can we reduce time-to-first-workflow for new trial users?"

# With smaller model
MEANWHILE_MODEL=gpt-4o-mini go run main.go

# Show full metadata
DUMP_METADATA_JSON=1 go run main.go
```

## Configuration Options

```go
ideo.Brainstorm(
    // Phase rounds
    ideo.WithInspirationRounds(2),
    ideo.WithReframeRounds(3),
    ideo.WithIdeationRounds(2),
    ideo.WithSynthesisRounds(2),

    // Output targets
    ideo.WithTargetHMWs(8),
    ideo.WithTargetConcepts(15),
    ideo.WithFinalistCount(3),

    // Features
    ideo.WithArtifactTools(true),
    ideo.WithHumanInLoop(false),

    // Context transfer
    ideo.WithTransferStrategy(ideo.TransferWithHistory),
)
```

## Differences from BrainstormingLab

| Feature | BrainstormingLab | Brainstorm IDEO |
|---------|------------------|-----------------|
| Phase separation | Single session | Distinct phases with handoffs |
| Context transfer | Full thread | Curated summaries |
| Reframe rounds | 1 round | 3 rounds (configurable) |
| Diversity injection | Limited | Explicit nudges per round |
| Artifact tools | None | Diagram, card, journey |
| Human validation | Limited | Validation hook (manual integration) |

## Architecture

See [docs/design/brainstorm-ideo-architecture.md](../../docs/design/brainstorm-ideo-architecture.md) for the full design document.
