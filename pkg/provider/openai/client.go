package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
	tiktoken "github.com/weaviate/tiktoken-go"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	responsesPath  = "/responses"
)

var (
	// ErrMissingAPIKey indicates a missing API key.
	ErrMissingAPIKey = errors.New("missing api key")
)

// Config configures the OpenAI client.
type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// Client implements the OpenAI Responses provider.
type Client struct {
	apiKey         string
	baseURL        string
	http           *http.Client
	tokenMu        sync.Mutex
	tokenEncodings map[string]*tiktoken.Tiktoken
}

// NewClient creates a new OpenAI provider.
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, ErrMissingAPIKey
	}

	baseURL := cfg.BaseURL
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

	return &Client{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		http:    httpClient,
	}, nil
}

// ID returns the provider ID.
func (c *Client) ID() string { return "openai" }

// Stream starts a streaming response from OpenAI.
func (c *Client) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	payload, err := buildRequestPayload(req)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	url := c.baseURL + responsesPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer func() {
			_ = resp.Body.Close()
		}()
		var payloadErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&payloadErr)
		if payloadErr.Error.Message == "" {
			payloadErr.Error.Message = resp.Status
		}
		return nil, fmt.Errorf("openai error: %s", payloadErr.Error.Message)
	}

	return newStream(resp.Body), nil
}

func buildRequestPayload(req provider.Request) (map[string]any, error) {
	input, err := buildInput(req.Messages)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model":  req.Model,
		"input":  input,
		"stream": true,
	}

	if len(req.Tools) > 0 {
		payload["tools"] = buildTools(req.Tools)
	}

	for key, value := range req.Params {
		if key == "model" || key == "input" || key == "stream" {
			continue
		}
		if key == "response_format" {
			if format := normalizeResponseFormat(value); format != nil {
				payload["text"] = map[string]any{"format": format}
			}
			continue
		}
		payload[key] = value
	}

	return payload, nil
}

func normalizeResponseFormat(value any) map[string]any {
	if value == nil {
		return nil
	}
	if format, ok := value.(map[string]any); ok {
		return format
	}
	if raw, ok := value.(json.RawMessage); ok {
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err == nil {
			return out
		}
	}
	return nil
}

func buildInput(messages []agent.Message) ([]map[string]any, error) {
	inputs := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == agent.RoleTool {
			toolInputs, err := buildToolOutputItems(msg)
			if err != nil {
				return nil, err
			}
			if len(toolInputs) > 0 {
				inputs = append(inputs, toolInputs...)
			}
			continue
		}
		if msg.Role == "" {
			continue
		}
		entry := map[string]any{
			"role":    string(msg.Role),
			"content": buildMessageContent(wrapNamedMessage(msg)),
		}
		inputs = append(inputs, entry)
	}

	return inputs, nil
}

func wrapNamedMessage(msg agent.Message) agent.Message {
	if msg.Name == "" || msg.Role != agent.RoleAssistant {
		return msg
	}

	openTag := fmt.Sprintf("<agent:%s>\n", msg.Name)
	closeTag := fmt.Sprintf("\n</agent:%s>", msg.Name)

	if len(msg.Parts) == 0 {
		text := msg.Text()
		if strings.TrimSpace(text) == "" {
			return msg
		}
		msg.Parts = []agent.ContentPart{{
			Type: agent.ContentPartText,
			Text: openTag + text + closeTag,
		}}
		msg.Name = ""
		return msg
	}

	parts := make([]agent.ContentPart, 0, len(msg.Parts)+2)
	parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: openTag})
	parts = append(parts, msg.Parts...)
	parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: closeTag})

	msg.Parts = parts
	msg.Name = ""
	return msg
}

func buildMessageContent(msg agent.Message) any {
	parts := buildContentParts(msg.Parts, msg.Role)
	if len(parts) > 0 {
		return parts
	}
	text := msg.Text()
	if text == "" {
		return ""
	}
	if msg.Role == agent.RoleAssistant {
		return []map[string]any{{"type": "output_text", "text": text}}
	}
	return []map[string]any{{"type": "input_text", "text": text}}
}

func buildContentParts(parts []agent.ContentPart, role agent.Role) []map[string]any {
	out := make([]map[string]any, 0, len(parts))
	textType := "input_text"
	if role == agent.RoleAssistant {
		textType = "output_text"
	}
	for _, part := range parts {
		switch strings.ToLower(string(part.Type)) {
		case "text", "input_text":
			if part.Text == "" {
				continue
			}
			out = append(out, map[string]any{
				"type": textType,
				"text": part.Text,
			})
		case "image", "input_image":
			if part.URI == "" {
				continue
			}
			if role == agent.RoleAssistant {
				out = append(out, map[string]any{
					"type": textType,
					"text": fmt.Sprintf("[image:%s]", part.URI),
				})
				continue
			}
			entry := map[string]any{
				"type":      "input_image",
				"image_url": part.URI,
			}
			if part.Detail != "" {
				entry["detail"] = part.Detail
			}
			out = append(out, entry)
		case "json":
			if part.JSON == nil {
				continue
			}
			raw, err := json.Marshal(part.JSON)
			if err != nil {
				continue
			}
			out = append(out, map[string]any{
				"type": textType,
				"text": string(raw),
			})
		default:
			if part.Text != "" {
				out = append(out, map[string]any{
					"type": textType,
					"text": part.Text,
				})
			} else if role == agent.RoleAssistant && part.URI != "" {
				out = append(out, map[string]any{
					"type": textType,
					"text": fmt.Sprintf("[%s:%s]", part.Type, part.URI),
				})
			}
		}
	}
	return out
}

func buildToolOutputItems(msg agent.Message) ([]map[string]any, error) {
	if msg.ToolCallID == "" {
		return nil, fmt.Errorf("tool message missing call id")
	}

	arguments := "{}"
	if msg.Metadata != nil {
		if raw, ok := msg.Metadata["arguments"]; ok {
			switch value := raw.(type) {
			case json.RawMessage:
				if len(value) > 0 {
					arguments = string(value)
				}
			case []byte:
				if len(value) > 0 {
					arguments = string(value)
				}
			case string:
				if strings.TrimSpace(value) != "" {
					arguments = value
				}
			}
		}
	}

	outputText := msg.Text()
	if outputText == "" {
		outputText = "{}"
	}
	items := []map[string]any{
		{
			"type":      "function_call",
			"call_id":   msg.ToolCallID,
			"name":      toolNameFromMessage(msg),
			"arguments": arguments,
		},
		{
			"type":    "function_call_output",
			"call_id": msg.ToolCallID,
			"output":  outputText,
		},
	}

	nonTextParts := filterNonTextParts(msg.Parts)
	if len(nonTextParts) > 0 {
		parts := make([]agent.ContentPart, 0, len(nonTextParts)+1)
		parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: "Tool output:"})
		parts = append(parts, nonTextParts...)
		items = append(items, map[string]any{
			"role":    string(agent.RoleAssistant),
			"content": buildContentParts(parts, agent.RoleAssistant),
		})
	}

	return items, nil
}

func toolNameFromMessage(msg agent.Message) string {
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

func filterNonTextParts(parts []agent.ContentPart) []agent.ContentPart {
	out := make([]agent.ContentPart, 0, len(parts))
	for _, part := range parts {
		if part.Type == agent.ContentPartText {
			continue
		}
		out = append(out, part)
	}
	return out
}
func buildTools(defs []tool.Definition) []map[string]any {
	tools := make([]map[string]any, 0, len(defs))
	for _, def := range defs {
		var schema json.RawMessage
		if len(def.Schema.JSONSchema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		} else {
			schema = json.RawMessage(def.Schema.JSONSchema)
		}
		tools = append(tools, map[string]any{
			"type":       "function",
			"name":       def.ID,
			"parameters": schema,
		})
		if def.Description != "" {
			tools[len(tools)-1]["description"] = def.Description
		}
	}
	return tools
}
