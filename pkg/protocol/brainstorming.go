package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/minutes"
	"github.com/runmeanwhile/meanwhile/pkg/collab/roundtable"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
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

	brief := p.buildBrief(ctx, sess, msg, scope)
	seedMsg := PromptWithMedia(brief, msg)

	// Phase 1: Moderator intro - welcomes team, encourages questions/exploration
	moderatorOpening := scopeOpening
	if moderatorOpening == "" {
		moderatorOpening, err = p.runModeratorOpening(ctx, sess, scope, seedMsg)
		if err != nil {
			return err
		}
	}

	// Phase 2: Warm-up exploration (SEQUENTIAL) - messy questions, reactions, early thoughts
	explorationThread, err := p.runExploration(ctx, sess, agentParticipants, seedMsg, scope, moderatorOpening)
	if err != nil {
		return err
	}

	// Phase 3: Moderator sends them off to think in isolation
	isolationPrompt, err := p.runModeratorIsolationTransition(ctx, sess, scope, explorationThread)
	if err != nil {
		return err
	}

	// Phase 4: Parallel ideation - each thinks alone, comes back with 1-2 ideas they like
	ideationResults, err := p.runIdeation(ctx, sess, agentParticipants, seedMsg, scope, explorationThread, isolationPrompt)
	if err != nil {
		return err
	}

	// Phase 5: Moderator brings them back, sets up presentations
	presentationIntro, err := p.runModeratorPresentationIntro(ctx, sess, scope)
	if err != nil {
		return err
	}

	// Phase 6: Each presents their idea (sequential)
	presentationThread, err := p.runPresentations(ctx, sess, agentParticipants, scope, ideationResults, presentationIntro)
	if err != nil {
		return err
	}

	// Build idea board from presentations
	board, err := p.buildIdeaBoard(ctx, sess, scope, presentationThread, ideationResults)
	if err != nil {
		return err
	}

	// Phase 7: Discussion rounds (sequential) - debate, build, challenge
	discussionThread, err := p.runInteraction(ctx, sess, agentParticipants, seedMsg, scope, presentationIntro, board)
	if err != nil {
		return err
	}

	// Combine threads for shortlist building
	fullThread := append(presentationThread, discussionThread...)

	// Phase 8: Moderator nudges convergence to top 3
	shortlist, err := p.buildShortlist(ctx, sess, scope, board, fullThread)
	if err != nil {
		return err
	}

	// Phase 9: Vote
	votes, tally, err := p.runVoting(ctx, sess, agentParticipants, scope, shortlist)
	if err != nil {
		return err
	}

	// Phase 10: Moderator creates formal report (markdown OK)
	closing, err := p.runModeratorClosing(ctx, sess, scope, board, shortlist, tally)
	if err != nil {
		return err
	}

	mins := minutes.New()
	mins.Add("scope", scope)
	mins.Add("participants", participantNames(agentParticipants))
	mins.Add("exploration", map[string]any{
		"thread":  explorationThread,
		"opening": moderatorOpening,
	})
	mins.Add("ideation", map[string]any{
		"results":    ideationResults,
		"idea_board": board,
	})
	mins.Add("presentation", map[string]any{
		"thread": presentationThread,
	})
	mins.Add("discussion", map[string]any{
		"rounds": p.cfg.InteractionRounds,
		"thread": discussionThread,
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

// ideationResult holds one agent's ideas from the isolation phase.
type ideationResult struct {
	Agent string
	Ideas string
}

// runExploration runs the warm-up phase where agents explore the problem sequentially.
// This is messy - questions, reactions, early hunches. They see each other's responses.
func (p *brainstorming) runExploration(ctx context.Context, sess Session, participants []agent.Agent, seed agent.Message, scope, opening string) ([]agent.Message, error) {
	rt := roundtable.New(roundtable.WithMaxRounds(1))
	rt.Record(seed)

	if opening != "" && sess.Facilitator() != nil {
		openingMsg := message.Assistant(opening)
		openingMsg.Name = sess.Facilitator().Name
		rt.Record(openingMsg)
	}

	// One round of sequential exploration - each agent sees prior responses
	for _, participant := range participants {
		thread := recentThread(rt.Thread(), 6)
		messages := append([]agent.Message(nil), thread...)
		messages = append(messages, message.User("Your turn. React briefly to what's been said so far."))

		systemPrompt := fmt.Sprintf(`Warm-up round. React to the moderator framing and anything said so far.
Give one quick reaction: a question, a hunch, or a concern. Keep it to 1-2 sentences.
Don't propose full solutions yet. Stay in your own voice and keep it grounded in the brief.
Don't rephrase the brief—pick a specific angle or tension. Avoid speaker labels or headings.
Use a conversational tone (it's okay to use contractions).
Skip meta-praise like "great point" or "I appreciate".
If you see <agent:...> tags in the transcript, ignore them and never repeat them.

Focus: %s`, scope)

		resp, err := sess.RunAgent(ctx, participant, RunRequest{
			Messages:          messages,
			SystemMessages:    []agent.Message{message.System(systemPrompt)},
			Params:            p.runParamsFor(participant),
			MaxToolIterations: 1,
		})
		if err != nil {
			return nil, fmt.Errorf("exploration turn %s: %w", participant.Name, err)
		}
		resp.Name = participant.Name
		rt.Record(resp)
	}

	return rt.Thread(), nil
}

// runModeratorIsolationTransition has the moderator send agents off to think alone.
func (p *brainstorming) runModeratorIsolationTransition(ctx context.Context, sess Session, scope string, explorationThread []agent.Message) (string, error) {
	facilitator := sess.Facilitator()
	if facilitator == nil {
		return "", nil
	}

	threadSummary := ""
	for _, msg := range explorationThread {
		if msg.Name != "" {
			threadSummary += fmt.Sprintf("%s: %s\n", msg.Name, truncateForContext(msg.Summary(), 100))
		}
	}

	system := `You are the brainstorm moderator. You've just listened to the team's initial reactions and questions. Now you're sending them off to think independently before they come back to share ideas.`

	user := fmt.Sprintf(`Scope: %s

What the team just discussed:
%s

Call out 1-2 specific points (name the people, be concrete), then send everyone off to think individually for a few minutes.
Ask each person to come back with a handful of rough ideas and 1-2 favorites they want to champion.
Avoid cheerleading or recaps. Keep it conversational - 2-3 sentences. No bullet points.`, scope, threadSummary)

	prompt := ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
	resp, err := p.runModeratorPrompt(ctx, sess, *facilitator, prompt)
	if err != nil {
		return "", fmt.Errorf("moderator isolation transition: %w", err)
	}
	return strings.TrimSpace(resp), nil
}

// runIdeation runs parallel ideation where each agent thinks alone.
func (p *brainstorming) runIdeation(ctx context.Context, sess Session, participants []agent.Agent, seed agent.Message, scope string, explorationThread []agent.Message, isolationPrompt string) ([]ideationResult, error) {
	// Build context from exploration
	var contextSummary strings.Builder
	for _, m := range explorationThread {
		if m.Name != "" {
			fmt.Fprintf(&contextSummary, "%s: %s\n", m.Name, truncateForContext(m.Summary(), 150))
		}
	}

	turns := make([]roundtable.Turn, len(participants))
	for i, participant := range participants {
		ideaTarget := p.cfg.IdeaTarget
		if ideaTarget <= 0 {
			ideaTarget = defaultIdeaTarget
		}

		systemPrompt := fmt.Sprintf(`INDEPENDENT IDEATION (private scratchpad)
Problem: %s

Work alone. You will not see anyone else's answers.
Jot %d rough ideas as short, messy fragments (use "-" bullets, no numbering).
Finish with a line naming 1-2 ideas you'd champion and why.

Keep it plain text. Do not include your name or any speaker labels.
Avoid repeating ideas already hinted at in the warm-up—push for different angles.
If you see <agent:...> tags in the transcript, ignore them and never repeat them.`, scope, ideaTarget)

		messages := []agent.Message{
			PromptWithMedia(seed.Text(), seed),
			message.User(fmt.Sprintf("Here's what came up in our initial exploration:\n%s\n\n%s", contextSummary.String(), isolationPrompt)),
		}

		turns[i] = roundtable.Turn{
			Agent:             participant,
			Messages:          messages,
			SystemMessages:    []agent.Message{message.System(systemPrompt)},
			Params:            p.runParamsFor(participant),
			MaxToolIterations: 1,
		}
	}

	results, err := roundtable.RunParallel(ctx, roundtableRunner(sess), turns, roundtable.ParallelConfig{MaxConcurrent: p.cfg.MaxConcurrent})
	if err != nil {
		return nil, fmt.Errorf("ideation: %w", err)
	}

	ideationResults := make([]ideationResult, len(results))
	for i, result := range results {
		ideationResults[i] = ideationResult{
			Agent: participants[i].Name,
			Ideas: result.Message.Text(),
		}
	}

	return ideationResults, nil
}

// runModeratorPresentationIntro has the moderator bring everyone back.
func (p *brainstorming) runModeratorPresentationIntro(ctx context.Context, sess Session, scope string) (string, error) {
	facilitator := sess.Facilitator()
	if facilitator == nil {
		return "", nil
	}

	system := `You are the brainstorm moderator. The team has been thinking independently and now you're bringing them back together to share their ideas.`

	user := fmt.Sprintf(`Scope: %s

Bring the team back together. Ask each person to share their strongest idea and why it matters.
Keep it brief and conversational. Avoid "welcome back" or formalities. Call on someone to start.`, scope)

	prompt := ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
	resp, err := p.runModeratorPrompt(ctx, sess, *facilitator, prompt)
	if err != nil {
		return "", fmt.Errorf("moderator presentation intro: %w", err)
	}
	return strings.TrimSpace(resp), nil
}

// runPresentations has each agent present their idea sequentially.
func (p *brainstorming) runPresentations(ctx context.Context, sess Session, participants []agent.Agent, scope string, ideationResults []ideationResult, presentationIntro string) ([]agent.Message, error) {
	rt := roundtable.New(roundtable.WithMaxRounds(1))

	if presentationIntro != "" && sess.Facilitator() != nil {
		introMsg := message.Assistant(presentationIntro)
		introMsg.Name = sess.Facilitator().Name
		rt.Record(introMsg)
	}

	for i, participant := range participants {
		thread := recentThread(rt.Thread(), 6)
		messages := append([]agent.Message(nil), thread...)
		messages = append(messages, message.User("Your turn. Share the one idea you most want the team to ship next quarter."))
		if len(messages) == 0 {
			seed := strings.TrimSpace(scope)
			if seed == "" {
				seed = "Brainstorming session"
			}
			messages = []agent.Message{message.User(fmt.Sprintf("Brainstorming focus: %s", seed))}
		}

		// Include their own ideation result as private notes
		notes := ""
		if i < len(ideationResults) {
			notes = ideationResults[i].Ideas
		}
		notes = strings.TrimSpace(notes)
		if notes == "" {
			notes = "(No private notes available.)"
		}

		systemPrompt := fmt.Sprintf(`PRESENTATION PHASE
Problem: %s

Your private notes:
%s

Share your strongest idea with the group.
If someone already pitched something similar, build on it instead of repeating it.
Anchor it in one concrete workflow moment and one likely tradeoff or risk.
Say it like you'd pitch to colleagues, not a press release. Keep it to 2-4 sentences. Avoid lists or speaker labels.
Use a conversational tone (contractions are fine).
Stay in your own voice. If you see <agent:...> tags in the transcript, ignore them and never repeat them.`, scope, notes)

		resp, err := sess.RunAgent(ctx, participant, RunRequest{
			Messages:          messages,
			SystemMessages:    []agent.Message{message.System(systemPrompt)},
			Params:            p.runParamsFor(participant),
			MaxToolIterations: 1,
		})
		if err != nil {
			return nil, fmt.Errorf("presentation %s: %w", participant.Name, err)
		}
		resp.Name = participant.Name
		rt.Record(resp)
	}

	return rt.Thread(), nil
}

// buildIdeaBoard creates a summary of ideas from ideation results.
func (p *brainstorming) buildIdeaBoard(ctx context.Context, sess Session, scope string, presentationThread []agent.Message, ideationResults []ideationResult) (string, error) {
	if facilitator := sess.Facilitator(); facilitator != nil {
		board, err := p.runModeratorIdeaBoard(ctx, sess, *facilitator, scope, presentationThread)
		if err == nil && strings.TrimSpace(board) != "" {
			return strings.TrimSpace(board), nil
		}
	}

	if board := ideaBoardFromPresentations(presentationThread, sess.Facilitator()); strings.TrimSpace(board) != "" {
		return board, nil
	}

	return ideaBoardFromIdeation(ideationResults), nil
}

func (p *brainstorming) runModeratorIdeaBoard(ctx context.Context, sess Session, facilitator agent.Agent, scope string, presentationThread []agent.Message) (string, error) {
	var sb strings.Builder
	for _, msg := range presentationThread {
		if msg.Name == "" {
			continue
		}
		if msg.Name == facilitator.Name {
			continue
		}
		text := strings.TrimSpace(agent.TextFromParts(msg.Parts))
		if text == "" {
			text = strings.TrimSpace(msg.Summary())
		}
		if text == "" {
			continue
		}
		fmt.Fprintf(&sb, "%s: %s\n", msg.Name, truncateForContext(text, 220))
	}
	seed := strings.TrimSpace(sb.String())
	if seed == "" {
		return "", nil
	}

	system := `You are the brainstorm moderator. Turn the team's idea pitches into a concise idea board.`
	user := fmt.Sprintf(`Scope: %s

Idea pitches:
%s

Write a concise idea board as a bullet list. Each bullet should be a short title plus a short clause.
No speaker names, no extra commentary.`, scope, seed)

	req := RunRequest{
		Messages:          []agent.Message{message.User(user)},
		MaxToolIterations: 1,
		Silent:            true,
	}
	if strings.TrimSpace(system) != "" {
		req.SystemMessages = []agent.Message{message.System(system)}
	}
	resp, err := sess.RunAgent(ctx, facilitator, req)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Text()), nil
}

func ideaBoardFromPresentations(thread []agent.Message, facilitator *agent.Agent) string {
	var sb strings.Builder
	for _, msg := range thread {
		if msg.Role != agent.RoleAssistant || msg.Name == "" {
			continue
		}
		if facilitator != nil && msg.Name == facilitator.Name {
			continue
		}
		text := strings.TrimSpace(agent.TextFromParts(msg.Parts))
		if text == "" {
			text = strings.TrimSpace(msg.Summary())
		}
		if text == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("- ")
		sb.WriteString(text)
	}
	return strings.TrimSpace(sb.String())
}

func ideaBoardFromIdeation(results []ideationResult) string {
	var sb strings.Builder
	for _, r := range results {
		trimmed := strings.TrimSpace(r.Ideas)
		if trimmed == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		if r.Agent != "" {
			sb.WriteString(r.Agent)
			sb.WriteString(":\n")
		}
		sb.WriteString(trimmed)
	}
	return strings.TrimSpace(sb.String())
}

func (p *brainstorming) runHumanBrainstorm(ctx context.Context, sess Session, participants []Participant, msg agent.Message) error {
	scope, scopeOpening, err := p.resolveScope(ctx, sess, msg)
	if err != nil {
		return err
	}

	moderatorOpening := scopeOpening
	if moderatorOpening == "" {
		moderatorOpening, err = p.runModeratorOpening(ctx, sess, scope, msg)
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

		ordered := rotateParticipantOrder(participants, currentRound)
		return p.runHumanInteractionTurns(ctx, sess, ordered, scope, board, rt, func() error {
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
		if err := p.runInteractionTurn(ctx, sess, ag, scope, board, rt, idx+1, len(participants)); err != nil {
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
			Silent:            true,
		}
		if strings.TrimSpace(systemPrompt) != "" {
			req.SystemMessages = []agent.Message{message.System(systemPrompt)}
		}
		resp, err := sess.RunAgent(ctx, *facilitator, req)
		if err != nil {
			return "", "", fmt.Errorf("refine scope: %w", err)
		}
		if scopeText := strings.TrimSpace(resp.Text()); scopeText != "" {
			return scopeText, "", nil
		}
	}

	scope := p.scopeFallback(userQuestion)
	if strings.TrimSpace(scope) == "" {
		scope = userQuestion
	}
	return scope, "", nil
}

func (p *brainstorming) buildBrief(ctx context.Context, sess Session, msg agent.Message, scope string) string {
	seed := strings.TrimSpace(msg.Text())
	if seed == "" {
		seed = strings.TrimSpace(msg.Summary())
	}
	if seed == "" {
		return strings.TrimSpace(scope)
	}

	seed = truncateForContext(seed, 2000)

	if facilitator := sess.Facilitator(); facilitator != nil {
		system := `You are the brainstorm moderator. Summarize the user's brief for internal discussion.`
		user := fmt.Sprintf(`User brief:
%s

Write a 2-3 sentence summary in plain, conversational language. Highlight the core problem, constraints, and target user. No lists.`, seed)

		req := RunRequest{
			Messages:          []agent.Message{message.User(user)},
			MaxToolIterations: 1,
			Silent:            true,
		}
		req.SystemMessages = []agent.Message{message.System(system)}

		resp, err := sess.RunAgent(ctx, *facilitator, req)
		if err == nil {
			brief := strings.TrimSpace(resp.Text())
			if brief != "" {
				return brief
			}
		}
	}

	if strings.TrimSpace(scope) != "" {
		return strings.TrimSpace(scope)
	}
	return seed
}

func (p *brainstorming) runModeratorOpening(ctx context.Context, sess Session, scope string, msg agent.Message) (string, error) {
	facilitator := sess.Facilitator()
	if facilitator == nil {
		return "", nil
	}
	if p.cfg.ModeratorOpening == nil {
		return "", nil
	}
	seed := msg.Text()
	if strings.TrimSpace(seed) == "" {
		seed = msg.Summary()
	}
	seed = truncateForContext(strings.TrimSpace(seed), 300)
	prompt := p.cfg.ModeratorOpening(ModeratorOpeningInput{Scope: scope, Agenda: p.agendaContext(scope), Seed: seed})
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

		ordered := rotateAgentOrder(participants, currentRound)
		for turnIndex, participant := range ordered {
			if err := p.runInteractionTurn(ctx, sess, participant, scope, board, rt, turnIndex+1, len(ordered)); err != nil {
				return nil, err
			}
		}
	}

	return rt.Thread(), nil
}

func (p *brainstorming) runInteractionTurn(ctx context.Context, sess Session, participant agent.Agent, scope, board string, rt *roundtable.Roundtable, turnIndex, speakers int) error {
	thread := recentThread(rt.Thread(), 6)
	messages := append([]agent.Message(nil), thread...)
	messages = append(messages, message.User("Your turn. React to the thread above and keep it concise."))

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
		TurnIndex:  turnIndex,
		Speakers:   speakers,
		Move:       interactionTurnMove(rt.CurrentRound(), turnIndex),
	})

	resp, err := sess.RunAgent(ctx, participant, RunRequest{
		Messages:          messages,
		SystemMessages:    []agent.Message{message.System(systemPrompt)},
		Params:            p.runParamsFor(participant),
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

func ideationResultsToMessages(results []ideationResult) []agent.Message {
	if len(results) == 0 {
		return nil
	}
	out := make([]agent.Message, 0, len(results))
	for _, res := range results {
		msg := message.Assistant(res.Ideas)
		msg.Name = strings.TrimSpace(res.Agent)
		out = append(out, msg)
	}
	return out
}

func ideasFromResult(result map[string]any) []agent.Message {
	if len(result) == 0 {
		return nil
	}
	if ideas := ideasFromDivergent(result); len(ideas) > 0 {
		return ideas
	}
	return ideasFromIdeation(result)
}

func ideasFromDivergent(result map[string]any) []agent.Message {
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

func ideasFromIdeation(result map[string]any) []agent.Message {
	ideation, ok := result["ideation"].(map[string]any)
	if !ok {
		return nil
	}
	raw := ideation["results"]
	switch results := raw.(type) {
	case []ideationResult:
		return ideationResultsToMessages(results)
	case []any:
		out := make([]ideationResult, 0, len(results))
		for _, item := range results {
			switch v := item.(type) {
			case ideationResult:
				out = append(out, v)
			case map[string]any:
				agentName, _ := v["Agent"].(string)
				ideas, _ := v["Ideas"].(string)
				if strings.TrimSpace(agentName) == "" && strings.TrimSpace(ideas) == "" {
					continue
				}
				out = append(out, ideationResult{Agent: agentName, Ideas: ideas})
			}
		}
		return ideationResultsToMessages(out)
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

func (p *brainstorming) runParamsFor(ag agent.Agent) map[string]any {
	if len(p.cfg.Params) == 0 {
		return nil
	}
	if len(ag.Params) == 0 {
		return cloneParams(p.cfg.Params)
	}
	out := make(map[string]any, len(p.cfg.Params))
	for key, value := range p.cfg.Params {
		if _, ok := ag.Params[key]; ok {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
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

func looksLikeListItem(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "•") {
		return true
	}
	digitSeen := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch >= '0' && ch <= '9' {
			digitSeen = true
			continue
		}
		if digitSeen && (ch == '.' || ch == ')') {
			return true
		}
		break
	}
	return false
}

func cleanListItem(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "•") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-•*"))
		return stripLeadingOrdinalPrefix(line)
	}
	return stripLeadingOrdinalPrefix(line)
}

func stripLeadingOrdinalPrefix(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	// Strip true list ordinals like "1. " or "2) " while preserving numeric
	// titles such as "3-minute delta reel" or years like "2024. roadmap".
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i > 3 || i >= len(line) {
		return line
	}
	if line[i] != '.' && line[i] != ')' {
		return line
	}
	if i+1 >= len(line) || (line[i+1] != ' ' && line[i+1] != '\t') {
		return line
	}
	return strings.TrimSpace(line[i+1:])
}

func recentThread(thread []agent.Message, max int) []agent.Message {
	if max <= 0 || len(thread) <= max {
		return thread
	}
	return thread[len(thread)-max:]
}

func extractShortlist(text string, limit int) []string {
	lines := strings.Split(text, "\n")
	ideas := make([]string, 0, limit)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !looksLikeListItem(line) {
			continue
		}
		line = cleanListItem(line)
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

func rotateAgentOrder(participants []agent.Agent, round int) []agent.Agent {
	if len(participants) <= 1 {
		return participants
	}
	offset := (round - 1) % len(participants)
	if offset < 0 {
		offset = 0
	}
	out := make([]agent.Agent, 0, len(participants))
	out = append(out, participants[offset:]...)
	out = append(out, participants[:offset]...)
	return out
}

func rotateParticipantOrder(participants []Participant, round int) []Participant {
	if len(participants) <= 1 {
		return participants
	}
	offset := (round - 1) % len(participants)
	if offset < 0 {
		offset = 0
	}
	out := make([]Participant, 0, len(participants))
	out = append(out, participants[offset:]...)
	out = append(out, participants[:offset]...)
	return out
}

func interactionTurnMove(round, turnIndex int) string {
	moves := []string{
		"build on one specific point from another speaker",
		"pressure-test one assumption",
		"add one concrete workflow example",
		"name one tradeoff or delivery risk",
	}
	if len(moves) == 0 {
		return ""
	}
	idx := (round + turnIndex - 2) % len(moves)
	if idx < 0 {
		idx = 0
	}
	return moves[idx]
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
