# Consensus Recipe

```go
policy := consensus.Consensus(
    consensus.WithAgenda(agenda.WithScope("Policy-level decision")),
    consensus.WithChair(chair.WithInterventions(0.4, 0.7, 0.9)),
    consensus.WithPulseCheck(pulse.WithMaxConditions(3)),
)

sess, _ := eng.Session("Policy Review").
    Participants(dev, ops, sec).
    Facilitator(chairAgent).
    Protocol(policy).
    Build(ctx)

result, _ := sess.Run(ctx, message.User("Should we freeze deploys before holidays?"))
fmt.Println(result.Metadata["consensus"])
```
