// Package memory provides embedding functionality for semantic memory retrieval.
package memory

import (
	"context"
	"math"
)

// EmbeddingProvider generates vector embeddings from text.
//
// Embeddings are dense vector representations of text that capture semantic meaning,
// enabling similarity-based retrieval and semantic search capabilities.
//
// Example usage:
//
//	provider := memory.NewOpenAIEmbeddings(apiKey)
//	resp, err := provider.Embed(ctx, memory.EmbeddingRequest{
//	    Texts: []string{"Hello world", "Machine learning"},
//	})
type EmbeddingProvider interface {
	// Embed generates embeddings for the given texts.
	//
	// The provider should:
	// - Return embeddings in the same order as input texts
	// - Normalize vectors if Normalized is true
	// - Use the specified Model if provided
	// - Return dimensions matching the model's output
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)

	// Dimensions returns the dimensionality of embeddings produced by this provider.
	//
	// For example:
	// - OpenAI text-embedding-3-small: 1536 dimensions
	// - OpenAI text-embedding-3-large: 3072 dimensions
	Dimensions() int

	// Model returns the model identifier used by this provider.
	//
	// Examples: "text-embedding-3-small", "nomic-embed-text"
	Model() string
}

// EmbeddingRequest contains parameters for generating embeddings.
type EmbeddingRequest struct {
	// Texts is the list of text strings to embed.
	// Most providers support batch embedding for efficiency.
	Texts []string

	// Model specifies which embedding model to use.
	// If empty, the provider's default model is used.
	Model string

	// Normalized indicates whether to normalize vectors to unit length.
	// Normalized vectors are required for cosine similarity comparisons.
	Normalized bool

	// Metadata contains optional provider-specific parameters.
	// Example: {"user": "user-123"} for OpenAI usage tracking
	Metadata map[string]any
}

// EmbeddingResponse contains the generated embeddings.
type EmbeddingResponse struct {
	// Embeddings contains the vector representations of input texts.
	// Each embedding is a float64 slice with length equal to Dimensions.
	// Order matches the input Texts order.
	Embeddings [][]float64

	// Dimensions is the length of each embedding vector.
	Dimensions int

	// Model is the actual model used to generate embeddings.
	Model string

	// Usage contains token/request usage statistics if provided.
	Usage *EmbeddingUsage
}

// EmbeddingUsage contains usage statistics for embedding generation.
type EmbeddingUsage struct {
	// PromptTokens is the number of tokens in the input texts.
	PromptTokens int

	// TotalTokens is the total number of tokens processed.
	TotalTokens int
}

// CosineSimilarity computes the cosine similarity between two vectors.
//
// Returns a value between -1 and 1, where:
// - 1 means vectors are identical in direction (most similar)
// - 0 means vectors are orthogonal (unrelated)
// - -1 means vectors are opposite in direction (least similar)
//
// Vectors should be normalized for accurate similarity scoring.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// NormalizeVector normalizes a vector to unit length.
//
// Normalization ensures that cosine similarity can be computed
// as a simple dot product, improving performance for large-scale
// similarity searches.
func NormalizeVector(v []float64) []float64 {
	var norm float64
	for _, val := range v {
		norm += val * val
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return v
	}

	normalized := make([]float64, len(v))
	for i, val := range v {
		normalized[i] = val / norm
	}
	return normalized
}
