package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const webhookChannel = "webhook"

// WebhookOption configures the webhook integration.
type WebhookOption func(*WebhookIntegration)

// WebhookIntegration posts human requests to HTTP endpoints.
type WebhookIntegration struct {
	id      string
	client  *http.Client
	headers map[string]string
}

// NewWebhook creates a webhook integration.
func NewWebhook(opts ...WebhookOption) *WebhookIntegration {
	integration := &WebhookIntegration{
		id:     "webhook",
		client: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(integration)
		}
	}
	return integration
}

// WithWebhookID overrides the integration ID.
func WithWebhookID(id string) WebhookOption {
	return func(w *WebhookIntegration) {
		w.id = id
	}
}

// WithWebhookClient sets the HTTP client.
func WithWebhookClient(client *http.Client) WebhookOption {
	return func(w *WebhookIntegration) {
		if client != nil {
			w.client = client
		}
	}
}

// WithWebhookHeaders sets default headers for webhook requests.
func WithWebhookHeaders(headers map[string]string) WebhookOption {
	return func(w *WebhookIntegration) {
		if len(headers) == 0 {
			return
		}
		w.headers = make(map[string]string, len(headers))
		for key, value := range headers {
			w.headers[key] = value
		}
	}
}

// ID returns the integration ID.
func (w *WebhookIntegration) ID() string { return w.id }

// Channel returns the channel identifier for webhooks.
func (w *WebhookIntegration) Channel() string { return webhookChannel }

// Send posts the request to the webhook URL.
func (w *WebhookIntegration) Send(ctx context.Context, req Request) error {
	if w == nil {
		return fmt.Errorf("webhook integration required")
	}
	if w.client == nil {
		return fmt.Errorf("webhook http client required")
	}
	if req.Contact == "" {
		return fmt.Errorf("webhook url required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	parsed, err := url.Parse(req.Contact)
	if err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported webhook scheme: %s", parsed.Scheme)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode webhook request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.Contact, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range w.headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := w.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("webhook post: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status: %s", resp.Status)
	}
	return nil
}
