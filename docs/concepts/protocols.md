# Protocols

Protocols are composable meeting formats. They assemble Collaboration Kit components into a run loop.

## Assemblies

- **Consensus** = Agenda + Chair + Roundtable + PulseCheck + Minutes
- **Debate** = Roundtable + Minutes (+ optional Chair)
- **Brainstorming** = Scope refinement + moderator + Divergent caucus + Roundtable (interaction) + Minutes (+ optional voting)
- **Breakout** = Roundtable (grouped) + Minutes
- **Caucus** = Agenda + private rounds + Minutes

## Example

```go
consensus := consensus.Consensus(
    consensus.WithAgenda(agenda.WithScope("Policy-level decision")),
    consensus.WithChair(chair.WithInterventions(0.4, 0.7, 0.9)),
    consensus.WithPulseCheck(pulse.WithMaxConditions(3)),
)
```

Protocols emit `event.ProtocolAction` and the engine captures the final payload in `RunResult.Metadata`.
