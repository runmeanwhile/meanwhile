package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/telemetry"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// executeToolCalls runs tool calls through hooks and returns results.
func (s *Session) executeToolCalls(ctx context.Context, agentID string, calls []provider.ToolCall, allowed map[string]struct{}) ([]tool.Result, error) {
	results := make([]tool.Result, 0, len(calls))
	meta := hook.SessionMeta{SessionID: s.id, ProtocolID: s.protocol.ID()}

	for _, call := range calls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		toolCall := tool.Call{ID: call.ID, ToolID: call.ToolID, Arguments: call.Arguments, AgentID: agentID}
		decision, modifiedCall, err := s.applyPreToolHooks(ctx, meta, toolCall)
		if err != nil {
			return nil, err
		}
		if decision == hook.Block {
			res := tool.Result{ID: toolCall.ID, ToolID: toolCall.ToolID, Error: &tool.Error{Message: "blocked by hook"}}
			ev := event.New(event.ToolCallError, s.id, map[string]any{"result": res})
			ev.AgentID = agentID
			ev.ToolID = toolCall.ToolID
			_ = s.EmitWithContext(ctx, ev)
			results = append(results, res)
			continue
		}
		if decision == hook.Modify {
			toolCall = modifiedCall
		}

		toolSpan, toolCtx := s.engine.telemetry.StartSpan(ctx, telemetry.SpanInput{
			Name:       "tool.run",
			Attributes: toolCallAttrs(s.id, agentID, toolCall),
		})
		finishToolSpan := func(res tool.Result, eventType event.Type, err error) {
			attrs := toolResultAttrs(s.id, agentID, res, eventType)
			if err != nil {
				attrs["error"] = errorString(err)
			}
			toolSpan.AddEvent(string(eventType), attrs)
			toolSpan.End(err)
		}

		res := tool.Result{ID: toolCall.ID, ToolID: toolCall.ToolID}
		if len(toolCall.Arguments) > 0 {
			res.Meta = map[string]any{
				"arguments": json.RawMessage(toolCall.Arguments),
			}
		}
		if !toolAllowed(allowed, toolCall.ToolID) {
			res.Error = &tool.Error{Message: fmt.Sprintf("tool not allowed: %s", toolCall.ToolID)}
			ev := event.New(event.ToolCallError, s.id, map[string]any{"result": res})
			ev.AgentID = agentID
			ev.ToolID = toolCall.ToolID
			_ = s.EmitWithContext(toolCtx, ev)
			finishToolSpan(res, event.ToolCallError, fmt.Errorf("tool not allowed: %s", toolCall.ToolID))
			results = append(results, res)
			continue
		}
		// Try session registry first, then engine registry
		impl, ok := func() (tool.Tool, bool) {
			if s.tools != nil {
				if t, ok := s.tools.Get(toolCall.ToolID); ok {
					return t, true
				}
			}
			return s.engine.tools.Get(toolCall.ToolID)
		}()
		if !ok {
			res.Error = &tool.Error{Message: fmt.Sprintf("tool not found: %s", toolCall.ToolID)}
			ev := event.New(event.ToolCallError, s.id, map[string]any{"result": res})
			ev.AgentID = agentID
			ev.ToolID = toolCall.ToolID
			_ = s.EmitWithContext(toolCtx, ev)
			finishToolSpan(res, event.ToolCallError, fmt.Errorf("tool not found: %s", toolCall.ToolID))
			results = append(results, res)
			continue
		}

		startEv := event.New(event.ToolCallStarted, s.id, map[string]any{"call": toolCall})
		startEv.AgentID = agentID
		startEv.ToolID = toolCall.ToolID
		_ = s.EmitWithContext(toolCtx, startEv)
		toolSpan.AddEvent(string(event.ToolCallStarted), toolCallAttrs(s.id, agentID, toolCall))
		out, err := impl.Run(toolCtx, toolCall, toolEmitter{session: s, toolID: toolCall.ToolID, callID: toolCall.ID, agentID: agentID})
		var awaiting *protocol.AwaitingInputError
		if err != nil && errors.As(err, &awaiting) {
			if out.ID == "" {
				out.ID = res.ID
			}
			if out.ToolID == "" {
				out.ToolID = res.ToolID
			}
			if len(res.Meta) > 0 {
				if out.Meta == nil {
					out.Meta = make(map[string]any, len(res.Meta))
				}
				for key, value := range res.Meta {
					if _, ok := out.Meta[key]; !ok {
						out.Meta[key] = value
					}
				}
			}
			res = out
			decision, modifiedResult, hookErr := s.applyPostToolHooks(toolCtx, meta, res)
			if hookErr != nil {
				s.clearPendingRequest(toolCtx, awaiting.Request.RequestID)
				finishToolSpan(res, event.ToolCallError, hookErr)
				return nil, hookErr
			}
			if decision == hook.Block {
				if res.Error == nil {
					res.Error = &tool.Error{Message: "blocked by hook"}
				}
			}
			if decision == hook.Modify {
				res = modifiedResult
			}
			if res.Error != nil {
				s.clearPendingRequest(toolCtx, awaiting.Request.RequestID)
				ev := event.New(event.ToolCallError, s.id, map[string]any{"result": res})
				ev.AgentID = agentID
				ev.ToolID = toolCall.ToolID
				_ = s.EmitWithContext(toolCtx, ev)
				toolMsg := toolMessageFromResult(res)
				msgEv := event.New(event.AgentMessageComplete, s.id, map[string]any{"message": toolMsg})
				msgEv.AgentID = agentID
				msgEv.ToolID = toolCall.ToolID
				_ = s.EmitWithContext(toolCtx, msgEv)
				finishToolSpan(res, event.ToolCallError, errors.New(res.Error.Message))
				results = append(results, res)
				continue
			}

			ev := event.New(event.ToolCallCompleted, s.id, map[string]any{"result": res})
			ev.AgentID = agentID
			ev.ToolID = toolCall.ToolID
			_ = s.EmitWithContext(toolCtx, ev)
			toolMsg := toolMessageFromResult(res)
			msgEv := event.New(event.AgentMessageComplete, s.id, map[string]any{"message": toolMsg})
			msgEv.AgentID = agentID
			msgEv.ToolID = toolCall.ToolID
			_ = s.EmitWithContext(toolCtx, msgEv)
			finishToolSpan(res, event.ToolCallCompleted, nil)
			results = append(results, res)
			return results, err
		}

		if awaitingTool := toolAwaiting(err); awaitingTool != nil {
			res.Meta = mergeResultMeta(res.Meta, map[string]any{
				"awaiting":   true,
				"request_id": awaitingTool.Request.RequestID,
			})
			ev := event.New(event.ToolCallAwaiting, s.id, map[string]any{"request": awaitingTool.Request})
			ev.AgentID = agentID
			ev.ToolID = toolCall.ToolID
			_ = s.EmitWithContext(toolCtx, ev)
			finishToolSpan(res, event.ToolCallAwaiting, err)
			results = append(results, res)
			return results, err
		}

		if err != nil {
			res.Error = &tool.Error{Message: err.Error()}
		} else {
			if out.ID == "" {
				out.ID = res.ID
			}
			if out.ToolID == "" {
				out.ToolID = res.ToolID
			}
			if len(res.Meta) > 0 {
				if out.Meta == nil {
					out.Meta = make(map[string]any, len(res.Meta))
				}
				for key, value := range res.Meta {
					if _, ok := out.Meta[key]; !ok {
						out.Meta[key] = value
					}
				}
			}
			res = out
		}

		decision, modifiedResult, err := s.applyPostToolHooks(toolCtx, meta, res)
		if err != nil {
			finishToolSpan(res, event.ToolCallError, err)
			return nil, err
		}
		if decision == hook.Block {
			if res.Error == nil {
				res.Error = &tool.Error{Message: "blocked by hook"}
			}
		}
		if decision == hook.Modify {
			res = modifiedResult
		}

		if res.Error != nil {
			ev := event.New(event.ToolCallError, s.id, map[string]any{"result": res})
			ev.AgentID = agentID
			ev.ToolID = toolCall.ToolID
			_ = s.EmitWithContext(toolCtx, ev)
			finishToolSpan(res, event.ToolCallError, errors.New(res.Error.Message))
		} else {
			ev := event.New(event.ToolCallCompleted, s.id, map[string]any{"result": res})
			ev.AgentID = agentID
			ev.ToolID = toolCall.ToolID
			_ = s.EmitWithContext(toolCtx, ev)
			finishToolSpan(res, event.ToolCallCompleted, nil)
		}
		toolMsg := toolMessageFromResult(res)
		msgEv := event.New(event.AgentMessageComplete, s.id, map[string]any{"message": toolMsg})
		msgEv.AgentID = agentID
		msgEv.ToolID = toolCall.ToolID
		_ = s.EmitWithContext(toolCtx, msgEv)
		results = append(results, res)
	}

	return results, nil
}

func (s *Session) applyPreToolHooks(ctx context.Context, meta hook.SessionMeta, call tool.Call) (hook.Decision, tool.Call, error) {
	decision := hook.Allow
	modified := call
	for _, h := range s.engine.hooks.PreTool() {
		d, newCall, err := h.OnPreToolUse(ctx, meta, modified)
		if err != nil {
			return hook.Block, modified, err
		}
		if d == hook.Block {
			_ = s.EmitWithContext(ctx, event.New(event.HookBlocked, s.id, map[string]any{"hook": h.ID()}))
			return hook.Block, modified, nil
		}
		if d == hook.Modify {
			modified = newCall
			decision = hook.Modify
			_ = s.EmitWithContext(ctx, event.New(event.HookModified, s.id, map[string]any{"hook": h.ID()}))
		}
	}
	return decision, modified, nil
}

func (s *Session) applyPostToolHooks(ctx context.Context, meta hook.SessionMeta, result tool.Result) (hook.Decision, tool.Result, error) {
	decision := hook.Allow
	modified := result
	for _, h := range s.engine.hooks.PostTool() {
		d, newResult, err := h.OnPostToolUse(ctx, meta, modified)
		if err != nil {
			return hook.Block, modified, err
		}
		if d == hook.Block {
			_ = s.EmitWithContext(ctx, event.New(event.HookBlocked, s.id, map[string]any{"hook": h.ID()}))
			return hook.Block, modified, nil
		}
		if d == hook.Modify {
			modified = newResult
			decision = hook.Modify
			_ = s.EmitWithContext(ctx, event.New(event.HookModified, s.id, map[string]any{"hook": h.ID()}))
		}
	}
	return decision, modified, nil
}

type toolEmitter struct {
	session *Session
	toolID  string
	callID  string
	agentID string
}

func (e toolEmitter) Emit(eventType string, payload any) error {
	ev := event.New(event.ToolCallDelta, e.session.id, map[string]any{
		"call_id":    e.callID,
		"event_type": eventType,
		"payload":    payload,
	})
	ev.ToolID = e.toolID
	ev.AgentID = e.agentID
	return e.session.Emit(ev)
}
