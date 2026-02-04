package protocol

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/minutes"
	"github.com/runmeanwhile/meanwhile/pkg/collab/roundtable"
	"github.com/runmeanwhile/meanwhile/pkg/event"
)

var errNotEnoughParticipants = errors.New("adversarial protocol requires at least two participants")

type adversarial struct {
	cfg          adversarialConfig
	pro          agent.Message
	con          agent.Message
	synthesis    agent.Message
	participants []string
}

type adversarialConfig struct {
	ProPrefix   string
	ConPrefix   string
	SynthesisFn func(topic, pro, con string) string
}

// AdversarialOption configures adversarial behavior.
type AdversarialOption func(*adversarialConfig)

// Adversarial creates an adversarial debate protocol where two agents take
// opposing positions with optional facilitator synthesis.
func Adversarial(opts ...AdversarialOption) Protocol {
	cfg := adversarialConfig{
		ProPrefix:   "Argue in favor: ",
		ConPrefix:   "Argue against: ",
		SynthesisFn: buildDebateSynthesisPrompt,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &adversarial{cfg: cfg}
}

// Debate is a friendly alias for Adversarial.
// Creates a debate protocol where two agents take opposing positions.
func Debate(opts ...AdversarialOption) Protocol {
	return Adversarial(opts...)
}

// WithDebatePrefixes sets prompt prefixes for pro and con arguments.
func WithDebatePrefixes(proPrefix, conPrefix string) AdversarialOption {
	return func(cfg *adversarialConfig) {
		if proPrefix != "" {
			cfg.ProPrefix = proPrefix
		}
		if conPrefix != "" {
			cfg.ConPrefix = conPrefix
		}
	}
}

// WithDebateSynthesis overrides the synthesis prompt builder.
func WithDebateSynthesis(fn func(topic, pro, con string) string) AdversarialOption {
	return func(cfg *adversarialConfig) {
		if fn != nil {
			cfg.SynthesisFn = fn
		}
	}
}

// ID returns the protocol ID.
func (p *adversarial) ID() string { return "protocol.adversarial_debate" }

// Participants returns empty slice - adversarial gets participants from session.
func (p *adversarial) Participants() []Participant { return nil }

// Init is a no-op for adversarial.
func (p *adversarial) Init(ctx context.Context, _ Session) error {
	_ = ctx
	return nil
}

// OnMessage runs two agents in opposing positions with optional synthesis.
func (p *adversarial) OnMessage(ctx context.Context, sess Session, msg agent.Message) error {
	participants := sess.Participants()
	if len(participants) < 2 {
		return errNotEnoughParticipants
	}

	pro := participants[0]
	con := participants[1]

	topic := msg.Text()
	if strings.TrimSpace(topic) == "" {
		topic = msg.Summary()
	}
	proPrompt := PromptWithMedia(fmt.Sprintf("%s%s", p.cfg.ProPrefix, topic), msg)
	conPrompt := PromptWithMedia(fmt.Sprintf("%s%s", p.cfg.ConPrefix, topic), msg)

	var proResp agent.Message
	var conResp agent.Message

	finalize := func() error {
		var synthesis agent.Message
		if facilitator := sess.Facilitator(); facilitator != nil {
			prompt := p.cfg.SynthesisFn(topic, proResp.Summary(), conResp.Summary())
			resp, err := sess.RunAgent(ctx, *facilitator, RunRequest{Messages: []agent.Message{PromptWithMedia(prompt, msg)}})
			if err != nil {
				return err
			}
			synthesis = resp
		}
		if proResp.Name == "" {
			proResp.Name = pro.DisplayName()
		}
		if conResp.Name == "" {
			conResp.Name = con.DisplayName()
		}
		p.pro = proResp
		p.con = conResp
		p.synthesis = synthesis
		p.participants = []string{pro.DisplayName(), con.DisplayName()}

		mins := minutes.New()
		mins.Add("pro", proResp)
		mins.Add("con", conResp)
		mins.Add("synthesis", synthesis)
		mins.Add("participants", []string{pro.DisplayName(), con.DisplayName()})

		if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), mins.Payload())); err != nil {
			return fmt.Errorf("emit debate: %w", err)
		}
		return nil
	}

	if !pro.IsHuman() && !con.IsHuman() {
		proAgent, ok := pro.Agent()
		if !ok {
			return fmt.Errorf("pro participant must be an agent")
		}
		conAgent, ok := con.Agent()
		if !ok {
			return fmt.Errorf("con participant must be an agent")
		}
		turns := []roundtable.Turn{
			{Agent: proAgent, Messages: []agent.Message{proPrompt}},
			{Agent: conAgent, Messages: []agent.Message{conPrompt}},
		}
		results, err := roundtable.RunSequential(ctx, roundtableRunner(sess), turns)
		if err != nil {
			return err
		}
		proResp = results[0].Message
		conResp = results[1].Message
		return finalize()
	}

	runCon := func() error {
		if con.IsHuman() {
			return sess.AwaitInput(ctx, con, conPrompt.Summary(), func(ctx context.Context, resp agent.Message) error {
				_ = ctx
				conResp = resp
				return finalize()
			})
		}
		ag, ok := con.Agent()
		if !ok {
			return fmt.Errorf("con participant must be an agent")
		}
		resp, err := sess.RunAgent(ctx, ag, RunRequest{Messages: []agent.Message{conPrompt}})
		if err != nil {
			return err
		}
		conResp = resp
		return finalize()
	}

	if pro.IsHuman() {
		return sess.AwaitInput(ctx, pro, proPrompt.Summary(), func(ctx context.Context, resp agent.Message) error {
			_ = ctx
			proResp = resp
			return runCon()
		})
	}
	ag, ok := pro.Agent()
	if !ok {
		return fmt.Errorf("pro participant must be an agent")
	}
	resp, err := sess.RunAgent(ctx, ag, RunRequest{Messages: []agent.Message{proPrompt}})
	if err != nil {
		return err
	}
	proResp = resp
	return runCon()
}

// GetState returns the protocol state for checkpointing.
func (p *adversarial) GetState() (map[string]any, error) {
	state := adversarialState{
		Pro:          p.pro,
		Con:          p.con,
		Synthesis:    p.synthesis,
		Participants: append([]string(nil), p.participants...),
	}
	return EncodeState(state)
}

// SetState restores protocol state from checkpoint data.
func (p *adversarial) SetState(state map[string]any) error {
	var snapshot adversarialState
	if err := DecodeState(state, &snapshot); err != nil {
		return err
	}
	p.pro = snapshot.Pro
	p.con = snapshot.Con
	p.synthesis = snapshot.Synthesis
	p.participants = append([]string(nil), snapshot.Participants...)
	return nil
}

type adversarialState struct {
	Pro          agent.Message `json:"pro"`
	Con          agent.Message `json:"con"`
	Synthesis    agent.Message `json:"synthesis"`
	Participants []string      `json:"participants"`
}

// OnEvent is a no-op for adversarial.
func (p *adversarial) OnEvent(ctx context.Context, sess Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}

// Shutdown is a no-op for adversarial.
func (p *adversarial) Shutdown(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}

func buildDebateSynthesisPrompt(topic, pro, con string) string {
	var sb strings.Builder
	sb.WriteString("Synthesize a balanced summary for: ")
	sb.WriteString(topic)
	sb.WriteString("\n\nPro argument:\n")
	sb.WriteString(pro)
	sb.WriteString("\n\nCon argument:\n")
	sb.WriteString(con)
	return sb.String()
}
