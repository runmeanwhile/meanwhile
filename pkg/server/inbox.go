package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
)

const defaultInboxLimit = 100

// HumanRequestInboxHandler lists human requests for simple inbox views.
type HumanRequestInboxHandler struct {
	Engine   *engine.Engine
	MaxLimit int
}

// ServeHTTP implements http.Handler.
func (h *HumanRequestInboxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.Engine == nil {
		http.Error(w, "engine required", http.StatusInternalServerError)
		return
	}

	filter, err := parseInboxFilter(r, h.MaxLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	requests, err := h.Engine.ListHumanRequests(r.Context(), filter)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, engine.ErrHumanRequestStoreRequired) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}

	payload := map[string]any{
		"count":    len(requests),
		"requests": requests,
	}
	writeJSON(w, payload)
}

func parseInboxFilter(r *http.Request, maxLimit int) (engine.HumanRequestFilter, error) {
	if r == nil {
		return engine.HumanRequestFilter{}, errors.New("request required")
	}
	query := r.URL.Query()
	filter := engine.HumanRequestFilter{
		SessionID:     strings.TrimSpace(query.Get("session_id")),
		ParticipantID: strings.TrimSpace(query.Get("participant_id")),
		Limit:         defaultInboxLimit,
	}

	statuses, err := parseStatusList(query["status"])
	if err != nil {
		return engine.HumanRequestFilter{}, err
	}
	filter.Statuses = statuses

	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			return engine.HumanRequestFilter{}, errors.New("limit must be a positive integer")
		}
		filter.Limit = limit
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultInboxLimit
	}
	if maxLimit > 0 && filter.Limit > maxLimit {
		filter.Limit = maxLimit
	}

	if raw := strings.TrimSpace(query.Get("requested_after")); raw != "" {
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return engine.HumanRequestFilter{}, errors.New("requested_after must be RFC3339")
		}
		filter.RequestedAfter = ts
	}
	if raw := strings.TrimSpace(query.Get("requested_before")); raw != "" {
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return engine.HumanRequestFilter{}, errors.New("requested_before must be RFC3339")
		}
		filter.RequestedBefore = ts
	}
	return filter, nil
}

func parseStatusList(values []string) ([]engine.HumanRequestStatus, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]engine.HumanRequestStatus, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			status, err := engine.ParseHumanRequestStatus(part)
			if err != nil {
				return nil, err
			}
			out = append(out, status)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
