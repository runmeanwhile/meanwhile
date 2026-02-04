# Build a Protocol

This guide shows a thin protocol composed from Collaboration Kit primitives.

```go
// A lightweight "review" protocol: one round of turns + minutes
type Review struct{}

func (Review) ID() string { return "protocol.review" }
func (Review) Participants() []protocol.Participant { return nil }
func (Review) Init(ctx context.Context, _ protocol.Session) error { return nil }
func (Review) OnEvent(ctx context.Context, _ protocol.Session, _ event.Event) error { return nil }
func (Review) Shutdown(ctx context.Context, _ protocol.Session) error { return nil }

func (Review) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
    participants := sess.Participants()
    comments := make([]agent.Message, 0, len(participants))
    for _, participant := range participants {
        ag, ok := participant.Agent()
        if !ok {
            return fmt.Errorf("review protocol requires agent participants")
        }
        resp, err := sess.RunAgent(ctx, ag, protocol.RunRequest{Messages: []agent.Message{msg}})
        if err != nil {
            return err
        }
        comments = append(comments, resp)
    }

    mins := minutes.New()
    mins.Add("reviews", comments)
    return sess.EmitWithContext(ctx, event.New(event.ProtocolAction, sess.ID(), mins.Payload()))
}
```

Keep protocols thin: rely on Agenda, Chair, Roundtable, PulseCheck, and Minutes whenever possible.
