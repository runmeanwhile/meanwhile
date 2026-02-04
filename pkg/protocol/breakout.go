package protocol

import (
	"context"
	"fmt"
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/minutes"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/roundtable"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

type breakout struct {
	cfg breakoutConfig
}

type breakoutConfig struct {
	GroupSize int
	Groups    map[string][]agent.Agent
}

// BreakoutOption configures breakout behavior.
type BreakoutOption func(*breakoutConfig)

// Breakout creates a breakout-reconvene protocol where participants split
// into sub-groups, work in parallel, then reconvene for synthesis.
func Breakout(opts ...BreakoutOption) Protocol {
	cfg := breakoutConfig{GroupSize: 2}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &breakout{cfg: cfg}
}

// WithBreakoutGroupSize sets the group size for automatic splits.
func WithBreakoutGroupSize(size int) BreakoutOption {
	return func(cfg *breakoutConfig) {
		if size > 0 {
			cfg.GroupSize = size
		}
	}
}

// WithBreakoutGroups sets explicit breakout groups.
func WithBreakoutGroups(groups map[string][]agent.Agent) BreakoutOption {
	return func(cfg *breakoutConfig) {
		if len(groups) > 0 {
			cfg.Groups = groups
		}
	}
}

// ID returns the protocol ID.
func (p *breakout) ID() string { return "protocol.breakout_reconvene" }

// Participants returns empty slice - breakout gets participants from session groups.
func (p *breakout) Participants() []Participant { return nil }

// Init is a no-op for breakout.
func (p *breakout) Init(ctx context.Context, _ Session) error {
	_ = ctx
	return nil
}

// OnMessage splits participants into groups, gathers findings, and reconvenes.
func (p *breakout) OnMessage(ctx context.Context, sess Session, msg agent.Message) error {
	participants := sess.Participants()
	groups, err := resolveBreakoutGroups(p.cfg, sess, participants)
	if err != nil {
		return err
	}

	turns := make([]roundtable.Turn, 0, len(groups))
	for _, group := range groups {
		leader := group.Members[0]
		task := msg.Text()
		if strings.TrimSpace(task) == "" {
			task = msg.Summary()
		}
		prompt := PromptWithMedia(buildGroupPrompt(task, group), msg)
		turns = append(turns, roundtable.Turn{Agent: leader, Messages: []agent.Message{prompt}})
	}

	turnResults, err := roundtable.RunParallel(ctx, roundtableRunner(sess), turns, roundtable.ParallelConfig{})
	if err != nil {
		return err
	}

	summaries := make([]breakoutSummary, len(groups))
	for i, result := range turnResults {
		summaries[i] = breakoutSummary{Name: groups[i].Name, Message: result.Message}
	}

	facilitator := sess.Facilitator()
	if facilitator != nil {
		task := msg.Text()
		if strings.TrimSpace(task) == "" {
			task = msg.Summary()
		}
		synthPrompt := buildReconvenePrompt(task, summaries)
		resp, err := sess.RunAgent(ctx, *facilitator, RunRequest{Messages: []agent.Message{PromptWithMedia(synthPrompt, msg)}})
		if err != nil {
			return err
		}
		summaries = append(summaries, breakoutSummary{Name: "synthesis", Message: resp})
	}

	mins := minutes.New()
	mins.Add("breakouts", summaries)
	if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), mins.Payload())); err != nil {
		return fmt.Errorf("emit breakouts: %w", err)
	}

	return nil
}

// OnEvent is a no-op for breakout.
func (p *breakout) OnEvent(ctx context.Context, sess Session, ev event.Event) error {
	_ = ctx
	_ = sess
	_ = ev
	return nil
}

// Shutdown is a no-op for breakout.
func (p *breakout) Shutdown(ctx context.Context, sess Session) error {
	_ = ctx
	_ = sess
	return nil
}

type breakoutGroup struct {
	Name    string
	Members []agent.Agent
}

type breakoutSummary struct {
	Name    string
	Message agent.Message
}

func resolveBreakoutGroups(cfg breakoutConfig, sess Session, participants []Participant) ([]breakoutGroup, error) {
	agents, err := agentsFromParticipants(participants)
	if err != nil {
		return nil, err
	}
	if len(cfg.Groups) > 0 {
		return namedGroups(cfg.Groups)
	}
	sessionGroups := sess.Groups()
	if len(sessionGroups) > 0 {
		grouped, err := namedParticipantGroups(sessionGroups)
		if err != nil {
			return nil, err
		}
		return grouped, nil
	}
	return splitGroups(agents, cfg.GroupSize), nil
}

func agentsFromParticipants(participants []Participant) ([]agent.Agent, error) {
	if len(participants) == 0 {
		return nil, nil
	}
	agents := make([]agent.Agent, 0, len(participants))
	for _, participant := range participants {
		if participant == nil {
			return nil, fmt.Errorf("participant required")
		}
		ag, ok := participant.Agent()
		if !ok {
			return nil, fmt.Errorf("breakout requires agent participants")
		}
		agents = append(agents, ag)
	}
	return agents, nil
}

func namedParticipantGroups(groups map[string][]Participant) ([]breakoutGroup, error) {
	out := make([]breakoutGroup, 0, len(groups))
	for name, members := range groups {
		if len(members) == 0 {
			return nil, fmt.Errorf("breakout group %q has no members", name)
		}
		agents, err := agentsFromParticipants(members)
		if err != nil {
			return nil, err
		}
		out = append(out, breakoutGroup{Name: name, Members: agents})
	}
	return out, nil
}

func namedGroups(groups map[string][]agent.Agent) ([]breakoutGroup, error) {
	out := make([]breakoutGroup, 0, len(groups))
	for name, members := range groups {
		if len(members) == 0 {
			return nil, fmt.Errorf("breakout group %q has no members", name)
		}
		out = append(out, breakoutGroup{Name: name, Members: members})
	}
	return out, nil
}

func splitGroups(agents []agent.Agent, size int) []breakoutGroup {
	if size <= 0 {
		size = 2
	}
	out := make([]breakoutGroup, 0, (len(agents)+size-1)/size)
	for i := 0; i < len(agents); i += size {
		end := i + size
		if end > len(agents) {
			end = len(agents)
		}
		groupName := fmt.Sprintf("group-%d", len(out)+1)
		out = append(out, breakoutGroup{Name: groupName, Members: agents[i:end]})
	}
	return out
}

func buildGroupPrompt(task string, group breakoutGroup) string {
	var sb strings.Builder
	sb.WriteString("You are in a breakout group. Task: ")
	sb.WriteString(task)
	sb.WriteString("\nGroup: ")
	sb.WriteString(group.Name)
	sb.WriteString("\nGroup members: ")
	for i, member := range group.Members {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(member.Name)
	}
	sb.WriteString("\nProvide your group's findings.")
	return sb.String()
}

func buildReconvenePrompt(task string, summaries []breakoutSummary) string {
	var sb strings.Builder
	sb.WriteString("Reconvene the breakouts for: ")
	sb.WriteString(task)
	sb.WriteString("\n\nGroup findings:\n")
	for i, summary := range summaries {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, summary.Name, summary.Message.Summary()))
	}
	return sb.String()
}
