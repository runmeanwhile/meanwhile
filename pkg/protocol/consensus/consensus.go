package consensus

import (
	"context"
	"fmt"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/agenda"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/chair"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/minutes"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/pulse"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/roundtable"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

// consensus implements the consensus protocol.
type consensus struct {
	config     Config
	agenda     *agenda.Agenda
	chair      *chair.Chair
	roundtable *roundtable.Roundtable
	pulse      *pulse.PulseCheck
	pulseCount int
	lastResult map[string]any
}

// Consensus creates a new consensus protocol with the given options.
func Consensus(opts ...Option) protocol.Protocol {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return &consensus{config: cfg}
}

// ID returns the protocol identifier.
func (p *consensus) ID() string {
	return "protocol.consensus"
}

// Participants returns nil - consensus gets participants from session.
func (p *consensus) Participants() []protocol.Participant {
	return nil
}

// Init initializes the consensus protocol state.
func (p *consensus) Init(ctx context.Context, sess protocol.Session) error {
	_ = ctx

	participants := sess.Participants()
	if len(participants) == 0 {
		return fmt.Errorf("consensus requires at least one participant")
	}

	roundOpts := []roundtable.Option{roundtable.WithMaxRounds(p.config.MaxRounds)}
	roundOpts = append(roundOpts, p.config.RoundtableOptions...)
	p.roundtable = roundtable.New(roundOpts...)

	agendaOpts := make([]agenda.Option, 0, 2+len(p.config.AgendaOptions))
	if p.config.ScopeRefinement != nil {
		agendaOpts = append(agendaOpts, agenda.WithRefinementPrompt(p.config.ScopeRefinement))
	}
	if p.config.ScopeFallback != nil {
		agendaOpts = append(agendaOpts, agenda.WithFallback(p.config.ScopeFallback))
	}
	agendaOpts = append(agendaOpts, p.config.AgendaOptions...)
	p.agenda = agenda.New(agendaOpts...)
	p.chair = chair.New(p.config.ChairOptions...)

	pulseOpts := append([]pulse.Option{pulse.WithRoundProvider(p.roundtable.CurrentRound)}, p.config.PulseOptions...)
	agentParticipants, err := participantsToAgents(participants)
	if err != nil {
		return err
	}
	p.pulseCount = len(agentParticipants)
	p.pulse = pulse.New(agentParticipants, pulseOpts...)

	if err := p.pulse.Register(sess); err != nil {
		return fmt.Errorf("register pulse tool: %w", err)
	}

	return nil
}

// OnMessage orchestrates the round-robin consensus discussion.
func (p *consensus) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	participants := sess.Participants()
	p.lastResult = nil

	p.roundtable.Record(msg)

	refinedScope, err := p.agenda.RefineScope(ctx, sess, msg)
	if err != nil {
		return fmt.Errorf("refine user scope: %w", err)
	}

	scopeMsg := message.User(refinedScope)
	p.roundtable.Record(scopeMsg)

	if err := p.runDiscussion(ctx, sess, participants); err != nil {
		return err
	}

	result := p.buildResult()

	if facilitator := sess.Facilitator(); facilitator != nil {
		summary, err := p.runChairClosingSummary(ctx, sess, *facilitator, result)
		if err != nil {
			return fmt.Errorf("chair closing summary: %w", err)
		}
		if summary != "" {
			summaryMsg := message.Assistant(summary)
			summaryMsg.Name = facilitator.Name
			p.roundtable.Record(summaryMsg)
			result.Reasoning = summary
		}
	}

	mins := minutes.New()
	mins.Add("consensus", result)
	if result.Reasoning != "" {
		mins.SetSummary(result.Reasoning)
	}

	payload := mins.Payload()
	p.lastResult = payload

	if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload)); err != nil {
		return fmt.Errorf("emit consensus outcome: %w", err)
	}

	return nil
}

// OnEvent handles events during consensus.
func (p *consensus) OnEvent(ctx context.Context, sess protocol.Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}

// Shutdown cleans up protocol resources.
func (p *consensus) Shutdown(ctx context.Context, sess protocol.Session) error {
	_ = ctx
	_ = sess
	return nil
}

// Result returns the latest structured result payload.
func (p *consensus) Result() map[string]any {
	return cloneResult(p.lastResult)
}

// GetState returns the protocol state for checkpointing.
func (p *consensus) GetState() (map[string]any, error) {
	state := consensusState{
		LastResult: cloneResult(p.lastResult),
	}
	if p.roundtable != nil {
		state.Roundtable = p.roundtable.State()
	}
	if p.agenda != nil {
		state.Agenda = p.agenda.State()
	}
	if p.chair != nil {
		state.Chair = p.chair.State()
	}
	if p.pulse != nil {
		state.Pulse = p.pulse.Snapshot()
	}
	return protocol.EncodeState(state)
}

// SetState restores protocol state from checkpoint data.
func (p *consensus) SetState(state map[string]any) error {
	var snapshot consensusState
	if err := protocol.DecodeState(state, &snapshot); err != nil {
		return err
	}
	if p.roundtable != nil {
		p.roundtable.Restore(snapshot.Roundtable)
	}
	if p.agenda != nil {
		p.agenda.Restore(snapshot.Agenda)
	}
	if p.chair != nil {
		p.chair.Restore(snapshot.Chair)
	}
	if p.pulse != nil {
		p.pulse.Restore(snapshot.Pulse)
	}
	p.lastResult = cloneResult(snapshot.LastResult)
	return nil
}

type consensusState struct {
	Roundtable roundtable.State `json:"roundtable"`
	Agenda     agenda.State     `json:"agenda"`
	Chair      chair.State      `json:"chair"`
	Pulse      pulse.Snapshot   `json:"pulse"`
	LastResult map[string]any   `json:"last_result"`
}

func (p *consensus) runDiscussion(ctx context.Context, sess protocol.Session, participants []protocol.Participant) error {
	var runRound func() error

	runRound = func() error {
		if p.roundtable.CurrentRound() >= p.roundtable.MaxRounds() {
			return nil
		}
		if p.pulseCount > 0 && p.pulse.AllSignaled() {
			return nil
		}

		currentRound := p.roundtable.IncrementRound()

		if facilitator := sess.Facilitator(); facilitator != nil {
			if shouldSpeak, progress := p.chair.ShouldInterject(currentRound, p.roundtable.MaxRounds()); shouldSpeak {
				if err := p.runChairIntervention(ctx, sess, *facilitator, progress); err != nil {
					return fmt.Errorf("chair intervention: %w", err)
				}
			}
		}

		return p.runParticipantTurns(ctx, sess, participants, func() error {
			if err := p.emitRoundComplete(sess, currentRound); err != nil {
				return fmt.Errorf("emit round complete: %w", err)
			}
			return runRound()
		})
	}

	return runRound()
}

func (p *consensus) runParticipantTurns(ctx context.Context, sess protocol.Session, participants []protocol.Participant, onComplete func() error) error {
	var runFrom func(int) error

	runFrom = func(idx int) error {
		if idx >= len(participants) {
			if onComplete != nil {
				return onComplete()
			}
			return nil
		}

		participant := participants[idx]
		if participant.IsHuman() {
			contextMsg := p.humanContext()
			return sess.AwaitInput(ctx, participant, contextMsg, func(ctx context.Context, resp agent.Message) error {
				_ = ctx
				if resp.Name == "" {
					resp.Name = participant.DisplayName()
				}
				p.roundtable.Record(resp)
				if p.pulseCount > 0 && p.pulse.AllSignaled() {
					if onComplete != nil {
						return onComplete()
					}
					return nil
				}
				return runFrom(idx + 1)
			})
		}

		ag, ok := participant.Agent()
		if !ok {
			return fmt.Errorf("consensus participant must be an agent")
		}
		if err := p.runAgentTurn(ctx, sess, ag); err != nil {
			return err
		}
		if p.pulseCount > 0 && p.pulse.AllSignaled() {
			if onComplete != nil {
				return onComplete()
			}
			return nil
		}
		return runFrom(idx + 1)
	}

	return runFrom(0)
}

// runAgentTurn executes a single agent's turn in the round-robin.
func (p *consensus) runAgentTurn(ctx context.Context, sess protocol.Session, participant agent.Agent) error {
	thread := p.roundtable.Thread()
	messages := roundtable.FormatThread(thread)
	contextBuilder := p.config.ContextMessage
	if contextBuilder == nil {
		contextBuilder = defaultContextMessage
	}
	contextMsg := contextBuilder(thread, p.roundtable.CurrentRound(), p.roundtable.MaxRounds())
	messages = append(messages, contextMsg)

	var basePrompt string
	if participant.Profile != nil {
		basePrompt = participant.Profile.Prompt
	}

	promptBuilder := p.config.AgentPrompt
	if promptBuilder == nil {
		promptBuilder = buildAgentRefinementPrompt
	}
	refinedPrompt := promptBuilder(basePrompt, p.agenda.ResolvedScope(), p.roundtable.CurrentRound(), p.roundtable.MaxRounds())

	resp, err := sess.RunAgent(ctx, participant, protocol.RunRequest{
		Messages:          messages,
		SystemMessages:    []agent.Message{message.System(refinedPrompt)},
		MaxToolIterations: 10,
	})
	if err != nil {
		return fmt.Errorf("run agent %s: %w", participant.Name, err)
	}

	resp.Name = participant.Name
	p.roundtable.Record(resp)

	if p.config.BrevityReminder != nil && p.config.BrevityMaxChars > 0 {
		if len(resp.Text()) > p.config.BrevityMaxChars && p.roundtable.CurrentRound() >= p.config.BrevityMinRound {
			if facilitator := sess.Facilitator(); facilitator != nil {
				reminder := p.config.BrevityReminder(participant)
				if reminder != "" {
					brevityMsg := agent.Message{
						Role:  agent.RoleAssistant,
						Name:  facilitator.Name,
						Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: reminder}},
					}
					p.roundtable.Record(brevityMsg)
				}
			}
		}
	}

	return nil
}

func (p *consensus) runChairClosingSummary(ctx context.Context, sess protocol.Session, facilitator agent.Agent, result Result) (string, error) {
	builder := p.config.ClosingPrompt
	if builder == nil {
		return "", nil
	}
	input := ClosingSummaryInput{
		State:          result.State,
		RoundsUsed:     result.RoundsUsed,
		MaxRounds:      result.MaxRounds,
		Positions:      result.Positions,
		Conditions:     result.Conditions,
		BlockingIssues: result.BlockingIssues,
		RecentMessages: p.getRecentMessagesForChair(10),
	}
	prompt := builder(input)
	if prompt.User == "" {
		return "", nil
	}
	return p.chair.RunPrompt(ctx, sess, facilitator, prompt)
}

func (p *consensus) runChairIntervention(ctx context.Context, sess protocol.Session, facilitator agent.Agent, progress float64) error {
	builder := p.config.InterjectionPrompt
	if builder == nil {
		return nil
	}
	input := InterjectionInput{
		Progress:       progress,
		CurrentRound:   p.roundtable.CurrentRound(),
		MaxRounds:      p.roundtable.MaxRounds(),
		RecentMessages: p.getRecentMessagesForChair(5),
		Positions:      p.pulse.Positions(),
	}
	prompt := builder(input)
	if prompt.User == "" {
		return nil
	}
	interjection, err := p.chair.RunPrompt(ctx, sess, facilitator, prompt)
	if err != nil {
		return fmt.Errorf("run chair interjection: %w", err)
	}

	msg := message.Assistant(interjection)
	msg.Name = facilitator.Name
	p.roundtable.Record(msg)

	payload := map[string]any{"progress": progress, "interjection": interjection}
	if err := sess.Emit(event.New("consensus.moderator_intervention", sess.ID(), payload)); err != nil {
		return fmt.Errorf("emit moderator intervention: %w", err)
	}

	return nil
}

func (p *consensus) emitRoundComplete(sess protocol.Session, round int) error {
	payload := map[string]any{"round": round, "positions": p.pulse.Positions()}
	return sess.Emit(event.New("consensus.round_complete", sess.ID(), payload))
}

func (p *consensus) getRecentMessagesForChair(count int) []string {
	thread := p.roundtable.Thread()
	startIdx := len(thread) - count
	if startIdx < 0 {
		startIdx = 0
	}

	recent := make([]string, 0, count)
	for i := startIdx; i < len(thread); i++ {
		msg := thread[i]
		if msg.Role == agent.RoleAssistant && msg.Name != "" {
			recent = append(recent, fmt.Sprintf("[%s]: %s", msg.Name, truncateForContext(msg.Summary(), 200)))
		} else if msg.Role == agent.RoleUser {
			recent = append(recent, fmt.Sprintf("[USER]: %s", truncateForContext(msg.Summary(), 200)))
		}
	}

	return recent
}

// buildResult constructs the final consensus result.
func (p *consensus) buildResult() Result {
	state := p.pulse.State(p.roundtable.CurrentRound(), p.roundtable.MaxRounds())
	positions := p.pulse.Positions()
	conditions := p.pulse.Conditions()
	blockingIssues := p.pulse.BlockingIssues()
	messageThread := p.roundtable.Thread()

	reasoning := buildConsensusSummary(state, conditions, blockingIssues)

	return Result{
		State:          state,
		Reasoning:      reasoning,
		Positions:      positions,
		Conditions:     conditions,
		BlockingIssues: blockingIssues,
		RoundsUsed:     p.roundtable.CurrentRound(),
		MaxRounds:      p.roundtable.MaxRounds(),
		MessageThread:  messageThread,
	}
}

func cloneResult(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func participantsToAgents(participants []protocol.Participant) ([]agent.Agent, error) {
	if len(participants) == 0 {
		return nil, nil
	}
	agents := make([]agent.Agent, 0, len(participants))
	for _, participant := range participants {
		if participant == nil {
			return nil, fmt.Errorf("participant required")
		}
		if !participant.IsAgent() {
			continue
		}
		ag, ok := participant.Agent()
		if !ok {
			return nil, fmt.Errorf("agent participant required")
		}
		agents = append(agents, ag)
	}
	return agents, nil
}

func (p *consensus) humanContext() string {
	contextBuilder := p.config.ContextMessage
	if contextBuilder == nil {
		contextBuilder = defaultContextMessage
	}
	thread := p.roundtable.Thread()
	contextMsg := contextBuilder(thread, p.roundtable.CurrentRound(), p.roundtable.MaxRounds())
	return contextMsg.Summary()
}
