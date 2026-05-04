package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/contextpolicy"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime/compat"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/telemetry"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

var (
	// ErrProviderNotFound indicates a missing provider.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrModelRequired indicates a missing model parameter.
	ErrModelRequired = errors.New("model required")
	// ErrProfileNotFound indicates a missing agent profile.
	ErrProfileNotFound = errors.New("profile not found")
	// ErrToolsNotFound indicates unknown tool IDs.
	ErrToolsNotFound = errors.New("tools not found")
	// ErrToolsNotAllowed indicates tool IDs blocked by policy.
	ErrToolsNotAllowed = errors.New("tools not allowed")
	// ErrToolIterationsExceeded indicates tool loop exhaustion.
	ErrToolIterationsExceeded = errors.New("tool iterations exceeded")
	// ErrTurnBlocked indicates a turn was blocked by a hook.
	ErrTurnBlocked = errors.New("turn blocked by hook")
)

const defaultMaxToolIterations = 4

// RunAgent executes an agent against the provider and streams events.
func (s *Session) RunAgent(ctx context.Context, a agent.Agent, req protocol.RunRequest) (agent.Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(req.Messages) == 0 {
		return agent.Message{}, fmt.Errorf("messages required")
	}
	if req.MaxRunDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.MaxRunDuration)
		defer cancel()
	}

	meta := hook.SessionMeta{SessionID: s.id, ProtocolID: s.protocol.ID()}
	turn := hook.Turn{Agent: a, Request: req}
	decision, updatedTurn, interrupts, err := s.applyPreTurnHooks(ctx, meta, turn)
	if err != nil {
		return agent.Message{}, err
	}
	if decision == hook.Block {
		return agent.Message{}, ErrTurnBlocked
	}
	if decision == hook.Modify {
		turn = updatedTurn
	}

	if len(interrupts) > 0 {
		turn.Request.Messages = append(turn.Request.Messages, interruptMessages(interrupts)...)
		s.emitTurnInterrupts(interrupts)
	}

	req = turn.Request

	// Resolve model
	model := resolveModel(s.engine.cfg, a, req)
	if model == "" {
		return agent.Message{}, ErrModelRequired
	}

	// Resolve provider from model or agent-level override
	p, providerID, resolvedModel, err := s.engine.resolveProvider(model, a.ProviderID)
	if err != nil {
		return agent.Message{}, err
	}
	model = resolvedModel

	profile, err := s.resolveProfile(a)
	if err != nil {
		return agent.Message{}, err
	}

	// Merge tools: profile.Tools + agent.Tools + req.Tools + session.DefaultTools
	toolIDs := mergeStrings(profile.Tools, a.Tools, req.Tools, s.DefaultTools())
	toolPolicy := s.resolveToolPolicy(req.ToolPolicy)
	toolDefs, allowedToolIDs, err := s.resolveTools(toolIDs, toolPolicy)
	if err != nil {
		return agent.Message{}, err
	}

	params := mergeParams(s.engine.cfg.Defaults.Params, a.Params, req.Params)
	delete(params, "model")

	// Resolve output schema: req.OutputSchema overrides agent.OutputSchema
	outputSchema := req.OutputSchema
	if outputSchema == nil {
		outputSchema = a.OutputSchema
	}
	if outputSchema != nil {
		schemaJSON, err := deriveOutputSchema(outputSchema)
		if err != nil {
			return agent.Message{}, fmt.Errorf("derive output schema: %w", err)
		}
		// OpenAI unified API structured output format
		// Gets wrapped by client into: text: { format: {...} }
		params["response_format"] = map[string]any{
			"type":   "json_schema",
			"name":   "response",
			"strict": true,
			"schema": schemaJSON,
		}
	}

	var tokenEstimator contextpolicy.TokenEstimator
	if estimator, ok := p.(provider.TokenEstimator); ok {
		tokenEstimator = estimator
	}

	span, spanCtx := s.engine.telemetry.StartSpan(ctx, telemetry.SpanInput{
		Name: "agent.run",
		Attributes: map[string]any{
			"agent_name":  a.Name,
			"provider_id": providerID,
			"session_id":  s.id,
			"protocol_id": s.protocol.ID(),
			"model":       model,
		},
	})
	var runErr error
	defer func() {
		span.End(runErr)
	}()

	maxIterations := req.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = defaultMaxToolIterations
	}

	systemMessages := buildSystemMessages(profile, req.SystemMessages)
	history := append([]agent.Message(nil), req.Messages...)
	contextCfg := resolveContextConfig(s.engine.cfg.Context, req.Context)
	for attempt := 0; attempt <= maxIterations; attempt++ {
		perspectiveHistory := applyAgentPerspective(history, a.Name, s.engine.agentPerspectiveMode)
		selected, err := s.engine.contextPolicy.Select(spanCtx, buildContextInput(model, systemMessages, perspectiveHistory, contextCfg, s.engine.contextSummarizer, tokenEstimator))
		if err != nil {
			runErr = err
			return agent.Message{}, err
		}
		message, toolCalls, err := s.runProviderStream(spanCtx, p, provider.Request{
			Model:    model,
			Messages: toRuntimeMessages(selected),
			Tools:    compat.FromToolDefinitions(toolDefs),
			Params:   params,
		}, a.Name, span, req.Silent)
		if err != nil {
			runErr = err
			return agent.Message{}, err
		}
		if len(toolCalls) == 0 {
			return s.applyPostTurnHooksAndReturn(ctx, meta, a, req, message)
		}
		if attempt == maxIterations {
			runErr = ErrToolIterationsExceeded
			return agent.Message{}, ErrToolIterationsExceeded
		}

		// For Responses API: append assistant message to history before tool results
		if message.Role != "" || len(message.Parts) > 0 || message.Text() != "" {
			if message.Role == "" {
				message.Role = agent.RoleAssistant
			}
			history = append(history, message)
		}

		results, err := s.executeToolCalls(spanCtx, a.Name, toolCalls, allowedToolIDs)
		if err != nil {
			if awaiting := toolAwaiting(err); awaiting != nil {
				history = append(history, toolMessagesFromResults(results)...)
				reqCopy := req
				reqCopy.OutputSchema = nil
				continuation := toolContinuation{
					Agent:          a,
					Request:        reqCopy,
					ToolIDs:        toolIDs,
					History:        append([]agent.Message(nil), history...),
					SystemMessages: append([]agent.Message(nil), systemMessages...),
					Params:         cloneMetadata(params),
					Model:          model,
					Context:        contextCfg,
					MaxIterations:  maxIterations,
					NextAttempt:    attempt + 1,
				}
				if err := s.storePendingTool(spanCtx, awaiting.Request, continuation); err != nil {
					runErr = err
					return agent.Message{}, err
				}
				runErr = err
				return agent.Message{}, err
			}
			runErr = err
			return agent.Message{}, err
		}
		history = append(history, toolMessagesFromResults(results)...)

	}

	runErr = ErrToolIterationsExceeded
	return agent.Message{}, ErrToolIterationsExceeded
}

func (s *Session) runProviderStream(ctx context.Context, p provider.Provider, req provider.Request, agentID string, span telemetry.Span, silent bool) (agent.Message, []provider.ToolCall, error) {
	var stream provider.Stream
	if s.engine.providerRetryEnabled {
		stream = provider.NewResilientStream(ctx, func(ctx context.Context) (provider.Stream, error) {
			return p.Stream(ctx, req)
		}, s.engine.providerRetryConfig)
	} else {
		base, err := p.Stream(ctx, req)
		if err != nil {
			return agent.Message{}, nil, err
		}
		stream = base
	}
	defer func() {
		_ = stream.Close()
	}()

	var builder strings.Builder
	var lastMessage agent.Message
	toolCalls := make([]provider.ToolCall, 0)
	emit := func(ev event.Event) {
		if silent {
			return
		}
		_ = s.EmitWithContext(ctx, ev)
	}

	for {
		provEvent, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return agent.Message{}, nil, err
		}
		if provEvent.Err != nil {
			return agent.Message{}, nil, provEvent.Err
		}

		switch provEvent.Type {
		case provider.EventMessageDelta:
			builder.WriteString(provEvent.Delta)
			ev := event.New(event.AgentMessageDelta, s.id, map[string]any{
				"delta": provEvent.Delta,
			})
			ev.AgentID = agentID
			emit(ev)
			span.AddEvent(string(event.AgentMessageDelta), llmEventAttrs(s.id, agentID, event.AgentMessageDelta, provEvent.Delta))
		case provider.EventReasoningDelta:
			ev := event.New(event.AgentReasoningDelta, s.id, map[string]any{
				"delta": provEvent.Delta,
			})
			ev.AgentID = agentID
			emit(ev)
			span.AddEvent(string(event.AgentReasoningDelta), llmEventAttrs(s.id, agentID, event.AgentReasoningDelta, provEvent.Delta))
		case provider.EventReasoningSummaryDelta:
			ev := event.New(event.AgentReasoningSummaryDelta, s.id, map[string]any{
				"delta": provEvent.Delta,
			})
			ev.AgentID = agentID
			emit(ev)
			span.AddEvent(string(event.AgentReasoningSummaryDelta), llmEventAttrs(s.id, agentID, event.AgentReasoningSummaryDelta, provEvent.Delta))
		case provider.EventMessageCompleted:
			if len(provEvent.Message.Parts) == 0 {
				text := builder.String()
				if text != "" {
					provEvent.Message.Parts = []modelruntime.Part{{Type: modelruntime.PartText, Text: text}}
				}
			}
			if provEvent.Message.Role == "" {
				provEvent.Message.Role = modelruntime.RoleAssistant
			}
			lastMessage = compat.ToAgentMessage(provEvent.Message)
			ev := event.New(event.AgentMessageComplete, s.id, map[string]any{
				"message": lastMessage,
			})
			ev.AgentID = agentID
			emit(ev)
			span.AddEvent(string(event.AgentMessageComplete), messageCompleteAttrs(s.id, agentID, lastMessage))
		case provider.EventToolCall:
			span.AddEvent(string(provider.EventToolCall), providerToolCallAttrs(s.id, agentID, provEvent.ToolCalls))
			toolCalls = append(toolCalls, provEvent.ToolCalls...)
		case provider.EventToolResult:
			return agent.Message{}, nil, fmt.Errorf("provider tool results are not supported")
		case provider.EventRaw:
			if len(provEvent.Raw) > 0 {
				ev := event.New(event.ProviderRawEvent, s.id, map[string]any{
					"raw": provEvent.Raw,
				})
				ev.AgentID = agentID
				emit(ev)
				span.AddEvent(string(event.ProviderRawEvent), rawProviderEventAttrs(s.id, agentID, provEvent.Raw))
			}
		case provider.EventError:
			if provEvent.Err != nil {
				return agent.Message{}, nil, provEvent.Err
			}
			return agent.Message{}, nil, fmt.Errorf("provider error")
		}
	}

	if len(lastMessage.Parts) == 0 {
		text := builder.String()
		if text != "" {
			lastMessage = agent.Message{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: text}}}
		}
	}

	return lastMessage, toolCalls, nil
}

func toRuntimeMessages(messages []agent.Message) []modelruntime.Message {
	out := make([]modelruntime.Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, compat.FromAgentMessage(message))
	}
	return out
}

func (s *Session) resolveTools(toolIDs []string, policy tool.Policy) ([]tool.Definition, map[string]struct{}, error) {
	if len(toolIDs) == 0 {
		return nil, map[string]struct{}{}, nil
	}

	defs := make([]tool.Definition, 0, len(toolIDs))
	allowed := make(map[string]struct{}, len(toolIDs))
	missing := make([]string, 0)
	denied := make([]string, 0)
	for _, id := range toolIDs {
		// Try session registry first, then engine registry
		var t tool.Tool
		var ok bool
		if s.tools != nil {
			t, ok = s.tools.Get(id)
		}
		if !ok {
			t, ok = s.engine.tools.Get(id)
		}
		if !ok {
			missing = append(missing, id)
			continue
		}
		def := tool.DefinitionFromTool(t)
		if !policy.Allows(def.ID, def.Tags) {
			denied = append(denied, def.ID)
			continue
		}
		defs = append(defs, def)
		allowed[def.ID] = struct{}{}
	}

	if len(missing) > 0 {
		return nil, nil, fmt.Errorf("%w: %v", ErrToolsNotFound, missing)
	}
	if len(defs) == 0 && len(denied) > 0 {
		return nil, nil, fmt.Errorf("%w: %v", ErrToolsNotAllowed, denied)
	}

	return defs, allowed, nil
}

func (s *Session) applyPostTurnHooksAndReturn(ctx context.Context, meta hook.SessionMeta, a agent.Agent, req protocol.RunRequest, message agent.Message) (agent.Message, error) {
	result := hook.TurnResult{Agent: a, Request: req, Response: message}
	decision, updatedResult, interrupts, err := s.applyPostTurnHooks(ctx, meta, result)
	if err != nil {
		return agent.Message{}, err
	}
	if decision == hook.Block {
		return agent.Message{}, ErrTurnBlocked
	}
	if decision == hook.Modify {
		result = updatedResult
	}
	if len(interrupts) > 0 {
		s.emitTurnInterrupts(interrupts)
	}
	return result.Response, nil
}

func (s *Session) applyPreTurnHooks(ctx context.Context, meta hook.SessionMeta, turn hook.Turn) (hook.Decision, hook.Turn, []hook.Interrupt, error) {
	decision := hook.Allow
	modified := turn
	var interrupts []hook.Interrupt
	for _, h := range s.engine.hooks.PreTurn() {
		d, newTurn, newInterrupts, err := h.OnPreTurn(ctx, meta, modified)
		if err != nil {
			return hook.Block, modified, nil, err
		}
		if len(newInterrupts) > 0 {
			interrupts = append(interrupts, newInterrupts...)
		}
		if d == hook.Block {
			_ = s.EmitWithContext(ctx, event.New(event.HookBlocked, s.id, map[string]any{"hook": h.ID()}))
			return hook.Block, modified, interrupts, nil
		}
		if d == hook.Modify {
			modified = newTurn
			decision = hook.Modify
			_ = s.EmitWithContext(ctx, event.New(event.HookModified, s.id, map[string]any{"hook": h.ID()}))
		}
	}
	return decision, modified, interrupts, nil
}

func (s *Session) applyPostTurnHooks(ctx context.Context, meta hook.SessionMeta, result hook.TurnResult) (hook.Decision, hook.TurnResult, []hook.Interrupt, error) {
	decision := hook.Allow
	modified := result
	var interrupts []hook.Interrupt
	for _, h := range s.engine.hooks.PostTurn() {
		d, newResult, newInterrupts, err := h.OnPostTurn(ctx, meta, modified)
		if err != nil {
			return hook.Block, modified, nil, err
		}
		if len(newInterrupts) > 0 {
			interrupts = append(interrupts, newInterrupts...)
		}
		if d == hook.Block {
			_ = s.EmitWithContext(ctx, event.New(event.HookBlocked, s.id, map[string]any{"hook": h.ID()}))
			return hook.Block, modified, interrupts, nil
		}
		if d == hook.Modify {
			modified = newResult
			decision = hook.Modify
			_ = s.EmitWithContext(ctx, event.New(event.HookModified, s.id, map[string]any{"hook": h.ID()}))
		}
	}
	return decision, modified, interrupts, nil
}

func (s *Session) emitTurnInterrupts(interrupts []hook.Interrupt) {
	for _, interrupt := range interrupts {
		msg := interrupt.Message
		agentID := msg.Name
		if agentID == "" {
			agentID = "hook"
		}
		ev := event.New(event.AgentMessageComplete, s.id, map[string]any{"message": msg})
		ev.AgentID = agentID
		_ = s.Emit(ev)
	}
}

func interruptMessages(interrupts []hook.Interrupt) []agent.Message {
	out := make([]agent.Message, 0, len(interrupts))
	for _, interrupt := range interrupts {
		out = append(out, interrupt.Message)
	}
	return out
}

func resolveModel(cfg config.GlobalConfig, a agent.Agent, req protocol.RunRequest) string {
	// Priority: request params > agent.Model > agent.Params["model"] > global defaults
	if req.Params != nil {
		if v, ok := req.Params["model"]; ok {
			if model, ok := v.(string); ok {
				return model
			}
		}
	}
	if a.Model != "" {
		return a.Model
	}
	if a.Params != nil {
		if v, ok := a.Params["model"]; ok {
			if model, ok := v.(string); ok {
				return model
			}
		}
	}
	if cfg.Defaults.Params != nil {
		if v, ok := cfg.Defaults.Params["model"]; ok {
			if model, ok := v.(string); ok {
				return model
			}
		}
	}
	return ""
}

func mergeParams(params ...map[string]any) map[string]any {
	merged := make(map[string]any)
	for _, p := range params {
		for key, value := range p {
			merged[key] = value
		}
	}
	return merged
}

func mergeStrings(values ...[]string) []string {
	seen := make(map[string]struct{})
	merged := make([]string, 0)
	for _, list := range values {
		for _, value := range list {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			merged = append(merged, value)
		}
	}
	return merged
}

func (s *Session) resolveProfile(a agent.Agent) (agent.Profile, error) {
	if a.Profile != nil {
		return *a.Profile, nil
	}
	if a.ProfileID != "" {
		if s.engine != nil && s.engine.profiles != nil {
			if profile, ok := s.engine.profiles.Get(a.ProfileID); ok {
				return profile, nil
			}
		}
		return agent.Profile{}, fmt.Errorf("%w: %s", ErrProfileNotFound, a.ProfileID)
	}
	return agent.Profile{}, nil
}

// deriveOutputSchema generates a JSON schema for structured output.
func deriveOutputSchema(schema any) (map[string]any, error) {
	if schema == nil {
		return nil, fmt.Errorf("output schema cannot be nil")
	}

	typ := reflect.TypeOf(schema)
	if typ == nil {
		return nil, fmt.Errorf("unable to determine type of output schema")
	}

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("output schema must be a struct, got %s", typ.Kind())
	}

	// Derive JSON schema using tool package logic
	schemaBytes, err := tool.SchemaForStruct(typ)
	if err != nil {
		return nil, fmt.Errorf("derive schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}

	enforceStrictSchema(schemaMap)

	return schemaMap, nil
}

func enforceStrictSchema(node any) {
	switch v := node.(type) {
	case map[string]any:
		enforceStrictSchemaMap(v)
	case []any:
		for _, item := range v {
			enforceStrictSchema(item)
		}
	}
}

func enforceStrictSchemaMap(schema map[string]any) {
	if schema == nil {
		return
	}
	var props map[string]any
	if schemaType, ok := schema["type"].(string); ok && schemaType == "object" {
		if rawProps, hasProps := schema["properties"].(map[string]any); hasProps {
			props = rawProps
			if _, ok := schema["additionalProperties"]; !ok {
				schema["additionalProperties"] = false
			}
			ensureRequiredAllProperties(schema, props)
		}
	}

	if props != nil {
		for _, prop := range props {
			enforceStrictSchema(prop)
		}
	}
	if items, ok := schema["items"]; ok {
		enforceStrictSchema(items)
	}
	if additional, ok := schema["additionalProperties"]; ok {
		switch nested := additional.(type) {
		case map[string]any, []any:
			enforceStrictSchema(nested)
		}
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if variants, ok := schema[key].([]any); ok {
			for _, variant := range variants {
				enforceStrictSchema(variant)
			}
		}
	}
}

func ensureRequiredAllProperties(schema map[string]any, props map[string]any) {
	if len(props) == 0 {
		return
	}
	requiredSet := make(map[string]struct{}, len(props))
	switch raw := schema["required"].(type) {
	case []string:
		for _, item := range raw {
			requiredSet[item] = struct{}{}
		}
	case []any:
		for _, item := range raw {
			if str, ok := item.(string); ok {
				requiredSet[str] = struct{}{}
			}
		}
	}

	if len(requiredSet) != len(props) {
		required := make([]string, 0, len(props))
		for key := range props {
			required = append(required, key)
		}
		sort.Strings(required)
		schema["required"] = required
		return
	}

	for key := range props {
		if _, ok := requiredSet[key]; !ok {
			required := make([]string, 0, len(props))
			for propKey := range props {
				required = append(required, propKey)
			}
			sort.Strings(required)
			schema["required"] = required
			return
		}
	}
}
