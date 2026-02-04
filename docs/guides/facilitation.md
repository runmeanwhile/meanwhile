# Facilitation

Use the Chair and Agenda to guide tone, scope, and convergence.

## Chair Interventions

```go
consensus := consensus.Consensus(
    consensus.WithChair(chair.WithInterventions(0.4, 0.7, 0.9)),
)
```

## Agenda Scope

```go
consensus := consensus.Consensus(
    consensus.WithAgenda(agenda.WithScope("Policy-level decision")),
)
```

## PulseCheck Rules

```go
consensus := consensus.Consensus(
    consensus.WithPulseCheck(
        pulse.WithMinRounds(2),
        pulse.WithMaxConditions(3),
    ),
)
```

Facilitation should feel like a meeting chair: clear, brief, and progressively more assertive near the end.
