package memory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver

	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

// getTestDBURL returns a PostgreSQL connection string for testing.
// Set POSTGRES_TEST_URL environment variable or use default.
func getTestDBURL() string {
	url := os.Getenv("POSTGRES_TEST_URL")
	if url == "" {
		// Default to local PostgreSQL for testing
		url = "postgresql://postgres:postgres@localhost:5432/meanwhile_test?sslmode=disable"
	}
	return url
}

// skipIfNoPostgres skips the test if PostgreSQL is not available.
func skipIfNoPostgres(t *testing.T) {
	t.Helper()

	db, err := sql.Open("postgres", getTestDBURL())
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("PostgreSQL not reachable: %v", err)
	}
}

// cleanupTestTable drops the test table after tests.
func cleanupTestTable(t *testing.T, store *PostgresStore) {
	t.Helper()

	query := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", store.fullTableName())
	_, err := store.db.Exec(query)
	if err != nil {
		t.Logf("Warning: failed to cleanup table: %v", err)
	}
}

func TestNewPostgresStore(t *testing.T) {
	skipIfNoPostgres(t)

	tests := []struct {
		name    string
		connStr string
		opts    []PostgresOption
		wantErr bool
	}{
		{
			name:    "valid connection with defaults",
			connStr: getTestDBURL(),
			opts:    nil,
			wantErr: false,
		},
		{
			name:    "custom schema and table",
			connStr: getTestDBURL(),
			opts: []PostgresOption{
				WithSchema("test_schema"),
				WithTable("test_events"),
			},
			wantErr: false,
		},
		{
			name:    "auto migrate disabled",
			connStr: getTestDBURL(),
			opts: []PostgresOption{
				WithAutoMigrate(false),
			},
			wantErr: false,
		},
		{
			name:    "invalid connection string",
			connStr: "invalid://connection",
			opts:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewPostgresStore(tt.connStr, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPostgresStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if store != nil {
				defer store.Close()
				defer cleanupTestTable(t, store)
			}
		})
	}
}

func TestNewPostgresStoreRejectsInvalidIdentifiers(t *testing.T) {
	_, err := NewPostgresStore("postgresql://invalid", WithSchema("bad;drop"), WithTable("ok"))
	if err == nil {
		t.Fatalf("expected error for invalid schema identifier")
	}

	_, err = NewPostgresStore("postgresql://invalid", WithSchema("ok"), WithTable("bad space"))
	if err == nil {
		t.Fatalf("expected error for invalid table identifier")
	}
}

func TestPostgresStore_Migrate(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(
		getTestDBURL(),
		WithSchema("test_migrate"),
		WithTable("events"),
	)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	// Verify table exists
	query := `
		SELECT COUNT(*) 
		FROM information_schema.tables 
		WHERE table_schema = $1 AND table_name = $2
	`
	var count int
	err = store.db.QueryRow(query, "test_migrate", "events").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check table existence: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected table to exist, got count = %d", count)
	}

	// Verify indexes exist
	indexQuery := `
		SELECT COUNT(*) 
		FROM pg_indexes 
		WHERE schemaname = $1 AND tablename = $2
	`
	var indexCount int
	err = store.db.QueryRow(indexQuery, "test_migrate", "events").Scan(&indexCount)
	if err != nil {
		t.Fatalf("Failed to check indexes: %v", err)
	}
	if indexCount < 3 {
		t.Errorf("Expected at least 3 indexes, got %d", indexCount)
	}

	// Test idempotency - migrate again
	err = store.migrate()
	if err != nil {
		t.Errorf("Second migration should be idempotent, got error: %v", err)
	}
}

func TestPostgresStore_Append(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	sessionID := "test-session"

	ev := event.Event{
		ID:   "event-1",
		Type: event.AgentStarted,
		Time: time.Now(),
		Payload: map[string]interface{}{
			"agent": "test-agent",
			"data":  "test-data",
		},
	}

	err = store.Append(ctx, sessionID, ev)
	if err != nil {
		t.Fatalf("Failed to append event: %v", err)
	}

	// Verify event was stored
	items, err := store.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(items))
	}

	if items[0].Event.ID != ev.ID {
		t.Errorf("Expected event ID %s, got %s", ev.ID, items[0].Event.ID)
	}
	if items[0].Event.Type != ev.Type {
		t.Errorf("Expected event type %s, got %s", ev.Type, items[0].Event.Type)
	}
}

func TestPostgresStore_PreservesEventMetadata(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	sessionID := "meta-session"

	ev := event.Event{
		ID:         "event-meta-1",
		Type:       event.AgentMessageComplete,
		Time:       time.Now().UTC(),
		SessionID:  "ignored-session",
		AgentID:    "agent-123",
		ToolID:     "tool-abc",
		ProtocolID: "protocol.consensus",
		Payload: map[string]any{
			"note": "metadata test",
		},
	}

	if err := store.Append(ctx, sessionID, ev); err != nil {
		t.Fatalf("Failed to append event: %v", err)
	}

	items, err := store.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(items))
	}

	got := items[0].Event
	if got.SessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, got.SessionID)
	}
	if got.AgentID != ev.AgentID {
		t.Errorf("Expected agent ID %s, got %s", ev.AgentID, got.AgentID)
	}
	if got.ToolID != ev.ToolID {
		t.Errorf("Expected tool ID %s, got %s", ev.ToolID, got.ToolID)
	}
	if got.ProtocolID != ev.ProtocolID {
		t.Errorf("Expected protocol ID %s, got %s", ev.ProtocolID, got.ProtocolID)
	}
	if got.ID != ev.ID {
		t.Errorf("Expected event ID %s, got %s", ev.ID, got.ID)
	}
}

func TestPostgresStore_Append_EmptySessionID(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	ev := event.Event{
		ID:   "event-1",
		Type: event.AgentStarted,
	}

	err = store.Append(ctx, "", ev)
	if err == nil {
		t.Error("Expected error for empty sessionID, got nil")
	}
}

func TestPostgresStore_Query(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	sessionID := "test-session"

	// Insert multiple events
	events := []event.Event{
		{
			ID:      "event-1",
			Type:    event.AgentStarted,
			Time:    time.Now().Add(-2 * time.Minute),
			Payload: map[string]interface{}{"index": 1},
		},
		{
			ID:      "event-2",
			Type:    event.ToolCallStarted,
			Time:    time.Now().Add(-1 * time.Minute),
			Payload: map[string]interface{}{"index": 2},
		},
		{
			ID:      "event-3",
			Type:    event.AgentMessageComplete,
			Time:    time.Now(),
			Payload: map[string]interface{}{"index": 3},
		},
	}

	for _, ev := range events {
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("Failed to append event: %v", err)
		}
	}

	tests := []struct {
		name      string
		query     Query
		wantCount int
	}{
		{
			name:      "all events",
			query:     Query{SessionID: sessionID},
			wantCount: 3,
		},
		{
			name:      "with limit",
			query:     Query{SessionID: sessionID, Limit: 2},
			wantCount: 2,
		},
		{
			name:      "filter by type",
			query:     Query{SessionID: sessionID, Types: []event.Type{event.AgentStarted}},
			wantCount: 1,
		},
		{
			name:      "multiple types",
			query:     Query{SessionID: sessionID, Types: []event.Type{event.AgentStarted, event.AgentMessageComplete}},
			wantCount: 2,
		},
		{
			name:      "non-existent session",
			query:     Query{SessionID: "non-existent"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := store.Query(ctx, tt.query)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}
			if len(items) != tt.wantCount {
				t.Errorf("Expected %d items, got %d", tt.wantCount, len(items))
			}
		})
	}
}

func TestPostgresStore_Query_ReverseChronological(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	sessionID := "test-session"

	// Insert events with specific timestamps
	baseTime := time.Now()
	events := []event.Event{
		{ID: "event-1", Type: event.AgentStarted, Time: baseTime.Add(-3 * time.Second)},
		{ID: "event-2", Type: event.ToolCallStarted, Time: baseTime.Add(-2 * time.Second)},
		{ID: "event-3", Type: event.ToolCallCompleted, Time: baseTime.Add(-1 * time.Second)},
		{ID: "event-4", Type: event.AgentMessageComplete, Time: baseTime},
	}

	for _, ev := range events {
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("Failed to append event: %v", err)
		}
	}

	items, err := store.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Verify reverse chronological order (newest first)
	if len(items) != 4 {
		t.Fatalf("Expected 4 items, got %d", len(items))
	}

	expectedOrder := []string{"event-4", "event-3", "event-2", "event-1"}
	for i, item := range items {
		if item.Event.ID != expectedOrder[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expectedOrder[i], item.Event.ID)
		}
	}
}

func TestPostgresStore_ConcurrentAppend(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	sessionID := "concurrent-session"
	numGoroutines := 10
	eventsPerGoroutine := 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				ev := event.Event{
					ID:      fmt.Sprintf("event-%d-%d", goroutineID, j),
					Type:    event.AgentStarted,
					Time:    time.Now(),
					Payload: map[string]interface{}{"goroutine": goroutineID, "index": j},
				}
				if err := store.Append(ctx, sessionID, ev); err != nil {
					t.Errorf("Concurrent append failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all events were stored
	items, err := store.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	expectedCount := numGoroutines * eventsPerGoroutine
	if len(items) != expectedCount {
		t.Errorf("Expected %d events, got %d", expectedCount, len(items))
	}
}

func TestPostgresStore_Stats(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	sessionID := "stats-session"

	// Insert events with different types
	events := []event.Event{
		{ID: "1", Type: event.AgentStarted, Time: time.Now().Add(-5 * time.Minute)},
		{ID: "2", Type: event.ToolCallStarted, Time: time.Now().Add(-4 * time.Minute)},
		{ID: "3", Type: event.ToolCallStarted, Time: time.Now().Add(-3 * time.Minute)},
		{ID: "4", Type: event.ToolCallCompleted, Time: time.Now().Add(-2 * time.Minute)},
		{ID: "5", Type: event.AgentMessageComplete, Time: time.Now().Add(-1 * time.Minute)},
	}

	for _, ev := range events {
		if err := store.Append(ctx, sessionID, ev); err != nil {
			t.Fatalf("Failed to append event: %v", err)
		}
	}

	stats, err := store.Stats(ctx, sessionID, Policy{})
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.SessionID != sessionID {
		t.Errorf("Expected sessionID %s, got %s", sessionID, stats.SessionID)
	}
	if stats.TotalEvents != 5 {
		t.Errorf("Expected 5 total events, got %d", stats.TotalEvents)
	}
	if stats.FirstEvent.IsZero() {
		t.Error("FirstEvent should not be zero")
	}
	if stats.LastEvent.IsZero() {
		t.Error("LastEvent should not be zero")
	}

	// Verify type counts
	expectedCounts := map[event.Type]int{
		event.AgentStarted:         1,
		event.ToolCallStarted:      2,
		event.ToolCallCompleted:    1,
		event.AgentMessageComplete: 1,
	}

	for typ, expectedCount := range expectedCounts {
		if count, ok := stats.EventCounts[typ]; !ok || count != expectedCount {
			t.Errorf("Type %s: expected count %d, got %d", typ, expectedCount, count)
		}
	}
}

func TestPostgresStore_Stats_EmptySession(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	stats, err := store.Stats(ctx, "non-existent-session", Policy{})
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 events for empty session, got %d", stats.TotalEvents)
	}
}

func TestPostgresStore_MultipleSchemas(t *testing.T) {
	skipIfNoPostgres(t)

	// Create two stores with different schemas
	store1, err := NewPostgresStore(
		getTestDBURL(),
		WithSchema("schema1"),
		WithTable("events"),
	)
	if err != nil {
		t.Fatalf("Failed to create store1: %v", err)
	}
	defer store1.Close()
	defer cleanupTestTable(t, store1)

	store2, err := NewPostgresStore(
		getTestDBURL(),
		WithSchema("schema2"),
		WithTable("events"),
	)
	if err != nil {
		t.Fatalf("Failed to create store2: %v", err)
	}
	defer store2.Close()
	defer cleanupTestTable(t, store2)

	ctx := context.Background()
	sessionID := "test-session"

	// Append to store1
	ev1 := event.Event{
		ID:   "event-1",
		Type: event.AgentStarted,
		Time: time.Now(),
	}
	if err := store1.Append(ctx, sessionID, ev1); err != nil {
		t.Fatalf("Failed to append to store1: %v", err)
	}

	// Append to store2
	ev2 := event.Event{
		ID:   "event-2",
		Type: event.AgentMessageComplete,
		Time: time.Now(),
	}
	if err := store2.Append(ctx, sessionID, ev2); err != nil {
		t.Fatalf("Failed to append to store2: %v", err)
	}

	// Verify isolation
	items1, err := store1.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Failed to query store1: %v", err)
	}
	if len(items1) != 1 || items1[0].Event.ID != "event-1" {
		t.Errorf("Store1 should only have event-1")
	}

	items2, err := store2.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Failed to query store2: %v", err)
	}
	if len(items2) != 1 || items2[0].Event.ID != "event-2" {
		t.Errorf("Store2 should only have event-2")
	}
}

func TestPostgresStore_Close(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer cleanupTestTable(t, store)

	if err := store.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify connection is closed
	if err := store.db.Ping(); err == nil {
		t.Error("Expected ping to fail after close")
	}
}

func TestPostgresStore_LargePayload(t *testing.T) {
	skipIfNoPostgres(t)

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(t, store)

	ctx := context.Background()
	sessionID := "large-payload-session"

	// Create a large payload
	largeData := make([]byte, 1024*100) // 100KB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	ev := event.Event{
		ID:   "large-event",
		Type: event.AgentStarted,
		Time: time.Now(),
		Payload: map[string]interface{}{
			"data": largeData,
		},
	}

	if err := store.Append(ctx, sessionID, ev); err != nil {
		t.Fatalf("Failed to append large event: %v", err)
	}

	// Query and verify
	items, err := store.Query(ctx, Query{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	// Verify payload was stored correctly
	if items[0].Event.Payload == nil {
		t.Error("Large payload not retrieved correctly")
	}
}

func BenchmarkPostgresStore_Append(b *testing.B) {
	skipIfNoPostgres(&testing.T{})

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(&testing.T{}, store)

	ctx := context.Background()
	sessionID := "bench-session"

	ev := event.Event{
		ID:      "bench-event",
		Type:    event.AgentStarted,
		Time:    time.Now(),
		Payload: map[string]interface{}{"data": "benchmark"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.ID = fmt.Sprintf("event-%d", i)
		if err := store.Append(ctx, sessionID, ev); err != nil {
			b.Fatalf("Append failed: %v", err)
		}
	}
}

func BenchmarkPostgresStore_Query(b *testing.B) {
	skipIfNoPostgres(&testing.T{})

	store, err := NewPostgresStore(getTestDBURL())
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()
	defer cleanupTestTable(&testing.T{}, store)

	ctx := context.Background()
	sessionID := "bench-session"

	// Pre-populate with events
	for i := 0; i < 1000; i++ {
		ev := event.Event{
			ID:      fmt.Sprintf("event-%d", i),
			Type:    event.AgentStarted,
			Time:    time.Now(),
			Payload: map[string]interface{}{"index": i},
		}
		if err := store.Append(ctx, sessionID, ev); err != nil {
			b.Fatalf("Failed to append: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.Query(ctx, Query{SessionID: sessionID, Limit: 100})
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}
