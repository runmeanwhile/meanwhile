package openai

import (
	"encoding/json"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

func TestDecodeEventOutputTextDelta(t *testing.T) {
	s := &stream{}
	raw := []byte(`{"type":"response.output_text.delta","delta":"hello"}`)
	ev, err := s.decodeEvent(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ev.Type != provider.EventMessageDelta {
		t.Fatalf("expected message delta, got %s", ev.Type)
	}
	if ev.Delta != "hello" {
		t.Fatalf("unexpected delta: %s", ev.Delta)
	}
}

func TestDecodeEventReasoningTextDelta(t *testing.T) {
	s := &stream{}
	raw := []byte(`{"type":"response.reasoning_text.delta","delta":"step 1"}`)
	ev, err := s.decodeEvent(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ev.Type != provider.EventReasoningDelta {
		t.Fatalf("expected reasoning delta, got %s", ev.Type)
	}
	if ev.Delta != "step 1" {
		t.Fatalf("unexpected delta: %s", ev.Delta)
	}
}

func TestDecodeEventReasoningSummaryTextDelta(t *testing.T) {
	s := &stream{}
	raw := []byte(`{"type":"response.reasoning_summary_text.delta","delta":"summary"}`)
	ev, err := s.decodeEvent(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ev.Type != provider.EventReasoningSummaryDelta {
		t.Fatalf("expected reasoning summary delta, got %s", ev.Type)
	}
	if ev.Delta != "summary" {
		t.Fatalf("unexpected delta: %s", ev.Delta)
	}
}

func TestDecodeEventFunctionCallArgumentsDone(t *testing.T) {
	s := &stream{callIDByItemID: map[string]string{}, toolNameByItemID: map[string]string{}}
	payload := map[string]any{
		"type":      "response.function_call_arguments.done",
		"item_id":   "item_1",
		"name":      "search",
		"arguments": `{"q":"hi"}`,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ev, err := s.decodeEvent(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ev.Type != provider.EventToolCall {
		t.Fatalf("expected tool call, got %s", ev.Type)
	}
	if len(ev.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(ev.ToolCalls))
	}
	if ev.ToolCalls[0].ToolID != "search" {
		t.Fatalf("unexpected tool id: %s", ev.ToolCalls[0].ToolID)
	}
	if !json.Valid(ev.ToolCalls[0].Arguments) {
		t.Fatalf("arguments not valid json: %s", string(ev.ToolCalls[0].Arguments))
	}
}

func TestDecodeEventOutputItemMapsCallID(t *testing.T) {
	s := &stream{callIDByItemID: map[string]string{}, toolNameByItemID: map[string]string{}}
	raw, err := json.Marshal(map[string]any{
		"type": "response.output_item.added",
		"item": map[string]any{
			"id":      "item_2",
			"type":    "function_call",
			"name":    "lookup",
			"call_id": "call_2",
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ev, err := s.decodeEvent(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ev.Type != provider.EventRaw {
		t.Fatalf("expected raw event, got %s", ev.Type)
	}

	rawArgs, err := json.Marshal(map[string]any{
		"type":      "response.function_call_arguments.done",
		"item_id":   "item_2",
		"arguments": `{"id":1}`,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	ev, err = s.decodeEvent(rawArgs)
	if err != nil {
		t.Fatalf("decode args: %v", err)
	}
	if ev.ToolCalls[0].ID != "call_2" {
		t.Fatalf("expected call id mapping, got %s", ev.ToolCalls[0].ID)
	}
}

func TestDecodeEventToolCallDeduplication(t *testing.T) {
	// Test that tool calls are deduplicated when both function_call_arguments.done
	// and output_item.done events are received for the same call
	s := &stream{
		callIDByItemID:   make(map[string]string),
		toolNameByItemID: make(map[string]string),
		emittedToolCalls: make(map[string]struct{}),
	}

	// First, output_item.added registers the call ID and name
	rawAdded, _ := json.Marshal(map[string]any{
		"type": "response.output_item.added",
		"item": map[string]any{
			"id":      "item_3",
			"type":    "function_call",
			"name":    "recall_context",
			"call_id": "call_3",
		},
	})
	ev, err := s.decodeEvent(rawAdded)
	if err != nil {
		t.Fatalf("decode added: %v", err)
	}
	if ev.Type != provider.EventRaw {
		t.Fatalf("expected raw for added event, got %s", ev.Type)
	}

	// Second, function_call_arguments.done emits the tool call
	rawArgsDone, _ := json.Marshal(map[string]any{
		"type":      "response.function_call_arguments.done",
		"item_id":   "item_3",
		"arguments": `{"query":"test"}`,
	})
	ev, err = s.decodeEvent(rawArgsDone)
	if err != nil {
		t.Fatalf("decode args done: %v", err)
	}
	if ev.Type != provider.EventToolCall {
		t.Fatalf("expected tool call for args done, got %s", ev.Type)
	}
	if len(ev.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(ev.ToolCalls))
	}
	if ev.ToolCalls[0].ID != "call_3" {
		t.Fatalf("expected call_3, got %s", ev.ToolCalls[0].ID)
	}

	// Third, output_item.done arrives with the same arguments - should be deduplicated
	rawItemDone, _ := json.Marshal(map[string]any{
		"type": "response.output_item.done",
		"item": map[string]any{
			"id":        "item_3",
			"type":      "function_call",
			"name":      "recall_context",
			"call_id":   "call_3",
			"arguments": `{"query":"test"}`,
		},
	})
	ev, err = s.decodeEvent(rawItemDone)
	if err != nil {
		t.Fatalf("decode item done: %v", err)
	}
	// Should be raw event (deduplicated), not another tool call
	if ev.Type != provider.EventRaw {
		t.Fatalf("expected deduplicated tool call to return raw event, got %s", ev.Type)
	}
}

func TestDecodeEventUnknown(t *testing.T) {
	s := &stream{}
	raw := []byte(`{"type":"response.some_future_event","foo":"bar"}`)
	ev, err := s.decodeEvent(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ev.Type != provider.EventRaw {
		t.Fatalf("expected raw event, got %s", ev.Type)
	}
}
