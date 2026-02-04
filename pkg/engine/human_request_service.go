package engine

import (
	"context"
)

// GetHumanRequest retrieves a single human request record.
func (e *Engine) GetHumanRequest(ctx context.Context, requestID string) (HumanRequestRecord, error) {
	if e == nil || e.humanRequestStore == nil {
		return HumanRequestRecord{}, ErrHumanRequestStoreRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return e.humanRequestStore.Get(ctx, requestID)
}

// ListHumanRequests returns human request records for inbox-style views.
func (e *Engine) ListHumanRequests(ctx context.Context, filter HumanRequestFilter) ([]HumanRequestRecord, error) {
	if e == nil || e.humanRequestStore == nil {
		return nil, ErrHumanRequestStoreRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return e.humanRequestStore.List(ctx, filter)
}

// DispatchHumanRequests attempts delivery for pending requests in the store.
func (e *Engine) DispatchHumanRequests(ctx context.Context, filter HumanRequestFilter) (int, error) {
	if e == nil || e.humanRequestStore == nil {
		return 0, ErrHumanRequestStoreRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if len(filter.Statuses) == 0 {
		filter.Statuses = []HumanRequestStatus{HumanRequestStatusPending, HumanRequestStatusFailed}
	}
	records, err := e.humanRequestStore.List(ctx, filter)
	if err != nil {
		return 0, err
	}
	dispatched := 0
	for _, record := range records {
		switch record.Status {
		case HumanRequestStatusSent, HumanRequestStatusAnswered, HumanRequestStatusTimedOut:
			continue
		}
		if record.Request.RequestID == "" || record.Request.SessionID == "" {
			continue
		}
		sess, err := e.session(ctx, record.Request.SessionID)
		if err != nil {
			return dispatched, err
		}
		e.dispatchHumanRequest(ctx, sess, record.Request)
		dispatched++
	}
	return dispatched, nil
}
