package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/integration"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

type contactProvider interface {
	Contacts() map[string]string
	PreferredChannel() string
}

func (e *Engine) subscribeIntegrations(sess *Session) {
	if e == nil || sess == nil || sess.bus == nil {
		return
	}
	_, _ = sess.bus.Subscribe(func(ev event.Event) {
		if ev.Type != event.HumanRequestCreated {
			return
		}
		req, ok := humanRequestFromPayload(ev.Payload)
		if !ok {
			return
		}
		go e.dispatchHumanRequest(context.Background(), sess, req)
	})
}

func (e *Engine) dispatchHumanRequest(ctx context.Context, sess *Session, req HumanRequest) {
	if e == nil || sess == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if e.humanRequestStore != nil && req.RequestID != "" {
		record, err := e.humanRequestStore.Get(ctx, req.RequestID)
		if err == nil {
			switch record.Status {
			case HumanRequestStatusSent, HumanRequestStatusAnswered, HumanRequestStatusTimedOut:
				return
			}
		}
	}
	if e.integrations == nil {
		_ = sess.EmitWithContext(ctx, event.New(event.HumanRequestFailed, sess.id, map[string]any{
			"request_id": req.RequestID,
			"error":      "no integrations registered",
		}))
		return
	}

	participant := resolveHumanParticipant(sess.participants, req.ParticipantID)
	contacts, preferred := humanContacts(participant)
	if len(contacts) == 0 {
		_ = sess.EmitWithContext(ctx, event.New(event.HumanRequestFailed, sess.id, map[string]any{
			"request_id": req.RequestID,
			"error":      "no contact channels configured",
		}))
		return
	}

	router := integration.NewRouter(e.integrations)
	dispatchReq := integration.Request{
		RequestID:          req.RequestID,
		SessionID:          req.SessionID,
		ProtocolID:         req.ProtocolID,
		ParticipantID:      req.ParticipantID,
		ParticipantName:    req.ParticipantName,
		Question:           req.Question,
		Context:            req.Context,
		Urgency:            req.Urgency,
		Required:           req.Required,
		SuggestedResponses: req.SuggestedResponses,
		RequestedAt:        req.RequestedAt,
		TimeoutAt:          req.TimeoutAt,
		ToolCallID:         req.ToolCallID,
		ToolID:             req.ToolID,
		AgentID:            req.AgentID,
	}

	result, err := router.Dispatch(ctx, dispatchReq, contacts, preferred)
	if err != nil {
		_ = sess.EmitWithContext(ctx, event.New(event.HumanRequestFailed, sess.id, map[string]any{
			"request_id": req.RequestID,
			"error":      err.Error(),
		}))
		return
	}
	_ = sess.EmitWithContext(ctx, event.New(event.HumanRequestSent, sess.id, map[string]any{
		"request_id":    req.RequestID,
		"integration":   result.IntegrationID,
		"channel":       result.Channel,
		"contact":       result.Contact,
		"dispatched_at": time.Now().UTC(),
	}))
}

func humanRequestFromPayload(payload any) (HumanRequest, bool) {
	switch value := payload.(type) {
	case HumanRequest:
		return value, true
	case *HumanRequest:
		if value == nil {
			return HumanRequest{}, false
		}
		return *value, true
	case map[string]any:
		raw, err := json.Marshal(value)
		if err != nil {
			return HumanRequest{}, false
		}
		var req HumanRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return HumanRequest{}, false
		}
		return req, true
	default:
		return HumanRequest{}, false
	}
}

func resolveHumanParticipant(participants []protocol.Participant, key string) protocol.Participant {
	if key == "" {
		return nil
	}
	participant, _ := participantByKey(participants, key)
	return participant
}

func humanContacts(participant protocol.Participant) (map[string]string, string) {
	if participant == nil {
		return nil, ""
	}
	provider, ok := participant.(contactProvider)
	if !ok {
		return nil, ""
	}
	contacts := provider.Contacts()
	if len(contacts) == 0 {
		return nil, ""
	}
	return contacts, provider.PreferredChannel()
}

func (e *Engine) ensureIntegrations() error {
	if e.integrations != nil {
		return nil
	}
	e.integrations = integration.NewRegistry()
	return nil
}

func (e *Engine) registerIntegration(integration integration.Integration) error {
	if integration == nil {
		return fmt.Errorf("integration required")
	}
	if err := e.ensureIntegrations(); err != nil {
		return err
	}
	return e.integrations.Register(integration)
}
