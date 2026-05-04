package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
)

const (
	defaultBaseURL   = "https://api.anthropic.com"
	messagesPath     = "/v1/messages"
	apiVersion       = "2023-06-01"
	defaultTimeout   = 5 * time.Minute
	defaultMaxTokens = 512
)

var (
	// ErrMissingAPIKey indicates a missing API key.
	ErrMissingAPIKey = errors.New("missing api key")
)

// Config configures the Anthropic client.
type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// Client implements the Anthropic Messages API through the shared modelruntime.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewClient creates a new Anthropic modelruntime provider.
func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, ErrMissingAPIKey
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
	}, nil
}

// ID returns the provider ID.
func (c *Client) ID() string { return "anthropic" }

// Stream sends one Anthropic request and returns a single-shot stream.
func (c *Client) Stream(ctx context.Context, req modelruntime.Request) (modelruntime.Stream, error) {
	payload, err := buildRequestPayload(req)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+messagesPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build anthropic request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, decodeAnthropicError(resp.StatusCode, responseBody)
	}

	var payloadResponse anthropicResponse
	if err := json.Unmarshal(responseBody, &payloadResponse); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}

	message := modelruntime.Message{
		Role:  modelruntime.RoleAssistant,
		Parts: extractParts(payloadResponse.Content),
	}
	return newStaticStream(message), nil
}

type anthropicRequest struct {
	Model       string                    `json:"model"`
	MaxTokens   int                       `json:"max_tokens"`
	Temperature float64                   `json:"temperature,omitempty"`
	System      string                    `json:"system,omitempty"`
	Messages    []anthropicMessageRequest `json:"messages"`
}

type anthropicMessageRequest struct {
	Role    string                  `json:"role"`
	Content []anthropicContentInput `json:"content"`
}

type anthropicContentInput struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type anthropicResponse struct {
	Content []anthropicContentOutput `json:"content"`
}

type anthropicContentOutput struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type anthropicErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func buildRequestPayload(req modelruntime.Request) (anthropicRequest, error) {
	if strings.TrimSpace(req.Model) == "" {
		return anthropicRequest{}, fmt.Errorf("model required")
	}
	messages := make([]anthropicMessageRequest, 0, len(req.Messages))
	for _, message := range req.Messages {
		content := buildMessageContent(message)
		if len(content) == 0 {
			continue
		}
		role := string(message.Role)
		if role == "" {
			role = string(modelruntime.RoleUser)
		}
		if message.Role == modelruntime.RoleTool {
			role = string(modelruntime.RoleUser)
		}
		messages = append(messages, anthropicMessageRequest{
			Role:    role,
			Content: content,
		})
	}
	maxTokens := defaultMaxTokens
	temperature := 0.0
	for key, value := range req.Params {
		switch key {
		case "max_tokens":
			switch typed := value.(type) {
			case int:
				if typed > 0 {
					maxTokens = typed
				}
			case float64:
				if typed > 0 {
					maxTokens = int(typed)
				}
			}
		case "temperature":
			switch typed := value.(type) {
			case float64:
				if typed >= 0 {
					temperature = typed
				}
			case int:
				if typed >= 0 {
					temperature = float64(typed)
				}
			}
		}
	}

	return anthropicRequest{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Messages:    messages,
	}, nil
}

func buildMessageContent(message modelruntime.Message) []anthropicContentInput {
	content := make([]anthropicContentInput, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch part.Type {
		case modelruntime.PartText:
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			content = append(content, anthropicContentInput{
				Type: "text",
				Text: part.Text,
			})
		case modelruntime.PartJSON:
			if part.JSON == nil {
				continue
			}
			raw, err := json.Marshal(part.JSON)
			if err != nil {
				continue
			}
			content = append(content, anthropicContentInput{
				Type: "text",
				Text: string(raw),
			})
		default:
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			content = append(content, anthropicContentInput{
				Type: "text",
				Text: part.Text,
			})
		}
	}
	return content
}

func extractParts(content []anthropicContentOutput) []modelruntime.Part {
	parts := make([]modelruntime.Part, 0, len(content))
	for _, part := range content {
		if strings.TrimSpace(part.Type) != "text" || strings.TrimSpace(part.Text) == "" {
			continue
		}
		parts = append(parts, modelruntime.Part{
			Type: modelruntime.PartText,
			Text: part.Text,
		})
	}
	return parts
}

type staticStream struct {
	message modelruntime.Message
	sent    bool
}

func newStaticStream(message modelruntime.Message) modelruntime.Stream {
	return &staticStream{message: message}
}

func (s *staticStream) Recv() (modelruntime.Event, error) {
	if s.sent {
		return modelruntime.Event{}, io.EOF
	}
	s.sent = true
	return modelruntime.Event{
		Type:    modelruntime.EventMessageCompleted,
		Message: s.message,
	}, nil
}

func (s *staticStream) Close() error { return nil }

func decodeAnthropicError(statusCode int, body []byte) error {
	var payload anthropicErrorResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		if message := strings.TrimSpace(payload.Error.Message); message != "" {
			return fmt.Errorf("anthropic error %d (%s): %s", statusCode, strings.TrimSpace(payload.Error.Type), message)
		}
	}
	bodyText := strings.TrimSpace(string(body))
	if bodyText == "" {
		bodyText = http.StatusText(statusCode)
	}
	return fmt.Errorf("anthropic error %d: %s", statusCode, bodyText)
}
