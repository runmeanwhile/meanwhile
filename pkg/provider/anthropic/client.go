package anthropic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

const (
	defaultBaseURL = "https://api.anthropic.com"
	messagesPath   = "/v1/messages"
	apiVersion     = "2023-06-01"
)

var ErrMissingAPIKey = errors.New("missing api key")

type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, ErrMissingAPIKey
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 5 * time.Minute
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{apiKey: cfg.APIKey, baseURL: baseURL, http: httpClient}, nil
}

func FromEnv() (*Client, error) {
	return NewClient(Config{
		APIKey:  os.Getenv("ANTHROPIC_API_KEY"),
		BaseURL: os.Getenv("ANTHROPIC_BASE_URL"),
	})
}

func (c *Client) ID() string { return "anthropic" }

func (c *Client) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	payload, err := buildPayload(req)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+messagesPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)
	httpReq.Header.Set("content-type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer func() { _ = resp.Body.Close() }()
		var payloadErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&payloadErr)
		if payloadErr.Error.Message == "" {
			payloadErr.Error.Message = resp.Status
		}
		return nil, fmt.Errorf("anthropic error: %s", payloadErr.Error.Message)
	}
	return newStream(resp.Body), nil
}

func buildPayload(req provider.Request) (map[string]any, error) {
	system, messages, err := buildMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model":      req.Model,
		"messages":   messages,
		"stream":     true,
		"max_tokens": 1024,
	}
	if system != "" {
		payload["system"] = system
	}
	if len(req.Tools) > 0 {
		payload["tools"] = buildTools(req.Tools)
	}
	for key, value := range req.Params {
		switch key {
		case "model", "messages", "stream", "tools", "system":
			continue
		case "max_output_tokens":
			payload["max_tokens"] = value
		default:
			payload[key] = value
		}
	}
	return payload, nil
}

func buildMessages(messages []modelruntime.Message) (string, []map[string]any, error) {
	systemParts := make([]string, 0)
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case modelruntime.RoleSystem:
			if text := msg.Text(); text != "" {
				systemParts = append(systemParts, text)
			}
		case modelruntime.RoleUser:
			out = append(out, map[string]any{"role": "user", "content": buildContent(msg.Parts)})
		case modelruntime.RoleAssistant:
			if text := msg.Text(); text != "" {
				out = append(out, map[string]any{"role": "assistant", "content": text})
			}
		case modelruntime.RoleTool:
			out = append(out, anthropicToolExchange(msg)...)
		}
	}
	return strings.Join(systemParts, "\n\n"), out, nil
}

func anthropicToolExchange(msg modelruntime.Message) []map[string]any {
	args := "{}"
	if msg.Metadata != nil {
		switch value := msg.Metadata["arguments"].(type) {
		case json.RawMessage:
			if len(value) > 0 {
				args = string(value)
			}
		case string:
			if strings.TrimSpace(value) != "" {
				args = value
			}
		}
	}
	var input any = map[string]any{}
	_ = json.Unmarshal([]byte(args), &input)
	name := toolName(msg)
	return []map[string]any{
		{"role": "assistant", "content": []map[string]any{{
			"type":  "tool_use",
			"id":    msg.ToolCallID,
			"name":  name,
			"input": input,
		}}},
		{"role": "user", "content": []map[string]any{{
			"type":        "tool_result",
			"tool_use_id": msg.ToolCallID,
			"content":     anthropicToolResultContent(msg.Parts),
		}}},
	}
}

func buildContent(parts []modelruntime.Part) any {
	if len(parts) == 0 {
		return ""
	}
	out := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		switch strings.ToLower(string(part.Type)) {
		case "image", "input_image":
			if image := anthropicImage(part); image != nil {
				out = append(out, image)
			}
		default:
			if part.Text != "" {
				out = append(out, map[string]any{"type": "text", "text": part.Text})
			}
		}
	}
	if len(out) == 1 {
		if out[0]["type"] == "text" {
			return out[0]["text"]
		}
	}
	return out
}

func anthropicImage(part modelruntime.Part) map[string]any {
	uri := part.URI
	if uri == "" && len(part.Data) > 0 {
		mimeType := strings.TrimSpace(part.MIMEType)
		if mimeType == "" {
			mimeType = "image/png"
		}
		uri = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(part.Data)
	}
	if uri == "" || !strings.HasPrefix(uri, "data:") {
		return nil
	}
	header, data, ok := strings.Cut(uri, ",")
	if !ok {
		return nil
	}
	mediaType := strings.TrimPrefix(strings.TrimSuffix(header, ";base64"), "data:")
	if mediaType == "" {
		mediaType = mime.TypeByExtension(".jpg")
	}
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": mediaType,
			"data":       data,
		},
	}
}

func anthropicToolResultContent(parts []modelruntime.Part) any {
	content := buildContent(parts)
	if text, ok := content.(string); ok {
		if text != "" {
			return text
		}
		return "{}"
	}
	return content
}

func buildTools(defs []modelruntime.ToolDefinition) []map[string]any {
	tools := make([]map[string]any, 0, len(defs))
	for _, def := range defs {
		var schema any = map[string]any{"type": "object", "properties": map[string]any{}}
		if len(def.JSONSchema) > 0 {
			_ = json.Unmarshal(def.JSONSchema, &schema)
		}
		tool := map[string]any{"name": def.ID, "input_schema": schema}
		if def.Description != "" {
			tool["description"] = def.Description
		}
		tools = append(tools, tool)
	}
	return tools
}

func toolName(msg modelruntime.Message) string {
	if msg.Name != "" {
		return msg.Name
	}
	if msg.Metadata != nil {
		if name, ok := msg.Metadata["tool_name"].(string); ok {
			return name
		}
	}
	return "tool"
}

func init() {
	provider.RegisterFactory("anthropic", func(cfg config.ProviderConfig) (provider.Provider, error) {
		apiKey := cfg.APIKey
		baseURL := cfg.BaseURL
		if cfg.Params != nil {
			if value, ok := cfg.Params["api_key"].(string); ok && value != "" {
				apiKey = value
			}
			if value, ok := cfg.Params["base_url"].(string); ok && value != "" {
				baseURL = value
			}
		}
		return NewClient(Config{APIKey: apiKey, BaseURL: baseURL})
	})
}
