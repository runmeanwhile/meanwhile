package memory

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIEmbeddings_Embed(t *testing.T) {
	// Mock OpenAI API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Generate mock embeddings
		resp := openAIResponse{
			Object: "list",
			Model:  req.Model,
			Data: make([]struct {
				Object    string    `json:"object"`
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}, len(req.Input)),
			Usage: struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			}{
				PromptTokens: len(req.Input) * 10,
				TotalTokens:  len(req.Input) * 10,
			},
		}

		for i := range req.Input {
			// Create a simple mock embedding (3 dimensions for testing)
			resp.Data[i].Object = "embedding"
			resp.Data[i].Index = i
			resp.Data[i].Embedding = []float64{float64(i), float64(i + 1), float64(i + 2)}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	provider := NewOpenAIEmbeddings(
		"test-key",
		WithEndpoint(server.URL),
		WithModel("text-embedding-3-small"),
	)

	t.Run("basic embedding", func(t *testing.T) {
		req := EmbeddingRequest{
			Texts: []string{"hello", "world"},
		}

		resp, err := provider.Embed(context.Background(), req)
		if err != nil {
			t.Fatalf("Embed failed: %v", err)
		}

		if len(resp.Embeddings) != 2 {
			t.Errorf("got %d embeddings, want 2", len(resp.Embeddings))
		}

		if resp.Model != "text-embedding-3-small" {
			t.Errorf("model = %q, want %q", resp.Model, "text-embedding-3-small")
		}

		if resp.Usage.PromptTokens != 20 {
			t.Errorf("prompt tokens = %d, want 20", resp.Usage.PromptTokens)
		}
	})

	t.Run("normalized embeddings", func(t *testing.T) {
		req := EmbeddingRequest{
			Texts:      []string{"test"},
			Normalized: true,
		}

		resp, err := provider.Embed(context.Background(), req)
		if err != nil {
			t.Fatalf("Embed failed: %v", err)
		}

		// Check that vector is normalized (magnitude should be ~1)
		emb := resp.Embeddings[0]
		var mag float64
		for _, val := range emb {
			mag += val * val
		}
		mag = math.Sqrt(mag)

		if diff := mag - 1.0; diff < -0.001 || diff > 0.001 {
			t.Errorf("normalized vector magnitude = %f, want 1.0", mag)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		req := EmbeddingRequest{
			Texts: []string{},
		}

		resp, err := provider.Embed(context.Background(), req)
		if err != nil {
			t.Fatalf("Embed failed: %v", err)
		}

		if len(resp.Embeddings) != 0 {
			t.Errorf("got %d embeddings, want 0", len(resp.Embeddings))
		}
	})

	t.Run("custom model", func(t *testing.T) {
		req := EmbeddingRequest{
			Texts: []string{"test"},
			Model: "text-embedding-3-large",
		}

		resp, err := provider.Embed(context.Background(), req)
		if err != nil {
			t.Fatalf("Embed failed: %v", err)
		}

		if resp.Model != "text-embedding-3-large" {
			t.Errorf("model = %q, want %q", resp.Model, "text-embedding-3-large")
		}
	})

	t.Run("with user metadata", func(t *testing.T) {
		req := EmbeddingRequest{
			Texts: []string{"test"},
			Metadata: map[string]any{
				"user": "user-123",
			},
		}

		resp, err := provider.Embed(context.Background(), req)
		if err != nil {
			t.Fatalf("Embed failed: %v", err)
		}

		if len(resp.Embeddings) != 1 {
			t.Errorf("got %d embeddings, want 1", len(resp.Embeddings))
		}
	})
}

func TestOpenAIEmbeddings_Dimensions(t *testing.T) {
	tests := []struct {
		model      string
		dimensions int
	}{
		{"text-embedding-3-small", 1536},
		{"text-embedding-3-large", 3072},
		{"text-embedding-ada-002", 1536},
		{"unknown-model", 1536}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider := NewOpenAIEmbeddings(
				"test-key",
				WithModel(tt.model),
			)

			if provider.Dimensions() != tt.dimensions {
				t.Errorf("Dimensions() = %d, want %d", provider.Dimensions(), tt.dimensions)
			}
		})
	}
}

func TestOpenAIEmbeddings_Model(t *testing.T) {
	provider := NewOpenAIEmbeddings(
		"test-key",
		WithModel("text-embedding-3-large"),
	)

	if provider.Model() != "text-embedding-3-large" {
		t.Errorf("Model() = %q, want %q", provider.Model(), "text-embedding-3-large")
	}
}

func TestOpenAIEmbeddings_ErrorHandling(t *testing.T) {
	t.Run("unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
		}))
		defer server.Close()

		provider := NewOpenAIEmbeddings(
			"invalid-key",
			WithEndpoint(server.URL),
		)

		req := EmbeddingRequest{
			Texts: []string{"test"},
		}

		_, err := provider.Embed(context.Background(), req)
		if err == nil {
			t.Error("expected error for unauthorized request, got nil")
		}
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{invalid json`))
		}))
		defer server.Close()

		provider := NewOpenAIEmbeddings(
			"test-key",
			WithEndpoint(server.URL),
		)

		req := EmbeddingRequest{
			Texts: []string{"test"},
		}

		_, err := provider.Embed(context.Background(), req)
		if err == nil {
			t.Error("expected error for malformed response, got nil")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Never respond
			<-r.Context().Done()
		}))
		defer server.Close()

		provider := NewOpenAIEmbeddings(
			"test-key",
			WithEndpoint(server.URL),
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		req := EmbeddingRequest{
			Texts: []string{"test"},
		}

		_, err := provider.Embed(ctx, req)
		if err == nil {
			t.Error("expected error for cancelled context, got nil")
		}
	})
}

func TestOpenAIOptions(t *testing.T) {
	customClient := &http.Client{}

	provider := NewOpenAIEmbeddings(
		"test-key",
		WithEndpoint("https://custom.endpoint"),
		WithModel("custom-model"),
		WithHTTPClient(customClient),
	)

	if provider.endpoint != "https://custom.endpoint" {
		t.Errorf("endpoint = %q, want %q", provider.endpoint, "https://custom.endpoint")
	}

	if provider.model != "custom-model" {
		t.Errorf("model = %q, want %q", provider.model, "custom-model")
	}

	if provider.client != customClient {
		t.Error("client was not set correctly")
	}
}
