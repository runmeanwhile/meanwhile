package memory

import (
	"context"
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name      string
		a         []float64
		b         []float64
		expected  float64
		tolerance float64
	}{
		{
			name:      "identical vectors",
			a:         []float64{1, 0, 0},
			b:         []float64{1, 0, 0},
			expected:  1.0,
			tolerance: 0.001,
		},
		{
			name:      "orthogonal vectors",
			a:         []float64{1, 0, 0},
			b:         []float64{0, 1, 0},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "opposite vectors",
			a:         []float64{1, 0, 0},
			b:         []float64{-1, 0, 0},
			expected:  -1.0,
			tolerance: 0.001,
		},
		{
			name:      "similar vectors",
			a:         []float64{1, 2, 3},
			b:         []float64{2, 4, 6},
			expected:  1.0,
			tolerance: 0.001,
		},
		{
			name:      "different length vectors",
			a:         []float64{1, 2, 3},
			b:         []float64{1, 2},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "zero vector",
			a:         []float64{0, 0, 0},
			b:         []float64{1, 2, 3},
			expected:  0.0,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.a, tt.b)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("CosineSimilarity(%v, %v) = %f, expected %f",
					tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestNormalizeVector(t *testing.T) {
	tests := []struct {
		name      string
		input     []float64
		expected  []float64
		tolerance float64
	}{
		{
			name:      "unit vector unchanged",
			input:     []float64{1, 0, 0},
			expected:  []float64{1, 0, 0},
			tolerance: 0.001,
		},
		{
			name:      "scale down",
			input:     []float64{3, 4, 0},
			expected:  []float64{0.6, 0.8, 0},
			tolerance: 0.001,
		},
		{
			name:      "zero vector",
			input:     []float64{0, 0, 0},
			expected:  []float64{0, 0, 0},
			tolerance: 0.001,
		},
		{
			name:      "negative values",
			input:     []float64{-1, -1, 0},
			expected:  []float64{-0.707, -0.707, 0},
			tolerance: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeVector(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(result), len(tt.expected))
			}
			for i := range result {
				diff := result[i] - tt.expected[i]
				if diff < 0 {
					diff = -diff
				}
				if diff > tt.tolerance {
					t.Errorf("NormalizeVector(%v)[%d] = %f, expected %f",
						tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestNormalizedVectorHasUnitLength(t *testing.T) {
	vectors := [][]float64{
		{1, 2, 3},
		{-5, 10, -15},
		{0.1, 0.2, 0.3},
		{100, 200, 300},
	}

	for _, v := range vectors {
		normalized := NormalizeVector(v)

		// Compute magnitude
		var mag float64
		for _, val := range normalized {
			mag += val * val
		}
		mag = math.Sqrt(mag)

		if diff := mag - 1.0; diff < -0.001 || diff > 0.001 {
			t.Errorf("NormalizeVector(%v) magnitude = %f, expected 1.0", v, mag)
		}
	}
}

type mockEmbeddingProvider struct {
	dimensions int
	model      string
	embedFn    func(context.Context, EmbeddingRequest) (*EmbeddingResponse, error)
}

func (m *mockEmbeddingProvider) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if m.embedFn != nil {
		return m.embedFn(ctx, req)
	}

	embeddings := make([][]float64, len(req.Texts))
	for i := range req.Texts {
		embeddings[i] = make([]float64, m.dimensions)
		for j := range embeddings[i] {
			embeddings[i][j] = float64(i + j)
		}
	}

	return &EmbeddingResponse{
		Embeddings: embeddings,
		Dimensions: m.dimensions,
		Model:      m.model,
	}, nil
}

func (m *mockEmbeddingProvider) Dimensions() int {
	return m.dimensions
}

func (m *mockEmbeddingProvider) Model() string {
	return m.model
}

func TestEmbeddingProvider(t *testing.T) {
	provider := &mockEmbeddingProvider{
		dimensions: 128,
		model:      "test-model",
	}

	req := EmbeddingRequest{
		Texts:      []string{"hello", "world"},
		Normalized: true,
	}

	resp, err := provider.Embed(context.Background(), req)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if resp.Dimensions != 128 {
		t.Errorf("Dimensions = %d, want 128", resp.Dimensions)
	}

	if resp.Model != "test-model" {
		t.Errorf("Model = %q, want %q", resp.Model, "test-model")
	}

	if len(resp.Embeddings) != 2 {
		t.Errorf("len(Embeddings) = %d, want 2", len(resp.Embeddings))
	}

	for i, emb := range resp.Embeddings {
		if len(emb) != 128 {
			t.Errorf("len(Embeddings[%d]) = %d, want 128", i, len(emb))
		}
	}
}
