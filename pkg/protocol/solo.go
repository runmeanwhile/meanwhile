package protocol

import (
	"context"
	"fmt"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

// Solo runs a single agent for each incoming message.
type solo struct{}

// Solo creates a single-agent protocol.
func Solo() Protocol {
	return &solo{}
}

// ID returns the protocol ID.
func (p *solo) ID() string { return "protocol.solo" }

// Participants returns empty slice - solo doesn't predetermine participants.
func (p *solo) Participants() []Participant { return nil }

// Init is a no-op for solo.
func (p *solo) Init(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}

// OnMessage runs the first participant and emits the result.
func (p *solo) OnMessage(ctx context.Context, sess Session, msg agent.Message) error {
	participants := sess.Participants()
	if len(participants) == 0 {
		return fmt.Errorf("solo protocol requires at least one participant")
	}

	participant := participants[0]
	emit := func(resp agent.Message) error {
		payload := map[string]any{
			"agent":   participant.DisplayName(),
			"message": resp,
		}
		if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload)); err != nil {
			return fmt.Errorf("emit solo: %w", err)
		}
		return nil
	}

	if participant.IsHuman() {
		_, err := sess.RunTurn(ctx, participant, RunRequest{Messages: []agent.Message{msg}},
			WithTurnContext(msg.Summary()),
			WithTurnResume(func(ctx context.Context, resp agent.Message) error {
				_ = ctx
				return emit(resp)
			}),
		)
		return err
	}

	ag, ok := participant.Agent()
	if !ok {
		return fmt.Errorf("solo participant must be an agent")
	}
	resp, err := sess.RunAgent(ctx, ag, RunRequest{Messages: []agent.Message{msg}})
	if err != nil {
		return err
	}
	return emit(resp)
}

// OnEvent is a no-op for solo.
func (p *solo) OnEvent(ctx context.Context, sess Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}

// Shutdown is a no-op for solo.
func (p *solo) Shutdown(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}
