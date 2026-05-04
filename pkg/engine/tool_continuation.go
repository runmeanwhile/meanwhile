package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/contextpolicy"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/telemetry"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

type toolContinuation struct {
	Agent          agent.Agent
	Request        protocol.RunRequest
	ToolIDs        []string
	History        []agent.Message
	SystemMessages []agent.Message
	Params         map[string]any
	Model          string
	Context        resolvedContextConfig
	MaxIterations  int
	NextAttempt    int
}

type pendingTool struct {
	request      tool.Request
	continuation toolContinuation
}

// ResumeTool delivers a tool result and resumes the pending run.
func (s *Session) ResumeTool(ctx context.Context, requestID string, result tool.Result) (*RunResult, error) {
	if requestID == "" {
		return nil, fmt.Errorf("request id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()

	pending, ok := s.pendingTool(requestID)
	if !ok {
		return nil, ErrRequestNotFound
	}

	_, _ = s.removePendingTool(requestID)

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
		Name: "session.resume_tool",
		Attributes: map[string]any{
			"session_id":  s.id,
			"protocol_id": s.protocol.ID(),
		},
	})
	var runErr error
	defer func() {
		traceSpan.End(runErr)
	}()

	if result.ID == "" {
		result.ID = pending.request.ToolCallID
	}
	if result.ToolID == "" {
		result.ToolID = pending.request.ToolID
	}

	toolMsg := toolMessageFromResult(result)
	history := append([]agent.Message(nil), pending.continuation.History...)
	history = append(history, toolMsg)
	pending.continuation.History = history

	_ = s.EmitWithContext(traceCtx, event.New(event.ToolCallCompleted, s.id, map[string]any{"result": result}))
	_ = s.EmitWithContext(traceCtx, event.New(event.AgentMessageComplete, s.id, map[string]any{"message": toolMsg}))

	if s.engine != nil {
		_ = s.engine.persistSessionState(traceCtx, s)
	}

	resp, err := s.resumeToolContinuation(traceCtx, pending.continuation)
	if err != nil {
		if awaiting := s.awaitingResult(err, events, messages, finalContent); awaiting != nil {
			runErr = err
			return awaiting, nil
		}
		runErr = err
		return nil, err
	}
	finalResp, err := s.applyPostTurnHooksAndReturn(traceCtx, hookMeta(s), pending.continuation.Agent, pending.continuation.Request, resp)
	if err != nil {
		runErr = err
		return nil, err
	}
	if finalContent == "" && (finalResp.Role != "" || len(finalResp.Parts) > 0 || finalResp.Text() != "") {
		finalContent = finalResp.Summary()
	}
	if !s.IsPaused() {
		_ = s.EmitWithContext(traceCtx, event.New(event.SessionResumed, s.id, s.State()))
	}
	_ = s.engine.persistSessionState(traceCtx, s)

	mu.Lock()
	runResult := &RunResult{
		Status:     StatusCompleted,
		Final:      finalContent,
		Transcript: append([]agent.Message(nil), messages...),
		Events:     append([]event.Event(nil), events...),
		Metadata:   make(map[string]any),
	}
	mu.Unlock()

	runErr = nil
	return runResult, nil
}

func (s *Session) storePendingTool(ctx context.Context, req tool.Request, continuation toolContinuation) error {
	if req.RequestID == "" {
		return fmt.Errorf("request id required")
	}
	s.pendingToolsMu.Lock()
	if s.pendingTools == nil {
		s.pendingTools = make(map[string]pendingTool)
	}
	s.pendingTools[req.RequestID] = pendingTool{request: req, continuation: continuation}
	s.pendingToolsMu.Unlock()

	_ = s.EmitWithContext(ctx, event.New(event.SessionPaused, s.id, s.State()))

	if s.engine != nil {
		_ = s.engine.persistSessionState(ctx, s)
	}
	return nil
}

func (s *Session) resumeToolContinuation(ctx context.Context, cont toolContinuation) (agent.Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	model := cont.Model
	if model == "" {
		return agent.Message{}, ErrModelRequired
	}

	p, providerID, resolvedModel, err := s.engine.resolveProvider(model, cont.Agent.ProviderID)
	if err != nil {
		return agent.Message{}, err
	}
	model = resolvedModel

	toolPolicy := s.resolveToolPolicy(cont.Request.ToolPolicy)
	toolDefs, allowedToolIDs, err := s.resolveTools(cont.ToolIDs, toolPolicy)
	if err != nil {
		return agent.Message{}, err
	}

	params := mergeParams(s.engine.cfg.Defaults.Params, cont.Agent.Params, cont.Request.Params)
	delete(params, "model")
	params = mergeParams(params, cont.Params)

	contextCfg := cont.Context

	var tokenEstimator contextpolicy.TokenEstimator
	if estimator, ok := p.(provider.TokenEstimator); ok {
		tokenEstimator = estimator
	}

	span, spanCtx := s.engine.telemetry.StartSpan(ctx, telemetry.SpanInput{
		Name: "agent.run.resume",
		Attributes: map[string]any{
			"agent_name":  cont.Agent.Name,
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

	maxIterations := cont.MaxIterations
	if maxIterations <= 0 {
		maxIterations = defaultMaxToolIterations
	}

	history := append([]agent.Message(nil), cont.History...)
	systemMessages := append([]agent.Message(nil), cont.SystemMessages...)

	for attempt := cont.NextAttempt; attempt <= maxIterations; attempt++ {
		perspectiveHistory := applyAgentPerspective(history, cont.Agent.Name, s.engine.agentPerspectiveMode)
		selected, err := s.engine.contextPolicy.Select(spanCtx, buildContextInput(model, systemMessages, perspectiveHistory, contextCfg, s.engine.contextSummarizer, tokenEstimator))
		if err != nil {
			runErr = err
			return agent.Message{}, err
		}
		message, toolCalls, err := s.runProviderStream(spanCtx, p, provider.Request{
			Model:    model,
			Messages: selected,
			Tools:    toolDefs,
			Params:   params,
		}, cont.Agent.Name, span, false)
		if err != nil {
			runErr = err
			return agent.Message{}, err
		}
		if len(toolCalls) == 0 {
			return message, nil
		}
		if attempt == maxIterations {
			runErr = ErrToolIterationsExceeded
			return agent.Message{}, ErrToolIterationsExceeded
		}

		if message.Role != "" || len(message.Parts) > 0 || message.Text() != "" {
			if message.Role == "" {
				message.Role = agent.RoleAssistant
			}
			history = append(history, message)
		}

		results, err := s.executeToolCalls(spanCtx, cont.Agent.Name, toolCalls, allowedToolIDs)
		if err != nil {
			if awaiting := toolAwaiting(err); awaiting != nil {
				history = append(history, toolMessagesFromResults(results)...)
				cont.History = append([]agent.Message(nil), history...)
				cont.NextAttempt = attempt + 1
				if err := s.storePendingTool(spanCtx, awaiting.Request, cont); err != nil {
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

func hookMeta(s *Session) hook.SessionMeta {
	return hook.SessionMeta{SessionID: s.id, ProtocolID: s.protocol.ID()}
}
