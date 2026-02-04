package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

// FileChatStore stores events in JSONL files, one file per session.
// Each event is stored as a newline-delimited JSON record for durability.
// Safe for concurrent use within a single process.
type FileChatStore struct {
	basePath string
	mu       sync.RWMutex
	files    map[string]*fileEntry
	syncEvery int
	openFile  func(path string) (syncFile, error)
}

type fileEntry struct {
	mu   sync.Mutex
	file syncFile
	syncCount int
}

type syncFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

// FileChatStoreOption configures FileChatStore behavior.
type FileChatStoreOption func(*FileChatStore)

// WithSyncEvery sets how many writes occur before fsync. Use 1 to sync each write.
func WithSyncEvery(count int) FileChatStoreOption {
	return func(store *FileChatStore) {
		if count > 0 {
			store.syncEvery = count
		}
	}
}

// NewFileChatStore creates a new file-based chat store.
// The basePath directory will be created if it doesn't exist.
// Returns an error if the directory cannot be created or accessed.
func NewFileChatStore(basePath string, opts ...FileChatStoreOption) (*FileChatStore, error) {
	if basePath == "" {
		return nil, errors.New("basePath cannot be empty")
	}

	// Clean and validate path
	basePath = filepath.Clean(basePath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	store := &FileChatStore{
		basePath: basePath,
		files:    make(map[string]*fileEntry),
		syncEvery: 1,
		openFile: func(path string) (syncFile, error) {
			return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // #nosec G304 G302 - user-only access, path validated
		},
	}
	for _, opt := range opts {
		opt(store)
	}
	if store.syncEvery <= 0 {
		store.syncEvery = 1
	}
	return store, nil
}

// Append stores an event to the session's JSONL file.
// Events are immediately fsynced to disk for durability.
// Thread-safe for concurrent appends to different sessions.
func (f *FileChatStore) Append(ctx context.Context, sessionID string, ev event.Event) error {
	if sessionID == "" {
		return errors.New("sessionID cannot be empty")
	}

	// Validate sessionID to prevent path traversal
	if strings.Contains(sessionID, "..") || strings.ContainsAny(sessionID, "/\\") {
		return errors.New("invalid sessionID: must not contain path separators")
	}

	// Check context before acquiring lock
	if err := ctx.Err(); err != nil {
		return err
	}

	entry, err := f.getOrOpenFile(sessionID)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	file := entry.file

	// Marshal event to JSON
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Write with newline delimiter
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Sync to disk for durability
	entry.syncCount++
	if f.syncEvery <= 1 || entry.syncCount >= f.syncEvery {
		if err := file.Sync(); err != nil {
			return fmt.Errorf("failed to sync file: %w", err)
		}
		entry.syncCount = 0
	}

	return nil
}

// Query returns stored events matching the query criteria.
// Events are returned in chronological order (oldest first).
// Thread-safe for concurrent queries.
func (f *FileChatStore) Query(ctx context.Context, query Query) ([]Item, error) {
	if query.SessionID == "" {
		return nil, errors.New("sessionID cannot be empty")
	}

	// Validate sessionID
	if strings.Contains(query.SessionID, "..") || strings.ContainsAny(query.SessionID, "/\\") {
		return nil, errors.New("invalid sessionID: must not contain path separators")
	}

	filePath := f.sessionFilePath(query.SessionID)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []Item{}, nil // No events yet
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat session file: %w", err)
	}

	// Open file for reading
	file, err := os.Open(filePath) // #nosec G304 - filePath is validated against path traversal
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	// Build type filter map for efficient lookup
	typeFilter := make(map[event.Type]struct{}, len(query.Types))
	for _, t := range query.Types {
		typeFilter[t] = struct{}{}
	}

	var items []Item
	scanner := bufio.NewScanner(file)
	// Set max buffer size to handle large events (default 64KB might be too small)
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		// Check context periodically
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ev event.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			// Log error but continue processing other events
			// This allows recovery from corrupted lines
			continue
		}

		// Apply type filter if specified
		if len(typeFilter) > 0 {
			if _, ok := typeFilter[ev.Type]; !ok {
				continue
			}
		}

		items = append(items, Item{Event: ev})

		// Apply limit if specified
		if query.Limit > 0 && len(items) >= query.Limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading session file: %w", err)
	}

	return items, nil
}

// Stats calculates structured statistics about stored events.
func (f *FileChatStore) Stats(ctx context.Context, sessionID string, policy Policy) (EventStats, error) {
	items, err := f.Query(ctx, Query{
		SessionID: sessionID,
		Types:     policy.Types,
		Limit:     policy.MaxItems,
	})
	if err != nil {
		return EventStats{}, err
	}

	if len(items) == 0 {
		return EventStats{
			SessionID:   sessionID,
			TotalEvents: 0,
			EventCounts: make(map[event.Type]int),
		}, nil
	}

	stats := EventStats{
		SessionID:   sessionID,
		TotalEvents: len(items),
		EventCounts: make(map[event.Type]int),
		FirstEvent:  items[0].Event.Time,
		LastEvent:   items[len(items)-1].Event.Time,
	}

	for _, item := range items {
		stats.EventCounts[item.Event.Type]++
	}

	return stats, nil
}

// Summarize returns a text summary of session events.
// Deprecated: Use Stats() instead for structured event statistics.
func (f *FileChatStore) Summarize(ctx context.Context, sessionID string, policy Policy) (Summary, error) {
	stats, err := f.Stats(ctx, sessionID, policy)
	if err != nil {
		return Summary{}, err
	}

	text := fmt.Sprintf("Session %s has %d events", sessionID, stats.TotalEvents)
	return Summary{
		Text:       text,
		EventCount: stats.TotalEvents,
	}, nil
}

// Close releases all open file handles.
// Should be called when the store is no longer needed.
func (f *FileChatStore) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var errs []error
	for sessionID, entry := range f.files {
		entry.mu.Lock()
		err := entry.file.Close()
		entry.mu.Unlock()
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to close file for session %s: %w", sessionID, err))
		}
		delete(f.files, sessionID)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing files: %v", errs)
	}
	return nil
}

// getOrOpenFile returns an open file entry for the session.
func (f *FileChatStore) getOrOpenFile(sessionID string) (*fileEntry, error) {
	f.mu.RLock()
	entry, ok := f.files[sessionID]
	f.mu.RUnlock()
	if ok && entry != nil {
		return entry, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if entry, ok := f.files[sessionID]; ok && entry != nil {
		return entry, nil
	}

	filePath := f.sessionFilePath(sessionID)

	// Open or create file in append mode
	file, err := f.openFile(filePath)
	if err != nil {
		return nil, err
	}

	entry = &fileEntry{file: file}
	f.files[sessionID] = entry
	return entry, nil
}

func (f *FileChatStore) lockSession(sessionID string) (func(), error) {
	entry, err := f.getOrOpenFile(sessionID)
	if err != nil {
		return nil, err
	}
	entry.mu.Lock()
	return entry.mu.Unlock, nil
}

// SyncEvery returns the current fsync cadence.
func (f *FileChatStore) SyncEvery() int {
	if f.syncEvery <= 0 {
		return 1
	}
	return f.syncEvery
}

// sessionFilePath returns the file path for a given session ID.
func (f *FileChatStore) sessionFilePath(sessionID string) string {
	return filepath.Join(f.basePath, sessionID+".jsonl")
}
