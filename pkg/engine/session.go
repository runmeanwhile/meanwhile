package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/telemetry"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
	"github.com/runmeanwhile/meanwhile/pkg/toolkit"
)

// Session represents a collaboration session.
type Session struct {
	id             string
	name           string
	tags           []string
	metadata       map[string]any
	protocol       protocol.Protocol
	participants   []protocol.Participant
	facilitator    *agent.Agent
	groups         map[string][]protocol.Participant
	bus            *event.Bus
	memory         memory.Store
	engine         *Engine
	tools          *tool.Registry
	defaultToolIDs []string
	toolPolicy     tool.Policy
	toolPolicySet  bool
	toolkitIDs     []string
	pendingToolsMu sync.Mutex
	pendingTools   map[string]pendingTool
	participation  ParticipationMode
	timeoutPolicy  *TimeoutPolicy
	pendingMu      sync.Mutex
	pending        map[string]pendingInput
	resultMu       sync.RWMutex
	lastAction     any
	runMu          sync.Mutex
}

// ID returns the session ID.
func (s *Session) ID() string { return s.id }

// Name returns the session name.
func (s *Session) Name() string { return s.name }

// Tags returns a copy of session tags.
func (s *Session) Tags() []string {
	return append([]string(nil), s.tags...)
}

// Metadata returns a shallow copy of session metadata.
func (s *Session) Metadata() map[string]any {
	return cloneMetadata(s.metadata)
}

// ProtocolID returns the protocol ID.
func (s *Session) ProtocolID() string { return s.protocol.ID() }

// Participants returns all session participants.
func (s *Session) Participants() []protocol.Participant {
	return append([]protocol.Participant(nil), s.participants...)
}

// HumanParticipants returns human participants in the session.
func (s *Session) HumanParticipants() []protocol.Participant {
	if len(s.participants) == 0 {
		return nil
	}
	humans := make([]protocol.Participant, 0, len(s.participants))
	for _, participant := range s.participants {
		if participant != nil && participant.IsHuman() {
			humans = append(humans, participant)
		}
	}
	if len(humans) == 0 {
		return nil
	}
	return humans
}

// Facilitator returns the facilitator agent if set.
func (s *Session) Facilitator() *agent.Agent { return s.facilitator }

// Groups returns a copy of named groups.
func (s *Session) Groups() map[string][]protocol.Participant {
	return cloneGroups(s.groups)
}

// Emit publishes an event to subscribers and memory using a background context.
func (s *Session) Emit(ev event.Event) error {
	return s.EmitWithContext(context.Background(), ev)
}

// EmitWithContext publishes an event to subscribers and memory with the provided context.
func (s *Session) EmitWithContext(ctx context.Context, ev event.Event) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ev.SessionID == "" {
		ev.SessionID = s.id
	}
	if ev.ProtocolID == "" && s.protocol != nil {
		ev.ProtocolID = s.protocol.ID()
	}
	if ev.ID == "" {
		ev.ID = event.NewID()
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	if ev.Type == event.ProtocolAction {
		s.resultMu.Lock()
		if payload, ok := ev.Payload.(map[string]any); ok {
			s.lastAction = cloneMetadata(payload)
		} else {
			s.lastAction = ev.Payload
		}
		s.resultMu.Unlock()
	}

	var memErr error
	if s.memory != nil {
		memErr = s.memory.Append(ctx, s.id, ev)
	}
	busErr := s.bus.Publish(ev)
	if memErr != nil {
		return memErr
	}
	return busErr
}

// Run executes a protocol turn with the given message and returns the result.
// This is a convenience method that wraps engine.Run() to hide session ID plumbing.
func (s *Session) Run(ctx context.Context, msg agent.Message) (*RunResult, error) {
	return s.engine.Run(ctx, s.id, msg)
}

// RunTurn executes a participant turn, awaiting input if the participant is human.
func (s *Session) RunTurn(ctx context.Context, participant protocol.Participant, req protocol.RunRequest, opts ...protocol.TurnOption) (agent.Message, error) {
	if participant == nil {
		return agent.Message{}, fmt.Errorf("participant required")
	}

	if participant.IsHuman() {
		options := protocol.TurnOptions{}
		for _, opt := range opts {
			if opt != nil {
				opt(&options)
			}
		}
		if options.Resume == nil {
			return agent.Message{}, fmt.Errorf("human turn requires resume")
		}
		requestOpts := []protocol.InputOption{}
		if options.Timeout > 0 {
			requestOpts = append(requestOpts, protocol.WithInputTimeout(options.Timeout))
		}
		if !options.Deadline.IsZero() {
			requestOpts = append(requestOpts, protocol.WithInputDeadline(options.Deadline))
		}
		return agent.Message{}, s.AwaitInput(ctx, participant, options.Context, options.Resume, requestOpts...)
	}

	ag, ok := participant.Agent()
	if !ok {
		return agent.Message{}, fmt.Errorf("agent participant required")
	}
	return s.RunAgent(ctx, ag, req)
}

// AwaitInput registers a pending human input request and pauses execution.
func (s *Session) AwaitInput(ctx context.Context, participant protocol.Participant, turnContext string, resume protocol.TurnResume, opts ...protocol.InputOption) error {
	if participant == nil {
		return fmt.Errorf("participant required")
	}
	if !participant.IsHuman() {
		return fmt.Errorf("await input requires human participant")
	}
	if resume == nil {
		return fmt.Errorf("resume function required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	options := protocol.InputOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	requestedAt := time.Now().UTC()
	timeoutAt := time.Time{}
	if options.Timeout > 0 {
		timeoutAt = requestedAt.Add(options.Timeout)
	}
	if !options.Deadline.IsZero() {
		deadline := options.Deadline.UTC()
		if timeoutAt.IsZero() || deadline.Before(timeoutAt) {
			timeoutAt = deadline
		}
	}

	request := protocol.InputRequest{
		RequestID:       event.NewID(),
		ParticipantID:   participantKey(participant),
		ParticipantName: participant.DisplayName(),
		Context:         turnContext,
		RequestedAt:     requestedAt,
		TimeoutAt:       timeoutAt,
	}

	s.pendingMu.Lock()
	if s.pending == nil {
		s.pending = make(map[string]pendingInput)
	}
	wasPaused := len(s.pending) > 0
	s.pending[request.RequestID] = pendingInput{request: request, resume: resume, resumable: true}
	s.pendingMu.Unlock()

	if err := s.scheduleTimeout(ctx, request); err != nil {
		_ = s.EmitWithContext(ctx, event.New(event.HumanRequestTimeoutScheduleFailed, s.id, map[string]any{
			"request_id": request.RequestID,
			"error":      err.Error(),
		}))
	}

	if s.engine != nil && s.engine.requestRegistry != nil {
		if err := s.engine.requestRegistry.Register(ctx, request.RequestID, s.id); err != nil {
			_ = s.EmitWithContext(ctx, event.New(event.HumanRequestRegistryFailed, s.id, map[string]any{
				"request_id": request.RequestID,
				"error":      err.Error(),
			}))
		}
	}

	_ = s.EmitWithContext(ctx, event.New(event.AwaitingUserInput, s.id, request))
	if !wasPaused {
		_ = s.EmitWithContext(ctx, event.New(event.SessionPaused, s.id, s.State()))
	}
	if s.engine != nil {
		_ = s.engine.persistSessionState(ctx, s)
	}
	return &protocol.AwaitingInputError{Request: request}
}

// Respond supplies human input and resumes a paused session.
func (s *Session) Respond(ctx context.Context, requestID string, msg agent.Message, opts ...RespondOption) (*RunResult, error) {
	if requestID == "" {
		return nil, fmt.Errorf("request id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.runMu.Lock()
	defer s.runMu.Unlock()

	pending, ok := s.pendingRequest(requestID)
	if !ok {
		return nil, ErrRequestNotFound
	}
	if !pending.resumable {
		s.clearPendingRequest(ctx, requestID)
		return nil, ErrSessionNotResumable
	}
	options := respondOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if isMessageEmpty(msg) && options.timeoutPolicy == nil && s.timeoutPolicy != nil {
		policy := *s.timeoutPolicy
		options.timeoutPolicy = &policy
	}

	events := make([]event.Event, 0)
	messages := make([]agent.Message, 0)
	var finalContent string
	var mu sync.Mutex

	unsubscribe, _ := s.bus.SubscribeSync(func(ev event.Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()

		if ev.Type == event.AgentMessageComplete {
			var msg agent.Message
			switch payload := ev.Payload.(type) {
			case agent.Message:
				msg = payload
			case map[string]any:
				if direct, ok := payload["message"].(agent.Message); ok {
					msg = direct
				} else if msgMap, ok := payload["message"].(map[string]any); ok {
					msg = agent.MessageFromMap(msgMap)
				}
			}
			if msg.Role != "" || len(msg.Parts) > 0 || msg.Text() != "" {
				mu.Lock()
				messages = append(messages, msg)
				if msg.Role == agent.RoleAssistant {
					finalContent = msg.Summary()
				}
				mu.Unlock()
			}
		}
	})
	defer unsubscribe()

	traceSpan, traceCtx := s.engine.telemetry.StartTrace(ctx, telemetry.SpanInput{
		Name: "session.respond",
		Attributes: map[string]any{
			"session_id":  s.id,
			"protocol_id": s.protocol.ID(),
		},
	})
	var runErr error
	defer func() {
		traceSpan.End(runErr)
	}()

	meta := hook.SessionMeta{SessionID: s.id, ProtocolID: s.protocol.ID()}
	if isMessageEmpty(msg) {
		timeoutMsg, err := s.resolveTimeout(traceCtx, pending, options)
		if err != nil {
			if awaiting := s.awaitingResult(err, events, messages, finalContent); awaiting != nil {
				runErr = err
				return awaiting, nil
			}
			runErr = err
			return nil, err
		}
		msg = timeoutMsg
	}

	if msg.Name == "" && pending.request.ParticipantName != "" && msg.Role != agent.RoleSystem {
		msg.Name = pending.request.ParticipantName
	}

	for _, h := range s.engine.hooks.PreMessage() {
		decision, newMsg, err := h.OnPreMessage(traceCtx, meta, msg)
		if err != nil {
			runErr = err
			return nil, err
		}
		if decision == hook.Block {
			_ = s.EmitWithContext(traceCtx, event.New(event.HookBlocked, s.id, map[string]any{"hook": h.ID()}))
			return &RunResult{
				Status:   StatusCompleted,
				Events:   events,
				Metadata: map[string]any{"blocked_by": h.ID()},
			}, nil
		}
		if decision == hook.Modify {
			msg = newMsg
			_ = s.EmitWithContext(traceCtx, event.New(event.HookModified, s.id, map[string]any{"hook": h.ID()}))
		}
	}

	if msg.Role != agent.RoleSystem {
		response := HumanResponse{
			RequestID:       pending.request.RequestID,
			SessionID:       s.id,
			ProtocolID:      s.protocol.ID(),
			ParticipantID:   pending.request.ParticipantID,
			ParticipantName: pending.request.ParticipantName,
			Response:        msg,
			ReceivedAt:      time.Now().UTC(),
		}
		_ = s.EmitWithContext(traceCtx, event.New(event.HumanResponseReceived, s.id, response))
	}

	if err := pending.resume(traceCtx, msg); err != nil {
		_, _ = s.removePending(requestID)
		if s.engine != nil {
			_ = s.engine.persistSessionState(traceCtx, s)
		}
		if awaiting := s.awaitingResult(err, events, messages, finalContent); awaiting != nil {
			runErr = err
			return awaiting, nil
		}
		runErr = err
		return nil, fmt.Errorf("protocol resume: %w", err)
	}

	remaining, _ := s.removePending(requestID)
	if remaining == 0 {
		_ = s.EmitWithContext(traceCtx, event.New(event.SessionResumed, s.id, s.State()))
	}
	if s.engine != nil {
		_ = s.engine.persistSessionState(traceCtx, s)
	}

	mu.Lock()
	result := &RunResult{
		Status:     StatusCompleted,
		Final:      finalContent,
		Transcript: append([]agent.Message(nil), messages...),
		Events:     append([]event.Event(nil), events...),
		Metadata:   make(map[string]any),
	}
	mu.Unlock()

	if rp, ok := s.protocol.(protocol.ResultProvider); ok {
		if meta := rp.Result(); len(meta) > 0 {
			result.Metadata = cloneMetadata(meta)
			return result, nil
		}
	}

	if action := s.LastProtocolAction(); action != nil {
		if payload, ok := action.(map[string]any); ok {
			result.Metadata = cloneMetadata(payload)
		} else {
			result.Metadata["protocol_action"] = action
		}
	}

	return result, nil
}

// RegisterTool adds a tool to the session-scoped registry.
func (s *Session) RegisterTool(t any) error {
	toolImpl, ok := t.(tool.Tool)
	if !ok {
		return fmt.Errorf("expected tool.Tool, got %T", t)
	}
	if s.tools == nil {
		s.tools = tool.NewRegistry()
	}
	s.tools.Register(toolImpl)
	return nil
}

// RegisterTools adds multiple tools to the session-scoped registry.
func (s *Session) RegisterTools(tools ...any) error {
	for _, t := range tools {
		if err := s.RegisterTool(t); err != nil {
			return err
		}
	}
	return nil
}

// AddDefaultTools appends tool IDs to the session's default tools list.
func (s *Session) AddDefaultTools(ids ...string) {
	// Deduplicate
	existing := make(map[string]bool)
	for _, id := range s.defaultToolIDs {
		existing[id] = true
	}
	for _, id := range ids {
		if !existing[id] {
			s.defaultToolIDs = append(s.defaultToolIDs, id)
			existing[id] = true
		}
	}
}

// DefaultTools returns a copy of the session's default tool IDs.
func (s *Session) DefaultTools() []string {
	return append([]string(nil), s.defaultToolIDs...)
}

// SetToolPolicy sets the session tool policy.
func (s *Session) SetToolPolicy(policy tool.Policy) {
	s.toolPolicy = policy
	s.toolPolicySet = !policy.Empty()
}

// ToolPolicy returns the session tool policy and whether it is set.
func (s *Session) ToolPolicy() (tool.Policy, bool) {
	if !s.toolPolicySet {
		return tool.Policy{}, false
	}
	policy := s.toolPolicy
	policy.AllowIDs = append([]string(nil), s.toolPolicy.AllowIDs...)
	policy.DenyIDs = append([]string(nil), s.toolPolicy.DenyIDs...)
	policy.AllowTags = append([]string(nil), s.toolPolicy.AllowTags...)
	policy.DenyTags = append([]string(nil), s.toolPolicy.DenyTags...)
	return policy, true
}

// Toolkits returns the session toolkit IDs.
func (s *Session) Toolkits() []string {
	return append([]string(nil), s.toolkitIDs...)
}

// RegisterToolkit registers tools from a toolkit and adds defaults.
func (s *Session) RegisterToolkit(ctx context.Context, tk toolkit.Toolkit) error {
	if tk == nil {
		return fmt.Errorf("toolkit required")
	}
	if tk.ID() == "" {
		return fmt.Errorf("toolkit id required")
	}
	tools, err := tk.Tools(ctx)
	if err != nil {
		return fmt.Errorf("toolkit %s: %w", tk.ID(), err)
	}
	for _, toolInst := range tools {
		if err := s.RegisterTool(toolInst); err != nil {
			return fmt.Errorf("toolkit %s: %w", tk.ID(), err)
		}
	}
	if binder, ok := tk.(toolkit.RegistryBinder); ok && s.tools != nil {
		binder.BindRegistry(s.tools)
	}
	defaults := tk.DefaultToolIDs()
	if len(defaults) == 0 {
		defaults = toolIDsFromTools(tools)
	}
	if len(defaults) > 0 {
		s.AddDefaultTools(defaults...)
	}
	if !containsString(s.toolkitIDs, tk.ID()) {
		s.toolkitIDs = append(s.toolkitIDs, tk.ID())
	}
	return nil
}

// RegisterToolkits registers multiple toolkits.
func (s *Session) RegisterToolkits(ctx context.Context, kits ...toolkit.Toolkit) error {
	for _, tk := range kits {
		if err := s.RegisterToolkit(ctx, tk); err != nil {
			return err
		}
	}
	return nil
}

// TimeoutPolicy returns the session default timeout policy.
func (s *Session) TimeoutPolicy() *TimeoutPolicy {
	if s.timeoutPolicy == nil {
		return nil
	}
	policy := *s.timeoutPolicy
	return &policy
}

// SetTimeoutPolicy updates the session default timeout policy.
func (s *Session) SetTimeoutPolicy(policy TimeoutPolicy) {
	s.timeoutPolicy = &policy
}

// LastProtocolAction returns the most recent protocol action payload.
func (s *Session) LastProtocolAction() any {
	s.resultMu.RLock()
	defer s.resultMu.RUnlock()
	return s.lastAction
}

type pendingInput struct {
	request   protocol.InputRequest
	resume    protocol.TurnResume
	resumable bool
}

func (s *Session) pendingRequest(id string) (pendingInput, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if len(s.pending) == 0 {
		return pendingInput{}, false
	}
	pending, ok := s.pending[id]
	if !ok {
		return pendingInput{}, false
	}
	return pending, true
}

func (s *Session) pendingTool(id string) (pendingTool, bool) {
	s.pendingToolsMu.Lock()
	defer s.pendingToolsMu.Unlock()
	if len(s.pendingTools) == 0 {
		return pendingTool{}, false
	}
	pending, ok := s.pendingTools[id]
	if !ok {
		return pendingTool{}, false
	}
	return pending, true
}

func (s *Session) removePending(id string) (int, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if len(s.pending) == 0 {
		return 0, false
	}
	if _, ok := s.pending[id]; !ok {
		return len(s.pending), false
	}
	delete(s.pending, id)
	if s.engine != nil && s.engine.timeoutScheduler != nil {
		_ = s.engine.timeoutScheduler.CancelTimeout(context.Background(), id)
	}
	if s.engine != nil && s.engine.requestRegistry != nil {
		_ = s.engine.requestRegistry.Delete(context.Background(), id)
	}
	if len(s.pending) == 0 {
		s.pending = nil
		return 0, true
	}
	return len(s.pending), true
}

func (s *Session) removePendingTool(id string) (int, bool) {
	s.pendingToolsMu.Lock()
	defer s.pendingToolsMu.Unlock()
	if len(s.pendingTools) == 0 {
		return 0, false
	}
	if _, ok := s.pendingTools[id]; !ok {
		return len(s.pendingTools), false
	}
	delete(s.pendingTools, id)
	if len(s.pendingTools) == 0 {
		s.pendingTools = nil
		return 0, true
	}
	return len(s.pendingTools), true
}

func (s *Session) awaitingResult(err error, events []event.Event, messages []agent.Message, finalContent string) *RunResult {
	var awaiting *protocol.AwaitingInputError
	if !errors.As(err, &awaiting) || awaiting == nil {
		var awaitingTool *tool.AwaitingResultError
		if !errors.As(err, &awaitingTool) || awaitingTool == nil {
			return nil
		}
		request := awaitingTool.Request
		result := &RunResult{
			Status:       StatusAwaitingTool,
			RequestID:    request.RequestID,
			Context:      request.Context,
			AwaitingTool: &request,
			Final:        finalContent,
			Transcript:   append([]agent.Message(nil), messages...),
			Events:       append([]event.Event(nil), events...),
		}
		return result
	}

	request := awaiting.Request
	result := &RunResult{
		Status:        StatusAwaitingInput,
		RequestID:     request.RequestID,
		Context:       request.Context,
		AwaitingInput: &request,
		Final:         finalContent,
		Transcript:    append([]agent.Message(nil), messages...),
		Events:        append([]event.Event(nil), events...),
	}
	return result
}
