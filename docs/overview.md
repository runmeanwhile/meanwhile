# Overview

## Meanwhile in 5 minutes

Meanwhile treats collaboration as a first-class runtime. A **session** is a meeting, **protocols** are meeting formats, and **agents** are participants.

```go
eng, _ := engine.New()

policy := consensus.Consensus(
    consensus.WithAgenda(agenda.WithScope("Policy-level decision")),
    consensus.WithChair(chair.WithInterventions(0.4, 0.7, 0.9)),
    consensus.WithPulseCheck(pulse.WithMaxConditions(3)),
)

sess, _ := eng.Session("Friday Deploy Policy").
    Participants(dev, ops, sec).
    Facilitator(chairAgent).
    Protocol(policy).
    Tags("policy", "release").
    Build(ctx)

result, _ := sess.Run(ctx, message.User("Should we freeze deploys before holidays?"))
fmt.Println(result.Metadata["consensus"])
```

## Human-in-the-loop escalation

Meanwhile supports human participants and the `ask_human` tool for escalation. Requests can route through outbound integrations (Slack, email, webhooks), and responses can return via inbound handlers with a request registry and optional timeout scheduling.

## Toolkits + guardrails

Sessions can attach toolkits (filesystem, system, MCP, internal APIs) and enforce allow/deny policies so agents can act safely. Tools can also pause for long-running work and resume later via `ResumeTool`.

## Why collaboration, not orchestration

Traditional agent frameworks orchestrate steps. Meanwhile structures **collaboration**:

- **Agenda** refines scope and sets boundaries.
- **Chair** nudges facilitation and convergence.
- **Roundtable** manages turn-taking and context packaging.
- **PulseCheck** captures explicit positions.
- **Minutes** provide structured, typed results.
- **Interrupts** let hooks interject at any phase.

Protocols are just compositions of these primitives. That keeps the runtime minimal while the user experience feels cohesive and meeting-like.
