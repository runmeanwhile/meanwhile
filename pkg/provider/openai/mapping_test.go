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
