package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// DocumentChunk represents a chunk of a document with its embedding.
type DocumentChunk struct {
	// ID is a unique identifier for this chunk
	ID string
	// DocumentPath is the path to the source document
	DocumentPath string
	// Category categorizes the document (e.g., "wiki/product", "sales")
	Category string
	// Title is the document title
	Title string
	// SectionTitle is the section title within the document (if applicable)
	SectionTitle string
	// Content is the text content of this chunk
	Content string
	// StartLine is the starting line number in the source document
	StartLine int
	// EndLine is the ending line number in the source document
	EndLine int
}

// DocumentSearchResult contains a chunk and its relevance score.
type DocumentSearchResult struct {
	Chunk DocumentChunk
	Score float64 // Cosine similarity (0-1)
}

// DocumentStore provides semantic search over a corpus of documents.
//
// It loads documents from the filesystem, chunks them, generates embeddings,
// and supports semantic search queries.
//
// Example usage:
//
//	embedder := memory.NewOpenAIEmbeddings(apiKey)
//	store := memory.NewDocumentStore(embedder)
//
//	// Index a directory of markdown files
//	err := store.IndexDirectory(ctx, "/path/to/docs")
//
//	// Search semantically
//	results, err := store.Search(ctx, DocumentQuery{
//	    Text:      "how to improve onboarding",
//	    Limit:     10,
//	    Threshold: 0.5,
//	})
type DocumentStore struct {
	embedder   EmbeddingProvider
	mu         sync.RWMutex
	chunks     []DocumentChunk
	embeddings [][]float64
	indexed    bool
}

// NewDocumentStore creates a new document store.
//
// The embedder is used to generate vector embeddings for each document chunk.
func NewDocumentStore(embedder EmbeddingProvider) *DocumentStore {
	return &DocumentStore{
		embedder: embedder,
		chunks:   make([]DocumentChunk, 0),
	}
}

// DocumentStoreOption configures document indexing behavior.
type DocumentStoreOption func(*indexConfig)

type indexConfig struct {
	// ChunkSize is the target size of each chunk in characters
	ChunkSize int
	// ChunkOverlap is the overlap between consecutive chunks
	ChunkOverlap int
	// Extensions is the list of file extensions to index
	Extensions []string
	// CategoryFunc extracts category from a file path
	CategoryFunc func(relPath string) string
}

// WithChunkSize sets the target chunk size in characters.
func WithChunkSize(size int) DocumentStoreOption {
	return func(c *indexConfig) {
		c.ChunkSize = size
	}
}

// WithChunkOverlap sets the overlap between chunks.
func WithChunkOverlap(overlap int) DocumentStoreOption {
	return func(c *indexConfig) {
		c.ChunkOverlap = overlap
	}
}

// WithExtensions sets the file extensions to index.
func WithExtensions(exts ...string) DocumentStoreOption {
	return func(c *indexConfig) {
		c.Extensions = exts
	}
}

// WithCategoryFunc sets a custom function to extract categories from paths.
func WithCategoryFunc(fn func(relPath string) string) DocumentStoreOption {
	return func(c *indexConfig) {
		c.CategoryFunc = fn
	}
}

// defaultCategoryFunc extracts a category from a relative path.
func defaultCategoryFunc(relPath string) string {
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) > 1 {
		switch parts[0] {
		case "wiki":
			if len(parts) > 2 {
				return "wiki/" + parts[1]
			}
			return "wiki"
		case "customer-feedback":
			return "customer-feedback"
		case "sales":
			return "sales"
		}
	}
	return "general"
}

// IndexDirectory loads and indexes all documents from a directory.
//
// Files are chunked and embedded. This operation may take time and make
// API calls for embedding generation.
func (d *DocumentStore) IndexDirectory(ctx context.Context, dir string, opts ...DocumentStoreOption) error {
	cfg := &indexConfig{
		ChunkSize:    1500,
		ChunkOverlap: 200,
		Extensions:   []string{".md", ".txt"},
		CategoryFunc: defaultCategoryFunc,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Collect all documents
	var allChunks []DocumentChunk

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}
		if info.IsDir() {
			return nil
		}

		// Check extension
		ext := strings.ToLower(filepath.Ext(path))
		validExt := false
		for _, e := range cfg.Extensions {
			if ext == e {
				validExt = true
				break
			}
		}
		if !validExt {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		relPath, _ := filepath.Rel(dir, path)
		category := cfg.CategoryFunc(relPath)
		title := extractDocTitle(string(content), filepath.Base(path))

		// Chunk the document
		chunks := chunkDocument(
			relPath,
			category,
			title,
			string(content),
			cfg.ChunkSize,
			cfg.ChunkOverlap,
		)
		allChunks = append(allChunks, chunks...)

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk directory: %w", err)
	}

	if len(allChunks) == 0 {
		return nil
	}

	// Generate embeddings in batches
	const batchSize = 50
	allEmbeddings := make([][]float64, len(allChunks))

	for i := 0; i < len(allChunks); i += batchSize {
		end := i + batchSize
		if end > len(allChunks) {
			end = len(allChunks)
		}

		batch := allChunks[i:end]
		texts := make([]string, len(batch))
		for j, chunk := range batch {
			// Create embedding text from title + content for better semantic matching
			texts[j] = chunk.Title + "\n" + chunk.SectionTitle + "\n" + chunk.Content
		}

		resp, err := d.embedder.Embed(ctx, EmbeddingRequest{
			Texts:      texts,
			Normalized: true,
		})
		if err != nil {
			return fmt.Errorf("embed batch %d: %w", i/batchSize, err)
		}

		for j, emb := range resp.Embeddings {
			allEmbeddings[i+j] = emb
		}
	}

	// Store results
	d.mu.Lock()
	d.chunks = allChunks
	d.embeddings = allEmbeddings
	d.indexed = true
	d.mu.Unlock()

	return nil
}

// DocumentQuery defines parameters for semantic document search.
type DocumentQuery struct {
	// Text is the query text to search for
	Text string
	// Limit is the maximum number of results to return
	Limit int
	// Threshold is the minimum similarity score (0-1)
	Threshold float64
	// Categories filters results to specific categories
	Categories []string
}

// Search performs semantic search over indexed documents.
//
// Returns documents ranked by semantic similarity to the query text.
func (d *DocumentStore) Search(ctx context.Context, query DocumentQuery) ([]DocumentSearchResult, error) {
	d.mu.RLock()
	if !d.indexed || len(d.chunks) == 0 {
		d.mu.RUnlock()
		return []DocumentSearchResult{}, nil
	}
	chunks := d.chunks
	embeddings := d.embeddings
	d.mu.RUnlock()

	// Generate query embedding
	resp, err := d.embedder.Embed(ctx, EmbeddingRequest{
		Texts:      []string{query.Text},
		Normalized: true,
	})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no query embedding returned")
	}
	queryEmb := resp.Embeddings[0]

	// Compute similarities
	var results []DocumentSearchResult
	for i, chunk := range chunks {
		// Apply category filter
		if len(query.Categories) > 0 {
			found := false
			for _, cat := range query.Categories {
				if chunk.Category == cat || strings.HasPrefix(chunk.Category, cat+"/") {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		score := CosineSimilarity(queryEmb, embeddings[i])

		if score >= query.Threshold {
			results = append(results, DocumentSearchResult{
				Chunk: chunk,
				Score: score,
			})
		}
	}

	// Sort by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply limit
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

// ChunkCount returns the number of indexed chunks.
func (d *DocumentStore) ChunkCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.chunks)
}

// IsIndexed returns whether documents have been indexed.
func (d *DocumentStore) IsIndexed() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.indexed
}

// chunkDocument splits a document into overlapping chunks.
func chunkDocument(path, category, title, content string, chunkSize, overlap int) []DocumentChunk {
	var chunks []DocumentChunk

	// First, try to split by markdown headers
	sections := splitByHeaders(content)

	chunkID := 0
	for _, section := range sections {
		sectionTitle := section.title
		sectionContent := strings.TrimSpace(section.content)

		if len(sectionContent) == 0 {
			continue
		}

		// If section is small enough, keep it as one chunk
		if len(sectionContent) <= chunkSize {
			chunks = append(chunks, DocumentChunk{
				ID:           fmt.Sprintf("%s#%d", path, chunkID),
				DocumentPath: path,
				Category:     category,
				Title:        title,
				SectionTitle: sectionTitle,
				Content:      sectionContent,
				StartLine:    section.startLine,
				EndLine:      section.endLine,
			})
			chunkID++
			continue
		}

		// Split large sections into overlapping chunks
		subChunks := splitIntoChunks(sectionContent, chunkSize, overlap)
		for _, subContent := range subChunks {
			chunks = append(chunks, DocumentChunk{
				ID:           fmt.Sprintf("%s#%d", path, chunkID),
				DocumentPath: path,
				Category:     category,
				Title:        title,
				SectionTitle: sectionTitle,
				Content:      subContent,
				StartLine:    section.startLine,
				EndLine:      section.endLine,
			})
			chunkID++
		}
	}

	return chunks
}

type docSection struct {
	title     string
	content   string
	startLine int
	endLine   int
}

// splitByHeaders splits markdown content by headers.
func splitByHeaders(content string) []docSection {
	lines := strings.Split(content, "\n")
	var sections []docSection

	currentTitle := ""
	currentContent := strings.Builder{}
	startLine := 1

	for i, line := range lines {
		lineNum := i + 1

		// Check for markdown headers (## or ###)
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			// Save previous section if it has content
			if currentContent.Len() > 0 {
				sections = append(sections, docSection{
					title:     currentTitle,
					content:   currentContent.String(),
					startLine: startLine,
					endLine:   lineNum - 1,
				})
			}

			// Start new section
			currentTitle = strings.TrimPrefix(strings.TrimPrefix(line, "### "), "## ")
			currentContent.Reset()
			startLine = lineNum
		} else {
			if currentContent.Len() > 0 {
				currentContent.WriteString("\n")
			}
			currentContent.WriteString(line)
		}
	}

	// Don't forget the last section
	if currentContent.Len() > 0 {
		sections = append(sections, docSection{
			title:     currentTitle,
			content:   currentContent.String(),
			startLine: startLine,
			endLine:   len(lines),
		})
	}

	// If no sections were created (no headers), treat the whole document as one section
	if len(sections) == 0 && len(content) > 0 {
		sections = append(sections, docSection{
			title:     "",
			content:   content,
			startLine: 1,
			endLine:   len(lines),
		})
	}

	return sections
}

// splitIntoChunks splits text into overlapping chunks at sentence boundaries.
func splitIntoChunks(text string, chunkSize, overlap int) []string {
	if len(text) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(text) {
		end := start + chunkSize
		if end >= len(text) {
			chunks = append(chunks, strings.TrimSpace(text[start:]))
			break
		}

		// Try to find a good break point (sentence end, paragraph, etc.)
		breakPoint := findBreakPoint(text[start:end])
		if breakPoint > 0 {
			end = start + breakPoint
		}

		chunk := strings.TrimSpace(text[start:end])
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}

		// Move start with overlap
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}

	return chunks
}

// findBreakPoint finds a good break point in text.
func findBreakPoint(text string) int {
	// Try to break at paragraph
	if idx := strings.LastIndex(text, "\n\n"); idx > len(text)/2 {
		return idx + 2
	}

	// Try to break at sentence end
	for _, ending := range []string{". ", ".\n", "? ", "?\n", "! ", "!\n"} {
		if idx := strings.LastIndex(text, ending); idx > len(text)/2 {
			return idx + len(ending)
		}
	}

	// Try to break at newline
	if idx := strings.LastIndex(text, "\n"); idx > len(text)/2 {
		return idx + 1
	}

	// Try to break at space
	if idx := strings.LastIndex(text, " "); idx > len(text)/2 {
		return idx + 1
	}

	return 0
}

// extractDocTitle extracts the title from markdown content.
func extractDocTitle(content, filename string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}
