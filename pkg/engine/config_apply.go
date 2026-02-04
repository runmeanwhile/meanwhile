package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/contextpolicy"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/telemetry"
	"github.com/runmeanwhile/meanwhile/pkg/telemetry/langfuse"
)

var (
	// ErrConfigNotFound indicates a missing config entry.
	ErrConfigNotFound = fmt.Errorf("config entry not found")
)

// WithConfig applies a full configuration to the engine.
func WithConfig(cfg config.Config) Option {
	return func(e *Engine) error {
		return e.ApplyConfig(cfg)
	}
}

// ApplyConfig applies a full configuration to the engine.
func (e *Engine) ApplyConfig(cfg config.Config) error {
	e.cfg = cfg.Global
	e.fullConfig = cfg

	for id, providerCfg := range cfg.Global.Providers {
		if providerCfg.Type == "" {
			providerCfg.Type = id
		}
		p, err := provider.NewFromConfig(providerCfg)
		if err != nil {
			return fmt.Errorf("provider %s: %w", id, err)
		}
		if id != "" && id != p.ID() {
			p = provider.WithID(p, id)
		}
		e.providers.Register(p)
	}

	if cfg.Global.Defaults.ProviderID != "" {
		e.defaultProvider = cfg.Global.Defaults.ProviderID
	} else if e.defaultProvider == "" && len(cfg.Global.Providers) == 1 {
		for id := range cfg.Global.Providers {
			e.defaultProvider = id
		}
	}

	if err := e.applyMemoryConfig(cfg.Global.Memory); err != nil {
		return err
	}

	if err := e.applyTelemetryConfig(cfg.Global.Telemetry); err != nil {
		return err
	}

	if err := e.applyContextConfig(cfg.Global.Context); err != nil {
		return err
	}

	e.applyProviderRetryConfig(cfg.Global.ProviderRetry)

	if cfg.Global.RunTimeoutSeconds != 0 {
		if cfg.Global.RunTimeoutSeconds < 0 {
			e.defaultRunTimeout = 0
		} else {
			e.defaultRunTimeout = time.Duration(cfg.Global.RunTimeoutSeconds) * time.Second
		}
	}

	if err := e.applyToolConfig(cfg.Tools); err != nil {
		return err
	}

	if cfg.Global.MemoryAutomation.Enabled && e.memoryAutomator == nil && !e.memoryAutomationDisabled {
		e.memoryAutomator = &engineMemoryAutomator{engine: e, cfg: cfg.Global.MemoryAutomation}
	}

	return nil
}

func (e *Engine) applyMemoryConfig(cfg config.MemoryConfig) error {
	if strings.TrimSpace(cfg.Store) == "" {
		return nil
	}

	storeID := strings.ToLower(strings.TrimSpace(cfg.Store))
	params := cfg.Params

	switch storeID {
	case "memory", "inmemory", "in-memory":
		return WithMemoryStore(memory.NewInMemoryStore())(e)
	case "file", "filestore":
		path := firstString(params, "path", "base_path")
		if path == "" {
			return fmt.Errorf("memory store %q requires params.path", storeID)
		}
		options := make([]memory.FileChatStoreOption, 0)
		if syncEvery, ok := intParam(params, "sync_every"); ok {
			options = append(options, memory.WithSyncEvery(syncEvery))
		}
		store, err := memory.NewFileChatStore(path, options...)
		if err != nil {
			return err
		}
		return WithMemoryStore(store)(e)
	case "postgres":
		dsn := firstString(params, "dsn", "url")
		if dsn == "" {
			return fmt.Errorf("memory store %q requires params.dsn", storeID)
		}
		opts := make([]memory.PostgresOption, 0)
		if schema := firstString(params, "schema"); schema != "" {
			opts = append(opts, memory.WithSchema(schema))
		}
		if table := firstString(params, "table"); table != "" {
			opts = append(opts, memory.WithTable(table))
		}
		if autoMigrate, ok := boolParam(params, "auto_migrate"); ok {
			opts = append(opts, memory.WithAutoMigrate(autoMigrate))
		}
		store, err := memory.NewPostgresStore(dsn, opts...)
		if err != nil {
			return err
		}
		return WithMemoryStore(store)(e)
	default:
		return fmt.Errorf("unknown memory store %q", storeID)
	}
}

func (e *Engine) applyTelemetryConfig(cfg config.TelemetryConfig) error {
	if strings.TrimSpace(cfg.Provider) == "" {
		return nil
	}
	providerID := strings.ToLower(strings.TrimSpace(cfg.Provider))
	params := cfg.Params

	switch providerID {
	case "noop":
		e.telemetry = telemetry.NewNoop()
		return nil
	case "langfuse":
		publicKey := firstString(params, "public_key")
		secretKey := firstString(params, "secret_key")
		endpoint := firstString(params, "endpoint")
		serviceName := firstString(params, "service_name")
		timeout, err := durationParam(params, "timeout")
		if err != nil {
			return err
		}
		client, err := langfuse.NewClient(langfuse.Config{
			PublicKey:   publicKey,
			SecretKey:   secretKey,
			Endpoint:    endpoint,
			ServiceName: serviceName,
			Timeout:     timeout,
		})
		if err != nil {
			return err
		}
		e.telemetry = client
		return nil
	default:
		return fmt.Errorf("unknown telemetry provider %q", providerID)
	}
}

func firstString(params map[string]any, keys ...string) string {
	for _, key := range keys {
		if params == nil {
			return ""
		}
		if value, ok := params[key]; ok {
			if text, ok := value.(string); ok {
				if strings.TrimSpace(text) != "" {
					return text
				}
			}
		}
	}
	return ""
}

func boolParam(params map[string]any, key string) (bool, bool) {
	if params == nil {
		return false, false
	}
	value, ok := params[key]
	if !ok {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

func durationParam(params map[string]any, key string) (time.Duration, error) {
	if params == nil {
		return 0, nil
	}
	value, ok := params[key]
	if !ok {
		return 0, nil
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		return time.ParseDuration(v)
	case float64:
		if v <= 0 {
			return 0, nil
		}
		return time.Duration(v * float64(time.Second)), nil
	case int:
		if v <= 0 {
			return 0, nil
		}
		return time.Duration(v) * time.Second, nil
	case int64:
		if v <= 0 {
			return 0, nil
		}
		return time.Duration(v) * time.Second, nil
	default:
		return 0, fmt.Errorf("invalid duration type %T", value)
	}
}

func intParam(params map[string]any, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	value, ok := params[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func (e *Engine) applyProviderRetryConfig(cfg config.ProviderRetryConfig) {
	if cfg.Enabled != nil {
		e.providerRetryEnabled = *cfg.Enabled
	}
	if cfg.MaxRetries != 0 || cfg.InitialInterval != 0 || cfg.MaxInterval != 0 || cfg.Multiplier != 0 {
		e.providerRetryConfig = provider.ResilientConfig{
			MaxRetries:      cfg.MaxRetries,
			InitialInterval: cfg.InitialInterval,
			MaxInterval:     cfg.MaxInterval,
			Multiplier:      cfg.Multiplier,
		}
	}
}

func (e *Engine) applyContextConfig(cfg config.ContextConfig) error {
	enabled, configured := autoSummarizeEnabled(cfg.AutoSummarize)
	if !configured {
		return nil
	}

	base := e.contextPolicy
	if auto, ok := base.(*contextpolicy.AutoSummarizePolicy); ok {
		base = auto.Base()
	}

	if !enabled {
		if base == nil {
			base = contextpolicy.NewDefaultPolicy()
		}
		e.contextPolicy = base
		return nil
	}

	autos := contextpolicy.AutoSummarizeConfig{
		SummarizeAtTokens: cfg.AutoSummarize.SummarizeAtTokens,
		MinKeepMessages:   cfg.AutoSummarize.MinKeepMessages,
	}
	if autos.SummarizeAtTokens <= 0 {
		autos.SummarizeAtTokens = defaultAutoSummarizeTokens
	}
	if base == nil {
		base = contextpolicy.NewDefaultPolicy()
	}
	e.contextPolicy = contextpolicy.NewAutoSummarizePolicy(base, autos)
	return nil
}

func (e *Engine) applyToolConfig(toolCfg map[string]config.ToolConfig) error {
	if len(toolCfg) == 0 {
		return nil
	}
	if e.toolFactories == nil {
		return fmt.Errorf("tool factories required for tool config")
	}
	for id, cfg := range toolCfg {
		factory, ok := e.toolFactories.Get(id)
		if !ok {
			return fmt.Errorf("tool factory not found: %s", id)
		}
		toolInst, err := factory.Build(cfg.Params)
		if err != nil {
			return fmt.Errorf("tool %s: %w", id, err)
		}
		e.tools.Register(toolInst)
	}
	return nil
}

func autoSummarizeEnabled(cfg config.AutoSummarizeConfig) (bool, bool) {
	if cfg.Enabled != nil {
		return *cfg.Enabled, true
	}
	if cfg.SummarizeAtTokens > 0 || cfg.MinKeepMessages > 0 {
		return true, true
	}
	return false, false
}

// AgentFromConfig builds an agent from the stored configuration.
func (e *Engine) AgentFromConfig(id string) (agent.Agent, error) {
	cfg, ok := e.fullConfig.Agents[id]
	if !ok {
		return agent.Agent{}, fmt.Errorf("%w: agent %s", ErrConfigNotFound, id)
	}

	name := cfg.Name
	if name == "" {
		name = id
	}

	out := agent.Agent{
		Name:       name,
		Tools:      append([]string(nil), cfg.Tools...),
		Params:     cfg.Params,
		ProviderID: cfg.ProviderID,
		ProfileID:  cfg.ProfileID,
	}

	if cfg.ProfileID != "" {
		profile, ok := e.profiles.Get(cfg.ProfileID)
		if !ok {
			return agent.Agent{}, fmt.Errorf("%w: %s", ErrProfileNotFound, cfg.ProfileID)
		}
		out.Profile = &profile
	}

	return out, nil
}

// SessionFromConfig returns a session builder configured from stored config.
func (e *Engine) SessionFromConfig(id string) (*SessionBuilder, error) {
	cfg, ok := e.fullConfig.Sessions[id]
	if !ok {
		return nil, fmt.Errorf("%w: session %s", ErrConfigNotFound, id)
	}

	protoID := cfg.ProtocolID
	if protoID == "" {
		return nil, fmt.Errorf("protocol_id required for session %s", id)
	}
	factory, ok := e.protocols.Get(protoID)
	if !ok {
		return nil, fmt.Errorf("protocol not found: %s", protoID)
	}
	proto := factory(protocol.Config(cfg.Params))

	name := cfg.Name
	if name == "" {
		name = id
	}

	builder := e.Session(name).Protocol(proto)
	if len(cfg.Tags) > 0 {
		builder.Tags(cfg.Tags...)
	}
	for key, value := range cfg.Metadata {
		builder.Metadata(key, value)
	}

	participants := make([]protocol.Participant, 0, len(cfg.Participants))
	for _, participantID := range cfg.Participants {
		participant, err := e.AgentFromConfig(participantID)
		if err != nil {
			return nil, err
		}
		participants = append(participants, participant)
	}
	if len(participants) > 0 {
		builder.Participants(participants...)
	}

	if cfg.Facilitator != "" {
		facilitator, err := e.AgentFromConfig(cfg.Facilitator)
		if err != nil {
			return nil, err
		}
		builder.Facilitator(facilitator)
	}

	if len(cfg.Groups) > 0 {
		groups := make(map[string][]protocol.Participant, len(cfg.Groups))
		for groupName, ids := range cfg.Groups {
			members := make([]protocol.Participant, 0, len(ids))
			for _, memberID := range ids {
				member, err := e.AgentFromConfig(memberID)
				if err != nil {
					return nil, err
				}
				members = append(members, member)
			}
			groups[groupName] = members
		}
		builder.Groups(groups)
	}

	if cfg.TimeoutPolicy != nil {
		policy, err := timeoutPolicyFromConfig(cfg.TimeoutPolicy)
		if err != nil {
			return nil, err
		}
		builder.TimeoutPolicy(policy)
	}

	return builder, nil
}

func timeoutPolicyFromConfig(cfg *config.TimeoutPolicyConfig) (TimeoutPolicy, error) {
	if cfg == nil {
		return TimeoutPolicy{}, fmt.Errorf("timeout policy required")
	}
	strategy := strings.ToLower(strings.TrimSpace(cfg.Strategy))
	switch TimeoutStrategy(strategy) {
	case TimeoutContinue:
		if strings.TrimSpace(cfg.Note) == "" {
			return TimeoutPolicy{}, fmt.Errorf("timeout policy note required")
		}
		return ContinueWithNote(cfg.Note), nil
	case TimeoutRetry:
		if strings.TrimSpace(cfg.RetryParticipant) == "" {
			return TimeoutPolicy{}, fmt.Errorf("timeout policy retry_participant required")
		}
		policy := RetryWith(cfg.RetryParticipant)
		if strings.TrimSpace(cfg.Note) != "" {
			policy.Note = cfg.Note
		}
		return policy, nil
	case TimeoutFail:
		return MarkIncomplete(), nil
	case "":
		return TimeoutPolicy{}, fmt.Errorf("timeout policy strategy required")
	default:
		return TimeoutPolicy{}, fmt.Errorf("unknown timeout policy %q", cfg.Strategy)
	}
}
