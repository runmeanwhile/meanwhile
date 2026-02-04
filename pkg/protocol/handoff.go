package protocol

import (
	"context"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
)

type handoff struct {
	caller Participant
	callee Participant
}

// Handoff creates a handoff protocol that delegates work from one agent to another.
func Handoff(caller, callee Participant) Protocol {
	return &handoff{
		caller: caller,
		callee: callee,
	}
}

// ID returns the protocol ID.
func (p *handoff) ID() string { return "protocol.handoff" }

// Participants returns the caller and callee agents.
func (p *handoff) Participants() []Participant {
	return []Participant{p.caller, p.callee}
}

// Init is a no-op for handoff.
func (p *handoff) Init(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}

// OnMessage runs the callee participant and emits the result.
func (p *handoff) OnMessage(ctx context.Context, sess Session, msg agent.Message) error {
	emit := func(resp agent.Message) error {
		payload := map[string]any{
			"caller":  p.caller.DisplayName(),
			"callee":  p.callee.DisplayName(),
			"message": resp,
		}
		if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload)); err != nil {
			return fmt.Errorf("emit handoff: %w", err)
		}
		return nil
	}

	if p.callee.IsHuman() {
		_, err := sess.RunTurn(ctx, p.callee, RunRequest{Messages: []agent.Message{msg}},
			WithTurnContext(msg.Summary()),
			WithTurnResume(func(ctx context.Context, resp agent.Message) error {
				_ = ctx
				return emit(resp)
			}),
		)
		return err
	}

	ag, ok := p.callee.Agent()
	if !ok {
		return fmt.Errorf("handoff callee must be an agent")
	}
	resp, err := sess.RunAgent(ctx, ag, RunRequest{Messages: []agent.Message{msg}})
	if err != nil {
		return err
	}
	return emit(resp)
}

// OnEvent is a no-op for handoff.
func (p *handoff) OnEvent(ctx context.Context, sess Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}

// Shutdown is a no-op for handoff.
func (p *handoff) Shutdown(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}
