package brainstorm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/engine"
	"github.com/runmeanwhile/meanwhile/pkg/eval"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

// VariantSpec defines one protocol config variant.
type VariantSpec struct {
	Name         string
	Description  string
	Perspective  engine.AgentPerspectiveMode
	Temperature  float64
	ProtocolOpts []protocol.BrainstormingOption
}

// Config controls evaluation runs.
type Config struct {
	Models      []string
	RunsPerCase int
	Scenarios   []eval.Scenario
	Variants    []VariantSpec
	ShowTurns   bool
	RunTimeout  time.Duration
	Judge       eval.Judge
}

// Runner executes brainstorming eval matrices.
type Runner struct {
	provider *openai.Client
}

// NewRunner creates a brainstorming eval runner.
func NewRunner(provider *openai.Client) *Runner {
	return &Runner{provider: provider}
}

// Run executes all model/variant/scenario runs and returns an aggregated report.
func (r *Runner) Run(ctx context.Context, cfg Config) (eval.AggregateResult, error) {
	if r == nil || r.provider == nil {
		return eval.AggregateResult{}, fmt.Errorf("provider required")
	}
	if len(cfg.Models) == 0 {
		return eval.AggregateResult{}, fmt.Errorf("at least one model required")
	}
	if cfg.RunsPerCase <= 0 {
		return eval.AggregateResult{}, fmt.Errorf("runs_per_case must be > 0")
	}
	if len(cfg.Scenarios) == 0 {
		cfg.Scenarios = DefaultScenarios()
	}
	if len(cfg.Variants) == 0 {
		cfg.Variants = DefaultVariants(5)
	}

	started := time.Now()
	runs := make([]eval.RunRecord, 0, len(cfg.Models)*len(cfg.Variants)*len(cfg.Scenarios)*cfg.RunsPerCase)
	variantDesc := make(map[string]string, len(cfg.Variants))
	for _, v := range cfg.Variants {
		variantDesc[v.Name] = v.Description
	}

	for _, model := range cfg.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		for _, variant := range cfg.Variants {
			for _, scenario := range cfg.Scenarios {
				for runIndex := 1; runIndex <= cfg.RunsPerCase; runIndex++ {
					record := r.executeRun(ctx, model, variant, scenario, runIndex, cfg)
					runs = append(runs, record)
				}
			}
		}
	}

	report := eval.Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Protocol:    "brainstorming",
		Models:      append([]string(nil), cfg.Models...),
		RunsPerCase: cfg.RunsPerCase,
		Scenarios:   append([]eval.Scenario(nil), cfg.Scenarios...),
		Summaries:   summarizeRuns("brainstorming", runs, variantDesc),
		Runs:        runs,
	}

	return eval.AggregateResult{Report: report, Duration: time.Since(started)}, nil
}

func (r *Runner) executeRun(ctx context.Context, model string, variant VariantSpec, scenario eval.Scenario, runIndex int, cfg Config) eval.RunRecord {
	record := eval.RunRecord{
		Protocol:   "brainstorming",
		Model:      model,
		Variant:    variant.Name,
		ScenarioID: scenario.ID,
		Run:        runIndex,
	}

	started := time.Now()
	runCtx := ctx
	var cancel context.CancelFunc
	if cfg.RunTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, cfg.RunTimeout)
		defer cancel()
	}

	eng, err := engine.New(
		engine.WithProvider(r.provider),
		engine.WithDefaultModel(model),
		engine.WithAgentPerspectiveMode(variant.Perspective),
	)
	if err != nil {
		record.Error = fmt.Sprintf("new engine: %v", err)
		record.DurationMs = time.Since(started).Milliseconds()
		return record
	}

	moderator, marketing, engineeringAgent, design := buildAgents(eng)

	params := map[string]any{"temperature": variant.Temperature}
	opts := []protocol.BrainstormingOption{
		protocol.WithBrainstormingShortlistSize(3),
		protocol.WithBrainstormingVotesPerAgent(2),
		protocol.WithBrainstormingParams(params),
	}
	opts = append(opts, variant.ProtocolOpts...)

	sess, err := eng.Session(fmt.Sprintf("Eval %s", variant.Name)).
		Participants(marketing, engineeringAgent, design).
		Facilitator(moderator).
		Protocol(protocol.Brainstorming(opts...)).
		Start(runCtx)
	if err != nil {
		record.Error = fmt.Sprintf("start session: %v", err)
		record.DurationMs = time.Since(started).Milliseconds()
		return record
	}

	result, err := eng.Run(runCtx, sess.ID(), message.User(scenario.Prompt))
	if err != nil {
		record.Error = fmt.Sprintf("run: %v", err)
		record.DurationMs = time.Since(started).Milliseconds()
		return record
	}

	turns := extractTranscriptTurns(result.Events)
	record.Turns = turns
	record.Proxy = eval.ComputeProxyMetrics(turns)
	record.Shortlist = normalizeShortlist(result.Metadata["shortlist"])
	record.Final = strings.TrimSpace(result.Final)
	record.DurationMs = time.Since(started).Milliseconds()

	if cfg.Judge != nil {
		score, err := cfg.Judge.Score(runCtx, eval.JudgeInput{
			Protocol:  "brainstorming",
			Model:     model,
			Variant:   variant.Name,
			Scenario:  scenario,
			Turns:     turns,
			Shortlist: record.Shortlist,
			Final:     record.Final,
		})
		if err != nil {
			record.Error = fmt.Sprintf("judge: %v", err)
			return record
		}
		record.Judge = &score
	}

	if cfg.ShowTurns {
		fmt.Printf("\n[%s %s %s run %d]\n", model, variant.Name, scenario.ID, runIndex)
		for _, turn := range turns {
			fmt.Printf("- %s: %s\n", turn.Speaker, turn.Text)
		}
	}

	return record
}

func summarizeRuns(protocolID string, runs []eval.RunRecord, variantDesc map[string]string) []eval.Summary {
	type accum struct {
		runs      int
		successes int
		failures  int
		duration  int64
		proxy     []eval.ProxyMetrics
		dims      []eval.DimensionScores
		overall   []float64
		model     string
		variant   string
	}

	index := make(map[string]*accum)
	for _, run := range runs {
		key := protocolID + "|" + run.Model + "|" + run.Variant
		item, ok := index[key]
		if !ok {
			item = &accum{model: run.Model, variant: run.Variant}
			index[key] = item
		}
		item.runs++
		item.duration += run.DurationMs
		if run.Error != "" {
			item.failures++
			continue
		}
		item.successes++
		item.proxy = append(item.proxy, run.Proxy)
		if run.Judge != nil {
			item.dims = append(item.dims, run.Judge.Dimensions)
			item.overall = append(item.overall, run.Judge.Overall)
		}
	}

	summaries := make([]eval.Summary, 0, len(index))
	for _, item := range index {
		s := eval.Summary{
			Protocol:       protocolID,
			Model:          item.model,
			Variant:        item.variant,
			Description:    variantDesc[item.variant],
			Runs:           item.runs,
			Successes:      item.successes,
			Failures:       item.failures,
			AvgDurationMs:  0,
			Proxy:          eval.AggregateProxyMetrics(item.proxy),
			JudgeDimension: eval.AggregateDimensionScores(item.dims),
		}
		if item.runs > 0 {
			s.SuccessRate = float64(item.successes) / float64(item.runs)
			s.AvgDurationMs = item.duration / int64(item.runs)
		}
		if len(item.overall) > 0 {
			var total float64
			for _, v := range item.overall {
				total += v
			}
			s.JudgeOverall = total / float64(len(item.overall))
		}
		summaries = append(summaries, s)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Model == summaries[j].Model {
			return summaries[i].Variant < summaries[j].Variant
		}
		return summaries[i].Model < summaries[j].Model
	})
	return summaries
}

func extractTranscriptTurns(events []event.Event) []eval.TranscriptTurn {
	turns := make([]eval.TranscriptTurn, 0)
	for _, ev := range events {
		if ev.Type != event.AgentMessageComplete {
			continue
		}
		msg, ok := extractEventMessage(ev.Payload)
		if !ok {
			continue
		}
		text := strings.TrimSpace(agent.TextFromParts(msg.Parts))
		if text == "" {
			text = strings.TrimSpace(msg.Text())
		}
		if text == "" {
			text = strings.TrimSpace(msg.Summary())
		}
		if text == "" {
			continue
		}
		speaker := strings.TrimSpace(ev.AgentID)
		if speaker == "" {
			speaker = strings.TrimSpace(msg.Name)
		}
		if speaker == "" {
			continue
		}
		turns = append(turns, eval.TranscriptTurn{Speaker: speaker, Text: text})
	}
	return turns
}

func extractEventMessage(payload any) (agent.Message, bool) {
	switch value := payload.(type) {
	case agent.Message:
		return value, true
	case map[string]any:
		if raw, ok := value["message"]; ok {
			switch msg := raw.(type) {
			case agent.Message:
				return msg, true
			case map[string]any:
				return agent.MessageFromMap(msg), true
			}
		}
	}
	return agent.Message{}, false
}

func normalizeShortlist(raw any) []string {
	switch value := raw.(type) {
	case []string:
		out := make([]string, 0, len(value))
		for _, item := range value {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

// DefaultScenarios returns a built-in baseline dataset for brainstorming.
func DefaultScenarios() []eval.Scenario {
	return []eval.Scenario{
		{
			ID:          "signalthread-week3",
			Description: "SignalThread week-3 KPI engagement drop",
			Prompt:      `SignalThread serves ops managers at mid-market retail/logistics/supply chain teams who run multiple weekly KPI reviews. Onboarding works, but by week 3 meetings feel like reruns and by week 4-6 task follow-through collapses, so teams drift back to spreadsheets and Slack. We need one shippable-this-quarter feature that makes meetings feel fresh and makes follow-up tasks urgent, without changing their workflow or adding a new app.`,
		},
	}
}

// DefaultVariants returns built-in protocol variants.
func DefaultVariants(rounds int) []VariantSpec {
	return []VariantSpec{
		{
			Name:        "legacy-default",
			Description: "Legacy role handling + default brainstorming prompts",
			Perspective: engine.AgentPerspectiveLegacy,
			Temperature: 0.9,
			ProtocolOpts: []protocol.BrainstormingOption{
				protocol.WithBrainstormingInteractionRounds(rounds),
			},
		},
		{
			Name:        "speaker-aware-default",
			Description: "Speaker-aware role handling + default brainstorming prompts",
			Perspective: engine.AgentPerspectiveSpeakerAware,
			Temperature: 0.9,
			ProtocolOpts: []protocol.BrainstormingOption{
				protocol.WithBrainstormingInteractionRounds(rounds),
			},
		},
		{
			Name:        "speaker-aware-lean-prompts",
			Description: "Speaker-aware role handling + lean discussion/interjection prompts",
			Perspective: engine.AgentPerspectiveSpeakerAware,
			Temperature: 1.0,
			ProtocolOpts: []protocol.BrainstormingOption{
				protocol.WithBrainstormingInteractionRounds(rounds),
				protocol.WithBrainstormingInteractionPrompt(leanInteractionPrompt),
				protocol.WithBrainstormingModeratorInterjection(leanInterjectionPrompt),
			},
		},
		{
			Name:        "speaker-aware-lean-no-interjections",
			Description: "Speaker-aware role handling + lean prompts with interjections disabled",
			Perspective: engine.AgentPerspectiveSpeakerAware,
			Temperature: 1.0,
			ProtocolOpts: []protocol.BrainstormingOption{
				protocol.WithBrainstormingInteractionRounds(rounds),
				protocol.WithBrainstormingInteractionPrompt(leanInteractionPrompt),
				protocol.WithBrainstormingDisableInterjections(),
			},
		},
	}
}

func buildAgents(eng *engine.Engine) (agent.Agent, agent.Agent, agent.Agent, agent.Agent) {
	moderator := eng.Agent("Moderator").
		Prompt(`Role: Moderator (product lead facilitating the room)
Character traits: incisive, pragmatic, calm, lightly challenging, time-aware.
Voice & tone: direct, conversational, short sentences. No hype, no corporate fluff.
Behavior:
- Move the group forward with pointed questions.
- Push for specificity and tradeoffs.
- Call out generic answers and anchor back to the ops-manager workflow.

Good examples:
1) "Week 3 is where it drops. Which part of the meeting feels like a rerun?"
2) "Hold solutions for a second. Which KPI discussion actually changes what they do this week?"
3) "Pick one real task ops managers avoid. Why does it feel like homework?"

Bad examples:
1) "Love the energy here. Let's ideate some synergies."
2) "As we know, the objective of this session is to brainstorm a feature."
3) "Great discussion. Let's reflect on the scope and objectives."`).
		Build()

	marketing := eng.Agent("Marketing").
		Prompt(`Role: Marketing
Character traits: customer-empathetic, curious, pragmatic, lightly optimistic.
Voice & tone: warm, concise, grounded in customer behavior. Avoid buzzwords.
Focus: positioning, retention, emotional hooks, why teams keep showing up.

Good examples:
1) "If week 3 feels repetitive, what new signal can we surface so it feels worth showing up?"
2) "Retention drops when the meeting stops feeling useful. What is the smallest moment we can make feel essential?"
3) "Ops managers respond to momentum. How do we show week-over-week progress without extra work?"

Bad examples:
1) "We should leverage synergy to maximize engagement across verticals."
2) "This is super exciting. Let's create a magical experience."
3) "Our value prop is strong, therefore this feature will retain users."`).
		Build()

	engineeringAgent := eng.Agent("Engineering").
		Prompt(`Role: Engineering
Character traits: skeptical, practical, risk-aware, detail-oriented.
Voice & tone: blunt but fair, concise, no fluff.
Focus: feasibility, data integrity, failure modes, complexity cost.

Good examples:
1) "If we add alerts, how do we avoid false positives and notification fatigue?"
2) "This sounds good, but what is the simplest version we can ship in a quarter?"
3) "Can the data pipeline support near real-time signals, or do we stage it with scheduled rollups?"

Bad examples:
1) "Amazing idea. Let's do all of it."
2) "We can automate everything; it will be easy."
3) "Users will figure it out; edge cases do not matter."`).
		Build()

	design := eng.Agent("Design").
		Prompt(`Role: Design
Character traits: human-centered, thoughtful, systems-minded, pragmatic.
Voice & tone: clear, reflective, concrete. No fluff.
Focus: workflow fit, clarity, cognitive load, what feels natural in real meetings.

Good examples:
1) "Where in the meeting would this show up so it feels natural, not bolted on?"
2) "If we add urgency, how do we keep it from feeling like pressure or blame?"
3) "What is the one screen or moment ops managers would actually look forward to?"

Bad examples:
1) "Let's make it delightful and beautiful."
2) "The UX will solve the engagement problem."
3) "We should redesign the entire meeting experience."`).
		Build()

	return moderator, marketing, engineeringAgent, design
}

func leanInteractionPrompt(input protocol.InteractionPromptInput) string {
	board := strings.TrimSpace(input.Board)
	if board == "" {
		board = "- (no board yet)"
	}
	move := strings.TrimSpace(input.Move)
	if move == "" {
		move = "respond to one concrete point from another speaker"
	}
	return fmt.Sprintf(`Live product brainstorm.
Problem: %s
Idea board:
%s
Turn cue (%d/%d): %s

Respond to one concrete point from the recent thread. Build on it, challenge it, or ask one sharp question.
Keep it to 1-3 sentences. Prefer declarative statements; use a question only when it drives a specific next action.
Stay conversational. No headings or lists.`, input.Scope, board, input.TurnIndex, input.Speakers, move)
}

func leanInterjectionPrompt(input protocol.ModeratorInterjectionInput) protocol.ModeratorPrompt {
	recent := strings.Join(input.Recent, "\n")
	if strings.TrimSpace(recent) == "" {
		recent = "(no recent messages captured)"
	}
	system := `You are the brainstorm moderator. Keep the room moving with concise, concrete nudges.`
	user := fmt.Sprintf(`Scope: %s
Round: %d of %d
Idea board:
%s
Recent:
%s

Write a natural 2-3 sentence interjection that does two things:
1) Name one specific tension or opportunity from what was said.
2) Ask one focused question to drive the next turns.`, input.Scope, input.CurrentRound, input.MaxRounds, input.Board, recent)
	return protocol.ModeratorPrompt{System: system, User: user, MaxToolIterations: 1}
}
