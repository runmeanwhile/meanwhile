package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultOpenAIEndpoint is the default OpenAI API endpoint.
	DefaultOpenAIEndpoint = "https://api.openai.com/v1/embeddings"

	// DefaultModel is the default embedding model.
	DefaultModel = "text-embedding-3-small"

	// DefaultTimeout is the default HTTP timeout.
	DefaultTimeout = 30 * time.Second
)

// OpenAIEmbeddings provides embedding generation using OpenAI's API.
//
// Example usage:
//
//	provider := memory.NewOpenAIEmbeddings("sk-...")
//	resp, err := provider.Embed(ctx, memory.EmbeddingRequest{
//	    Texts: []string{"Hello world"},
//	})
type OpenAIEmbeddings struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
}

// OpenAIOption configures an OpenAIEmbeddings provider.
type OpenAIOption func(*OpenAIEmbeddings)

// WithEndpoint sets a custom API endpoint.
func WithEndpoint(endpoint string) OpenAIOption {
	return func(o *OpenAIEmbeddings) {
		o.endpoint = endpoint
	}
}

// WithModel sets the embedding model.
//
// Available models:
// - text-embedding-3-small (1536 dimensions, $0.02/1M tokens)
// - text-embedding-3-large (3072 dimensions, $0.13/1M tokens)
// - text-embedding-ada-002 (1536 dimensions, legacy)
func WithModel(model string) OpenAIOption {
	return func(o *OpenAIEmbeddings) {
		o.model = model
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) OpenAIOption {
	return func(o *OpenAIEmbeddings) {
		o.client = client
	}
}

// NewOpenAIEmbeddings creates a new OpenAI embedding provider.
//
// The API key should be your OpenAI API key (starts with "sk-").
// Get your API key from: https://platform.openai.com/api-keys
func NewOpenAIEmbeddings(apiKey string, opts ...OpenAIOption) *OpenAIEmbeddings {
	o := &OpenAIEmbeddings{
		apiKey:   apiKey,
		endpoint: DefaultOpenAIEndpoint,
		model:    DefaultModel,
		client: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// Embed generates embeddings using OpenAI's API.
func (o *OpenAIEmbeddings) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if len(req.Texts) == 0 {
		return &EmbeddingResponse{
			Embeddings: [][]float64{},
			Dimensions: o.Dimensions(),
			Model:      o.model,
		}, nil
	}

	model := req.Model
	if model == "" {
		model = o.model
	}

	// Build OpenAI API request
	apiReq := openAIRequest{
		Input: req.Texts,
		Model: model,
	}

	// Add metadata if provided
	if user, ok := req.Metadata["user"].(string); ok {
		apiReq.User = user
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't override return error
			_ = closeErr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai api error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract embeddings
	embeddings := make([][]float64, len(apiResp.Data))
	var dimensions int
	for i, data := range apiResp.Data {
		embeddings[i] = data.Embedding
		if i == 0 {
			dimensions = len(data.Embedding)
		}
	}

	// Normalize if requested
	if req.Normalized {
		for i := range embeddings {
			embeddings[i] = NormalizeVector(embeddings[i])
		}
	}

	return &EmbeddingResponse{
		Embeddings: embeddings,
		Dimensions: dimensions,
		Model:      apiResp.Model,
		Usage: &EmbeddingUsage{
			PromptTokens: apiResp.Usage.PromptTokens,
			TotalTokens:  apiResp.Usage.TotalTokens,
		},
	}, nil
}

// Dimensions returns the embedding dimensions for the configured model.
func (o *OpenAIEmbeddings) Dimensions() int {
	switch o.model {
	case "text-embedding-3-small", "text-embedding-ada-002":
		return 1536
	case "text-embedding-3-large":
		return 3072
	default:
		return 1536 // Default to small model dimensions
	}
}

// Model returns the configured model name.
func (o *OpenAIEmbeddings) Model() string {
	return o.model
}

// openAIRequest represents the OpenAI embeddings API request.
type openAIRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
	User  string   `json:"user,omitempty"`
}

// openAIResponse represents the OpenAI embeddings API response.
type openAIResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}
