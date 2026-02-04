package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
)

const signatureHeader = "X-Meanwhile-Signature"

// HumanResponsePayload captures an inbound human response.
type HumanResponsePayload struct {
	RequestID string    `json:"request_id"`
	Response  string    `json:"response"`
	Responder string    `json:"responder,omitempty"`
	Source    string    `json:"source,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Signature string    `json:"signature,omitempty"`
}

// HumanResponseHandler handles inbound human responses via webhook.
type HumanResponseHandler struct {
	Engine       *engine.Engine
	Verifier     SignatureVerifier
	MaxBodyBytes int64
}

// ServeHTTP implements http.Handler.
func (h *HumanResponseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.Engine == nil {
		http.Error(w, "engine required", http.StatusInternalServerError)
		return
	}

	maxBytes := h.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	body, err := readBody(r, maxBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var payload HumanResponsePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	signature := strings.TrimSpace(r.Header.Get(signatureHeader))
	if signature == "" {
		signature = strings.TrimSpace(payload.Signature)
	}
	if h.Verifier != nil {
		if err := h.Verifier.Verify(r.Context(), body, signature); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.Response = strings.TrimSpace(payload.Response)
	if payload.RequestID == "" || payload.Response == "" {
		http.Error(w, "request_id and response required", http.StatusBadRequest)
		return
	}

	sess, err := h.Engine.SessionForRequest(r.Context(), payload.RequestID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, engine.ErrRequestNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, engine.ErrRequestRegistryRequired) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}

	resp := message.User(payload.Response)
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]any)
	}
	resp.Metadata["request_id"] = payload.RequestID
	if payload.Source != "" {
		resp.Metadata["source"] = payload.Source
	}
	if payload.Responder != "" {
		resp.Metadata["responder"] = payload.Responder
	}
	if !payload.Timestamp.IsZero() {
		resp.Metadata["received_at"] = payload.Timestamp
	} else {
		resp.Metadata["received_at"] = time.Now().UTC()
	}

	result, err := sess.Respond(r.Context(), payload.RequestID, resp)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, engine.ErrRequestNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	response := map[string]any{
		"status":     "ok",
		"request_id": payload.RequestID,
		"session_id": sess.ID(),
		"run_status": result.Status,
	}
	writeJSON(w, response)
}

func readBody(r *http.Request, maxBytes int64) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("request required")
	}
	defer func() {
		_ = r.Body.Close()
	}()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}
	return body, nil
}

func writeJSON(w http.ResponseWriter, payload any) {
	if w == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
}
