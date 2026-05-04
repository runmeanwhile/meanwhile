package provider

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

type scriptedStream struct {
	steps []recvStep
	idx   int
}

type recvStep struct {
	ev  Event
	err error
}

func (s *scriptedStream) Recv() (Event, error) {
	if s.idx >= len(s.steps) {
		return Event{}, io.EOF
	}
	step := s.steps[s.idx]
	s.idx++
	return step.ev, step.err
}

func (s *scriptedStream) Close() error { return nil }

func TestResilientStream_ReplaysTransientErrors(t *testing.T) {
	ev1 := Event{Type: EventMessageDelta, Delta: "hello "}
	ev2 := Event{Type: EventMessageDelta, Delta: "world"}
	ev3 := Event{
		Type:    EventMessageCompleted,
		Message: modelruntime.Message{Role: modelruntime.RoleAssistant, Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: "hello world"}}},
	}

	streams := []Stream{
		&scriptedStream{steps: []recvStep{{ev: ev1}, {ev: ev2}, {err: timeoutErr{}}}},
		&scriptedStream{steps: []recvStep{{ev: ev1}, {ev: ev2}, {ev: ev3}}},
	}
	callCount := 0
	create := func(ctx context.Context) (Stream, error) {
		_ = ctx
		if callCount >= len(streams) {
			return nil, errors.New("unexpected stream creation")
		}
		s := streams[callCount]
		callCount++
		return s, nil
	}

	rs := NewResilientStream(context.Background(), create, ResilientConfig{
		MaxRetries:      3,
		InitialInterval: 0,
		MaxInterval:     0,
		Multiplier:      1,
	})

	var got []Event
	for {
		ev, err := rs.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("recv error: %v", err)
		}
		got = append(got, ev)
	}

	if callCount != 2 {
		t.Fatalf("expected 2 stream creations, got %d", callCount)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}
	if got[2].Type != EventMessageCompleted {
		t.Fatalf("expected final completed event, got %v", got[2].Type)
	}
}

func TestResilientStream_MaxRetriesExceeded(t *testing.T) {
	streams := []Stream{
		&scriptedStream{steps: []recvStep{{err: timeoutErr{}}}},
		&scriptedStream{steps: []recvStep{{err: timeoutErr{}}}},
		&scriptedStream{steps: []recvStep{{err: timeoutErr{}}}},
	}
	callCount := 0
	create := func(ctx context.Context) (Stream, error) {
		_ = ctx
		if callCount >= len(streams) {
			return nil, errors.New("unexpected stream creation")
		}
		s := streams[callCount]
		callCount++
		return s, nil
	}

	rs := NewResilientStream(context.Background(), create, ResilientConfig{
		MaxRetries:      2,
		InitialInterval: 0,
		MaxInterval:     0,
		Multiplier:      1,
	})

	_, err := rs.Recv()
	if err == nil {
		t.Fatalf("expected error after retries")
	}
	if callCount != 3 {
		t.Fatalf("expected 3 stream creations, got %d", callCount)
	}
}

func TestResilientStream_ReplayMismatch(t *testing.T) {
	ev1 := Event{Type: EventMessageDelta, Delta: "a"}
	ev2 := Event{Type: EventMessageDelta, Delta: "b"}

	streams := []Stream{
		&scriptedStream{steps: []recvStep{{ev: ev1}, {err: timeoutErr{}}}},
		&scriptedStream{steps: []recvStep{{ev: ev2}}},
	}
	callCount := 0
	create := func(ctx context.Context) (Stream, error) {
		_ = ctx
		if callCount >= len(streams) {
			return nil, errors.New("unexpected stream creation")
		}
		s := streams[callCount]
		callCount++
		return s, nil
	}

	rs := NewResilientStream(context.Background(), create, ResilientConfig{
		MaxRetries:      2,
		InitialInterval: 0,
		MaxInterval:     0,
		Multiplier:      1,
	})

	_, err := rs.Recv()
	if err != nil {
		t.Fatalf("unexpected error on first recv: %v", err)
	}
	_, err = rs.Recv()
	if !errors.Is(err, ErrResilientReplayMismatch) {
		t.Fatalf("expected replay mismatch, got %v", err)
	}
}
