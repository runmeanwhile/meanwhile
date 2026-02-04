package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

// PostgresStore implements Store using PostgreSQL for persistent, scalable event storage.
// It's designed for production use cases requiring durability, multi-process access,
// and advanced querying capabilities.
//
// The store automatically creates tables on first use and supports custom schema/table naming.
// All queries use parameterized statements for SQL injection protection.
//
// Thread-safe for concurrent access across goroutines and processes.
type PostgresStore struct {
	db     *sql.DB
	schema string
	table  string
	mu     sync.RWMutex
}

// PostgresOption configures PostgresStore behavior.
type PostgresOption func(*postgresConfig)

type postgresConfig struct {
	schema      string
	table       string
	autoMigrate bool
}

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func validateIdentifier(value, label string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", label)
	}
	if !identifierRe.MatchString(value) {
		return fmt.Errorf("invalid %s: %q", label, value)
	}
	return nil
}

// WithSchema sets the database schema name for the events table.
// Default: "public"
func WithSchema(name string) PostgresOption {
	return func(c *postgresConfig) {
		c.schema = name
	}
}

// WithTable sets the table name for storing events.
// Default: "meanwhile_events"
func WithTable(name string) PostgresOption {
	return func(c *postgresConfig) {
		c.table = name
	}
}

// WithAutoMigrate enables automatic table creation on initialization.
// Default: true
func WithAutoMigrate(enable bool) PostgresOption {
	return func(c *postgresConfig) {
		c.autoMigrate = enable
	}
}

// NewPostgresStore creates a new PostgreSQL-backed memory store.
//
// The connection string format is standard PostgreSQL:
//
//	postgresql://user:password@host:port/database?sslmode=disable
//
// The framework handles table creation and schema management.
// Users must add the PostgreSQL driver to their go.mod:
//
//	import _ "github.com/lib/pq"
//
// Example:
//
//	store, err := memory.NewPostgresStore(
//	    os.Getenv("DATABASE_URL"),
//	    memory.WithSchema("meanwhile"),
//	    memory.WithAutoMigrate(true),
//	)
func NewPostgresStore(connString string, opts ...PostgresOption) (*PostgresStore, error) {
	cfg := &postgresConfig{
		schema:      "public",
		table:       "meanwhile_events",
		autoMigrate: true,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if err := validateIdentifier(cfg.schema, "schema"); err != nil {
		return nil, err
	}
	if err := validateIdentifier(cfg.table, "table"); err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &PostgresStore{
		db:     db,
		schema: cfg.schema,
		table:  cfg.table,
	}

	if cfg.autoMigrate {
		if err := store.migrate(); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to migrate schema: %w", err)
		}
	}

	return store, nil
}

// migrate creates the events table and indexes if they don't exist.
// Idempotent - safe to call multiple times.
func (s *PostgresStore) migrate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create schema if it doesn't exist
	if s.schema != "public" {
		schemaSQL := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", s.schema)
		if _, err := s.db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
	}

	// Create table with full qualified name
	fullTable := s.fullTableName()
	tableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			session_id TEXT NOT NULL,
			event_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			agent_id TEXT,
			tool_id TEXT,
			protocol_id TEXT,
			payload JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`, fullTable)

	if _, err := s.db.Exec(tableSQL); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indexes for common queries
	indexes := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_session ON %s(session_id)", s.table, fullTable),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_type ON %s(event_type)", s.table, fullTable),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_created ON %s(created_at DESC)", s.table, fullTable),
	}

	for _, idxSQL := range indexes {
		if _, err := s.db.Exec(idxSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Ensure new columns exist for backward-compatible upgrades.
	alterStatements := []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS agent_id TEXT", fullTable),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS tool_id TEXT", fullTable),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS protocol_id TEXT", fullTable),
	}
	for _, alterSQL := range alterStatements {
		if _, err := s.db.Exec(alterSQL); err != nil {
			return fmt.Errorf("failed to alter table: %w", err)
		}
	}

	return nil
}

// fullTableName returns the fully qualified table name (schema.table).
func (s *PostgresStore) fullTableName() string {
	return fmt.Sprintf("%s.%s", s.schema, s.table)
}

// Append stores an event in the PostgreSQL database.
// Thread-safe and supports concurrent writes from multiple goroutines/processes.
func (s *PostgresStore) Append(ctx context.Context, sessionID string, ev event.Event) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}

	payload, err := json.Marshal(ev.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event payload: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (session_id, event_id, event_type, agent_id, tool_id, protocol_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, s.fullTableName())

	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err = s.db.ExecContext(ctx, query,
		sessionID,
		ev.ID,
		ev.Type,
		ev.AgentID,
		ev.ToolID,
		ev.ProtocolID,
		payload,
		ev.Time,
	)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// Query retrieves events from the PostgreSQL database matching the given criteria.
// Results are returned in chronological order (oldest first).
func (s *PostgresStore) Query(ctx context.Context, q Query) ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build WHERE clause
	var whereClauses []string
	var args []any
	argPos := 1

	if q.SessionID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("session_id = $%d", argPos))
		args = append(args, q.SessionID)
		argPos++
	}

	if len(q.Types) > 0 {
		// Convert event types to strings
		typeStrs := make([]any, len(q.Types))
		for i, t := range q.Types {
			typeStrs[i] = string(t)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("event_type = ANY($%d)", argPos))
		args = append(args, typeStrs)
		argPos++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + whereClauses[0]
		for i := 1; i < len(whereClauses); i++ {
			whereClause += " AND " + whereClauses[i]
		}
	}

	// Build ORDER BY and LIMIT
	orderClause := "ORDER BY created_at DESC"
	limitClause := ""
	if q.Limit > 0 {
		limitClause = fmt.Sprintf("LIMIT $%d", argPos)
		args = append(args, q.Limit)
	}

	query := fmt.Sprintf(`
		SELECT session_id, event_id, event_type, agent_id, tool_id, protocol_id, payload, created_at
		FROM %s
		%s
		%s
		%s
	`, s.fullTableName(), whereClause, orderClause, limitClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		var sessionID string
		var agentID sql.NullString
		var toolID sql.NullString
		var protocolID sql.NullString
		var payloadJSON []byte
		var createdAt time.Time

		err := rows.Scan(
			&sessionID,
			&item.Event.ID,
			&item.Event.Type,
			&agentID,
			&toolID,
			&protocolID,
			&payloadJSON,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Unmarshal payload
		if err := json.Unmarshal(payloadJSON, &item.Event.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		item.Event.SessionID = sessionID
		if agentID.Valid {
			item.Event.AgentID = agentID.String
		}
		if toolID.Valid {
			item.Event.ToolID = toolID.String
		}
		if protocolID.Valid {
			item.Event.ProtocolID = protocolID.String
		}
		item.Event.Time = createdAt
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Reverse to chronological order (oldest first).
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}

	return items, nil
}

// Stats returns statistics about events in a session.
// Implements the Store interface.
func (s *PostgresStore) Stats(ctx context.Context, sessionID string, policy Policy) (EventStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total,
			MIN(created_at) as first_event,
			MAX(created_at) as last_event
		FROM %s
		WHERE session_id = $1
	`, s.fullTableName())

	var stats EventStats
	var firstEvent, lastEvent sql.NullTime

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&stats.TotalEvents,
		&firstEvent,
		&lastEvent,
	)
	if err != nil {
		return EventStats{}, fmt.Errorf("failed to query stats: %w", err)
	}

	stats.SessionID = sessionID
	if firstEvent.Valid {
		stats.FirstEvent = firstEvent.Time
	}
	if lastEvent.Valid {
		stats.LastEvent = lastEvent.Time
	}

	// Get event type breakdown
	typeQuery := fmt.Sprintf(`
		SELECT event_type, COUNT(*) as count
		FROM %s
		WHERE session_id = $1
		GROUP BY event_type
		ORDER BY count DESC
	`, s.fullTableName())

	rows, err := s.db.QueryContext(ctx, typeQuery, sessionID)
	if err != nil {
		return EventStats{}, fmt.Errorf("failed to query type breakdown: %w", err)
	}
	defer rows.Close()

	stats.EventCounts = make(map[event.Type]int)
	for rows.Next() {
		var eventType string
		var count int
		if err := rows.Scan(&eventType, &count); err != nil {
			return EventStats{}, fmt.Errorf("failed to scan type count: %w", err)
		}
		stats.EventCounts[event.Type(eventType)] = count
	}

	return stats, nil
}

// Summarize returns a text summary of session events.
// Deprecated: Use Stats() instead for structured event statistics.
func (s *PostgresStore) Summarize(ctx context.Context, sessionID string, policy Policy) (Summary, error) {
	stats, err := s.Stats(ctx, sessionID, policy)
	if err != nil {
		return Summary{}, err
	}

	// Simple text summary for backward compatibility
	text := fmt.Sprintf("Session %s has %d events", sessionID, stats.TotalEvents)
	return Summary{
		Text:       text,
		EventCount: stats.TotalEvents,
	}, nil
}

// Close closes the database connection.
// After calling Close, the store cannot be used.
func (s *PostgresStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
