package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

const doneToken = "[DONE]"

var knownEventTypes = map[string]struct{}{
	"response.created":                             {},
	"response.in_progress":                         {},
	"response.completed":                           {},
	"response.failed":                              {},
	"response.incomplete":                          {},
	"response.queued":                              {},
	"response.output_item.added":                   {},
	"response.output_item.done":                    {},
	"response.content_part.added":                  {},
	"response.content_part.done":                   {},
	"response.output_text.delta":                   {},
	"response.output_text.done":                    {},
	"response.output_text.annotation.added":        {},
	"response.refusal.delta":                       {},
	"response.refusal.done":                        {},
	"response.function_call_arguments.delta":       {},
	"response.function_call_arguments.done":        {},
	"response.custom_tool_call_input.delta":        {},
	"response.custom_tool_call_input.done":         {},
	"response.file_search_call.in_progress":        {},
	"response.file_search_call.searching":          {},
	"response.file_search_call.completed":          {},
	"response.web_search_call.in_progress":         {},
	"response.web_search_call.searching":           {},
	"response.web_search_call.completed":           {},
	"response.reasoning_summary_part.added":        {},
	"response.reasoning_summary_part.done":         {},
	"response.reasoning_summary_text.delta":        {},
	"response.reasoning_summary_text.done":         {},
	"response.reasoning_text.delta":                {},
	"response.reasoning_text.done":                 {},
	"response.image_generation_call.completed":     {},
	"response.image_generation_call.generating":    {},
	"response.image_generation_call.in_progress":   {},
	"response.image_generation_call.partial_image": {},
	"response.mcp_call_arguments.delta":            {},
	"response.mcp_call_arguments.done":             {},
	"response.mcp_call.completed":                  {},
	"response.mcp_call.failed":                     {},
	"response.mcp_call.in_progress":                {},
	"response.mcp_list_tools.completed":            {},
	"response.mcp_list_tools.failed":               {},
	"response.mcp_list_tools.in_progress":          {},
	"response.code_interpreter_call.in_progress":   {},
	"response.code_interpreter_call.interpreting":  {},
	"response.code_interpreter_call.completed":     {},
	"response.code_interpreter_call_code.delta":    {},
	"response.code_interpreter_call_code.done":     {},
	"response.output_audio_transcript.delta":       {},
	"response.output_audio_transcript.done":        {},
	"error":                                        {},
}

type stream struct {
	decoder          *sseDecoder
	body             io.Closer
	callIDByItemID   map[string]string
	toolNameByItemID map[string]string
}

func newStream(body io.ReadCloser) provider.Stream {
	return &stream{
		decoder:          newSSEDecoder(body),
		body:             body,
		callIDByItemID:   make(map[string]string),
		toolNameByItemID: make(map[string]string),
	}
}

func (s *stream) Recv() (provider.Event, error) {
	data, err := s.decoder.Next()
	if err != nil {
		return provider.Event{}, err
	}
	if len(data) == 0 {
		return provider.Event{}, io.EOF
	}

	if string(data) == doneToken {
		return provider.Event{}, io.EOF
	}

	ev, err := s.decodeEvent(data)
	if err != nil {
		return provider.Event{}, err
	}
	return ev, nil
}

func (s *stream) Close() error {
	return s.body.Close()
}

type eventEnvelope struct {
	Type string `json:"type"`
}

type outputTextDelta struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

type outputTextDone struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type functionCallArgsDone struct {
	Type      string `json:"type"`
	ItemID    string `json:"item_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	CallID    string `json:"call_id"`
}

type customToolCallInputDone struct {
	Type   string `json:"type"`
	ItemID string `json:"item_id"`
	Name   string `json:"name"`
	Input  string `json:"input"`
	CallID string `json:"call_id"`
}

type outputItemEvent struct {
	Type string `json:"type"`
	Item struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Name      string `json:"name"`
		CallID    string `json:"call_id"`
		Arguments string `json:"arguments"`
		Input     string `json:"input"`
	} `json:"item"`
}

type errorEvent struct {
	Type  string `json:"type"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *stream) decodeEvent(data []byte) (provider.Event, error) {
	var env eventEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return provider.Event{}, fmt.Errorf("decode event envelope: %w", err)
	}

	if _, ok := knownEventTypes[env.Type]; !ok {
		return provider.Event{Type: provider.EventRaw, Raw: data}, nil
	}

	switch env.Type {
	case "response.output_text.delta":
		var payload outputTextDelta
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Event{}, fmt.Errorf("decode output_text.delta: %w", err)
		}
		return provider.Event{Type: provider.EventMessageDelta, Delta: payload.Delta}, nil
	case "response.reasoning_text.delta":
		var payload outputTextDelta
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Event{}, fmt.Errorf("decode reasoning_text.delta: %w", err)
		}
		return provider.Event{Type: provider.EventReasoningDelta, Delta: payload.Delta}, nil
	case "response.reasoning_summary_text.delta":
		var payload outputTextDelta
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Event{}, fmt.Errorf("decode reasoning_summary_text.delta: %w", err)
		}
		return provider.Event{Type: provider.EventReasoningSummaryDelta, Delta: payload.Delta}, nil
	case "response.output_text.done":
		var payload outputTextDone
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Event{}, fmt.Errorf("decode output_text.done: %w", err)
		}
		return provider.Event{
			Type: provider.EventMessageCompleted,
			Message: modelruntime.Message{
				Role:  modelruntime.RoleAssistant,
				Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: payload.Text}},
			},
		}, nil
	case "response.function_call_arguments.done":
		var payload functionCallArgsDone
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Event{}, fmt.Errorf("decode function_call_arguments.done: %w", err)
		}
		callID := payload.CallID
		if callID == "" {
			callID = s.callIDByItemID[payload.ItemID]
		}
		if callID == "" {
			callID = payload.ItemID
		}
		toolName := payload.Name
		if toolName == "" {
			toolName = s.toolNameByItemID[payload.ItemID]
		}
		return provider.Event{
			Type: provider.EventToolCall,
			ToolCalls: []provider.ToolCall{{
				ID:        callID,
				ToolID:    toolName,
				Arguments: json.RawMessage(payload.Arguments),
			}},
		}, nil
	case "response.custom_tool_call_input.done":
		var payload customToolCallInputDone
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Event{}, fmt.Errorf("decode custom_tool_call_input.done: %w", err)
		}
		callID := payload.CallID
		if callID == "" {
			callID = s.callIDByItemID[payload.ItemID]
		}
		if callID == "" {
			callID = payload.ItemID
		}
		toolName := payload.Name
		if toolName == "" {
			toolName = s.toolNameByItemID[payload.ItemID]
		}
		encoded, err := json.Marshal(payload.Input)
		if err != nil {
			return provider.Event{}, fmt.Errorf("encode custom tool input: %w", err)
		}
		return provider.Event{
			Type: provider.EventToolCall,
			ToolCalls: []provider.ToolCall{{
				ID:        callID,
				ToolID:    toolName,
				Arguments: json.RawMessage(encoded),
			}},
		}, nil
	case "response.output_item.added", "response.output_item.done":
		var payload outputItemEvent
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Event{}, fmt.Errorf("decode output_item: %w", err)
		}
		if payload.Item.ID != "" {
			if payload.Item.CallID != "" {
				s.callIDByItemID[payload.Item.ID] = payload.Item.CallID
			}
			if payload.Item.Name != "" {
				s.toolNameByItemID[payload.Item.ID] = payload.Item.Name
			}
		}
		if payload.Item.Type == "function_call" && payload.Item.Arguments != "" {
			callID := payload.Item.CallID
			if callID == "" {
				callID = payload.Item.ID
			}
			return provider.Event{
				Type: provider.EventToolCall,
				ToolCalls: []provider.ToolCall{{
					ID:        callID,
					ToolID:    payload.Item.Name,
					Arguments: json.RawMessage(payload.Item.Arguments),
				}},
			}, nil
		}
		return provider.Event{Type: provider.EventRaw, Raw: data}, nil
	case "error":
		var payload errorEvent
		if err := json.Unmarshal(data, &payload); err != nil {
			return provider.Event{}, fmt.Errorf("decode error event: %w", err)
		}
		return provider.Event{Type: provider.EventError, Err: errors.New(payload.Error.Message)}, nil
	default:
		return provider.Event{Type: provider.EventRaw, Raw: data}, nil
	}
}
