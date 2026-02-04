package memory

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

func TestNewFileChatStore(t *testing.T) {
	t.Run("creates directory if not exists", func(t *testing.T) {
		tmpDir := filepath.Join(t.TempDir(), "sessions")
		store, err := NewFileChatStore(tmpDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				t.Logf("cleanup: %v", err)
			}
		}()

		if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
			t.Error("directory was not created")
		}
	})

	t.Run("rejects empty base path", func(t *testing.T) {
		_, err := NewFileChatStore("")
		if err == nil {
			t.Error("expected error for empty base path")
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		tmpDir := filepath.Join(t.TempDir(), "a", "b", "c")
		store, err := NewFileChatStore(tmpDir)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				t.Logf("cleanup: %v", err)
			}
		}()

		if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
			t.Error("nested directories were not created")
		}
	})
}

func TestFileChatStore_Append(t *testing.T) {
	ctx := context.Background()

	t.Run("appends event successfully", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		ev := event.New(event.AgentStarted, "session-1", nil)
		err := store.Append(ctx, "session-1", ev)
		if err != nil {
			t.Fatalf("failed to append event: %v", err)
		}

		// Verify file was created
		filePath := store.sessionFilePath("session-1")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("session file was not created")
		}
	})

	t.Run("rejects empty session ID", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		ev := event.New(event.AgentStarted, "test-session", nil)
		err := store.Append(ctx, "", ev)
		if err == nil {
			t.Error("expected error for empty session ID")
		}
	})

	t.Run("rejects invalid session ID with path traversal", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		ev := event.New(event.AgentStarted, "test-session", nil)

		invalidIDs := []string{"../etc/passwd", "foo/bar", "foo\\bar", ".."}
		for _, id := range invalidIDs {
			err := store.Append(ctx, id, ev)
			if err == nil {
				t.Errorf("expected error for invalid session ID: %s", id)
			}
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		ev := event.New(event.AgentStarted, "session-1", nil)
		err := store.Append(ctx, "session-1", ev)
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("appends multiple events to same session", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		sessionID := "multi-event-session"
		for i := 0; i < 5; i++ {
			ev := event.New(event.AgentStarted, sessionID, map[string]any{"index": i})
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append event %d: %v", i, err)
			}
		}

		items, err := store.Query(ctx, Query{SessionID: sessionID})
		if err != nil {
			t.Fatalf("failed to query: %v", err)
		}
		if len(items) != 5 {
			t.Errorf("expected 5 events, got %d", len(items))
		}
	})
}

func TestFileChatStore_Query(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty for non-existent session", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		items, err := store.Query(ctx, Query{SessionID: "non-existent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("returns events in chronological order", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		sessionID := "ordered-session"

		for i := 0; i < 3; i++ {
			time.Sleep(10 * time.Millisecond) // Ensure different timestamps
			ev := event.New(event.AgentStarted, sessionID, map[string]any{"index": i})
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}

		items, err := store.Query(ctx, Query{SessionID: sessionID})
		if err != nil {
			t.Fatalf("failed to query: %v", err)
		}

		for i := 0; i < len(items)-1; i++ {
			if items[i].Event.Time.After(items[i+1].Event.Time) {
				t.Error("events not in chronological order")
			}
		}
	})

	t.Run("filters by event type", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		sessionID := "filtered-session"

		// Add different event types
		events := []event.Type{
			event.AgentStarted,
			event.AgentMessageComplete,
			event.AgentStarted,
			event.ToolCallStarted,
		}

		for _, evType := range events {
			ev := event.New(evType, sessionID, nil)
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}

		// Query only AgentStarted events
		items, err := store.Query(ctx, Query{
			SessionID: sessionID,
			Types:     []event.Type{event.AgentStarted},
		})
		if err != nil {
			t.Fatalf("failed to query: %v", err)
		}

		if len(items) != 2 {
			t.Errorf("expected 2 AgentStarted events, got %d", len(items))
		}

		for _, item := range items {
			if item.Event.Type != event.AgentStarted {
				t.Errorf("unexpected event type: %s", item.Event.Type)
			}
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		sessionID := "limited-session"

		// Add 10 events
		for i := 0; i < 10; i++ {
			ev := event.New(event.AgentStarted, sessionID, nil)
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}

		// Query with limit
		items, err := store.Query(ctx, Query{
			SessionID: sessionID,
			Limit:     3,
		})
		if err != nil {
			t.Fatalf("failed to query: %v", err)
		}

		if len(items) != 3 {
			t.Errorf("expected 3 items (limit), got %d", len(items))
		}
	})

	t.Run("handles corrupted lines gracefully", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		sessionID := "corrupted-session"

		// Add valid event
		ev := event.New(event.AgentStarted, sessionID, nil)
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("failed to append: %v", err)
		}

		// Manually append corrupted data
		filePath := store.sessionFilePath(sessionID)
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0600) // #nosec G304 G302 - test file
		if err != nil {
			t.Fatalf("failed to open file: %v", err)
		}
		_, _ = f.WriteString("{invalid json}\n") // Intentional corrupted line for test
		_ = f.Close()

		// Add another valid event
		ev2 := event.New(event.AgentMessageComplete, sessionID, nil)
		if err := store.Append(ctx, sessionID, ev2); err != nil {
			t.Fatalf("failed to append: %v", err)
		}

		// Query should skip corrupted line and return valid events
		items, err := store.Query(ctx, Query{SessionID: sessionID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(items) != 2 {
			t.Errorf("expected 2 valid events, got %d", len(items))
		}
	})
}

func TestFileChatStore_ConcurrentAppend(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupFileStore(t)
	defer cleanup()

	const goroutines = 10
	const eventsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			sessionID := "concurrent-session"

			for j := 0; j < eventsPerGoroutine; j++ {
				ev := event.New(event.AgentStarted, sessionID, map[string]any{
					"goroutine": id,
					"index":     j,
				})
				if err := store.Append(ctx, sessionID, ev); err != nil {
					t.Errorf("goroutine %d: failed to append: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all events were stored
	items, err := store.Query(ctx, Query{SessionID: "concurrent-session"})
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	expected := goroutines * eventsPerGoroutine
	if len(items) != expected {
		t.Errorf("expected %d events, got %d", expected, len(items))
	}
}

func TestFileChatStore_ConcurrentReadWrite(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupFileStore(t)
	defer cleanup()

	sessionID := "rw-session"
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			ev := event.New(event.AgentStarted, sessionID, nil)
			store.Append(ctx, sessionID, ev)
			time.Sleep(time.Millisecond)
		}
		close(done)
	}()

	// Reader goroutines
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_, _ = store.Query(ctx, Query{SessionID: sessionID}) // Ignore errors in test goroutine
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()

	// Verify final count
	items, err := store.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}
	if len(items) != 100 {
		t.Errorf("expected 100 events, got %d", len(items))
	}
}

func TestFileChatStore_PerSessionLock(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupFileStore(t)
	defer cleanup()

	unlock, err := store.lockSession("session-a")
	if err != nil {
		t.Fatalf("lock session: %v", err)
	}
	defer unlock()

	done := make(chan struct{})
	go func() {
		ev := event.New(event.AgentStarted, "session-b", nil)
		_ = store.Append(ctx, "session-b", ev)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("append to different session blocked")
	}
}

type countingFile struct {
	file      *os.File
	syncCount int
}

func (c *countingFile) Write(p []byte) (int, error) { return c.file.Write(p) }
func (c *countingFile) Sync() error {
	c.syncCount++
	return c.file.Sync()
}
func (c *countingFile) Close() error { return c.file.Close() }

func TestFileChatStoreSyncEvery(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupFileStore(t)
	defer cleanup()

	var countFile *countingFile
	store.openFile = func(path string) (syncFile, error) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // #nosec G304 G302 - test file
		if err != nil {
			return nil, err
		}
		countFile = &countingFile{file: f}
		return countFile, nil
	}
	store.syncEvery = 3

	sessionID := "sync-every"
	for i := 0; i < 5; i++ {
		if err := store.Append(ctx, sessionID, event.New(event.AgentStarted, sessionID, nil)); err != nil {
			t.Fatalf("append failed: %v", err)
		}
	}

	if countFile == nil {
		t.Fatalf("expected counting file to be used")
	}
	if countFile.syncCount != 1 {
		t.Fatalf("expected 1 sync with syncEvery=3, got %d", countFile.syncCount)
	}
}

func TestFileChatStore_MultipleReaders(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupFileStore(t)
	defer cleanup()

	sessionID := "multi-reader"

	// Populate with events
	for i := 0; i < 50; i++ {
		ev := event.New(event.AgentStarted, sessionID, nil)
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("failed to append: %v", err)
		}
	}

	// Multiple concurrent readers
	var wg sync.WaitGroup
	const readers = 10
	wg.Add(readers)

	for i := 0; i < readers; i++ {
		go func(id int) {
			defer wg.Done()
			items, err := store.Query(ctx, Query{SessionID: sessionID})
			if err != nil {
				t.Errorf("reader %d: failed to query: %v", id, err)
				return
			}
			if len(items) != 50 {
				t.Errorf("reader %d: expected 50 items, got %d", id, len(items))
			}
		}(i)
	}

	wg.Wait()
}

func TestFileChatStore_LargeSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large session test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupFileStore(t)
	defer cleanup()

	sessionID := "large-session"
	const eventCount = 10000

	// Append many events
	for i := 0; i < eventCount; i++ {
		ev := event.New(event.AgentStarted, sessionID, map[string]any{
			"index": i,
			"data":  "some payload to make event larger",
		})
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("failed to append event %d: %v", i, err)
		}
	}

	// Query all events
	items, err := store.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if len(items) != eventCount {
		t.Errorf("expected %d events, got %d", eventCount, len(items))
	}
}

func TestFileChatStore_Close(t *testing.T) {
	ctx := context.Background()
	store, cleanup := setupFileStore(t)
	defer cleanup()

	// Create some sessions
	sessions := []string{"session-1", "session-2", "session-3"}
	for _, sessionID := range sessions {
		ev := event.New(event.AgentStarted, sessionID, nil)
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("failed to append: %v", err)
		}
	}

	// Close store
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close store: %v", err)
	}

	// Verify all file handles are closed
	if len(store.files) != 0 {
		t.Errorf("expected no open files, got %d", len(store.files))
	}
}

func TestFileChatStore_Stats(t *testing.T) {
	ctx := context.Background()

	t.Run("calculates event statistics", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		sessionID := "stats-session"

		// Add various event types
		events := []event.Type{
			event.AgentStarted,
			event.AgentMessageComplete,
			event.AgentStarted,
			event.ToolCallStarted,
		}

		for _, evType := range events {
			ev := event.New(evType, sessionID, nil)
			if err := store.Append(ctx, sessionID, ev); err != nil {
				t.Fatalf("failed to append: %v", err)
			}
		}

		stats, err := store.Stats(ctx, sessionID, Policy{})
		if err != nil {
			t.Fatalf("failed to get stats: %v", err)
		}

		if stats.TotalEvents != 4 {
			t.Errorf("expected 4 total events, got %d", stats.TotalEvents)
		}

		if stats.EventCounts[event.AgentStarted] != 2 {
			t.Errorf("expected 2 AgentStarted events, got %d", stats.EventCounts[event.AgentStarted])
		}

		if stats.SessionID != sessionID {
			t.Errorf("expected session ID %s, got %s", sessionID, stats.SessionID)
		}
	})

	t.Run("returns empty stats for non-existent session", func(t *testing.T) {
		store, cleanup := setupFileStore(t)
		defer cleanup()

		stats, err := store.Stats(ctx, "non-existent", Policy{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if stats.TotalEvents != 0 {
			t.Errorf("expected 0 events, got %d", stats.TotalEvents)
		}
	})
}

// Helper function to set up a file store with cleanup
func setupFileStore(t *testing.T) (*FileChatStore, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := NewFileChatStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return store, func() {
		if err := store.Close(); err != nil {
			t.Logf("cleanup: failed to close store: %v", err)
		}
	}
}
