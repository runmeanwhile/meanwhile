package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/memory"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

const (
	MemoryAutomationPromptKey  = "memory_automation_prompt"
	MemoryAutomationEnabledKey = "memory_automation_enabled"
	MemoryAutomationSubjectKey = "memory_automation_subject"
)

const defaultMemoryAutomationTimeout = 20 * time.Second

const DefaultMemoryAutomationPrompt = `You are a memory extractor.
Write a concise memory string that captures durable facts, preferences, and key findings.
Use newline-separated "key: value" when helpful, otherwise short sentences.
Rules:
- Do NOT store secrets, API keys, or credentials.
- Skip ephemeral details unless the user says they are important.
- Be brief and precise.
- Return plain text only.`

// MemoryAutomator captures a session's memory on close.
type MemoryAutomator interface {
	Capture(ctx context.Context, sess *Session) error
}

// WithMemoryAutomator sets a custom memory automator.
func WithMemoryAutomator(automator MemoryAutomator) Option {
	return func(e *Engine) error {
		e.memoryAutomator = automator
		e.memoryAutomationDisabled = automator == nil
		return nil
	}
}

// WithMemoryAutomation configures the default memory automator.
func WithMemoryAutomation(cfg config.MemoryAutomationConfig) Option {
	return func(e *Engine) error {
		if !cfg.Enabled {
			e.memoryAutomationDisabled = true
			return nil
		}
		e.memoryAutomator = &engineMemoryAutomator{engine: e, cfg: cfg}
		e.memoryAutomationDisabled = false
		return nil
	}
}

type engineMemoryAutomator struct {
	engine *Engine
	cfg    config.MemoryAutomationConfig
}

func (a *engineMemoryAutomator) Capture(ctx context.Context, sess *Session) error {
	if a == nil || a.engine == nil || sess == nil {
		return nil
	}
	if !a.cfg.Enabled {
		return nil
	}
	if sess.memory == nil {
		return nil
	}
	if disabledFromMetadata(sess.metadata) {
		return nil
	}

	ctx, cancel := a.withTimeout(ctx)
	defer cancel()

	prompt := resolveMemoryPrompt(sess.metadata, a.cfg.Prompt)
	subject := resolveMemorySubject(sess.metadata)

	messageTypes := []event.Type{event.AgentMessageComplete}
	if a.cfg.Context.IncludeToolResults {
		messageTypes = append(messageTypes, event.ToolCallCompleted)
	}

	contextOpts := []memory.ContextOption{
		memory.WithMessageTypes(messageTypes...),
	}
	if a.cfg.Context.RecentMessages > 0 {
		contextOpts = append(contextOpts, memory.WithRecent(a.cfg.Context.RecentMessages))
	}
	if a.cfg.Context.TokenLimit > 0 {
		contextOpts = append(contextOpts, memory.WithTokenLimit(a.cfg.Context.TokenLimit))
	}

	conversation, err := memory.BuildConversationContext(ctx, sess.memory, sess.id, contextOpts...)
	if err != nil {
		return fmt.Errorf("memory automation context: %w", err)
	}
	if len(conversation) == 0 {
		return nil
	}

	lastMessageTime, err := lastRelevantEventTime(ctx, sess.memory, sess.id, messageTypes)
	if err != nil {
		return fmt.Errorf("memory automation last message: %w", err)
	}
	if lastMessageTime.IsZero() {
		return nil
	}

	summaryType := resolveSummaryEventType(a.cfg.StoreEvent)
	lastSummaryTime, err := lastSummaryEventTime(ctx, sess.memory, sess.id, summaryType)
	if err != nil {
		return fmt.Errorf("memory automation last summary: %w", err)
	}
	if !lastSummaryTime.IsZero() && !lastSummaryTime.Before(lastMessageTime) {
		return nil
	}

	model := resolveMemoryModel(a.engine.cfg, a.cfg)
	if model == "" {
		return errors.New("memory automation requires a model")
	}

	providerID := a.cfg.ProviderID
	if providerID == "" {
		providerID = a.engine.defaultProvider
	}
	p, ok := a.engine.providers.Get(providerID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, providerID)
	}

	messages := buildMemoryAutomationMessages(prompt, subject, conversation)
	params := mergeParams(a.engine.cfg.Defaults.Params, a.cfg.Params)

	resp, err := runProvider(ctx, p, provider.Request{
		Model:    model,
		Messages: messages,
		Params:   params,
	}, a.engine.providerRetryEnabled, a.engine.providerRetryConfig)
	if err != nil {
		return fmt.Errorf("memory automation provider: %w", err)
	}

	memoryText := strings.TrimSpace(resp.Text())
	if memoryText == "" {
		return nil
	}

	summaryEvent := event.New(summaryType, sess.id, memoryText)
	if subject != "" {
		summaryEvent.AgentID = subject
	}
	if sess.protocol != nil {
		summaryEvent.ProtocolID = sess.protocol.ID()
	}
	_ = sess.Emit(summaryEvent)

	return nil
}

func (a *engineMemoryAutomator) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	base := ctx
	if base == nil || base.Err() != nil {
		base = context.Background()
	}
	timeout := defaultMemoryAutomationTimeout
	if a.cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(a.cfg.TimeoutSeconds) * time.Second
	}
	return context.WithTimeout(base, timeout)
}

func resolveMemoryPrompt(metadata map[string]any, configured string) string {
	if prompt, ok := metadata[MemoryAutomationPromptKey].(string); ok {
		if strings.TrimSpace(prompt) != "" {
			return prompt
		}
	}
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	return DefaultMemoryAutomationPrompt
}

func resolveMemorySubject(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[MemoryAutomationSubjectKey].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func disabledFromMetadata(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	if value, ok := metadata[MemoryAutomationEnabledKey]; ok {
		if enabled, ok := value.(bool); ok {
			return !enabled
		}
	}
	return false
}

func resolveMemoryModel(cfg config.GlobalConfig, automation config.MemoryAutomationConfig) string {
	if automation.Model != "" {
		return automation.Model
	}
	if cfg.Defaults.Params != nil {
		if value, ok := cfg.Defaults.Params["model"]; ok {
			if model, ok := value.(string); ok {
				return model
			}
		}
	}
	return ""
}

func resolveSummaryEventType(value string) event.Type {
	if strings.TrimSpace(value) == "" {
		return event.MemorySummary
	}
	return event.Type(value)
}

func buildMemoryAutomationMessages(prompt, subject string, conversation []agent.Message) []agent.Message {
	messages := make([]agent.Message, 0, len(conversation)+3)
	messages = append(messages, textMessage(agent.RoleSystem, prompt))
	if subject != "" {
		messages = append(messages, textMessage(agent.RoleSystem, "Focus on memory about: "+subject))
	}
	messages = append(messages, conversation...)
	messages = append(messages, textMessage(agent.RoleUser, "Write the memory now."))
	return messages
}

func lastRelevantEventTime(ctx context.Context, store memory.Store, sessionID string, types []event.Type) (time.Time, error) {
	items, err := store.Query(ctx, memory.Query{SessionID: sessionID, Types: types})
	if err != nil {
		return time.Time{}, err
	}
	return latestEventTime(items), nil
}

func lastSummaryEventTime(ctx context.Context, store memory.Store, sessionID string, summaryType event.Type) (time.Time, error) {
	items, err := store.Query(ctx, memory.Query{SessionID: sessionID, Types: []event.Type{summaryType}})
	if err != nil {
		return time.Time{}, err
	}
	return latestEventTime(items), nil
}

func latestEventTime(items []memory.Item) time.Time {
	var latest time.Time
	for _, item := range items {
		if item.Event.Time.After(latest) {
			latest = item.Event.Time
		}
	}
	return latest
}

func runProvider(ctx context.Context, p provider.Provider, req provider.Request, retryEnabled bool, retryCfg provider.ResilientConfig) (agent.Message, error) {
	var stream provider.Stream
	if retryEnabled {
		stream = provider.NewResilientStream(ctx, func(ctx context.Context) (provider.Stream, error) {
			return p.Stream(ctx, req)
		}, retryCfg)
	} else {
		base, err := p.Stream(ctx, req)
		if err != nil {
			return agent.Message{}, err
		}
		stream = base
	}
	defer func() {
		_ = stream.Close()
	}()

	var sb strings.Builder
	var final agent.Message

	for {
		ev, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return agent.Message{}, err
		}

		switch ev.Type {
		case provider.EventMessageDelta:
			sb.WriteString(ev.Delta)
		case provider.EventMessageCompleted:
			final = ev.Message
		case provider.EventError:
			if ev.Err != nil {
				return agent.Message{}, ev.Err
			}
			return agent.Message{}, errors.New("provider error")
		case provider.EventToolCall:
			return agent.Message{}, errors.New("unexpected tool call during memory automation")
		default:
			continue
		}
	}

	if strings.TrimSpace(final.Text()) == "" && len(final.Parts) == 0 {
		if sb.Len() == 0 {
			return agent.Message{}, errors.New("empty memory automation response")
		}
		final = textMessage(agent.RoleAssistant, sb.String())
	}

	return final, nil
}

func textMessage(role agent.Role, text string) agent.Message {
	return agent.Message{
		Role:  role,
		Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: text}},
	}
}
