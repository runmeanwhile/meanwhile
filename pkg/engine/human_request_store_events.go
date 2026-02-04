package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
)

func (e *Engine) subscribeHumanRequestStore(sess *Session) {
	if e == nil || sess == nil || sess.bus == nil || e.humanRequestStore == nil {
		return
	}
	_, _ = sess.bus.Subscribe(func(ev event.Event) {
		switch ev.Type {
		case event.AwaitingUserInput,
			event.HumanRequestCreated,
			event.HumanRequestSent,
			event.HumanRequestFailed,
			event.HumanRequestTimedOut,
			event.HumanResponseReceived:
			go e.applyHumanRequestEvent(context.Background(), sess, ev)
		default:
			return
		}
	})
}

func (e *Engine) applyHumanRequestEvent(ctx context.Context, sess *Session, ev event.Event) {
	if e == nil || e.humanRequestStore == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	switch ev.Type {
	case event.AwaitingUserInput:
		req, ok := inputRequestFromPayload(ev.Payload)
		if !ok || req.RequestID == "" {
			return
		}
		record, err := loadHumanRequestRecord(ctx, e.humanRequestStore, req.RequestID, ev.SessionID)
		if err != nil {
			return
		}
		record.Request = mergeHumanRequest(record.Request, humanRequestFromInput(req, sess))
		if allowStatusUpdate(record.Status, HumanRequestStatusPending) {
			record.Status = HumanRequestStatusPending
			record.StatusUpdatedAt = ev.Time
		}
		_ = e.humanRequestStore.Upsert(ctx, record)
	case event.HumanRequestCreated:
		req, ok := humanRequestFromPayload(ev.Payload)
		if !ok || req.RequestID == "" {
			return
		}
		record, err := loadHumanRequestRecord(ctx, e.humanRequestStore, req.RequestID, ev.SessionID)
		if err != nil {
			return
		}
		record.Request = req
		if allowStatusUpdate(record.Status, HumanRequestStatusPending) {
			record.Status = HumanRequestStatusPending
			record.StatusUpdatedAt = ev.Time
		}
		_ = e.humanRequestStore.Upsert(ctx, record)
	case event.HumanRequestSent:
		updateHumanRequestDispatch(ctx, e.humanRequestStore, ev)
	case event.HumanRequestFailed:
		updateHumanRequestFailure(ctx, e.humanRequestStore, ev)
	case event.HumanRequestTimedOut:
		updateHumanRequestTimeout(ctx, e.humanRequestStore, ev)
	case event.HumanResponseReceived:
		updateHumanRequestResponse(ctx, e.humanRequestStore, ev)
	}
}

func updateHumanRequestDispatch(ctx context.Context, store HumanRequestStore, ev event.Event) {
	requestID, dispatch := dispatchFromPayload(ev)
	if requestID == "" || dispatch == nil {
		return
	}
	record, err := loadHumanRequestRecord(ctx, store, requestID, ev.SessionID)
	if err != nil {
		return
	}
	record.Dispatch = dispatch
	if allowStatusUpdate(record.Status, HumanRequestStatusSent) {
		record.Status = HumanRequestStatusSent
		record.StatusUpdatedAt = ev.Time
		record.Failure = nil
	}
	if err := store.Upsert(ctx, record); err != nil {
		return
	}
}

func updateHumanRequestFailure(ctx context.Context, store HumanRequestStore, ev event.Event) {
	requestID, failure := failureFromPayload(ev)
	if requestID == "" || failure == nil {
		return
	}
	record, err := loadHumanRequestRecord(ctx, store, requestID, ev.SessionID)
	if err != nil {
		return
	}
	if allowStatusUpdate(record.Status, HumanRequestStatusFailed) {
		record.Status = HumanRequestStatusFailed
		record.StatusUpdatedAt = ev.Time
		record.Failure = failure
	}
	if err := store.Upsert(ctx, record); err != nil {
		return
	}
}

func updateHumanRequestTimeout(ctx context.Context, store HumanRequestStore, ev event.Event) {
	requestID := requestIDFromPayload(ev.Payload)
	if requestID == "" {
		return
	}
	record, err := loadHumanRequestRecord(ctx, store, requestID, ev.SessionID)
	if err != nil {
		return
	}
	if allowStatusUpdate(record.Status, HumanRequestStatusTimedOut) {
		record.Status = HumanRequestStatusTimedOut
		record.StatusUpdatedAt = ev.Time
		record.TimedOutAt = ev.Time
	}
	if err := store.Upsert(ctx, record); err != nil {
		return
	}
}

func updateHumanRequestResponse(ctx context.Context, store HumanRequestStore, ev event.Event) {
	response, ok := humanResponseFromPayload(ev.Payload)
	if !ok || response.RequestID == "" {
		return
	}
	record, err := loadHumanRequestRecord(ctx, store, response.RequestID, ev.SessionID)
	if err != nil {
		return
	}
	if allowStatusUpdate(record.Status, HumanRequestStatusAnswered) {
		record.Status = HumanRequestStatusAnswered
		record.StatusUpdatedAt = ev.Time
		record.Response = &response
	}
	if err := store.Upsert(ctx, record); err != nil {
		return
	}
}

func loadHumanRequestRecord(ctx context.Context, store HumanRequestStore, requestID, sessionID string) (HumanRequestRecord, error) {
	if store == nil {
		return HumanRequestRecord{}, ErrHumanRequestStoreRequired
	}
	record, err := store.Get(ctx, requestID)
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, ErrHumanRequestNotFound) && !errors.Is(err, ErrRequestNotFound) {
		return HumanRequestRecord{}, err
	}
	record = HumanRequestRecord{
		Request: HumanRequest{
			RequestID: requestID,
			SessionID: sessionID,
		},
		Status:          HumanRequestStatusPending,
		StatusUpdatedAt: time.Now().UTC(),
	}
	return record, nil
}

func allowStatusUpdate(current, next HumanRequestStatus) bool {
	if current == "" || current == next {
		return true
	}
	if current == HumanRequestStatusAnswered || current == HumanRequestStatusTimedOut {
		return false
	}
	return statusOrder(next) >= statusOrder(current)
}

func statusOrder(status HumanRequestStatus) int {
	switch status {
	case HumanRequestStatusPending:
		return 1
	case HumanRequestStatusFailed:
		return 2
	case HumanRequestStatusSent:
		return 3
	case HumanRequestStatusAnswered, HumanRequestStatusTimedOut:
		return 4
	default:
		return 0
	}
}

func dispatchFromPayload(ev event.Event) (string, *HumanRequestDispatch) {
	payload, ok := ev.Payload.(map[string]any)
	if !ok {
		return "", nil
	}
	requestID, _ := payload["request_id"].(string)
	if requestID == "" {
		return "", nil
	}
	dispatch := &HumanRequestDispatch{
		IntegrationID: stringFromPayload(payload["integration"]),
		Channel:       stringFromPayload(payload["channel"]),
		Contact:       stringFromPayload(payload["contact"]),
	}
	dispatch.DispatchedAt = timeFromPayload(payload["dispatched_at"], ev.Time)
	return requestID, dispatch
}

func failureFromPayload(ev event.Event) (string, *HumanRequestFailure) {
	payload, ok := ev.Payload.(map[string]any)
	if !ok {
		return "", nil
	}
	requestID, _ := payload["request_id"].(string)
	if requestID == "" {
		return "", nil
	}
	errText := stringFromPayload(payload["error"])
	if errText == "" {
		errText = "dispatch failed"
	}
	return requestID, &HumanRequestFailure{
		Error:    errText,
		FailedAt: timeFromPayload(payload["failed_at"], ev.Time),
	}
}

func requestIDFromPayload(payload any) string {
	switch value := payload.(type) {
	case protocol.InputRequest:
		return value.RequestID
	case *protocol.InputRequest:
		if value == nil {
			return ""
		}
		return value.RequestID
	case map[string]any:
		if raw, ok := value["request_id"].(string); ok {
			return raw
		}
	}
	return ""
}

func inputRequestFromPayload(payload any) (protocol.InputRequest, bool) {
	switch value := payload.(type) {
	case protocol.InputRequest:
		return value, true
	case *protocol.InputRequest:
		if value == nil {
			return protocol.InputRequest{}, false
		}
		return *value, true
	case map[string]any:
		raw, err := json.Marshal(value)
		if err != nil {
			return protocol.InputRequest{}, false
		}
		var req protocol.InputRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return protocol.InputRequest{}, false
		}
		return req, true
	default:
		return protocol.InputRequest{}, false
	}
}

func humanResponseFromPayload(payload any) (HumanResponse, bool) {
	switch value := payload.(type) {
	case HumanResponse:
		return value, true
	case *HumanResponse:
		if value == nil {
			return HumanResponse{}, false
		}
		return *value, true
	case map[string]any:
		raw, err := json.Marshal(value)
		if err != nil {
			return HumanResponse{}, false
		}
		var resp HumanResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return HumanResponse{}, false
		}
		return resp, true
	default:
		return HumanResponse{}, false
	}
}

func stringFromPayload(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func timeFromPayload(value any, fallback time.Time) time.Time {
	switch v := value.(type) {
	case time.Time:
		if v.IsZero() {
			return fallback
		}
		return v
	case string:
		if v == "" {
			return fallback
		}
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func humanRequestFromInput(req protocol.InputRequest, sess *Session) HumanRequest {
	request := HumanRequest{
		RequestID:       req.RequestID,
		SessionID:       "",
		ProtocolID:      "",
		ParticipantID:   req.ParticipantID,
		ParticipantName: req.ParticipantName,
		Question:        strings.TrimSpace(req.Context),
		Context:         "",
		RequestedAt:     req.RequestedAt,
		TimeoutAt:       req.TimeoutAt,
		Required:        true,
	}
	if sess != nil {
		request.SessionID = sess.id
		if sess.protocol != nil {
			request.ProtocolID = sess.protocol.ID()
		}
	}
	return request
}

func mergeHumanRequest(current, incoming HumanRequest) HumanRequest {
	if current.RequestID == "" {
		current.RequestID = incoming.RequestID
	}
	if current.SessionID == "" {
		current.SessionID = incoming.SessionID
	}
	if current.ProtocolID == "" {
		current.ProtocolID = incoming.ProtocolID
	}
	if current.ParticipantID == "" {
		current.ParticipantID = incoming.ParticipantID
	}
	if current.ParticipantName == "" {
		current.ParticipantName = incoming.ParticipantName
	}
	if current.Question == "" {
		current.Question = incoming.Question
	}
	if current.Context == "" {
		current.Context = incoming.Context
	}
	if current.Urgency == "" {
		current.Urgency = incoming.Urgency
	}
	if current.Required == false && incoming.Required {
		current.Required = incoming.Required
	}
	if current.RequestedAt.IsZero() {
		current.RequestedAt = incoming.RequestedAt
	}
	if current.TimeoutAt.IsZero() {
		current.TimeoutAt = incoming.TimeoutAt
	}
	if len(current.SuggestedResponses) == 0 && len(incoming.SuggestedResponses) > 0 {
		current.SuggestedResponses = append([]string(nil), incoming.SuggestedResponses...)
	}
	if current.ToolCallID == "" {
		current.ToolCallID = incoming.ToolCallID
	}
	if current.ToolID == "" {
		current.ToolID = incoming.ToolID
	}
	if current.AgentID == "" {
		current.AgentID = incoming.AgentID
	}
	return current
}
