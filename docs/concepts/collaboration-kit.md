# Collaboration Kit

The Collaboration Kit is a set of reusable primitives that encode human meeting behavior while staying protocol-agnostic.
Protocols own the language and prompts; the kit exposes structure and hooks to plug them in.

## Components

- **Agenda**: Refines scope, sets boundaries, outcomes, and optional agenda items/briefs.
- **Chair**: Facilitates interventions, tone, and closing summaries.
- **Roundtable**: Manages turn order and context packaging.
- **PulseCheck**: Captures explicit positions or votes.
- **Minutes**: Aggregates structured outcomes (decisions, actions, risks, questions).
- **Caucus**: Runs private per-participant rounds to preserve independent thinking.
- **Interrupts**: Hook-driven interjections that can modify or inject turns.
- **Planning**: Creates structured implementation plans using a planning agent.

## Example

```go
agenda := agenda.New(agenda.WithScope("Policy-level decision"))
chair := chair.New(chair.WithInterventions(0.4, 0.7, 0.9))
rounds := roundtable.New(roundtable.WithMaxRounds(6))
pulse := pulse.New(participants, pulse.WithRoundProvider(rounds.CurrentRound))
mins := minutes.New()
```

Use these pieces to assemble custom protocols without rewriting core orchestration logic.

**Planning** is unique: it doesn't orchestrate meetings, it creates structured plans that main agents/protocols execute. Pattern: planning agent creates plan → main agent executes using plan as context.
