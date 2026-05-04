package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/contextpolicy"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/integration"
	"github.com/runmeanwhile/meanwhile/pkg/logger"
	"github.com/runmeanwhile/meanwhile/pkg/mcp"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/scheduler"
	"github.com/runmeanwhile/meanwhile/pkg/telemetry"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
	"github.com/runmeanwhile/meanwhile/pkg/toolkit"
)

var (
	// ErrSessionNotFound indicates a missing session.
	ErrSessionNotFound = errors.New("session not found")
)

const protocolStateMetadataKey = "_protocol_state"
const defaultAutoSummarizeTokens = 4000

// Engine is the core runtime for sessions and agents.
type Engine struct {
	cfg        config.GlobalConfig
	fullConfig config.Config

	providers         *provider.Registry
	defaultProvider   string
	protocols         *protocol.Registry
	hooks             *hook.Registry
	tools             *tool.Registry
	toolFactories     *tool.FactoryRegistry
	toolkits          *toolkit.Registry
	mcp               *mcp.Registry
	integrations      *integration.Registry
	requestRegistry   RequestRegistry
	humanRequestStore HumanRequestStore
	timeoutScheduler  TimeoutScheduler
	memory            memory.Store
	memoryAutomator   MemoryAutomator
	// memoryAutomationDisabled prevents auto-enabling memory automation via WithMemoryStore.
	memoryAutomationDisabled bool
	profiles                 *agent.Registry
	telemetry                telemetry.Client
	logger                   logger.Logger
	contextPolicy            contextpolicy.Policy
	contextSummarizer        contextpolicy.Summarizer
	agentPerspectiveMode     AgentPerspectiveMode
	providerRetryConfig      provider.ResilientConfig
	providerRetryEnabled     bool
	defaultRunTimeout        time.Duration

	mu            sync.RWMutex
	sessionLoadMu sync.Mutex
	sessions      map[string]*Session
	sessionStore  SessionStore
}

// Option configures the Engine.
type Option func(*Engine) error

// WithProviderRegistry sets the provider registry.
func WithProviderRegistry(reg *provider.Registry) Option {
	return func(e *Engine) error {
		e.providers = reg
		return nil
	}
}

// WithProtocolRegistry sets the protocol registry.
func WithProtocolRegistry(reg *protocol.Registry) Option {
	return func(e *Engine) error {
		e.protocols = reg
		return nil
	}
}

// WithHookRegistry sets the hook registry.
func WithHookRegistry(reg *hook.Registry) Option {
	return func(e *Engine) error {
		e.hooks = reg
		return nil
	}
}

// WithToolRegistry sets the tool registry.
func WithToolRegistry(reg *tool.Registry) Option {
	return func(e *Engine) error {
		e.tools = reg
		return nil
	}
}

// WithToolFactoryRegistry sets the tool factory registry.
func WithToolFactoryRegistry(reg *tool.FactoryRegistry) Option {
	return func(e *Engine) error {
		e.toolFactories = reg
		return nil
	}
}

// WithToolkitRegistry sets the toolkit registry.
func WithToolkitRegistry(reg *toolkit.Registry) Option {
	return func(e *Engine) error {
		e.toolkits = reg
		return nil
	}
}

// WithMCPRegistry sets the MCP registry.
func WithMCPRegistry(reg *mcp.Registry) Option {
	return func(e *Engine) error {
		e.mcp = reg
		return nil
	}
}

// WithIntegrationRegistry sets the integration registry.
func WithIntegrationRegistry(reg *integration.Registry) Option {
	return func(e *Engine) error {
		e.integrations = reg
		return nil
	}
}

// WithRequestRegistry sets the request registry.
func WithRequestRegistry(reg RequestRegistry) Option {
	return func(e *Engine) error {
		if reg == nil {
			return fmt.Errorf("request registry required")
		}
		e.requestRegistry = reg
		return nil
	}
}

// WithHumanRequestStore sets the human request store.
func WithHumanRequestStore(store HumanRequestStore) Option {
	return func(e *Engine) error {
		if store == nil {
			return fmt.Errorf("human request store required")
		}
		e.humanRequestStore = store
		return nil
	}
}

// WithTimeoutScheduler sets the timeout scheduler.
func WithTimeoutScheduler(scheduler TimeoutScheduler) Option {
	return func(e *Engine) error {
		if scheduler == nil {
			return fmt.Errorf("timeout scheduler required")
		}
		e.timeoutScheduler = scheduler
		return nil
	}
}

// WithTimeoutSchedulerDriver builds and sets a timeout scheduler from a driver.
func WithTimeoutSchedulerDriver(driver scheduler.Driver, opts ...scheduler.WorkerOption) Option {
	return func(e *Engine) error {
		service, err := e.NewTimeoutScheduler(driver, opts...)
		if err != nil {
			return err
		}
		e.timeoutScheduler = service
		return nil
	}
}

// SetTimeoutScheduler updates the engine timeout scheduler.
func (e *Engine) SetTimeoutScheduler(scheduler TimeoutScheduler) error {
	if e == nil {
		return fmt.Errorf("engine required")
	}
	if scheduler == nil {
		return fmt.Errorf("timeout scheduler required")
	}
	e.timeoutScheduler = scheduler
	return nil
}

// TimeoutScheduler returns the configured timeout scheduler.
func (e *Engine) TimeoutScheduler() TimeoutScheduler {
	if e == nil {
		return nil
	}
	return e.timeoutScheduler
}

// HumanRequestStore returns the configured human request store.
func (e *Engine) HumanRequestStore() HumanRequestStore {
	if e == nil {
		return nil
	}
	return e.humanRequestStore
}

// WithMemoryStore sets the memory store.
func WithMemoryStore(store memory.Store) Option {
	return func(e *Engine) error {
		e.memory = store
		if store != nil && e.memoryAutomator == nil && !e.memoryAutomationDisabled && e.cfg.MemoryAutomation.Enabled {
			cfg := e.cfg.MemoryAutomation
			e.memoryAutomator = &engineMemoryAutomator{engine: e, cfg: cfg}
		}
		return nil
	}
}

// WithSessionStore sets the session persistence store.
func WithSessionStore(store SessionStore) Option {
	return func(e *Engine) error {
		e.sessionStore = store
		return nil
	}
}

// WithProfileRegistry sets the agent profile registry.
func WithProfileRegistry(reg *agent.Registry) Option {
	return func(e *Engine) error {
		e.profiles = reg
		return nil
	}
}

// WithGlobalConfig sets global config defaults.
func WithGlobalConfig(cfg config.GlobalConfig) Option {
	return func(e *Engine) error {
		e.cfg = cfg
		e.fullConfig.Global = cfg
		if cfg.Defaults.ProviderID != "" {
			e.defaultProvider = cfg.Defaults.ProviderID
		}
		return nil
	}
}

// WithTelemetryClient sets the telemetry client.
func WithTelemetryClient(client telemetry.Client) Option {
	return func(e *Engine) error {
		e.telemetry = client
		return nil
	}
}

// WithLogger sets the logger that auto-subscribes to all session events.
func WithLogger(log logger.Logger) Option {
	return func(e *Engine) error {
		e.logger = log
		return nil
	}
}

// WithContextPolicy sets the context selection policy.
func WithContextPolicy(policy contextpolicy.Policy) Option {
	return func(e *Engine) error {
		if policy == nil {
			return fmt.Errorf("context policy required")
		}
		e.contextPolicy = policy
		return nil
	}
}

// WithContextSummarizer sets the summarizer used by the default policy.
func WithContextSummarizer(summarizer contextpolicy.Summarizer) Option {
	return func(e *Engine) error {
		e.contextSummarizer = summarizer
		return nil
	}
}

// WithAgentPerspectiveMode configures how prior named assistant messages are interpreted for each agent turn.
func WithAgentPerspectiveMode(mode AgentPerspectiveMode) Option {
	return func(e *Engine) error {
		e.agentPerspectiveMode = normalizeAgentPerspectiveMode(mode)
		return nil
	}
}

// WithProviderRetryConfig sets retry behavior for provider streams.
func WithProviderRetryConfig(cfg provider.ResilientConfig) Option {
	return func(e *Engine) error {
		e.providerRetryConfig = cfg
		e.providerRetryEnabled = true
		return nil
	}
}

// WithDefaultRunTimeout sets the default timeout for session Run calls.
func WithDefaultRunTimeout(timeout time.Duration) Option {
	return func(e *Engine) error {
		if timeout < 0 {
			return fmt.Errorf("timeout must be non-negative")
		}
		e.defaultRunTimeout = timeout
		return nil
	}
}

// WithProvider registers a provider with the engine.
// The first provider registered becomes the default provider.
func WithProvider(p provider.Provider) Option {
	return func(e *Engine) error {
		if p == nil {
			return fmt.Errorf("provider required")
		}
		e.providers.Register(p)
		// First provider becomes default if no default set yet
		if e.defaultProvider == "" {
			e.defaultProvider = p.ID()
		}
		return nil
	}
}

// WithDefaultProvider sets the default provider by ID.
func WithDefaultProvider(providerID string) Option {
	return func(e *Engine) error {
		e.defaultProvider = providerID
		return nil
	}
}

// WithProviders registers multiple providers with the engine.
func WithProviders(ps ...provider.Provider) Option {
	return func(e *Engine) error {
		for _, p := range ps {
			if p == nil {
				return fmt.Errorf("provider required")
			}
			e.providers.Register(p)
		}
		return nil
	}
}

// WithIntegration registers an integration with the engine.
func WithIntegration(integration integration.Integration) Option {
	return func(e *Engine) error {
		return e.registerIntegration(integration)
	}
}

// WithDefaultModel sets the global default model.
func WithDefaultModel(model string) Option {
	return func(e *Engine) error {
		if model == "" {
			return fmt.Errorf("model required")
		}
		if e.cfg.Defaults.Params == nil {
			e.cfg.Defaults.Params = make(map[string]any)
		}
		e.cfg.Defaults.Params["model"] = model
		return nil
	}
}

// WithDefaultParam sets a default parameter for all agents.
func WithDefaultParam(key string, value any) Option {
	return func(e *Engine) error {
		if key == "" {
			return fmt.Errorf("param key required")
		}
		if e.cfg.Defaults.Params == nil {
			e.cfg.Defaults.Params = make(map[string]any)
		}
		e.cfg.Defaults.Params[key] = value
		return nil
	}
}

// New creates a new Engine with defaults.
func New(opts ...Option) (*Engine, error) {
	e := &Engine{
		providers:         provider.NewRegistry(),
		protocols:         protocol.NewRegistry(),
		hooks:             hook.NewRegistry(),
		tools:             tool.NewRegistry(),
		toolFactories:     tool.NewFactoryRegistry(),
		toolkits:          toolkit.NewRegistry(),
		mcp:               mcp.NewRegistry(),
		integrations:      integration.NewRegistry(),
		requestRegistry:   NewInMemoryRequestRegistry(),
		humanRequestStore: NewInMemoryHumanRequestStore(),
		memory:            memory.NewInMemoryStore(),
		profiles:          agent.NewRegistry(),
		telemetry:         telemetry.NewNoop(),
		contextPolicy: contextpolicy.NewAutoSummarizePolicy(contextpolicy.NewDefaultPolicy(), contextpolicy.AutoSummarizeConfig{
			SummarizeAtTokens: defaultAutoSummarizeTokens,
		}),
		agentPerspectiveMode: AgentPerspectiveSpeakerAware,
		providerRetryConfig:  provider.DefaultResilientConfig(),
		providerRetryEnabled: true,
		defaultRunTimeout:    10 * time.Minute,
		sessions:             make(map[string]*Session),
		sessionStore:         NewInMemorySessionStore(),
	}

	for _, opt := range opts {
		if err := opt(e); err != nil {
			return nil, err
		}
	}

	return e, nil
}

// Agent creates a new agent. If prompt is provided, builds immediately with that prompt.
// Otherwise returns a builder for additional configuration.
func (e *Engine) Agent(name string, prompt ...string) *agent.Builder {
	builder := agent.NewBuilder(e, name)
	if len(prompt) > 0 && prompt[0] != "" {
		builder.Prompt(prompt[0])
	}
	return builder
}

// RegisterProfile registers an agent profile (called by agent.Builder).
func (e *Engine) RegisterProfile(profile agent.Profile) {
	e.profiles.Register(profile)
}

// RunAgent creates an ephemeral solo session and runs the agent with the given messages.
// This is a convenience method for simple one-off agent interactions.
// Returns the final assistant message content.
// For more control, use Session builder and explicit protocol selection.
func (e *Engine) RunAgent(a agent.Agent, messages ...agent.Message) (agent.Message, error) {
	return e.RunAgentWithContext(context.Background(), a, messages...)
}

// RunAgentWithContext runs an agent using the provided context.
func (e *Engine) RunAgentWithContext(ctx context.Context, a agent.Agent, messages ...agent.Message) (agent.Message, error) {
	result, err := e.RunAgentDetailedWithContext(ctx, a, messages...)
	if err != nil {
		return agent.Message{}, err
	}

	// Return final assistant message
	return message.Assistant(result.Final), nil
}

// RunAgentDetailed creates an ephemeral solo session, runs the agent, and returns
// the full run result including events, transcript, and protocol metadata.
func (e *Engine) RunAgentDetailed(a agent.Agent, messages ...agent.Message) (*RunResult, error) {
	return e.RunAgentDetailedWithContext(context.Background(), a, messages...)
}

// RunAgentDetailedWithContext runs an agent using the provided context and
// returns the full run result including reasoning, tool, and final-output events.
func (e *Engine) RunAgentDetailedWithContext(ctx context.Context, a agent.Agent, messages ...agent.Message) (*RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Create ephemeral session
	sess, err := e.NewSession(ctx, SessionConfig{
		Name:         "ephemeral",
		Participants: []protocol.Participant{a},
		Protocol:     protocol.Solo(),
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = e.CloseSession(context.Background(), sess.ID())
	}()

	// Run the first message (typically a user message)
	if len(messages) == 0 {
		return nil, fmt.Errorf("at least one message required")
	}

	result, err := e.Run(ctx, sess.ID(), messages[0])
	if err != nil {
		return nil, err
	}

	return result, nil
}

// resolveProvider finds the provider for a given model.
// Supports "provider:model" syntax or uses default provider.
func (e *Engine) resolveProvider(model string, overrideProvider string) (provider.Provider, string, string, error) {
	// Check for "provider:model" syntax
	if idx := strings.Index(model, ":"); idx > 0 {
		providerID := model[:idx]
		modelName := model[idx+1:]
		if strings.TrimSpace(modelName) == "" {
			return nil, "", "", fmt.Errorf("model required")
		}
		p, ok := e.providers.Get(providerID)
		if !ok {
			return nil, "", "", fmt.Errorf("%w: %s", ErrProviderNotFound, providerID)
		}
		return p, providerID, modelName, nil
	}

	providerID := overrideProvider
	if providerID == "" {
		providerID = e.defaultProvider
	}
	if providerID == "" {
		return nil, "", "", fmt.Errorf("no default provider configured")
	}
	p, ok := e.providers.Get(providerID)
	if !ok {
		return nil, "", "", fmt.Errorf("%w: %s", ErrProviderNotFound, providerID)
	}
	return p, providerID, model, nil
}

// SessionConfig creates a new session.
type SessionConfig struct {
	ID            string
	Name          string
	Tags          []string
	Metadata      map[string]any
	Protocol      protocol.Protocol
	Participants  []protocol.Participant
	Facilitator   *agent.Agent
	Groups        map[string][]protocol.Participant
	Participation ParticipationMode
	TimeoutPolicy *TimeoutPolicy
	DefaultTools  []string
	ToolPolicy    tool.Policy
	Toolkits      []string
}

// NewSession creates and initializes a session.
func (e *Engine) NewSession(ctx context.Context, cfg SessionConfig) (*Session, error) {
	if cfg.Protocol == nil {
		return nil, fmt.Errorf("protocol required")
	}

	if cfg.ID == "" {
		cfg.ID = event.NewID()
	}

	if err := validateParticipants(cfg.Participants); err != nil {
		return nil, err
	}
	if err := validateGroups(cfg.Participants, cfg.Groups); err != nil {
		return nil, err
	}

	if cfg.Participation == nil {
		cfg.Participation = TurnBased()
	}

	var timeoutPolicy *TimeoutPolicy
	if cfg.TimeoutPolicy != nil {
		policy := *cfg.TimeoutPolicy
		timeoutPolicy = &policy
	}

	sess := &Session{
		id:            cfg.ID,
		name:          cfg.Name,
		tags:          append([]string(nil), cfg.Tags...),
		metadata:      cloneMetadata(cfg.Metadata),
		protocol:      cfg.Protocol,
		participants:  cfg.Participants,
		facilitator:   cfg.Facilitator,
		groups:        cloneGroups(cfg.Groups),
		bus:           event.NewBus(),
		memory:        e.memory,
		engine:        e,
		participation: cfg.Participation,
		timeoutPolicy: timeoutPolicy,
	}
	if len(cfg.DefaultTools) > 0 {
		sess.AddDefaultTools(cfg.DefaultTools...)
	}
	if !cfg.ToolPolicy.Empty() {
		sess.toolPolicy = cfg.ToolPolicy
		sess.toolPolicySet = true
	}
	if len(cfg.Toolkits) > 0 {
		sess.toolkitIDs = append([]string(nil), cfg.Toolkits...)
	}

	if err := sess.registerConfiguredToolkits(ctx, cfg.Toolkits); err != nil {
		return nil, err
	}

	// Auto-subscribe logger if configured
	if e.logger != nil {
		sess.bus.Subscribe(func(ev event.Event) {
			_ = e.logger.Log(ev)
		})
	}
	e.subscribeIntegrations(sess)
	e.subscribeHumanRequestStore(sess)

	if err := cfg.Protocol.Init(ctx, sess); err != nil {
		return nil, fmt.Errorf("protocol init: %w", err)
	}

	// Subscribe protocol to all session events.
	_, _ = sess.bus.Subscribe(func(ev event.Event) {
		_ = cfg.Protocol.OnEvent(context.Background(), sess, ev)
	})

	if err := e.persistSession(ctx, sess); err != nil {
		_ = cfg.Protocol.Shutdown(ctx, sess)
		sess.bus.Close()
		return nil, err
	}

	e.mu.Lock()
	e.sessions[cfg.ID] = sess
	e.mu.Unlock()

	return sess, nil
}

// Run processes a message through the session protocol and returns the result.
func (e *Engine) Run(ctx context.Context, sessionID string, msg agent.Message) (*RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	sess, err := e.session(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	timeout := e.defaultRunTimeout
	if tp, ok := sess.protocol.(protocol.TimeoutProvider); ok {
		if t := tp.DefaultTimeout(); t > 0 {
			timeout = t
		}
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	sess.runMu.Lock()
	defer sess.runMu.Unlock()

	// Create a collector for events
	events := make([]event.Event, 0)
	messages := make([]agent.Message, 0)
	var finalContent string
	var mu sync.Mutex

	unsubscribe, _ := sess.bus.SubscribeSync(func(ev event.Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()

		// Collect assistant messages
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

	traceSpan, traceCtx := e.telemetry.StartTrace(ctx, telemetry.SpanInput{
		Name: "session.run",
		Attributes: map[string]any{
			"session_id":  sessionID,
			"protocol_id": sess.protocol.ID(),
		},
	})
	var runErr error
	defer func() {
		traceSpan.End(runErr)
	}()

	meta := hook.SessionMeta{SessionID: sessionID, ProtocolID: sess.protocol.ID()}
	for _, h := range e.hooks.PreMessage() {
		decision, newMsg, err := h.OnPreMessage(traceCtx, meta, msg)
		if err != nil {
			runErr = err
			return nil, err
		}
		if decision == hook.Block {
			_ = sess.EmitWithContext(traceCtx, event.New(event.HookBlocked, sessionID, map[string]any{"hook": h.ID()}))
			return &RunResult{
				Status:   StatusCompleted,
				Events:   events,
				Metadata: map[string]any{"blocked_by": h.ID()},
			}, nil
		}
		if decision == hook.Modify {
			msg = newMsg
			_ = sess.EmitWithContext(traceCtx, event.New(event.HookModified, sessionID, map[string]any{"hook": h.ID()}))
		}
	}

	if err := sess.protocol.OnMessage(traceCtx, sess, msg); err != nil {
		if awaiting := sess.awaitingResult(err, events, messages, finalContent); awaiting != nil {
			runErr = err
			return awaiting, nil
		}
		runErr = err
		return nil, fmt.Errorf("protocol message: %w", err)
	}

	if stateful, ok := sess.protocol.(protocol.StatefulProtocol); ok {
		if state, err := stateful.GetState(); err == nil {
			if sess.metadata == nil {
				sess.metadata = make(map[string]any)
			}
			sess.metadata[protocolStateMetadataKey] = state
			_ = e.persistSession(traceCtx, sess)
		}
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

	if rp, ok := sess.protocol.(protocol.ResultProvider); ok {
		if meta := rp.Result(); len(meta) > 0 {
			result.Metadata = cloneMetadata(meta)
			return result, nil
		}
	}

	if action := sess.LastProtocolAction(); action != nil {
		if payload, ok := action.(map[string]any); ok {
			result.Metadata = cloneMetadata(payload)
		} else {
			result.Metadata["protocol_action"] = action
		}
	}

	return result, nil
}

// RunProtocol is a convenience method for one-shot protocol execution.
// It creates an ephemeral session, runs the protocol with the given message,
// closes the session, and returns the result. Useful for quick protocol tests
// or stateless protocol executions.
//
// Example:
//
//	result, _ := eng.RunProtocol(ctx, protocol.Handoff(alice, bob), message.User("hello"))
//	fmt.Println(result.Final)
func (e *Engine) RunProtocol(ctx context.Context, proto protocol.Protocol, msg agent.Message) (*RunResult, error) {
	return e.Session("ephemeral").Protocol(proto).Run(ctx, msg)
}

// Subscribe registers an event handler for a session.
func (e *Engine) Subscribe(sessionID string, handler event.Subscriber) (event.Unsubscribe, error) {
	sess, err := e.session(context.Background(), sessionID)
	if err != nil {
		return nil, err
	}
	return sess.bus.Subscribe(handler)
}

// ResumeTool delivers a tool result to a session and resumes pending execution.
func (e *Engine) ResumeTool(ctx context.Context, sessionID string, requestID string, result tool.Result) (*RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	sess, err := e.session(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return sess.ResumeTool(ctx, requestID, result)
}

func (e *Engine) session(ctx context.Context, id string) (*Session, error) {
	e.mu.RLock()
	sess, ok := e.sessions[id]
	e.mu.RUnlock()
	if ok {
		return sess, nil
	}
	if e.sessionStore == nil {
		return nil, ErrSessionNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}

	e.sessionLoadMu.Lock()
	defer e.sessionLoadMu.Unlock()

	e.mu.RLock()
	sess, ok = e.sessions[id]
	e.mu.RUnlock()
	if ok {
		return sess, nil
	}

	record, err := e.sessionStore.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.sessionFromRecord(ctx, record)
}
