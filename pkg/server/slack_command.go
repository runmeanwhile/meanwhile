package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
	"github.com/darkostanimirovic/meanwhile/pkg/message"
	"github.com/slack-go/slack"
)

// SlackCommandHandler handles Slack slash commands for responses.
type SlackCommandHandler struct {
	Engine        *engine.Engine
	SigningSecret string
	Command       string
	MaxBodyBytes  int64
}

// ServeHTTP implements http.Handler for Slack slash commands.
func (h *SlackCommandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if strings.TrimSpace(h.SigningSecret) != "" {
		verifier, err := slack.NewSecretsVerifier(r.Header, h.SigningSecret)
		if err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		if _, err := verifier.Write(body); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		if err := verifier.Ensure(); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if expected := strings.TrimSpace(h.Command); expected != "" {
		if cmd := strings.TrimSpace(values.Get("command")); cmd != expected {
			http.Error(w, "unknown command", http.StatusBadRequest)
			return
		}
	}

	text := strings.TrimSpace(values.Get("text"))
	requestID, responseText, err := parseSlackResponseText(text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	sess, err := h.Engine.SessionForRequest(ctx, requestID)
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

	resp := message.User(responseText)
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]any)
	}
	resp.Metadata["request_id"] = requestID
	resp.Metadata["source"] = "slack"
	if user := strings.TrimSpace(values.Get("user_name")); user != "" {
		resp.Metadata["responder"] = user
	}
	resp.Metadata["received_at"] = time.Now().UTC()

	if _, err := sess.Respond(ctx, requestID, resp); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, engine.ErrRequestNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("Response received."))
}

func parseSlackResponseText(text string) (string, string, error) {
	if text == "" {
		return "", "", fmt.Errorf("command text required")
	}
	parts := strings.Fields(text)
	if len(parts) < 3 {
		return "", "", fmt.Errorf("expected: respond <request_id> <response>")
	}
	if parts[0] != "respond" {
		return "", "", fmt.Errorf("expected respond command")
	}
	requestID := parts[1]
	idx := strings.Index(text, requestID)
	if idx == -1 {
		return "", "", fmt.Errorf("request id missing")
	}
	response := strings.TrimSpace(text[idx+len(requestID):])
	response = strings.TrimSpace(response)
	if response == "" {
		return "", "", fmt.Errorf("response text required")
	}
	return requestID, response, nil
}
