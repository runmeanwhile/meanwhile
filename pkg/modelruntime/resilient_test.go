package modelruntime

import (
	"context"
	"errors"
	"io"
	"testing"
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

func TestResilientStreamReplaysTransientErrors(t *testing.T) {
	t.Parallel()

	ev1 := Event{Type: EventMessageDelta, Delta: "hello "}
	ev2 := Event{Type: EventMessageDelta, Delta: "world"}
	ev3 := Event{
		Type:    EventMessageCompleted,
		Message: Message{Role: RoleAssistant, Parts: []Part{{Type: PartText, Text: "hello world"}}},
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
			t.Fatalf("Recv() error = %v", err)
		}
		got = append(got, ev)
	}

	if callCount != 2 {
		t.Fatalf("stream creations = %d, want 2", callCount)
	}
	if len(got) != 3 {
		t.Fatalf("events = %d, want 3", len(got))
	}
}
