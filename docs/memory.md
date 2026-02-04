# Memory Management

**Status:** Experimental (v0.1.0+)

The memory package provides event storage and retrieval for agent collaboration sessions, enabling conversation persistence, context building, and session continuity.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Storage Implementations](#storage-implementations)
- [Context Building](#context-building)
- [Statistics](#statistics)
- [Architecture](#architecture)
- [Custom Stores](#custom-stores)
- [Examples](#examples)
- [Roadmap](#roadmap)
- [FAQ](#faq)

---

## Overview

Memory in Meanwhile differs from traditional "AI memory" systems. Instead of semantic embeddings or vector search (coming in v0.2.0), it provides:

1. **Event Persistence**: All session events are stored durably
2. **Context Building**: Extract conversation history for agent prompts
3. **Session Continuity**: Resume conversations across process restarts
4. **Statistics**: Track event counts, types, and timing

### What Memory Stores

Memory stores **all events** from a session, not just messages:

- `agent.started` - When an agent begins processing
- `agent.message.completed` - Agent responses
- `tool.call.started` / `tool.call.completed` - Tool executions
- `protocol.state.changed` - Protocol state transitions
- And more...

This event-centric approach enables rich observability and future features like replay, debugging, and analytics.

---

## Quick Start

### 1. Choose a Storage Backend

```go
import "github.com/runmeanwhile/meanwhile/pkg/memory"

// Option A: In-memory (for testing, lost on restart)
memStore := memory.NewInMemoryStore()

// Option B: File-based persistence (recommended)
memStore, err := memory.NewFileChatStore("./sessions")
if err != nil {
    log.Fatal(err)
}
defer memStore.Close()
```

### 2. Attach to Engine

```go
eng, err := engine.New(
    engine.WithProvider(provider),
    engine.WithMemoryStore(memStore), // Enable automatic event storage
)
```

### 3. Run Sessions

Events are automatically captured:

```go
sess, _ := eng.Session("ticket-1234").
    Participant(supportAgent).
    Start(ctx)

// All events from this run are automatically stored
result, _ := eng.Run(ctx, sess.ID(), 
    message.User("What's the status?"))
```

### 4. Load Context

```go
// Later (even after restart), load conversation history
history, err := memory.BuildConversationContext(
    ctx, 
    memStore, 
    "ticket-1234",
    memory.WithRecent(10),       // Last 10 messages
    memory.WithTokenLimit(4000), // Stay under model limit
)

// history contains []agent.Message suitable for prompt context
```

---

## Storage Implementations

### InMemoryStore

**Use for:** Testing, ephemeral sessions, development

**Characteristics:**
- Fast (no I/O)
- Thread-safe (sync.RWMutex)
- No persistence (lost on restart)
- Unbounded growth (no cleanup)

```go
store := memory.NewInMemoryStore()
```

### FileChatStore

**Use for:** Production, long-running applications, session continuity

**Characteristics:**
- **Persistent**: Events survive restarts
- **Format**: One JSONL file per session
- **Thread-safe**: Safe for concurrent access within a process
- **Durable**: Fsync after each append
- **Resilient**: Skips corrupted lines during read

```go
store, err := memory.NewFileChatStore("./sessions")
if err != nil {
    log.Fatal(err)
}
defer store.Close() // Important: close to flush file handles

// Creates: ./sessions/{sessionID}.jsonl
```

**File Format (JSONL):**
```jsonl
{"id":"evt_123","type":"agent.started","time":"2026-01-19T10:00:00Z","session_id":"ticket-1234","payload":{...}}
{"id":"evt_124","type":"agent.message.completed","time":"2026-01-19T10:00:01Z","session_id":"ticket-1234","payload":{"message":{...}}}
```

**Security Considerations:**
- Session IDs are validated (no path traversal)
- Directory permissions: `0750` (owner + group read)
- File permissions: `0640` (owner write, group read)

---

## Context Building

`BuildConversationContext` extracts agent messages from events for use in prompts.

### Basic Usage

```go
history, err := memory.BuildConversationContext(ctx, store, sessionID)
// Returns: []agent.Message in chronological order
```

### With Options

```go
history, err := memory.BuildConversationContext(
    ctx, 
    store, 
    sessionID,
    
    // Limit to last 20 messages
    memory.WithRecent(20),
    
    // Truncate to fit within token budget
    memory.WithTokenLimit(4000),
    
    // Filter specific event types
    memory.WithMessageTypes(event.AgentMessageComplete),
    
    // Hard cap on message count
    memory.WithMaxMessages(50),
)
```

### Options Explained

#### WithRecent(count)
Keeps only the N most recent messages. Useful for long conversations where older context is less relevant.

```go
memory.WithRecent(10) // Last 10 messages only
```

#### WithTokenLimit(tokens)
Truncates from the beginning to fit within a token budget. Uses approximate counting (4 chars ≈ 1 token).

```go
memory.WithTokenLimit(4000) // Stay under 4K tokens
```

#### WithMessageTypes(types...)
Filters events before extraction. By default, all event types are processed.

```go
memory.WithMessageTypes(
    event.AgentMessageComplete,
    event.ToolCallCompleted,
)
```

#### WithMaxMessages(max)
Absolute maximum on returned messages. Applied after other filters.

```go
memory.WithMaxMessages(100) // Never return more than 100
```

### Message Extraction

Only certain events contain extractable messages:

| Event Type | Extracted As | Notes |
|-----------|-------------|-------|
| `agent.message.completed` | User/Assistant message | Contains message payload |
| `tool.call.completed` | Tool message | Contains result + call_id |
| Others | (skipped) | No message content |

### Integration with Agents

Currently, you must manually prepend context to agent prompts. Automatic integration coming in v0.2.0.

```go
// Manual approach (current)
history, _ := memory.BuildConversationContext(ctx, store, sessionID)

// Build prompt with history
promptMessages := append(history, message.User("New query"))

// Run agent with full context
// (Note: Framework doesn't yet auto-inject history)
```

---

## Statistics

Get structured statistics about session events using `Stats()`.

### Usage

```go
stats, err := store.Stats(ctx, sessionID, memory.Policy{})

fmt.Printf("Total events: %d\n", stats.TotalEvents)
fmt.Printf("First event: %s\n", stats.FirstEvent)
fmt.Printf("Last event: %s\n", stats.LastEvent)
fmt.Printf("Session duration: %s\n", stats.LastEvent.Sub(stats.FirstEvent))

for eventType, count := range stats.EventCounts {
    fmt.Printf("  %s: %d\n", eventType, count)
}
```

### EventStats Fields

```go
type EventStats struct {
    TotalEvents int                    // Total event count
    EventCounts map[event.Type]int     // Count per event type
    FirstEvent  time.Time              // Timestamp of first event
    LastEvent   time.Time              // Timestamp of last event
    SessionID   string                 // Session identifier
}
```

### Policy Filters

```go
stats, err := store.Stats(ctx, sessionID, memory.Policy{
    MaxItems: 1000,                           // Limit to first 1000 events
    Types:    []event.Type{                   // Count only these types
        event.AgentMessageComplete,
        event.ToolCallCompleted,
    },
})
```

### vs. Summarize (Deprecated)

`Summarize()` is deprecated in favor of `Stats()`:

```go
// ❌ Old (broken, returns concatenated type names)
summary, _ := store.Summarize(ctx, sessionID, policy)
fmt.Println(summary.Text) // "agent.started, tool.call.completed, ..."

// ✅ New (structured statistics)
stats, _ := store.Stats(ctx, sessionID, policy)
fmt.Printf("Agent messages: %d\n", stats.EventCounts[event.AgentMessageComplete])
```

---

## Architecture

### Event Flow

```
Agent/Protocol
    │
    ├─> EmitWithContext(ctx, event) 
    │       │
    │       v
    │   Session.EmitWithContext()
    │       │
    │       v
    │   MemoryStore.Append(event)
    │       │
    │       v
    │   [Storage Backend]
    │       │
    │       v
    │   (Disk/Memory/Database)
```

### Session Scoping

Each session has an isolated event stream:

```go
// Session A events
store.Append(ctx, "session-a", eventA1)
store.Append(ctx, "session-a", eventA2)

// Session B events (separate)
store.Append(ctx, "session-b", eventB1)

// Query only session A
items, _ := store.Query(ctx, memory.Query{SessionID: "session-a"})
// Returns: eventA1, eventA2 (eventB1 not included)
```

### Interface

All stores implement:

```go
type Store interface {
    // Append stores an event
    Append(ctx context.Context, sessionID string, ev event.Event) error
    
    // Query retrieves events matching criteria
    Query(ctx context.Context, query Query) ([]Item, error)
    
    // Summarize builds a summary (deprecated, use Stats)
    Summarize(ctx context.Context, sessionID string, policy Policy) (Summary, error)
    
    // Stats calculates event statistics
    Stats(ctx context.Context, sessionID string, policy Policy) (EventStats, error)
}
```

---

## Custom Stores

Implement the `Store` interface to integrate custom backends.

### Example: PostgreSQL Store

```go
type PostgresStore struct {
    db *sql.DB
}

func (p *PostgresStore) Append(ctx context.Context, sessionID string, ev event.Event) error {
    data, _ := json.Marshal(ev)
    _, err := p.db.ExecContext(ctx,
        "INSERT INTO events (session_id, event_data) VALUES ($1, $2)",
        sessionID, data,
    )
    return err
}

func (p *PostgresStore) Query(ctx context.Context, query Query) ([]Item, error) {
    rows, err := p.db.QueryContext(ctx,
        "SELECT event_data FROM events WHERE session_id = $1 ORDER BY created_at",
        query.SessionID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var items []Item
    for rows.Next() {
        var data []byte
        rows.Scan(&data)
        
        var ev event.Event
        json.Unmarshal(data, &ev)
        items = append(items, Item{Event: ev})
    }
    return items, nil
}

// Implement Stats() and Summarize()...
```

### Testing Custom Stores

```go
func TestCustomStore(t *testing.T) {
    store := NewPostgresStore(testDB)
    ctx := context.Background()
    
    // Test Append
    ev := event.New(event.AgentStarted, "test-session", nil)
    if err := store.Append(ctx, "test-session", ev); err != nil {
        t.Fatal(err)
    }
    
    // Test Query
    items, err := store.Query(ctx, memory.Query{SessionID: "test-session"})
    if err != nil {
        t.Fatal(err)
    }
    if len(items) != 1 {
        t.Errorf("expected 1 item, got %d", len(items))
    }
}
```

### Thread Safety

Custom stores must be safe for concurrent access:

```go
type SafeStore struct {
    mu sync.RWMutex
    // ...
}

func (s *SafeStore) Append(ctx context.Context, sessionID string, ev event.Event) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    // ... write logic
}

func (s *SafeStore) Query(ctx context.Context, query Query) ([]Item, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // ... read logic
}
```

---

## Examples

### Resuming a Conversation

```go
// First session
sess, _ := eng.Session("support-42").Start(ctx)
eng.Run(ctx, sess.ID(), message.User("My server is down"))

// ... Process restarts ...

// New process, same session ID
store, _ := memory.NewFileChatStore("./sessions")
history, _ := memory.BuildConversationContext(ctx, store, "support-42")

// Continue with full context
sess, _ := eng.Session("support-42").Start(ctx)
eng.Run(ctx, sess.ID(), message.User("I tried rebooting, still down"))
```

### Multi-Session Analytics

```go
// Track all sessions
sessionIDs := []string{"ticket-1", "ticket-2", "ticket-3"}

for _, id := range sessionIDs {
    stats, _ := store.Stats(ctx, id, memory.Policy{})
    fmt.Printf("Session %s: %d events, duration: %s\n",
        id,
        stats.TotalEvents,
        stats.LastEvent.Sub(stats.FirstEvent),
    )
}
```

### Debugging Session Playback

```go
// Retrieve all events for debugging
items, _ := store.Query(ctx, memory.Query{SessionID: "failed-session"})

for _, item := range items {
    fmt.Printf("[%s] %s\n", item.Event.Time, item.Event.Type)
    if item.Event.Type == event.AgentMessageComplete {
        payload := item.Event.Payload.(map[string]any)
        message := payload["message"].(map[string]any)
        fmt.Printf("  Content: %s\n", message["content"])
    }
}
```

### Filtering Tool Events

```go
// Get only tool-related events
items, _ := store.Query(ctx, memory.Query{
    SessionID: "session-123",
    Types: []event.Type{
        event.ToolCallStarted,
        event.ToolCallCompleted,
        event.ToolCallError,
    },
})

fmt.Printf("Found %d tool events\n", len(items))
```

---

## Automatic Session Memory

Meanwhile can automatically capture a memory string when a session closes.
If you configure a memory store with `engine.WithMemoryStore`, memory automation
is enabled by default. The memory is stored as a `memory.summary` event with a
**string payload**.

### Enable Automation

```go
eng, _ := engine.New(
    engine.WithProvider(provider),
    engine.WithMemoryStore(store),
    engine.WithMemoryAutomation(config.MemoryAutomationConfig{
        Enabled:    true,
        ProviderID: "openai",
        Model:      "gpt-5-mini",
        Prompt:     "", // optional; default prompt is used when empty
        Context: config.MemoryAutomationContext{
            RecentMessages: 20,
            TokenLimit:     4000,
        },
    }),
)
```

### Session-Level Overrides

- Override prompt: set session metadata `engine.MemoryAutomationPromptKey`.
- Disable automation: set `engine.MemoryAutomationEnabledKey` to `false`, or
  pass `engine.WithMemoryAutomation(config.MemoryAutomationConfig{Enabled: false})`.
- Tag a subject agent: set `engine.MemoryAutomationSubjectKey` to an agent ID.

```go
sess, _ := eng.Session("support-42").
    Metadata(engine.MemoryAutomationPromptKey, "Remember key facts only.").
    Metadata(engine.MemoryAutomationSubjectKey, "support-agent").
    Protocol(protocol.Solo()).
    Start(ctx)
```

### Reading the Memory

```go
items, _ := store.Query(ctx, memory.Query{
    SessionID: sess.ID(),
    Types:     []event.Type{event.MemorySummary},
})

if len(items) > 0 {
    if mem, ok := items[len(items)-1].Event.Payload.(string); ok {
        fmt.Println(mem)
    }
}
```

---

## Roadmap

### v0.2.0 - Semantic Memory (shipped)

- **Embedding-based storage** with vector database integration
- **Semantic search**: Find relevant memories by meaning, not just recency
- **Smart context building**: Combine recent + relevant memories
- **Automatic session summaries**: Store `memory.summary` events on session close
- **Embedding providers**: OpenAI, Ollama, local models

### v0.3.0 - Advanced Features (2-3 months)

- **Cross-session memory**: Remember patterns across multiple sessions
- **Entity memory**: Track people, projects, concepts over time
- **Memory compression**: LLM-based summarization of old sessions
- **Protocol memory integration**: Protocols remember past interactions
- **Memory events**: Observability for memory operations

### v0.4.0+ - Enterprise (3-6 months)

- Multi-tenant isolation
- Access controls and audit logging
- Data retention policies
- GDPR compliance helpers
- Memory analytics dashboard

### What Won't Be in Core

These are intentionally left to users/libraries:

- **Storage backend choice** (Postgres, Redis, S3, etc.) - Use custom Store
- **Embedding model selection** - Bring your own
- **Retention policies** - Implement in your Store
- **Privacy/encryption** - Handle at infrastructure level

---

## FAQ

### Q: Why aren't agents automatically using memory?

**A:** Agents still require manual context injection via `BuildConversationContext`. Automatic memory summaries exist, but automatic prompt wiring is still planned for a future release.

### Q: How do I delete old sessions?

**A:** For FileChatStore, delete the corresponding `.jsonl` file:

```go
sessionFile := filepath.Join(sessionsDir, sessionID + ".jsonl")
os.Remove(sessionFile)
```

For custom stores, implement cleanup logic in your Store.

### Q: What's the performance impact?

**A:** Minimal for typical sessions (<1000 events):
- **InMemoryStore**: <1ms append/query
- **FileChatStore**: ~1-5ms append (includes fsync), ~10-50ms query for 1K events

For large sessions (>10K events), consider:
- Pagination in Query
- Periodic summarization/compression
- Moving old sessions to cold storage

### Q: Can I use memory across multiple processes?

**A:** FileChatStore supports concurrent **reads** from multiple processes, but **writes** should be from a single process. For multi-process writes, use a database-backed custom Store (Postgres, Redis, etc.).

### Q: How accurate is token counting?

**A:** The built-in estimator uses 4 chars ≈ 1 token, which is approximate. The engine can use provider token estimators when available, but the memory package itself stays heuristic. For precise counting in memory workflows, integrate a proper tokenizer (e.g., tiktoken for OpenAI models) in your own context-building logic.

### Q: Does memory work with all protocols?

**A:** Yes. Memory captures events regardless of protocol (Solo, Consensus, Brainstorming, etc.). Protocol-specific memory features coming in v0.3.0.

### Q: How do I migrate from Summarize() to Stats()?

**A:** Replace calls:

```go
// Old
summary, _ := store.Summarize(ctx, sessionID, policy)
fmt.Println(summary.Text)

// New
stats, _ := store.Stats(ctx, sessionID, policy)
fmt.Printf("Total: %d, By type: %v\n", stats.TotalEvents, stats.EventCounts)
```

### Q: What about GDPR/right to be forgotten?

**A:** Memory doesn't enforce retention policies. To comply:
1. Map user IDs to session IDs in your application
2. Delete session files when requested
3. Consider encrypting at-rest storage
4. Implement audit logging in your Store

---

## See Also

- [Example 10 - Memory Store](../../examples/10-memory-store/)
- [Example 14 - Semantic Memory](../../examples/14-semantic-memory/)
- [Example 15 - Postgres Memory](../../examples/15-postgres-memory/)
- [Example 16 - Memory Automation](../../examples/16-memory-automation/)
- [Architecture Documentation](./architecture.md)
- [Event Types Reference](./observability.md)
- [API Documentation](https://pkg.go.dev/github.com/runmeanwhile/meanwhile/pkg/memory)

---

**Status:** Experimental - API may change in future releases.
