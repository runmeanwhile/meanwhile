package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/scheduler"
)

const (
	timeoutJobType            = "human.request.timeout"
	minTimeoutRescheduleDelay = time.Second
)

// TimeoutRequest describes a scheduled timeout for a pending human request.
type TimeoutRequest struct {
	SessionID string
	RequestID string
	TimeoutAt time.Time
}

// TimeoutScheduler schedules timeout handling for pending requests.
type TimeoutScheduler interface {
	ScheduleTimeout(ctx context.Context, request TimeoutRequest) error
	CancelTimeout(ctx context.Context, requestID string) error
}

// TimeoutService schedules and handles request timeouts using a scheduler driver.
type TimeoutService struct {
	engine *Engine
	driver scheduler.Driver
	worker *scheduler.Worker
}

// NewTimeoutScheduler creates a timeout service backed by a scheduler driver.
func (e *Engine) NewTimeoutScheduler(driver scheduler.Driver, opts ...scheduler.WorkerOption) (*TimeoutService, error) {
	if e == nil {
		return nil, fmt.Errorf("engine required")
	}
	if driver == nil {
		return nil, fmt.Errorf("driver required")
	}
	service := &TimeoutService{
		engine: e,
		driver: driver,
	}
	handler := func(ctx context.Context, job scheduler.Job) error {
		return service.handleJob(ctx, job)
	}
	worker, err := scheduler.NewWorker(driver, handler, opts...)
	if err != nil {
		return nil, err
	}
	service.worker = worker
	return service, nil
}

// Run starts the timeout worker loop.
func (t *TimeoutService) Run(ctx context.Context) error {
	if t == nil || t.worker == nil {
		return nil
	}
	return t.worker.Run(ctx)
}

// Close shuts down the underlying driver.
func (t *TimeoutService) Close() error {
	if t == nil {
		return nil
	}
	if t.worker != nil {
		return t.worker.Close()
	}
	if t.driver != nil {
		return t.driver.Close()
	}
	return nil
}

// ScheduleTimeout schedules a timeout job.
func (t *TimeoutService) ScheduleTimeout(ctx context.Context, request TimeoutRequest) error {
	if t == nil || t.driver == nil {
		return fmt.Errorf("timeout scheduler required")
	}
	if request.RequestID == "" {
		return fmt.Errorf("request id required")
	}
	if request.SessionID == "" {
		return fmt.Errorf("session id required")
	}
	if request.TimeoutAt.IsZero() {
		return fmt.Errorf("timeout_at required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := json.Marshal(timeoutPayload{
		SessionID: request.SessionID,
		RequestID: request.RequestID,
	})
	if err != nil {
		return fmt.Errorf("encode timeout payload: %w", err)
	}
	job := scheduler.Job{
		ID:      request.RequestID,
		Type:    timeoutJobType,
		RunAt:   request.TimeoutAt.UTC(),
		Payload: payload,
	}
	return t.driver.Schedule(ctx, job)
}

// CancelTimeout cancels a scheduled timeout.
func (t *TimeoutService) CancelTimeout(ctx context.Context, requestID string) error {
	if t == nil || t.driver == nil {
		return fmt.Errorf("timeout scheduler required")
	}
	if requestID == "" {
		return fmt.Errorf("request id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := t.driver.Cancel(ctx, requestID); err != nil && !errors.Is(err, scheduler.ErrJobNotFound) {
		return err
	}
	return nil
}

func (t *TimeoutService) handleJob(ctx context.Context, job scheduler.Job) error {
	if job.Type != timeoutJobType {
		return nil
	}
	if len(job.Payload) == 0 {
		return fmt.Errorf("timeout payload required")
	}
	var payload timeoutPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode timeout payload: %w", err)
	}
	if payload.SessionID == "" || payload.RequestID == "" {
		return fmt.Errorf("timeout payload missing session or request id")
	}
	_, err := t.engine.HandleTimeout(ctx, payload.SessionID, payload.RequestID)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrRequestNotFound) ||
		errors.Is(err, ErrRequestNotTimedOut) ||
		errors.Is(err, ErrTimeoutPolicyRequired) ||
		errors.Is(err, ErrResponseRequired) ||
		errors.Is(err, ErrSessionIncomplete) ||
		errors.Is(err, ErrSessionNotResumable) {
		if errors.Is(err, ErrRequestNotTimedOut) {
			return t.rescheduleTimeout(ctx, payload, job.Payload)
		}
		return nil
	}
	return err
}

type timeoutPayload struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
}

func (t *TimeoutService) rescheduleTimeout(ctx context.Context, payload timeoutPayload, rawPayload []byte) error {
	if t == nil || t.driver == nil || t.engine == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sess, err := t.engine.session(ctx, payload.SessionID)
	if err != nil {
		return nil
	}
	req, ok := sess.PendingRequest(payload.RequestID)
	if !ok || req.TimeoutAt.IsZero() {
		return nil
	}
	runAt := req.TimeoutAt.UTC()
	now := time.Now().UTC()
	if !runAt.After(now) {
		runAt = now.Add(minTimeoutRescheduleDelay)
	}
	payloadBytes := rawPayload
	if len(payloadBytes) == 0 {
		payloadBytes, err = json.Marshal(timeoutPayload{
			SessionID: payload.SessionID,
			RequestID: payload.RequestID,
		})
		if err != nil {
			return fmt.Errorf("encode timeout payload: %w", err)
		}
	}
	return t.driver.Schedule(ctx, scheduler.Job{
		ID:      payload.RequestID,
		Type:    timeoutJobType,
		RunAt:   runAt,
		Payload: payloadBytes,
	})
}
