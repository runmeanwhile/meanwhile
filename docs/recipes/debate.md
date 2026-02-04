# Debate Recipe

```go
debate := protocol.Debate()

sess, _ := eng.Session("Policy Debate").
    Participants(pro, con).
    Facilitator(chairAgent).
    Protocol(debate).
    Build(ctx)

result, _ := sess.Run(ctx, message.User("Adopt a four-day workweek?"))
fmt.Println(result.Metadata["pro"], result.Metadata["con"])
```
