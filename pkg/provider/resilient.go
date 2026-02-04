package provider

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

const (
	defaultResilientMaxRetries      = 5
	defaultResilientInitialInterval = 1 * time.Second
	defaultResilientMaxInterval     = 10 * time.Second
	defaultResilientMultiplier      = 2.0
)

// ErrResilientReplayMismatch indicates the provider stream diverged after a retry.
var ErrResilientReplayMismatch = errors.New("provider stream replay mismatch")

// ResilientConfig configures retry behavior for provider streams.
type ResilientConfig struct {
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
}

// DefaultResilientConfig returns default retry settings.
func DefaultResilientConfig() ResilientConfig {
	return ResilientConfig{
		MaxRetries:      defaultResilientMaxRetries,
		InitialInterval: defaultResilientInitialInterval,
		MaxInterval:     defaultResilientMaxInterval,
		Multiplier:      defaultResilientMultiplier,
	}
}

func (c ResilientConfig) withDefaults() ResilientConfig {
	out := c
	if out.MaxRetries <= 0 {
		out.MaxRetries = defaultResilientMaxRetries
	}
	if out.InitialInterval <= 0 {
		out.InitialInterval = defaultResilientInitialInterval
	}
	if out.MaxInterval <= 0 {
		out.MaxInterval = defaultResilientMaxInterval
	}
	if out.Multiplier <= 0 {
		out.Multiplier = defaultResilientMultiplier
	}
	return out
}

// ResilientStream wraps a provider stream with retry logic.
type ResilientStream struct {
	ctx         context.Context
	create      func(context.Context) (Stream, error)
	backoff     backoff.BackOff
	maxRetries  int
	retryCount  int
	stream      Stream
	history     []Event
	replaying   bool
	replayIndex int
	closed      bool
}

// NewResilientStream creates a resilient stream that retries on transient errors.
func NewResilientStream(ctx context.Context, create func(context.Context) (Stream, error), cfg ResilientConfig) *ResilientStream {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg = cfg.withDefaults()
	expo := backoff.NewExponentialBackOff()
	expo.InitialInterval = cfg.InitialInterval
	expo.MaxInterval = cfg.MaxInterval
	expo.Multiplier = cfg.Multiplier
	expo.Reset()

	return &ResilientStream{
		ctx:        ctx,
		create:     create,
		backoff:    expo,
		maxRetries: cfg.MaxRetries,
	}
}

// Recv receives the next event, retrying transient stream errors.
func (s *ResilientStream) Recv() (Event, error) {
	for {
		if s.closed {
			return Event{}, io.EOF
		}
		if err := s.ctx.Err(); err != nil {
			return Event{}, err
		}
		if s.stream == nil {
			if err := s.openStream(); err != nil {
				if !isTransient(err) {
					return Event{}, err
				}
				if err := s.waitRetry(err); err != nil {
					return Event{}, err
				}
				continue
			}
		}

		ev, err := s.stream.Recv()
		if err == nil {
			s.resetBackoff()
			if s.replaying {
				if s.replayIndex >= len(s.history) {
					s.replaying = false
				}
				if s.replaying {
					if !eventsEqual(ev, s.history[s.replayIndex]) {
						return Event{}, ErrResilientReplayMismatch
					}
					s.replayIndex++
					if s.replayIndex >= len(s.history) {
						s.replaying = false
					}
					continue
				}
			}
			s.history = append(s.history, ev)
			return ev, nil
		}
		if errors.Is(err, io.EOF) {
			if s.replaying {
				return Event{}, ErrResilientReplayMismatch
			}
			return Event{}, io.EOF
		}
		if !isTransient(err) {
			return Event{}, err
		}
		s.resetStream()
		if err := s.waitRetry(err); err != nil {
			return Event{}, err
		}
	}
}

// Close closes the underlying stream.
func (s *ResilientStream) Close() error {
	s.closed = true
	if s.stream == nil {
		return nil
	}
	err := s.stream.Close()
	s.stream = nil
	return err
}

func (s *ResilientStream) openStream() error {
	if s.create == nil {
		return errors.New("stream factory required")
	}
	stream, err := s.create(s.ctx)
	if err != nil {
		return err
	}
	s.stream = stream
	s.replayIndex = 0
	s.replaying = len(s.history) > 0
	return nil
}

func (s *ResilientStream) resetStream() {
	if s.stream != nil {
		_ = s.stream.Close()
	}
	s.stream = nil
}

func (s *ResilientStream) resetBackoff() {
	s.retryCount = 0
	if s.backoff != nil {
		s.backoff.Reset()
	}
}

func (s *ResilientStream) waitRetry(lastErr error) error {
	s.retryCount++
	if s.maxRetries > 0 && s.retryCount > s.maxRetries {
		return lastErr
	}
	if s.backoff == nil {
		return lastErr
	}
	delay := s.backoff.NextBackOff()
	if delay == backoff.Stop {
		return lastErr
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "reset by peer") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection refused") {
		return true
	}
	return false
}

func eventsEqual(a, b Event) bool {
	if a.Type != b.Type {
		return false
	}
	if a.Delta != b.Delta {
		return false
	}
	if !messagesEqual(a.Message, b.Message) {
		return false
	}
	if !toolCallsEqual(a.ToolCalls, b.ToolCalls) {
		return false
	}
	if !rawEqual(a.Raw, b.Raw) {
		return false
	}
	return true
}

func messagesEqual(a, b agent.Message) bool {
	return agentMessageEqual(a, b)
}

func agentMessageEqual(a, b agent.Message) bool {
	if a.Role != b.Role || a.Name != b.Name || a.ToolCallID != b.ToolCallID {
		return false
	}
	if len(a.Parts) != len(b.Parts) {
		return false
	}
	for i := range a.Parts {
		if !contentPartEqual(a.Parts[i], b.Parts[i]) {
			return false
		}
	}
	if len(a.Metadata) != len(b.Metadata) {
		return false
	}
	for k, v := range a.Metadata {
		if b.Metadata == nil {
			return false
		}
		if !deepEqual(v, b.Metadata[k]) {
			return false
		}
	}
	return true
}

func contentPartEqual(a, b agent.ContentPart) bool {
	if a.Type != b.Type ||
		a.Text != b.Text ||
		a.URI != b.URI ||
		a.MIMEType != b.MIMEType ||
		a.Name != b.Name ||
		a.Detail != b.Detail {
		return false
	}
	if !bytesEqual(a.Data, b.Data) {
		return false
	}
	if !intPtrEqual(a.Size, b.Size) {
		return false
	}
	if !deepEqual(a.JSON, b.JSON) {
		return false
	}
	if len(a.Metadata) != len(b.Metadata) {
		return false
	}
	for k, v := range a.Metadata {
		if b.Metadata == nil {
			return false
		}
		if !deepEqual(v, b.Metadata[k]) {
			return false
		}
	}
	return true
}

func toolCallsEqual(a, b []ToolCall) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].ToolID != b[i].ToolID {
			return false
		}
		if !rawEqual(a[i].Arguments, b[i].Arguments) {
			return false
		}
	}
	return true
}

func rawEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func bytesEqual(a, b []byte) bool {
	return rawEqual(a, b)
}

func intPtrEqual(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}
