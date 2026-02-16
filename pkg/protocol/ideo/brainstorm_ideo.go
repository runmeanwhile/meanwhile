package ideo

import (
	"context"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/collab/minutes"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

// brainstormIDEO orchestrates a multi-session IDEO-inspired brainstorming protocol.
type brainstormIDEO struct {
	cfg        Config
	lastResult map[string]any
}

// Brainstorm creates a new IDEO-inspired brainstorming protocol.
// This protocol separates brainstorming into distinct phases with deliberate
// context transfer between them.
func Brainstorm(opts ...Option) protocol.Protocol {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &brainstormIDEO{cfg: cfg}
}

// ID returns the protocol identifier.
func (p *brainstormIDEO) ID() string { return "protocol.brainstorm_ideo" }

// Participants returns nil - participants come from the session.
func (p *brainstormIDEO) Participants() []protocol.Participant { return nil }

// Config returns the protocol configuration.
func (p *brainstormIDEO) Config() protocol.Config {
	return protocol.Config{
		"scope":               p.cfg.Scope,
		"inspiration_rounds":  p.cfg.InspirationRounds,
		"reframe_rounds":      p.cfg.ReframeRounds,
		"ideation_rounds":     p.cfg.IdeationRounds,
		"synthesis_rounds":    p.cfg.SynthesisRounds,
		"target_hmws":         p.cfg.TargetHMWs,
		"target_concepts":     p.cfg.TargetConcepts,
		"finalist_count":      p.cfg.FinalistCount,
		"transfer_strategy":   string(p.cfg.TransferStrategy),
		"artifact_tools":      p.cfg.ArtifactTools,
		"human_in_loop":       p.cfg.HumanInLoop,
	}
}

// Init initializes the protocol.
func (p *brainstormIDEO) Init(_ context.Context, _ protocol.Session) error {
	return nil
}

// OnMessage runs the full IDEO brainstorming cycle.
func (p *brainstormIDEO) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
	participants := sess.Participants()
	if len(participants) == 0 {
		return fmt.Errorf("brainstorm_ideo requires at least one participant")
	}

	agents, err := participantsToAgents(participants)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return fmt.Errorf("brainstorm_ideo requires agent participants")
	}

	p.lastResult = nil

	// Resolve scope from message or config
	scope := p.resolveScope(msg)

	//
	// PHASE 0: READINESS GATE
	// Goal: Moderator assesses if there's enough context to proceed
	//
	readinessResult, err := p.runReadinessGate(ctx, sess, agents, scope, msg)
	if err != nil {
		return fmt.Errorf("readiness gate: %w", err)
	}

	// Handle readiness decision
	switch readinessResult.Decision {
	case DecisionReject:
		// Emit rejection and stop
		return p.emitRejection(sess, readinessResult)

	case DecisionRequestInfo:
		// Emit request for more info and stop
		return p.emitInfoRequest(sess, readinessResult)

	case DecisionProceed, DecisionProceedWithAssumptions:
		// Continue with potentially refined scope
		if readinessResult.RefinedScope != "" {
			scope = readinessResult.RefinedScope
		}
	}

	//
	// PHASE 1: INSPIRATION
	// Goal: Empathize, observe, gather tensions before jumping to solutions
	//
	inspirationResult, err := p.runInspiration(ctx, sess, agents, scope, msg, readinessResult)
	if err != nil {
		return fmt.Errorf("inspiration phase: %w", err)
	}
	inspirationTransfer := p.buildInspirationTransfer(inspirationResult, scope)

	//
	// PHASE 2: REFRAME
	// Goal: Generate diverse HMW framings across multiple lenses
	//
	reframeResult, err := p.runReframe(ctx, sess, agents, scope, inspirationTransfer)
	if err != nil {
		return fmt.Errorf("reframe phase: %w", err)
	}
	reframeTransfer := p.buildReframeTransfer(reframeResult, inspirationTransfer, scope)

	//
	// PHASE 3: IDEATION
	// Goal: Generate divergent concepts with wild ideas and artifact-based thinking
	//
	ideationResult, err := p.runIdeation(ctx, sess, agents, scope, reframeTransfer)
	if err != nil {
		return fmt.Errorf("ideation phase: %w", err)
	}
	ideationTransfer := p.buildIdeationTransfer(ideationResult, reframeTransfer, scope)

	//
	// PHASE 4: SYNTHESIS
	// Goal: Converge to experiment-ready portfolio with evidence gates
	//
	synthesisResult, err := p.runSynthesis(ctx, sess, agents, scope, ideationTransfer)
	if err != nil {
		return fmt.Errorf("synthesis phase: %w", err)
	}

	// Build final result
	mins := minutes.New()
	mins.Add("scope", scope)
	mins.Add("participants", agentNames(agents))
	mins.Add("config", p.Config())

	// Include readiness gate results
	mins.Add("readiness", map[string]any{
		"decision":      string(readinessResult.Decision),
		"assumptions":   readinessResult.Assumptions,
		"refined_scope": readinessResult.RefinedScope,
	})

	mins.Add("inspiration", map[string]any{
		"tensions":     inspirationResult.Tensions,
		"observations": inspirationResult.Observations,
		"constraints":  inspirationResult.Constraints,
		"artifacts":    inspirationResult.Artifacts,
	})

	mins.Add("reframe", map[string]any{
		"all_frames":      reframeResult.Frames,
		"selected_frames": reframeResult.SelectedFrames,
	})

	mins.Add("ideation", map[string]any{
		"concepts":  ideationResult.Concepts,
		"artifacts": ideationResult.Artifacts,
	})

	mins.Add("synthesis", map[string]any{
		"cards":          synthesisResult.Cards,
		"eligible":       synthesisResult.Eligible,
		"rejected":       synthesisResult.Rejected,
		"portfolio":      synthesisResult.Portfolio,
		"human_feedback": synthesisResult.HumanFeedback,
	})

	if synthesisResult.Closing != "" {
		mins.SetSummary(synthesisResult.Closing)
	}

	payload := mins.Payload()
	p.lastResult = payload

	if err := sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload)); err != nil {
		return fmt.Errorf("emit brainstorm_ideo: %w", err)
	}

	return nil
}

// OnEvent handles protocol events - no longer needed for tool tracking
// since Session.History() now provides full conversation context.
func (p *brainstormIDEO) OnEvent(_ context.Context, _ protocol.Session, _ event.Event) error {
	return nil
}

// Shutdown cleans up the protocol.
func (p *brainstormIDEO) Shutdown(_ context.Context, _ protocol.Session) error {
	return nil
}

// Result returns the structured result from the last run.
func (p *brainstormIDEO) Result() map[string]any {
	if p.lastResult == nil {
		return nil
	}
	// Return a copy
	result := make(map[string]any, len(p.lastResult))
	for k, v := range p.lastResult {
		result[k] = v
	}
	return result
}

// resolveScope determines the scope from the message or config.
func (p *brainstormIDEO) resolveScope(msg agent.Message) string {
	// First try message text
	if text := msg.Text(); text != "" {
		return text
	}
	// Fall back to config
	if p.cfg.Scope != "" {
		return p.cfg.Scope
	}
	return "Brainstorming session"
}

// selectRunner returns the agent to use for moderator tasks.
func (p *brainstormIDEO) selectRunner(sess protocol.Session, agents []agent.Agent) agent.Agent {
	if facilitator := sess.Facilitator(); facilitator != nil {
		return *facilitator
	}
	return agents[0]
}

// runParamsFor returns runtime parameters for an agent.
func (p *brainstormIDEO) runParamsFor(participant agent.Agent) map[string]any {
	if len(p.cfg.Params) == 0 {
		return nil
	}
	if len(participant.Params) == 0 {
		// Return copy of config params
		out := make(map[string]any, len(p.cfg.Params))
		for k, v := range p.cfg.Params {
			out[k] = v
		}
		return out
	}
	// Merge, preferring participant params
	out := make(map[string]any, len(p.cfg.Params))
	for k, v := range p.cfg.Params {
		if _, ok := participant.Params[k]; ok {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// participantsToAgents converts protocol participants to agents.
func participantsToAgents(participants []protocol.Participant) ([]agent.Agent, error) {
	agents := make([]agent.Agent, 0, len(participants))
	for _, p := range participants {
		a, ok := p.Agent()
		if !ok {
			continue // Skip non-agent participants
		}
		agents = append(agents, a)
	}
	return agents, nil
}

// agentNames extracts names from agents.
func agentNames(agents []agent.Agent) []string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return names
}

// emitRejection emits a protocol action indicating the request was rejected.
func (p *brainstormIDEO) emitRejection(sess protocol.Session, result *ReadinessResult) error {
	payload := map[string]any{
		"status":    "rejected",
		"reason":    result.Rejection,
		"context":   result.Context,
		"message":   "The brainstorming request was rejected by the moderator due to insufficient clarity or scope.",
	}
	p.lastResult = payload
	return sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload))
}

// emitInfoRequest emits a protocol action requesting more information from the user.
func (p *brainstormIDEO) emitInfoRequest(sess protocol.Session, result *ReadinessResult) error {
	payload := map[string]any{
		"status":    "info_requested",
		"missing":   result.Missing,
		"context":   result.Context,
		"message":   "The moderator needs more information before the team can proceed productively.",
	}
	p.lastResult = payload
	return sess.Emit(event.New(event.ProtocolAction, sess.ID(), payload))
}
