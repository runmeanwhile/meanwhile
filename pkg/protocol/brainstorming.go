package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/minutes"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/roundtable"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
)

var errBrainstormNoParticipants = errors.New("brainstorming requires at least one participant")

// brainstorming orchestrates a multi-phase brainstorming session.
type brainstorming struct {
	cfg        brainstormingConfig
	moderator  *brainstormModerator
	lastResult map[string]any
}

// Brainstorming creates a brainstorming protocol with divergent ideation,
// interactive discussion, and optional voting.
func Brainstorming(opts ...BrainstormingOption) Protocol {
	cfg := defaultBrainstormingConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &brainstorming{cfg: cfg}
}

// ID returns the protocol ID.
func (p *brainstorming) ID() string { return "protocol.brainstorming" }

// Participants returns empty slice - brainstorming gets participants from session.
func (p *brainstorming) Participants() []Participant { return nil }

// Config exposes the current brainstorming configuration.
func (p *brainstorming) Config() Config {
	return p.cfg.asConfig()
}

// Init prepares agenda and chair components.
func (p *brainstorming) Init(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}

// OnMessage runs a full brainstorm cycle.
func (p *brainstorming) OnMessage(ctx context.Context, sess Session, msg agent.Message) error {
	participants := sess.Participants()
	if len(participants) == 0 {
		return errBrainstormNoParticipants
	}

	p.lastResult = nil
	p.moderator = newBrainstormModerator(p.cfg.InterventionPoints)

	if hasHumanParticipant(participants) {
		return p.runHumanBrainstorm(ctx, sess, participants, msg)
	}

	agentParticipants, err := participantsToAgents(participants)
	if err != nil {
		return err
	}

	scope, scopeOpening, err := p.resolveScope(ctx, sess, msg)
	if err != nil {
		return err
	}

	moderatorOpening := scopeOpening
	if moderatorOpening == "" {
		moderatorOpening, err = p.runModeratorOpening(ctx, sess, scope)
		if err != nil {
			return err
		}
	}

	divergent, err := p.runDivergent(ctx, sess, agentParticipants, msg, scope, moderatorOpening)
	if err != nil {
		return err
	}

	board, err := p.runModeratorSynthesis(ctx, sess, scope, divergent.Brief)
	if err != nil {
		return err
	}
	if board == "" {
		board = divergent.Brief
	}

	interactionThread, err := p.runInteraction(ctx, sess, agentParticipants, msg, scope, moderatorOpening, board)
	if err != nil {
		return err
	}

	shortlist, err := p.buildShortlist(ctx, sess, scope, board, interactionThread)
	if err != nil {
		return err
	}

	votes, tally, err := p.runVoting(ctx, sess, agentParticipants, scope, shortlist)
	if err != nil {
		return err
	}

	closing, err := p.runModeratorClosing(ctx, sess, scope, board, shortlist, tally)
	if err != nil {
		return err
	}

	mins := minutes.New()
	mins.Add("scope", scope)
	mins.Add("participants", participantNames(agentParticipants))
	mins.Add("divergent", map[string]any{
		"rounds":     p.cfg.DivergentRounds,
		"ideas":      divergent.Ideas,
		"brief":      divergent.Brief,
		"opening":    moderatorOpening,
		"idea_board": board,
	})
	mins.Add("interaction", map[string]any{
		"rounds":  p.cfg.InteractionRounds,
		"thread":  interactionThread,
		"enabled": p.cfg.InteractionRounds > 0,
	})
	if len(shortlist) > 0 {
		mins.Add("shortlist", shortlist)
	}
	if p.cfg.VoteEnabled && len(shortlist) > 0 {
		mins.Add("votes", map[string]any{
			"ballots":         votes,
			"tally":           tally,
			"picks_per_agent": p.cfg.VotesPerAgent,
		})
	}
	if closing != "" {
		mins.SetSummary(closing)
	}

	payload := mins.Payload()
	p.lastResult = payload

	if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload)); err != nil {
		return fmt.Errorf("emit brainstorm: %w", err)
	}

	return nil
}

func (p *brainstorming) runHumanBrainstorm(ctx context.Context, sess Session, participants []Participant, msg agent.Message) error {
	scope, scopeOpening, err := p.resolveScope(ctx, sess, msg)
	if err != nil {
		return err
	}

	moderatorOpening := scopeOpening
	if moderatorOpening == "" {
		moderatorOpening, err = p.runModeratorOpening(ctx, sess, scope)
		if err != nil {
			return err
		}
	}

	return p.runHumanDivergent(ctx, sess, participants, msg, scope, moderatorOpening, func(divergent divergentResult) error {
		board, err := p.runModeratorSynthesis(ctx, sess, scope, divergent.Brief)
		if err != nil {
			return err
		}
		if board == "" {
			board = divergent.Brief
		}

		return p.runHumanInteraction(ctx, sess, participants, msg, scope, moderatorOpening, board, func(thread []agent.Message) error {
			shortlist, err := p.buildShortlist(ctx, sess, scope, board, thread)
			if err != nil {
				return err
			}

			agentParticipants, err := participantsToAgents(participants)
			if err != nil {
				return err
			}

			votes, tally, err := p.runVoting(ctx, sess, agentParticipants, scope, shortlist)
			if err != nil {
				return err
			}

			closing, err := p.runModeratorClosing(ctx, sess, scope, board, shortlist, tally)
			if err != nil {
				return err
			}

			mins := minutes.New()
			mins.Add("scope", scope)
			mins.Add("participants", participantDisplayNames(participants))
			mins.Add("divergent", map[string]any{
				"rounds":     p.cfg.DivergentRounds,
				"ideas":      divergent.Ideas,
				"brief":      divergent.Brief,
				"opening":    moderatorOpening,
				"idea_board": board,
			})
			mins.Add("interaction", map[string]any{
				"rounds":  p.cfg.InteractionRounds,
				"thread":  thread,
				"enabled": p.cfg.InteractionRounds > 0,
			})
			if len(shortlist) > 0 {
				mins.Add("shortlist", shortlist)
			}
			if p.cfg.VoteEnabled && len(shortlist) > 0 && len(votes) > 0 {
				mins.Add("votes", map[string]any{
					"ballots":         votes,
					"tally":           tally,
					"picks_per_agent": p.cfg.VotesPerAgent,
				})
			}
			if closing != "" {
				mins.SetSummary(closing)
			}

			payload := mins.Payload()
			p.lastResult = payload

			if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload)); err != nil {
				return fmt.Errorf("emit brainstorm: %w", err)
			}

			return nil
		})
	})
}

func (p *brainstorming) runHumanDivergent(
	ctx context.Context,
	sess Session,
	participants []Participant,
	msg agent.Message,
	scope string,
	opening string,
	onComplete func(divergentResult) error,
) error {
	if p.cfg.DivergentRounds <= 0 {
		if onComplete == nil {
			return nil
		}
		return onComplete(divergentResult{})
	}

	seedText := msg.Text()
	if strings.TrimSpace(seedText) == "" {
		seedText = msg.Summary()
	}
	seed := []agent.Message{PromptWithMedia(seedText, msg)}
	if strings.TrimSpace(scope) != "" {
		seed = append(seed, message.User(fmt.Sprintf("Brainstorming focus: %s", truncateForContext(scope, 400))))
	}
	if strings.TrimSpace(opening) != "" {
		openingMsg := message.Assistant(opening)
		if facilitator := sess.Facilitator(); facilitator != nil {
			openingMsg.Name = facilitator.Name
		}
		seed = append(seed, openingMsg)
	}

	type divergentThread struct {
		Participant Participant
		Messages    []agent.Message
	}

	threads := make([]divergentThread, len(participants))
	for i, participant := range participants {
		if participant == nil {
			return fmt.Errorf("participant required")
		}
		seedCopy := append([]agent.Message(nil), seed...)
		threads[i] = divergentThread{Participant: participant, Messages: seedCopy}
	}
	seedCount := len(seed)

	var runRound func(int) error
	runRound = func(round int) error {
		if round > p.cfg.DivergentRounds {
			ideas := make([]agent.Message, 0, len(threads))
			for _, thread := range threads {
				for i := seedCount; i < len(thread.Messages); i++ {
					msg := thread.Messages[i]
					if msg.Name == "" {
						msg.Name = thread.Participant.DisplayName()
					}
					ideas = append(ideas, msg)
				}
			}
			brief := ideaBrief(ideas)
			if onComplete == nil {
				return nil
			}
			return onComplete(divergentResult{Ideas: ideas, Brief: brief})
		}

		var runFrom func(int) error
		runFrom = func(idx int) error {
			if idx >= len(threads) {
				return runRound(round + 1)
			}

			entry := &threads[idx]
			participant := entry.Participant
			if participant.IsHuman() {
				turnContext := p.humanDivergentContext(seedText, scope, opening, round, p.cfg.DivergentRounds)
				return sess.AwaitInput(ctx, participant, turnContext, func(ctx context.Context, resp agent.Message) error {
					_ = ctx
					if resp.Name == "" {
						resp.Name = participant.DisplayName()
					}
					entry.Messages = append(entry.Messages, resp)
					return runFrom(idx + 1)
				})
			}

			ag, ok := participant.Agent()
			if !ok {
				return fmt.Errorf("brainstorming participant must be an agent")
			}

			basePrompt := ""
			if ag.Profile != nil {
				basePrompt = ag.Profile.Prompt
			}
			promptBuilder := p.cfg.DivergentPrompt
			if promptBuilder == nil {
				promptBuilder = defaultDivergentPrompt
			}
			systemPrompt := promptBuilder(DivergentPromptInput{
				BasePrompt: basePrompt,
				Scope:      scope,
				Round:      round,
				MaxRounds:  p.cfg.DivergentRounds,
				IdeaTarget: p.cfg.IdeaTarget,
			})

			resp, err := sess.RunAgent(ctx, ag, RunRequest{
				Messages:          entry.Messages,
				SystemMessages:    []agent.Message{message.System(systemPrompt)},
				MaxToolIterations: 1,
			})
			if err != nil {
				return fmt.Errorf("run agent %s: %w", ag.Name, err)
			}
			if resp.Name == "" {
				resp.Name = ag.Name
			}
			entry.Messages = append(entry.Messages, resp)
			return runFrom(idx + 1)
		}

		return runFrom(0)
	}

	return runRound(1)
}

func (p *brainstorming) runHumanInteraction(
	ctx context.Context,
	sess Session,
	participants []Participant,
	msg agent.Message,
	scope string,
	opening string,
	board string,
	onComplete func([]agent.Message) error,
) error {
	if p.cfg.InteractionRounds <= 0 {
		if onComplete == nil {
			return nil
		}
		return onComplete(nil)
	}

	rt := roundtable.New(roundtable.WithMaxRounds(p.cfg.InteractionRounds))
	rt.Record(msg)

	if opening != "" && sess.Facilitator() != nil {
		openingMsg := message.Assistant(opening)
		openingMsg.Name = sess.Facilitator().Name
		rt.Record(openingMsg)
	}

	var runRound func() error
	runRound = func() error {
		if rt.CurrentRound() >= rt.MaxRounds() {
			if onComplete == nil {
				return nil
			}
			return onComplete(rt.Thread())
		}

		currentRound := rt.IncrementRound()
		if facilitator := sess.Facilitator(); facilitator != nil && p.moderator != nil {
			if shouldSpeak, progress := p.moderator.ShouldInterject(currentRound, rt.MaxRounds()); shouldSpeak {
				if err := p.runModeratorInterjection(ctx, sess, *facilitator, scope, board, progress, rt); err != nil {
					return err
				}
			}
		}

		return p.runHumanInteractionTurns(ctx, sess, participants, scope, board, rt, func() error {
			return runRound()
		})
	}

	return runRound()
}

func (p *brainstorming) runHumanInteractionTurns(
	ctx context.Context,
	sess Session,
	participants []Participant,
	scope string,
	board string,
	rt *roundtable.Roundtable,
	onComplete func() error,
) error {
	var runFrom func(int) error

	runFrom = func(idx int) error {
		if idx >= len(participants) {
			if onComplete != nil {
				return onComplete()
			}
			return nil
		}

		participant := participants[idx]
		if participant == nil {
			return fmt.Errorf("participant required")
		}
		if participant.IsHuman() {
			contextBuilder := p.cfg.ContextMessage
			if contextBuilder == nil {
				contextBuilder = defaultBrainstormingContextMessage
			}
			contextMsg := contextBuilder(rt.Thread(), rt.CurrentRound(), rt.MaxRounds())
			return sess.AwaitInput(ctx, participant, contextMsg.Summary(), func(ctx context.Context, resp agent.Message) error {
				_ = ctx
				if resp.Name == "" {
					resp.Name = participant.DisplayName()
				}
				rt.Record(resp)
				return runFrom(idx + 1)
			})
		}

		ag, ok := participant.Agent()
		if !ok {
			return fmt.Errorf("brainstorming participant must be an agent")
		}
		if err := p.runInteractionTurn(ctx, sess, ag, scope, board, rt); err != nil {
			return err
		}
		return runFrom(idx + 1)
	}

	return runFrom(0)
}

// OnEvent is a no-op for brainstorming.
func (p *brainstorming) OnEvent(ctx context.Context, sess Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}

// Shutdown is a no-op for brainstorming.
func (p *brainstorming) Shutdown(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}

// Result returns the latest structured result payload.
func (p *brainstorming) Result() map[string]any {
	return cloneResult(p.lastResult)
}

// GetState returns protocol state for checkpointing.
func (p *brainstorming) GetState() (map[string]any, error) {
	state := brainstormingState{
		LastResult: cloneResult(p.lastResult),
		Ideas:      ideasFromResult(p.lastResult),
	}
	return EncodeState(state)
}

// SetState restores protocol state from checkpoint data.
func (p *brainstorming) SetState(state map[string]any) error {
	var snapshot brainstormingState
	if err := DecodeState(state, &snapshot); err != nil {
		return err
	}
	p.lastResult = cloneResult(snapshot.LastResult)
	if p.lastResult == nil && len(snapshot.Ideas) > 0 {
		p.lastResult = map[string]any{
			"divergent": map[string]any{
				"ideas": append([]agent.Message(nil), snapshot.Ideas...),
			},
		}
	}
	return nil
}

type brainstormingState struct {
	LastResult map[string]any  `json:"last_result"`
	Ideas      []agent.Message `json:"ideas"`
}

type divergentResult struct {
	Ideas []agent.Message
	Brief string
}

type divergentThread struct {
	Agent    agent.Agent
	Messages []agent.Message
}

func divergentIdeas(threads []divergentThread, seedCount int) []agent.Message {
	if len(threads) == 0 {
		return nil
	}
	ideas := make([]agent.Message, 0, len(threads))
	for _, thread := range threads {
		for i := seedCount; i < len(thread.Messages); i++ {
			msg := thread.Messages[i]
			if msg.Name == "" {
				msg.Name = thread.Agent.Name
			}
			ideas = append(ideas, msg)
		}
	}
	return ideas
}

func divergentBrief(threads []divergentThread, seedCount int) string {
	if len(threads) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, thread := range threads {
		for i := seedCount; i < len(thread.Messages); i++ {
			msg := thread.Messages[i]
			summary := strings.TrimSpace(msg.Summary())
			if summary == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("[%s] %s\n", thread.Agent.Name, summary))
		}
	}
	return strings.TrimSpace(sb.String())
}

func (p *brainstorming) resolveScope(ctx context.Context, sess Session, msg agent.Message) (string, string, error) {
	userQuestion := msg.Text()
	if strings.TrimSpace(userQuestion) == "" {
		userQuestion = msg.Summary()
	}

	if facilitator := sess.Facilitator(); facilitator != nil && p.cfg.ScopeRefinement != nil {
		systemPrompt, userPrompt := p.cfg.ScopeRefinement(userQuestion, p.cfg.Scope)
		if strings.TrimSpace(userPrompt) == "" {
			userPrompt = userQuestion
		}
		scopePrompt := PromptWithMedia(userPrompt, msg)
		req := RunRequest{
			Messages:          []agent.Message{scopePrompt},
			MaxToolIterations: 1,
		}
		if strings.TrimSpace(systemPrompt) != "" {
			req.SystemMessages = []agent.Message{message.System(systemPrompt)}
		}
		resp, err := sess.RunAgent(ctx, *facilitator, req)
		if err != nil {
			return "", "", fmt.Errorf("refine scope: %w", err)
		}
		if scope := strings.TrimSpace(resp.Text()); scope != "" {
			return scope, scope, nil
		}
	}

	scope := p.scopeFallback(userQuestion)
	if strings.TrimSpace(scope) == "" {
		scope = userQuestion
	}
	return scope, "", nil
}

func (p *brainstorming) runModeratorOpening(ctx context.Context, sess Session, scope string) (string, error) {
	facilitator := sess.Facilitator()
	if facilitator == nil {
		return "", nil
	}
	if p.cfg.ModeratorOpening == nil {
		return "", nil
	}
	prompt := p.cfg.ModeratorOpening(ModeratorOpeningInput{Scope: scope, Agenda: p.agendaContext(scope)})
	if strings.TrimSpace(prompt.User) == "" {
		return "", nil
	}
	opening, err := p.runModeratorPrompt(ctx, sess, *facilitator, prompt)
	if err != nil {
		return "", fmt.Errorf("moderator opening: %w", err)
	}
	return strings.TrimSpace(opening), nil
}

func (p *brainstorming) runDivergent(ctx context.Context, sess Session, participants []agent.Agent, msg agent.Message, scope, opening string) (divergentResult, error) {
	if p.cfg.DivergentRounds <= 0 {
		return divergentResult{}, nil
	}

	seedText := msg.Text()
	if strings.TrimSpace(seedText) == "" {
		seedText = msg.Summary()
	}
	seed := []agent.Message{PromptWithMedia(seedText, msg)}
	if strings.TrimSpace(scope) != "" {
		seed = append(seed, message.User(fmt.Sprintf("Brainstorming focus: %s", truncateForContext(scope, 400))))
	}
	if strings.TrimSpace(opening) != "" {
		openingMsg := message.Assistant(opening)
		if facilitator := sess.Facilitator(); facilitator != nil {
			openingMsg.Name = facilitator.Name
		}
		seed = append(seed, openingMsg)
	}

	threads := make([]divergentThread, len(participants))
	for i, participant := range participants {
		seedCopy := append([]agent.Message(nil), seed...)
		threads[i] = divergentThread{Agent: participant, Messages: seedCopy}
	}
	seedCount := len(seed)

	for round := 1; round <= p.cfg.DivergentRounds; round++ {
		snapshots := make([][]agent.Message, len(threads))
		for i, thread := range threads {
			snapshots[i] = append([]agent.Message(nil), thread.Messages...)
		}

		turns := make([]roundtable.Turn, len(threads))
		for i, thread := range threads {
			basePrompt := ""
			if thread.Agent.Profile != nil {
				basePrompt = thread.Agent.Profile.Prompt
			}
			promptBuilder := p.cfg.DivergentPrompt
			if promptBuilder == nil {
				promptBuilder = defaultDivergentPrompt
			}
			systemPrompt := promptBuilder(DivergentPromptInput{
				BasePrompt: basePrompt,
				Scope:      scope,
				Round:      round,
				MaxRounds:  p.cfg.DivergentRounds,
				IdeaTarget: p.cfg.IdeaTarget,
			})

			turns[i] = roundtable.Turn{
				Agent:             thread.Agent,
				Messages:          snapshots[i],
				SystemMessages:    []agent.Message{message.System(systemPrompt)},
				MaxToolIterations: 1,
			}
		}

		results, err := roundtable.RunParallel(ctx, roundtableRunner(sess), turns, roundtable.ParallelConfig{MaxConcurrent: p.cfg.MaxConcurrent})
		if err != nil {
			return divergentResult{}, err
		}

		for i, result := range results {
			msg := result.Message
			if msg.Name == "" {
				msg.Name = threads[i].Agent.Name
			}
			threads[i].Messages = append(threads[i].Messages, msg)
		}
	}

	ideas := divergentIdeas(threads, seedCount)
	brief := strings.TrimSpace(divergentBrief(threads, seedCount))

	return divergentResult{Ideas: ideas, Brief: brief}, nil
}

func (p *brainstorming) runModeratorSynthesis(ctx context.Context, sess Session, scope, brief string) (string, error) {
	facilitator := sess.Facilitator()
	if facilitator == nil {
		return "", nil
	}
	if p.cfg.ModeratorSynthesis == nil {
		return "", nil
	}
	prompt := p.cfg.ModeratorSynthesis(ModeratorSynthesisInput{Scope: scope, DivergentBrief: brief})
	if strings.TrimSpace(prompt.User) == "" {
		return "", nil
	}
	resp, err := p.runModeratorPrompt(ctx, sess, *facilitator, prompt)
	if err != nil {
		return "", fmt.Errorf("moderator synthesis: %w", err)
	}
	return strings.TrimSpace(resp), nil
}

func (p *brainstorming) runInteraction(ctx context.Context, sess Session, participants []agent.Agent, msg agent.Message, scope, opening, board string) ([]agent.Message, error) {
	if p.cfg.InteractionRounds <= 0 {
		return nil, nil
	}

	rt := roundtable.New(roundtable.WithMaxRounds(p.cfg.InteractionRounds))
	rt.Record(msg)

	if opening != "" && sess.Facilitator() != nil {
		openingMsg := message.Assistant(opening)
		openingMsg.Name = sess.Facilitator().Name
		rt.Record(openingMsg)
	}

	for rt.CurrentRound() < rt.MaxRounds() {
		currentRound := rt.IncrementRound()

		if facilitator := sess.Facilitator(); facilitator != nil {
			if p.moderator != nil {
				if shouldSpeak, progress := p.moderator.ShouldInterject(currentRound, rt.MaxRounds()); shouldSpeak {
					if err := p.runModeratorInterjection(ctx, sess, *facilitator, scope, board, progress, rt); err != nil {
						return nil, err
					}
				}
			}
		}

		for _, participant := range participants {
			if err := p.runInteractionTurn(ctx, sess, participant, scope, board, rt); err != nil {
				return nil, err
			}
		}
	}

	return rt.Thread(), nil
}

func (p *brainstorming) runInteractionTurn(ctx context.Context, sess Session, participant agent.Agent, scope, board string, rt *roundtable.Roundtable) error {
	thread := rt.Thread()
	messages := roundtable.FormatThread(thread)

	contextBuilder := p.cfg.ContextMessage
	if contextBuilder == nil {
		contextBuilder = defaultBrainstormingContextMessage
	}
	contextMsg := contextBuilder(thread, rt.CurrentRound(), rt.MaxRounds())
	messages = append(messages, contextMsg)

	basePrompt := ""
	if participant.Profile != nil {
		basePrompt = participant.Profile.Prompt
	}

	promptBuilder := p.cfg.InteractionPrompt
	if promptBuilder == nil {
		promptBuilder = defaultInteractionPrompt
	}
	systemPrompt := promptBuilder(InteractionPromptInput{
		BasePrompt: basePrompt,
		Scope:      scope,
		Board:      board,
		Round:      rt.CurrentRound(),
		MaxRounds:  rt.MaxRounds(),
	})

	resp, err := sess.RunAgent(ctx, participant, RunRequest{
		Messages:          messages,
		SystemMessages:    []agent.Message{message.System(systemPrompt)},
		MaxToolIterations: 1,
	})
	if err != nil {
		return fmt.Errorf("run agent %s: %w", participant.Name, err)
	}
	resp.Name = participant.Name
	rt.Record(resp)
	return nil
}

func (p *brainstorming) runModeratorInterjection(ctx context.Context, sess Session, facilitator agent.Agent, scope, board string, progress float64, rt *roundtable.Roundtable) error {
	if p.cfg.ModeratorInterjection == nil {
		return nil
	}
	input := ModeratorInterjectionInput{
		Scope:        scope,
		Board:        board,
		Progress:     progress,
		CurrentRound: rt.CurrentRound(),
		MaxRounds:    rt.MaxRounds(),
		Recent:       recentMessages(rt.Thread(), 5),
	}
	prompt := p.cfg.ModeratorInterjection(input)
	if strings.TrimSpace(prompt.User) == "" {
		return nil
	}
	interjection, err := p.runModeratorPrompt(ctx, sess, facilitator, prompt)
	if err != nil {
		return fmt.Errorf("moderator interjection: %w", err)
	}

	msg := message.Assistant(interjection)
	msg.Name = facilitator.Name
	rt.Record(msg)

	return nil
}

func (p *brainstorming) buildShortlist(ctx context.Context, sess Session, scope, board string, thread []agent.Message) ([]string, error) {
	if p.cfg.ShortlistSize <= 0 {
		return nil, nil
	}

	facilitator := sess.Facilitator()
	if facilitator != nil && p.cfg.ModeratorShortlist != nil {
		prompt := p.cfg.ModeratorShortlist(ModeratorShortlistInput{
			Scope:  scope,
			Board:  board,
			Thread: recentMessages(thread, 6),
			Limit:  p.cfg.ShortlistSize,
		})
		if strings.TrimSpace(prompt.User) != "" {
			resp, err := p.runModeratorPrompt(ctx, sess, *facilitator, prompt)
			if err != nil {
				return nil, fmt.Errorf("moderator shortlist: %w", err)
			}
			shortlist := extractShortlist(resp, p.cfg.ShortlistSize)
			if len(shortlist) > 0 {
				return shortlist, nil
			}
		}
	}

	return fallbackShortlist(board, p.cfg.ShortlistSize), nil
}

func (p *brainstorming) runVoting(ctx context.Context, sess Session, participants []agent.Agent, scope string, shortlist []string) ([]VoteBallot, []VoteTally, error) {
	if !p.cfg.VoteEnabled || len(shortlist) == 0 || p.cfg.VotesPerAgent <= 0 {
		return nil, nil, nil
	}

	picks := p.cfg.VotesPerAgent
	if picks > len(shortlist) {
		picks = len(shortlist)
	}

	ballots := make([]VoteBallot, len(participants))
	var wg sync.WaitGroup
	errCh := make(chan error, len(participants))
	var sem chan struct{}
	if p.cfg.MaxConcurrent > 0 {
		sem = make(chan struct{}, p.cfg.MaxConcurrent)
	}

	for i, participant := range participants {
		i := i
		participant := participant
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			basePrompt := ""
			if participant.Profile != nil {
				basePrompt = participant.Profile.Prompt
			}
			promptBuilder := p.cfg.VotePrompt
			if promptBuilder == nil {
				promptBuilder = defaultVotePrompt
			}

			prompt := promptBuilder(VotePromptInput{
				BasePrompt: basePrompt,
				Scope:      scope,
				Shortlist:  shortlist,
				Picks:      picks,
			})

			resp, err := sess.RunAgent(ctx, participant, RunRequest{
				Messages:          []agent.Message{message.User(prompt)},
				MaxToolIterations: 1,
				OutputSchema:      voteResponse{},
			})
			if err != nil {
				errCh <- fmt.Errorf("vote from %s: %w", participant.Name, err)
				return
			}

			raw := strings.TrimSpace(agent.TextFromParts(resp.Parts))
			if raw == "" {
				raw = strings.TrimSpace(resp.Text())
			}
			ballot := parseVoteBallot(raw, shortlist)
			ballot.Agent = participant.Name
			ballot.Raw = raw
			ballot.Picks = trimAndLimit(ballot.Picks, picks)
			ballots[i] = ballot
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, nil, err
		}
	}

	tally := tallyVotes(ballots, shortlist, p.cfg.VoteWeights)
	return ballots, tally, nil
}

func (p *brainstorming) runModeratorClosing(ctx context.Context, sess Session, scope, board string, shortlist []string, tally []VoteTally) (string, error) {
	facilitator := sess.Facilitator()
	if facilitator == nil {
		return "", nil
	}
	if p.cfg.ModeratorClosing == nil {
		return "", nil
	}
	prompt := p.cfg.ModeratorClosing(ModeratorClosingInput{
		Scope:     scope,
		Board:     board,
		Shortlist: shortlist,
		Tally:     tally,
	})
	if strings.TrimSpace(prompt.User) == "" {
		return "", nil
	}
	resp, err := p.runModeratorPrompt(ctx, sess, *facilitator, prompt)
	if err != nil {
		return "", fmt.Errorf("moderator closing: %w", err)
	}
	return strings.TrimSpace(resp), nil
}

// VoteBallot captures a participant's vote.
type VoteBallot struct {
	Agent     string   `json:"agent"`
	Picks     []string `json:"picks"`
	Rationale string   `json:"rationale,omitempty"`
	Raw       string   `json:"raw,omitempty"`
}

type voteResponse struct {
	Picks     []string `json:"picks"`
	Rationale string   `json:"rationale,omitempty"`
}

// VoteTally captures the aggregated vote scores.
type VoteTally struct {
	Idea  string `json:"idea"`
	Score int    `json:"score"`
	Votes int    `json:"votes"`
}

func parseVoteBallot(text string, shortlist []string) VoteBallot {
	trimmed := strings.TrimSpace(stripCodeFence(text))
	resp := voteResponse{}

	if trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
			candidate := extractJSONObject(trimmed)
			if candidate != "" {
				_ = json.Unmarshal([]byte(candidate), &resp)
			}
		}
	}

	if len(resp.Picks) == 0 {
		resp.Picks = extractPicksFromText(text, shortlist)
	}

	resp.Picks = normalizePicks(resp.Picks, shortlist)
	return VoteBallot{Picks: resp.Picks, Rationale: strings.TrimSpace(resp.Rationale)}
}

func tallyVotes(ballots []VoteBallot, shortlist []string, weights []int) []VoteTally {
	if len(ballots) == 0 || len(shortlist) == 0 {
		return nil
	}
	if len(weights) == 0 {
		weights = defaultVoteWeights
	}

	lookup := make(map[string]string, len(shortlist))
	for _, idea := range shortlist {
		lookup[strings.ToLower(strings.TrimSpace(idea))] = idea
	}

	counts := make(map[string]int, len(shortlist))
	votes := make(map[string]int, len(shortlist))
	for _, ballot := range ballots {
		for i, pick := range ballot.Picks {
			if pick == "" {
				continue
			}
			canonical := pick
			if mapped, ok := lookup[strings.ToLower(strings.TrimSpace(pick))]; ok {
				canonical = mapped
			}
			weight := 1
			if i < len(weights) {
				weight = weights[i]
			}
			counts[canonical] += weight
			votes[canonical]++
		}
	}

	tally := make([]VoteTally, 0, len(counts))
	for idea, score := range counts {
		tally = append(tally, VoteTally{Idea: idea, Score: score, Votes: votes[idea]})
	}

	sort.Slice(tally, func(i, j int) bool {
		if tally[i].Score == tally[j].Score {
			return tally[i].Idea < tally[j].Idea
		}
		return tally[i].Score > tally[j].Score
	})

	return tally
}

func participantNames(participants []agent.Agent) []string {
	names := make([]string, 0, len(participants))
	for _, participant := range participants {
		if participant.Name == "" {
			continue
		}
		names = append(names, participant.Name)
	}
	return names
}

func participantDisplayNames(participants []Participant) []string {
	names := make([]string, 0, len(participants))
	for _, participant := range participants {
		if participant == nil {
			continue
		}
		name := strings.TrimSpace(participant.DisplayName())
		if name == "" && participant.IsAgent() {
			if ag, ok := participant.Agent(); ok {
				name = strings.TrimSpace(ag.Name)
			}
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func hasHumanParticipant(participants []Participant) bool {
	for _, participant := range participants {
		if participant != nil && participant.IsHuman() {
			return true
		}
	}
	return false
}

func participantsToAgents(participants []Participant) ([]agent.Agent, error) {
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

func ideaBrief(ideas []agent.Message) string {
	if len(ideas) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, msg := range ideas {
		summary := strings.TrimSpace(msg.Summary())
		if summary == "" {
			continue
		}
		name := strings.TrimSpace(msg.Name)
		if name == "" {
			name = "Participant"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", name, summary))
	}
	return strings.TrimSpace(sb.String())
}

func ideasFromResult(result map[string]any) []agent.Message {
	if len(result) == 0 {
		return nil
	}
	divergent, ok := result["divergent"].(map[string]any)
	if !ok {
		return nil
	}
	raw := divergent["ideas"]
	switch ideas := raw.(type) {
	case []agent.Message:
		return append([]agent.Message(nil), ideas...)
	case []any:
		out := make([]agent.Message, 0, len(ideas))
		for _, item := range ideas {
			switch msg := item.(type) {
			case agent.Message:
				out = append(out, msg)
			case map[string]any:
				out = append(out, agent.MessageFromMap(msg))
			}
		}
		return out
	default:
		return nil
	}
}

func (p *brainstorming) humanDivergentContext(seed, scope, opening string, round, maxRounds int) string {
	ideaTarget := p.cfg.IdeaTarget
	if ideaTarget <= 0 {
		ideaTarget = defaultIdeaTarget
	}
	phase := fmt.Sprintf("Divergent ideation (round %d of %d)", round, maxRounds)
	if maxRounds <= 1 {
		phase = "Divergent ideation"
	}

	var sb strings.Builder
	sb.WriteString("BRAINSTORMING: ")
	sb.WriteString(phase)
	sb.WriteString("\n\n")

	if strings.TrimSpace(seed) != "" {
		sb.WriteString("Question:\n")
		sb.WriteString(truncateForContext(strings.TrimSpace(seed), 400))
		sb.WriteString("\n\n")
	}
	if strings.TrimSpace(scope) != "" {
		sb.WriteString("Scope:\n")
		sb.WriteString(truncateForContext(strings.TrimSpace(scope), 400))
		sb.WriteString("\n\n")
	}
	if strings.TrimSpace(opening) != "" {
		sb.WriteString("Moderator:\n")
		sb.WriteString(truncateForContext(strings.TrimSpace(opening), 400))
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("Please list %d distinct ideas.\n", ideaTarget))
	sb.WriteString("One idea per line. Keep them short and specific.\n")
	sb.WriteString("Avoid repeating ideas you already shared.")

	return sb.String()
}

func (p *brainstorming) scopeFallback(userQuestion string) string {
	if p.cfg.ScopeFallback != nil {
		return p.cfg.ScopeFallback(userQuestion, p.cfg.Scope)
	}
	if strings.TrimSpace(p.cfg.Scope) != "" {
		return p.cfg.Scope
	}
	return userQuestion
}

func (p *brainstorming) agendaContext(scope string) string {
	var sb strings.Builder
	if strings.TrimSpace(scope) != "" {
		sb.WriteString("Scope: ")
		sb.WriteString(scope)
		sb.WriteString("\n")
	}
	if strings.TrimSpace(p.cfg.Outcome) != "" {
		sb.WriteString("Outcome: ")
		sb.WriteString(p.cfg.Outcome)
		sb.WriteString("\n")
	}
	if len(p.cfg.Briefs) > 0 {
		sb.WriteString("Brief:\n")
		sb.WriteString(strings.Join(p.cfg.Briefs, "\n"))
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func (p *brainstorming) runModeratorPrompt(ctx context.Context, sess Session, facilitator agent.Agent, prompt ModeratorPrompt) (string, error) {
	req := RunRequest{
		Messages:          []agent.Message{message.User(prompt.User)},
		MaxToolIterations: prompt.MaxToolIterations,
	}
	if req.MaxToolIterations <= 0 {
		req.MaxToolIterations = 1
	}
	if strings.TrimSpace(prompt.System) != "" {
		req.SystemMessages = []agent.Message{message.System(prompt.System)}
	}
	resp, err := sess.RunAgent(ctx, facilitator, req)
	if err != nil {
		return "", fmt.Errorf("run moderator prompt: %w", err)
	}
	return resp.Text(), nil
}

type brainstormModerator struct {
	interventionPts   []float64
	interventionsDone map[float64]bool
}

func newBrainstormModerator(points []float64) *brainstormModerator {
	return &brainstormModerator{interventionPts: append([]float64(nil), points...), interventionsDone: make(map[float64]bool)}
}

func (m *brainstormModerator) ShouldInterject(currentRound, maxRounds int) (bool, float64) {
	if maxRounds == 0 {
		return false, 0
	}
	progress := float64(currentRound) / float64(maxRounds)
	for _, point := range m.interventionPts {
		if progress >= point && !m.interventionsDone[point] {
			m.interventionsDone[point] = true
			return true, point
		}
	}
	return false, 0
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

func recentMessages(thread []agent.Message, count int) []string {
	if count <= 0 || len(thread) == 0 {
		return nil
	}
	start := len(thread) - count
	if start < 0 {
		start = 0
	}
	out := make([]string, 0, len(thread)-start)
	for i := start; i < len(thread); i++ {
		msg := thread[i]
		label := msg.Name
		if msg.Role == agent.RoleUser {
			label = "USER"
		}
		if label == "" {
			label = "UNKNOWN"
		}
		out = append(out, fmt.Sprintf("[%s] %s", label, truncateForContext(msg.Summary(), 200)))
	}
	return out
}

func truncateForContext(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractShortlist(text string, limit int) []string {
	lines := strings.Split(text, "\n")
	ideas := make([]string, 0, limit)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimLeft(line, "-•*0123456789.) ")
		if line == "" {
			continue
		}
		ideas = append(ideas, line)
		if limit > 0 && len(ideas) >= limit {
			break
		}
	}
	return ideas
}

func fallbackShortlist(board string, limit int) []string {
	if strings.TrimSpace(board) == "" {
		return nil
	}
	return extractShortlist(board, limit)
}

func stripCodeFence(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
	}
	return trimmed
}

func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return text[start : end+1]
}

func extractPicksFromText(text string, shortlist []string) []string {
	if len(shortlist) == 0 {
		return nil
	}
	lowerShortlist := make([]string, len(shortlist))
	for i, idea := range shortlist {
		lowerShortlist[i] = strings.ToLower(idea)
	}
	lines := strings.Split(text, "\n")
	picks := make([]string, 0, len(shortlist))
	for _, line := range lines {
		clean := strings.ToLower(strings.TrimSpace(line))
		if clean == "" {
			continue
		}
		clean = strings.TrimLeft(clean, "-•*0123456789.) ")
		for i, idea := range lowerShortlist {
			if idea == "" {
				continue
			}
			if strings.Contains(clean, idea) {
				picks = append(picks, shortlist[i])
				break
			}
		}
	}
	return picks
}

func normalizePicks(picks, shortlist []string) []string {
	if len(picks) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(picks))
	seen := make(map[string]struct{})
	for _, pick := range picks {
		pick = strings.TrimSpace(pick)
		if pick == "" {
			continue
		}
		canonical := matchShortlist(pick, shortlist)
		if canonical == "" {
			continue
		}
		key := strings.ToLower(canonical)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, canonical)
	}
	return normalized
}

func matchShortlist(pick string, shortlist []string) string {
	pickLower := strings.ToLower(strings.TrimSpace(pick))
	for _, idea := range shortlist {
		if strings.ToLower(strings.TrimSpace(idea)) == pickLower {
			return idea
		}
	}
	for _, idea := range shortlist {
		ideaLower := strings.ToLower(idea)
		if strings.Contains(pickLower, ideaLower) || strings.Contains(ideaLower, pickLower) {
			return idea
		}
	}
	return ""
}

func trimAndLimit(items []string, limit int) []string {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, limit)
	seen := make(map[string]struct{})
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}
