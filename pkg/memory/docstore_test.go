package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentStore_IndexAndSearch(t *testing.T) {
	// Create a mock embedder that produces deterministic embeddings
	mockEmbedder := &mockEmbeddingProvider{
		dimensions: 3,
		model:      "test-model",
		embedFn: func(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
			embeddings := make([][]float64, len(req.Texts))
			for i, text := range req.Texts {
				// Create embeddings based on content keywords
				var vec []float64
				switch {
				case contains(text, "onboarding"):
					vec = []float64{1.0, 0.2, 0.1}
				case contains(text, "sales"):
					vec = []float64{0.1, 1.0, 0.2}
				case contains(text, "engineering"):
					vec = []float64{0.2, 0.1, 1.0}
				default:
					vec = []float64{0.5, 0.5, 0.5}
				}
				if req.Normalized {
					vec = NormalizeVector(vec)
				}
				embeddings[i] = vec
			}
			return &EmbeddingResponse{
				Embeddings: embeddings,
				Dimensions: 3,
				Model:      "test-model",
			}, nil
		},
	}

	// Create temp directory with test documents
	tempDir, err := os.MkdirTemp("", "docstore-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test documents
	docs := map[string]string{
		"wiki/product/onboarding.md": `# Onboarding Guide

## Getting Started
This is the onboarding guide for new users.
It explains how to get started with our product.

## First Steps
Complete these steps to activate your account.
`,
		"sales/meeting-notes.md": `# Sales Meeting Notes

## Q4 Review
We discussed sales targets and customer feedback.
The team is hitting their goals.

## Pipeline
Current pipeline looks strong with many opportunities.
`,
		"wiki/engineering/architecture.md": `# System Architecture

## Overview
This document describes our engineering architecture.
We use microservices and Kubernetes.

## Components
The main components include API gateway, services, and databases.
`,
	}

	for relPath, content := range docs {
		fullPath := filepath.Join(tempDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Create document store and index
	store := NewDocumentStore(mockEmbedder)
	ctx := context.Background()

	err = store.IndexDirectory(ctx, tempDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	if !store.IsIndexed() {
		t.Error("expected store to be indexed")
	}

	if store.ChunkCount() == 0 {
		t.Error("expected chunks to be created")
	}

	t.Logf("Indexed %d chunks", store.ChunkCount())

	// Test search for onboarding-related content
	t.Run("search for onboarding", func(t *testing.T) {
		results, err := store.Search(ctx, DocumentQuery{
			Text:      "onboarding activation",
			Limit:     5,
			Threshold: 0.0,
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("expected results for onboarding query")
		}

		// The onboarding document should be highly ranked
		found := false
		for _, r := range results {
			if contains(r.Chunk.DocumentPath, "onboarding") {
				found = true
				t.Logf("Found onboarding doc with score %.3f", r.Score)
			}
		}
		if !found {
			t.Error("expected onboarding document in results")
		}
	})

	// Test search with category filter
	t.Run("search with category filter", func(t *testing.T) {
		results, err := store.Search(ctx, DocumentQuery{
			Text:       "meeting notes targets",
			Limit:      5,
			Threshold:  0.0,
			Categories: []string{"sales"},
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// All results should be from sales category
		for _, r := range results {
			if r.Chunk.Category != "sales" {
				t.Errorf("expected category 'sales', got %q", r.Chunk.Category)
			}
		}
	})

	// Test search with threshold
	t.Run("search with threshold", func(t *testing.T) {
		results, err := store.Search(ctx, DocumentQuery{
			Text:      "test query",
			Limit:     10,
			Threshold: 0.9, // High threshold
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// All results should have score >= threshold
		for _, r := range results {
			if r.Score < 0.9 {
				t.Errorf("result score %.3f below threshold 0.9", r.Score)
			}
		}
	})
}

func TestDocumentStore_EmptyDirectory(t *testing.T) {
	mockEmbedder := &mockEmbeddingProvider{
		dimensions: 3,
		model:      "test-model",
	}

	tempDir, err := os.MkdirTemp("", "docstore-empty-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := NewDocumentStore(mockEmbedder)
	ctx := context.Background()

	// Indexing empty directory should not error
	err = store.IndexDirectory(ctx, tempDir)
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}

	if store.ChunkCount() != 0 {
		t.Errorf("expected 0 chunks, got %d", store.ChunkCount())
	}

	// Search on empty store should return empty results
	results, err := store.Search(ctx, DocumentQuery{
		Text:  "anything",
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestChunkDocument(t *testing.T) {
	content := `# Test Document

## Section One
This is the first section with some content.
It has multiple lines.

## Section Two
This is the second section.
It also has multiple lines of content.

### Subsection
A nested subsection here.
`

	chunks := chunkDocument("test.md", "wiki", "Test Document", content, 500, 50)

	if len(chunks) == 0 {
		t.Fatal("expected chunks to be created")
	}

	for _, chunk := range chunks {
		if chunk.DocumentPath != "test.md" {
			t.Errorf("expected path 'test.md', got %q", chunk.DocumentPath)
		}
		if chunk.Title != "Test Document" {
			t.Errorf("expected title 'Test Document', got %q", chunk.Title)
		}
		t.Logf("Chunk: %s - %s (%d chars)", chunk.SectionTitle, chunk.ID, len(chunk.Content))
	}
}

func TestSplitByHeaders(t *testing.T) {
	content := `# Main Title

Introduction text here.

## First Section
Content of first section.

## Second Section
Content of second section.

### Nested Section
Nested content.
`

	sections := splitByHeaders(content)

	if len(sections) == 0 {
		t.Fatal("expected sections to be created")
	}

	for _, s := range sections {
		t.Logf("Section: %q (lines %d-%d)", s.title, s.startLine, s.endLine)
	}
}

func TestExtractDocTitle(t *testing.T) {
	tests := []struct {
		content  string
		filename string
		expected string
	}{
		{"# My Title\n\nContent here", "doc.md", "My Title"},
		{"No header\n\nJust content", "document.md", "document"},
		{"", "empty.md", "empty"},
		{"## Not main header\n\nContent", "test.md", "test"},
	}

	for _, tt := range tests {
		result := extractDocTitle(tt.content, tt.filename)
		if result != tt.expected {
			t.Errorf("extractDocTitle(%q, %q) = %q, want %q",
				tt.content[:min(20, len(tt.content))], tt.filename, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
