# Brainstorm IDEO Protocol: Multi-Session Architecture

## Design Rationale

This protocol applies first principles from IDEO/design agency practice:

1. **Distinct phases with distinct mindsets** - Inspiration, Reframing, Ideation, and Synthesis are cognitively different modes. Mixing them in one conversation leads to premature convergence.

2. **Deliberate context transfer** - Each phase should receive *only* what it needs from prior phases, not the full transcript. This prevents anchoring and allows fresh thinking.

3. **Artifact-based thinking** - Agents sketch concepts using structured tools (diagrams, cards, tables), not just prose. This forces concreteness.

4. **Diversity injection** - The moderator explicitly nudges agents toward different disciplines, mental models, and user perspectives.

5. **Human-in-the-loop evidence** - Human stakeholders can be consulted to validate assumptions during synthesis.

## IDEO Principles Applied

From IDEO's design thinking methodology:

- **Defer judgment** - Inspiration phase suspends evaluation; Synthesis evaluates
- **Encourage wild ideas** - Ideation uses operators that force unconventional thinking
- **Build on others' ideas** - Sessions see prior work and explicitly build
- **Stay on topic** - Each session has a focused scope derived from prior handoff
- **One conversation at a time** - Sequential turns within rounds, parallel across agents within phases
- **Be visual** - Artifact tools produce mermaid diagrams, concept cards, journey maps
- **Go for quantity** - Ideation rewards volume before filtering

## Session Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         BRAINSTORM IDEO PROTOCOL                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Session 1: INSPIRATION                                                  │
│  ├─ Goal: Empathize, observe, gather tensions                           │
│  ├─ Rounds: 2-3 discovery rounds                                        │
│  ├─ Tools: recall_context, sketch_diagram, user_insight                 │
│  ├─ Output: InsightPack (tensions, observations, constraints)           │
│  └─ Transfer: Full InsightPack → Reframe                                │
│                                                                          │
│  Session 2: REFRAME                                                      │
│  ├─ Goal: Generate diverse HMW framings                                 │
│  ├─ Rounds: 2-3 reframe rounds (each adds lenses)                       │
│  ├─ Tools: submit_hmw, sketch_diagram                                   │
│  ├─ Moderator: Nudges for discipline/perspective diversity              │
│  ├─ Output: FrameSet (8-12 HMW questions across lenses)                 │
│  └─ Transfer: Top 4-6 frames + key tensions → Ideation                  │
│                                                                          │
│  Session 3: IDEATION                                                     │
│  ├─ Goal: Generate divergent concepts with wild ideas                   │
│  ├─ Rounds: 2-3 concept rounds                                          │
│  ├─ Tools: sketch_concept_card, sketch_journey, sketch_diagram          │
│  ├─ Operators: analogy, inversion, constraint-flip, mashup              │
│  ├─ Output: ConceptPool (15-20 rough concepts with artifacts)           │
│  └─ Transfer: Clustered concepts + artifacts → Synthesis                │
│                                                                          │
│  Session 4: SYNTHESIS                                                    │
│  ├─ Goal: Converge to experiment-ready portfolio                        │
│  ├─ Rounds: 1-2 critique rounds, 1 evidence round                       │
│  ├─ Tools: ask_human (stakeholder validation), evidence_card            │
│  ├─ Human: Product manager can confirm/challenge assumptions            │
│  ├─ Output: Portfolio (safe/adjacent/bold bets with test plans)         │
│  └─ Transfer: Portfolio + closing summary → Final result                │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Context Transfer Strategy

### Why Not Transfer Everything?

1. **Anchoring bias** - Full transcripts anchor agents to early ideas
2. **Token efficiency** - Large context windows are expensive and slow
3. **Focus** - Each phase needs different information
4. **Fresh perspectives** - Synthesis shouldn't be colored by ideation debates

### Transfer Matrix

| From Session | To Session | What Transfers | What Doesn't |
|--------------|------------|----------------|--------------|
| Inspiration → Reframe | Tensions, observations, constraints, key quotes | Turn-by-turn debate, tool call details |
| Reframe → Ideation | Top HMW frames (4-6), rationales, key tensions | Rejected frames, HMW generation discussion |
| Ideation → Synthesis | Concept cards, artifacts, author attributions | Exploration tangents, rough drafts |
| Synthesis → Output | Portfolio bets, human feedback, closing | Internal critique, evidence collection details |

### Implementation: TransferPacket

```go
// TransferPacket carries curated context between sessions
type TransferPacket struct {
    // Structured data for programmatic access
    Data map[string]any `json:"data"`
    
    // Human-readable summary for system prompts
    Summary string `json:"summary"`
    
    // Optional: inject into session memory as prior messages
    PriorMessages []agent.Message `json:"prior_messages,omitempty"`
    
    // Strategy for how to use this packet
    Strategy TransferStrategy `json:"strategy"`
}

type TransferStrategy string
const (
    // Include summary in system prompt only
    TransferSummaryOnly TransferStrategy = "summary"
    
    // Inject prior messages + summary
    TransferWithHistory TransferStrategy = "with_history"
    
    // Full data available via context, summary in prompt
    TransferFull TransferStrategy = "full"
)
```

## Diversity Injection

The moderator uses explicit nudges to encourage cognitive diversity:

### Discipline Nudges (rotated per round)
- "Consider this from an operations perspective..."
- "What would a behavioral economist notice here?"
- "How would a service designer approach this?"
- "What does the data tell us vs. what do users say?"

### Mental Model Prompts
- Systems thinking: "What are the feedback loops?"
- Jobs-to-be-done: "What job is the user hiring this to do?"
- First principles: "What must be true for this to work?"
- Constraint analysis: "What if we removed [constraint]?"

### User Vantage Points
- Power users vs. newcomers
- Happy path vs. edge cases
- Individual vs. team context
- Mobile vs. desktop

## Artifact Tools

Each tool produces structured output that can be referenced in later sessions:

### sketch_diagram
```json
{
  "type": "mermaid",
  "title": "User Onboarding Journey",
  "content": "graph LR\n  A[Signup] --> B[Profile]\n  B --> C{Skip?}\n  C -->|Yes| D[Dashboard]\n  C -->|No| E[Tutorial]",
  "author": "Builder",
  "context": "Shows the two paths through onboarding"
}
```

### sketch_concept_card
```json
{
  "title": "Progressive Disclosure Signup",
  "problem": "Users abandon when asked for too much upfront",
  "mechanism": "Ask only email first, defer profile to first value moment",
  "value": "2x completion rate in prior A/B test",
  "risk": "May lose profile data forever",
  "author": "Strategist"
}
```

### sketch_journey
```json
{
  "title": "First Week Experience",
  "stages": [
    {"name": "Day 1", "user_action": "Sign up", "emotion": "curious", "touchpoint": "Landing page"},
    {"name": "Day 2", "user_action": "First task", "emotion": "confused", "touchpoint": "Dashboard"},
    {"name": "Day 7", "user_action": "Invite team", "emotion": "invested", "touchpoint": "Settings"}
  ],
  "author": "Critic"
}
```

## Human-in-the-Loop Integration

The synthesis session can consult human stakeholders for evidence:

```go
// AskHumanConfig configures human consultation
type AskHumanConfig struct {
    // Who to ask - defaults to configured stakeholders
    Stakeholders []Stakeholder
    
    // How long to wait for response
    Timeout time.Duration
    
    // What to do if no response
    TimeoutBehavior TimeoutBehavior
}

type Stakeholder struct {
    Name    string
    Email   string
    Role    string
    Context string // What they know about
}
```

For MVP, we hardcode a default stakeholder but the tool is available.

## Session System Prompts

Each session gets a tailored system prompt that:

1. Sets the mindset (explore vs. build vs. critique)
2. Lists available tools and when to use them
3. Explains what context they have from prior sessions
4. Defines success criteria for this phase

See individual session files for full prompts.

## Configuration

```go
type IDEOConfig struct {
    // Session configuration
    InspirationRounds   int // Default: 2
    ReframeRounds       int // Default: 3
    IdeationRounds      int // Default: 2
    SynthesisRounds     int // Default: 2
    
    // Output targets
    TargetHMWs          int // Default: 8
    TargetConcepts      int // Default: 15
    FinalistCount       int // Default: 3
    
    // Context transfer
    TransferStrategy    TransferStrategy // Default: summary
    
    // Tools
    ContextPlan         insightpack.Plan
    ArtifactTools       bool // Enable sketch_* tools
    HumanInLoop         bool // Enable ask_human
    
    // Human stakeholders (for ask_human tool)
    Stakeholders        []Stakeholder
    
    // Diversity injection
    DisciplineNudges    []string
    MentalModelPrompts  []string
}
```

## File Structure

```
pkg/protocol/ideo/
├── brainstorm_ideo.go          # Main protocol + orchestration
├── brainstorm_ideo_config.go   # Configuration + options
├── session_inspiration.go      # Inspiration phase
├── session_reframe.go          # Reframing phase  
├── session_ideation.go         # Ideation phase
├── session_synthesis.go        # Synthesis phase
├── transfer.go                 # Context transfer logic
├── artifacts.go                # Artifact tool definitions
├── prompts.go                  # System prompts
└── helpers.go                  # Shared utilities
```

## Example Usage

```go
eng.Session("Product Brainstorm").
    Participants(strategist, builder, critic).
    Facilitator(moderator).
    Protocol(ideo.Brainstorm(
        ideo.WithInspirationRounds(2),
        ideo.WithReframeRounds(3),
        ideo.WithArtifactTools(true),
        ideo.WithHumanInLoop(true),
        ideo.WithStakeholder(ideo.Stakeholder{
            Name:    "Darko",
            Email:   "darko.stanimirovic@gmail.com",
            Role:    "Product Manager",
            Context: "data classification, UX, AI agentic design",
        }),
    )).
    Start(ctx)
```
