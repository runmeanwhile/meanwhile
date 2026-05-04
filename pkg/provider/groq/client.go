package groq

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

const (
	defaultBaseURL = "https://api.groq.com/openai/v1"
	chatPath       = "/chat/completions"
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
		APIKey:  os.Getenv("GROQ_API_KEY"),
		BaseURL: os.Getenv("GROQ_BASE_URL"),
	})
}

func (c *Client) ID() string { return "groq" }

func (c *Client) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	payload := buildPayload(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("groq request: %w", err)
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
		return nil, fmt.Errorf("groq error: %s", payloadErr.Error.Message)
	}
	return newStream(resp.Body), nil
}

func buildPayload(req provider.Request) map[string]any {
	payload := map[string]any{
		"model":    req.Model,
		"messages": buildMessages(req.Messages),
		"stream":   true,
	}
	if len(req.Tools) > 0 && !hasToolResult(req.Messages) {
		payload["tools"] = buildTools(req.Tools)
		payload["tool_choice"] = "auto"
	}
	for key, value := range req.Params {
		switch key {
		case "model", "messages", "stream", "tools", "tool_choice", "response_format":
			continue
		case "max_output_tokens":
			payload["max_completion_tokens"] = value
		default:
			payload[key] = value
		}
	}
	return payload
}

func hasToolResult(messages []modelruntime.Message) bool {
	for _, msg := range messages {
		if msg.Role == modelruntime.RoleTool {
			return true
		}
	}
	return false
}

func buildMessages(messages []modelruntime.Message) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case modelruntime.RoleSystem:
			out = append(out, map[string]any{"role": string(msg.Role), "content": msg.Text()})
		case modelruntime.RoleUser:
			out = append(out, map[string]any{"role": "user", "content": buildContent(msg.Parts)})
		case modelruntime.RoleAssistant:
			if msg.Text() != "" {
				out = append(out, map[string]any{"role": "assistant", "content": msg.Text()})
			}
		case modelruntime.RoleTool:
			out = append(out, groqToolExchange(msg)...)
		}
	}
	return out
}

func groqToolExchange(msg modelruntime.Message) []map[string]any {
	items := []map[string]any{
		{
			"role":    "assistant",
			"content": nil,
			"tool_calls": []map[string]any{{
				"id":   msg.ToolCallID,
				"type": "function",
				"function": map[string]any{
					"name":      toolName(msg),
					"arguments": toolArguments(msg),
				},
			}},
		},
		{
			"role":         "tool",
			"tool_call_id": msg.ToolCallID,
			"name":         toolName(msg),
			"content":      msg.Text(),
		},
	}
	if parts := nonTextParts(msg.Parts); len(parts) > 0 {
		parts = append([]modelruntime.Part{{Type: modelruntime.PartText, Text: "Tool output:"}}, parts...)
		items = append(items, map[string]any{"role": "user", "content": buildContent(parts)})
	}
	return items
}

func toolArguments(msg modelruntime.Message) string {
	if msg.Metadata != nil {
		switch value := msg.Metadata["arguments"].(type) {
		case json.RawMessage:
			if len(value) > 0 {
				return string(value)
			}
		case []byte:
			if len(value) > 0 {
				return string(value)
			}
		case string:
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return "{}"
}

func buildContent(parts []modelruntime.Part) any {
	if len(parts) == 0 {
		return ""
	}
	out := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		switch strings.ToLower(string(part.Type)) {
		case "image", "input_image":
			if imageURL := imageURLFromPart(part); imageURL != "" {
				image := map[string]any{"url": imageURL}
				if part.Detail != "" {
					image["detail"] = part.Detail
				}
				out = append(out, map[string]any{"type": "image_url", "image_url": image})
			}
		default:
			if part.Text != "" {
				out = append(out, map[string]any{"type": "text", "text": part.Text})
			}
		}
	}
	if len(out) == 1 && out[0]["type"] == "text" {
		return out[0]["text"]
	}
	return out
}

func imageURLFromPart(part modelruntime.Part) string {
	if part.URI != "" {
		return part.URI
	}
	if len(part.Data) == 0 {
		return ""
	}
	mimeType := strings.TrimSpace(part.MIMEType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(part.Data)
}

func nonTextParts(parts []modelruntime.Part) []modelruntime.Part {
	out := make([]modelruntime.Part, 0, len(parts))
	for _, part := range parts {
		if part.Type == modelruntime.PartText {
			continue
		}
		out = append(out, part)
	}
	return out
}

func buildTools(defs []modelruntime.ToolDefinition) []map[string]any {
	tools := make([]map[string]any, 0, len(defs))
	for _, def := range defs {
		var schema json.RawMessage
		if len(def.JSONSchema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		} else {
			schema = json.RawMessage(def.JSONSchema)
		}
		fn := map[string]any{"name": def.ID, "parameters": schema}
		if def.Description != "" {
			fn["description"] = def.Description
		}
		tools = append(tools, map[string]any{"type": "function", "function": fn})
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
	provider.RegisterFactory("groq", func(cfg config.ProviderConfig) (provider.Provider, error) {
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
