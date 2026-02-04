# Example 15: PostgreSQL Memory Store

This example demonstrates how to use PostgreSQL as a persistent, scalable memory backend for Meanwhile sessions.

## Features

- **Persistent Storage**: Events survive application restarts
- **Multi-Process Safe**: Multiple application instances can share the same database
- **Scalable**: Handle millions of events with database-backed storage
- **Session Isolation**: Each session's events are properly isolated
- **Advanced Querying**: Leverage SQL for complex queries

## Prerequisites

1. **PostgreSQL Server**: Install and run PostgreSQL locally or use a hosted service
2. **OpenAI API Key**: Set `OPENAI_API_KEY` environment variable
3. **PostgreSQL Driver**: The example uses `github.com/lib/pq` (already included)

## Setup

### Install PostgreSQL (macOS)

```bash
brew install postgresql@14
brew services start postgresql@14
```

### Create Database

```bash
createdb meanwhile
```

Or connect with custom credentials:

```bash
psql -U postgres -c "CREATE DATABASE meanwhile;"
```

## Running the Example

### With Default Connection

```bash
go run examples/15-postgres-memory/main.go
```

This uses: `postgresql://postgres:postgres@localhost:5432/meanwhile?sslmode=disable`

### With Custom Connection

```bash
export DATABASE_URL="postgresql://user:password@host:port/dbname?sslmode=disable"
go run examples/15-postgres-memory/main.go
```

## What It Does

1. **Creates PostgreSQL Store**: Connects to database and auto-creates tables/indexes
2. **Session 1**: Runs two conversations in the same session (ticket-001)
3. **Session 2**: Starts a new session with different ID (ticket-002)
4. **Queries Memory**: Retrieves events and statistics from both sessions
5. **Verifies Isolation**: Confirms sessions don't interfere with each other

## Database Schema

The store automatically creates:

```sql
CREATE SCHEMA IF NOT EXISTS meanwhile;

CREATE TABLE meanwhile.events (
    id SERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_events_session ON meanwhile.events(session_id);
CREATE INDEX idx_events_type ON meanwhile.events(event_type);
CREATE INDEX idx_events_created ON meanwhile.events(created_at DESC);
```

## Inspecting the Data

```sql
-- View all events
SELECT session_id, event_type, created_at 
FROM meanwhile.events 
ORDER BY created_at DESC;

-- Count events per session
SELECT session_id, COUNT(*) as event_count
FROM meanwhile.events
GROUP BY session_id;

-- View event types breakdown
SELECT event_type, COUNT(*) as count
FROM meanwhile.events
GROUP BY event_type
ORDER BY count DESC;

-- Query specific session
SELECT * FROM meanwhile.events
WHERE session_id = 'ticket-001'
ORDER BY created_at;
```

## Configuration Options

```go
memStore, err := memory.NewPostgresStore(
    connString,
    memory.WithSchema("my_schema"),      // Custom schema (default: "public")
    memory.WithTable("my_events"),       // Custom table (default: "meanwhile_events")
    memory.WithAutoMigrate(true),        // Auto-create tables (default: true)
)
```

## Benefits Over JSONL

| Feature | JSONL | PostgreSQL |
|---------|-------|------------|
| Persistence | ✅ | ✅ |
| Multi-process | ❌ | ✅ |
| Concurrent writes | ⚠️ Limited | ✅ |
| Complex queries | ❌ | ✅ SQL |
| Scalability | MB-GB | GB-TB+ |
| Transactions | ❌ | ✅ |
| Backups | Manual | Database tools |

## Production Tips

1. **Connection Pooling**: The PostgreSQL driver handles connection pooling automatically
2. **Indexes**: The store creates indexes on session_id, event_type, and created_at
3. **Partitioning**: For high-volume, consider table partitioning by date
4. **Retention**: Implement cleanup policies for old sessions
5. **Monitoring**: Use PostgreSQL monitoring tools (pg_stat_statements, etc.)

## Troubleshooting

### Connection Refused

```
PostgreSQL not reachable: dial tcp [::1]:5432: connect: connection refused
```

**Solution**: Start PostgreSQL service:
```bash
brew services start postgresql@14
```

### Authentication Failed

```
pq: password authentication failed
```

**Solution**: Check credentials or use trust authentication for local development:
```bash
# Edit pg_hba.conf to allow local connections
local   all   all   trust
```

### Table Already Exists

The store is idempotent - it safely handles existing tables and will not error if the schema already exists.

## See Also

- [Example 10: JSONL Memory Store](../10-memory-store) - File-based memory for single-process apps
- [Example 14: Semantic Memory](../14-semantic-memory) - AI-powered memory search
- [docs/memory.md](../../docs/memory.md) - Memory architecture documentation
