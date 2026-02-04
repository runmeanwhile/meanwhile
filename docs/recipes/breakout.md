# Breakout Recipe

```go
breakout := protocol.Breakout(protocol.WithBreakoutGroupSize(3))

sess, _ := eng.Session("Parking Policy").
    Participants(participants...).
    Facilitator(chairAgent).
    Protocol(breakout).
    Build(ctx)

result, _ := sess.Run(ctx, message.User("Propose fixes for parking congestion"))
fmt.Println(result.Metadata["breakouts"])
```
