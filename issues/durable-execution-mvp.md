# Durable Execution MVP: Survive the Blips

## Problem

Today, Meanwhile sessions are **fragile**:

```go
sess.Run(ctx, message.User("Start analysis"))
// Network blip → session dead
// Process crash → start over
// Provider timeout → unrecoverable
```

**Reality check:** We can't promise 5-day sessions. But we should handle 1-hour sessions without catastrophic failures from transient issues.

## Goal

**Make network blips non-catastrophic. Enable 1-hour sessions with graceful recovery.**

Not aiming for:
- ❌ Multi-day sessions
- ❌ Temporal/Inngest integration  
- ❌ 100% uptime guarantees
- ❌ Distributed session management

Aiming for:
- ✅ Survive transient network failures
- ✅ Recover from provider timeouts
- ✅ Resume after process crash (manual restart)
- ✅ Sessions run for 1 hour without memory issues

## Technical Changes

### 1. Provider Stream Retry (Critical)

**Problem:** Any network hiccup permanently kills the stream.

```go
// Current: pkg/engine/agent_run.go
stream, err := p.Stream(ctx, req)
if err != nil {
    return agent.Message{}, nil, err  // ← GAME OVER
}

for {
    provEvent, err := stream.Recv()
    if err != nil && !errors.Is(err, io.EOF) {
        return agent.Message{}, nil, err  // ← NO RETRY
    }
}
```

**Solution:** Wrap provider streams with retry logic.

```go
// New: pkg/provider/resilient.go
type ResilientStream struct {
    createStream func(context.Context) (Stream, error)
    backoff      backoff.BackOff  // Use github.com/cenkalti/backoff (already in go.mod)
    maxRetries   int
}

func (s *ResilientStream) Recv() (Event, error) {
    var lastErr error
    operation := func() error {
        if s.stream == nil {
            stream, err := s.createStream(s.ctx)
            if err != nil {
                return err
            }
            s.stream = stream
        }
        
        ev, err := s.stream.Recv()
        if err == nil {
            s.lastEvent = ev
            return nil
        }
        
        if errors.Is(err, io.EOF) {
            return backoff.Permanent(err)  // Don't retry EOF
        }
        
        if isTransient(err) {
            s.stream = nil  // Force reconnect
            lastErr = err
            return err  // Retry
        }
        
        return backoff.Permanent(err)  // Fatal error
    }
    
    err := backoff.Retry(operation, s.backoff)
    if err != nil {
        return Event{}, lastErr
    }
    return s.lastEvent, nil
}

func isTransient(err error) bool {
    // Network timeouts, connection resets, etc.
    return strings.Contains(err.Error(), "timeout") ||
           strings.Contains(err.Error(), "connection reset") ||
           strings.Contains(err.Error(), "EOF")
}
```

**Integration point:**

```go
// Modify: pkg/engine/agent_run.go
func (s *Session) runProviderStream(...) {
    baseStream, err := p.Stream(ctx, req)
    if err != nil {
        return agent.Message{}, nil, err
    }
    
    // Wrap with retry logic
    stream := provider.NewResilientStream(baseStream, provider.ResilientConfig{
        MaxRetries:      5,
        InitialInterval: 1 * time.Second,
        MaxInterval:     10 * time.Second,
        Multiplier:      2.0,
    })
    defer stream.Close()
    
    // Rest stays the same
}
```

**Files to modify:**
- `pkg/provider/resilient.go` (new)
- `pkg/engine/agent_run.go` (wrap streams)

**LOC estimate:** ~150 lines

---

### 2. Protocol State Checkpointing (Critical)

**Problem:** Protocol state lives only in memory. Crash = lost progress.

```go
// Current: pkg/protocol/brainstorming.go
type brainstorming struct {
    currentRound int  // ← LOST ON CRASH
    ideas []string    // ← LOST ON CRASH
}
```

**Solution:** Auto-persist protocol state to session metadata after each step.

```go
// New interface in pkg/protocol/protocol.go
type StatefulProtocol interface {
    Protocol
    
    // GetState returns serializable protocol state
    GetState() (map[string]any, error)
    
    // SetState restores protocol state
    SetState(state map[string]any) error
}

// Modify existing protocols to implement StatefulProtocol
type brainstorming struct {
    currentRound int
    ideas []string
}

func (p *brainstorming) GetState() (map[string]any, error) {
    return map[string]any{
        "currentRound": p.currentRound,
        "ideas": p.ideas,
    }, nil
}

func (p *brainstorming) SetState(state map[string]any) error {
    if round, ok := state["currentRound"].(float64); ok {
        p.currentRound = int(round)
    }
    if ideas, ok := state["ideas"].([]any); ok {
        p.ideas = make([]string, len(ideas))
        for i, idea := range ideas {
            p.ideas[i] = idea.(string)
        }
    }
    return nil
}
```

**Auto-checkpoint after each protocol step:**

```go
// Modify: pkg/engine/engine.go Run()
func (e *Engine) Run(ctx context.Context, sessionID string, msg agent.Message) (*RunResult, error) {
    // ... existing code ...
    
    if err := sess.protocol.OnMessage(traceCtx, sess, msg); err != nil {
        return nil, fmt.Errorf("protocol message: %w", err)
    }
    
    // NEW: Auto-checkpoint if protocol is stateful
    if stateful, ok := sess.protocol.(protocol.StatefulProtocol); ok {
        state, err := stateful.GetState()
        if err == nil {
            sess.metadata["_protocol_state"] = state
            _ = e.persistSession(ctx, sess)  // Already exists!
        }
    }
    
    // ... rest of code ...
}
```

**Session rehydration loads state:**

```go
// Modify: pkg/engine/session_store.go sessionFromRecord()
func (e *Engine) sessionFromRecord(ctx context.Context, record SessionRecord) (*Session, error) {
    // ... existing code ...
    
    sess, err := e.NewSession(ctx, SessionConfig{...})
    if err != nil {
        return nil, err
    }
    
    // NEW: Restore protocol state
    if state, ok := record.Metadata["_protocol_state"].(map[string]any); ok {
        if stateful, ok := sess.protocol.(protocol.StatefulProtocol); ok {
            _ = stateful.SetState(state)
        }
    }
    
    return sess, nil
}
```

**Files to modify:**
- `pkg/protocol/protocol.go` (add interface)
- `pkg/protocol/brainstorming.go` (implement)
- `pkg/protocol/consensus/*.go` (implement)
- `pkg/protocol/adversarial.go` (implement)
- `pkg/engine/engine.go` (auto-checkpoint)
- `pkg/engine/session_store.go` (restore state)

**LOC estimate:** ~200 lines across all protocols

---

### 3. Context Overflow Prevention (Important)

**Problem:** Message history grows unbounded in memory.

**Solution:** Auto-summarize when history exceeds threshold.

```go
// New: pkg/contextpolicy/auto_summarize.go
type AutoSummarizePolicy struct {
    base              Policy
    summarizer        Summarizer
    maxHistoryTokens  int
    summarizeAtTokens int
}

func (p *AutoSummarizePolicy) Select(ctx context.Context, input ContextInput) ([]Message, error) {
    // Estimate tokens in history
    totalTokens := estimateTokens(input.History)
    
    if totalTokens > p.summarizeAtTokens {
        // Summarize old messages
        summary, err := p.summarizer.Summarize(ctx, input.History[:len(input.History)/2])
        if err == nil {
            input.History = append(
                []Message{message.System(summary)},
                input.History[len(input.History)/2:]...,
            )
        }
    }
    
    return p.base.Select(ctx, input)
}
```

**Enable by default:**

```go
// Modify: pkg/engine/engine.go New()
func New(opts ...Option) (*Engine, error) {
    e := &Engine{
        contextPolicy: contextpolicy.NewAutoSummarizePolicy(
            contextpolicy.NewDefaultPolicy(),
            4000,  // summarize at 4K tokens
        ),
    }
    // ...
}
```

**Files to modify:**
- `pkg/contextpolicy/auto_summarize.go` (new)
- `pkg/engine/engine.go` (use by default)

**LOC estimate:** ~100 lines

---

### 4. Per-Run Timeouts (Important)

**Problem:** Hung operations block sessions indefinitely.

**Solution:** Enforce max run duration per session.Run() call.

```go
// Add to: pkg/protocol/protocol.go RunRequest
type RunRequest struct {
    Messages         []Message
    MaxToolIterations int
    MaxRunDuration   time.Duration  // NEW
}

// Modify: pkg/engine/engine.go Run()
func (e *Engine) Run(ctx context.Context, sessionID string, msg agent.Message) (*RunResult, error) {
    sess, err := e.session(ctx, sessionID)
    if err != nil {
        return nil, err
    }
    
    // NEW: Apply default timeout if not set by caller
    if sess.protocol != nil {
        if req, ok := sess.protocol.(protocol.TimeoutProvider); ok {
            timeout := req.DefaultTimeout()
            if timeout > 0 {
                var cancel context.CancelFunc
                ctx, cancel = context.WithTimeout(ctx, timeout)
                defer cancel()
            }
        }
    }
    
    // Existing code with new ctx that has timeout
    // ...
}
```

**Files to modify:**
- `pkg/protocol/protocol.go` (add MaxRunDuration)
- `pkg/engine/engine.go` (enforce timeout)

**LOC estimate:** ~30 lines

---

## Acceptance Criteria

### AC1: Network Resilience
```bash
# Test: Simulate network blip during provider stream
go test -run TestProviderStreamResilience
```

**Must pass:**
- [ ] Temporary connection failure → auto-reconnect → session continues
- [ ] 3 failed retries → exponential backoff → eventual success
- [ ] Permanent failure after max retries → clear error

### AC2: Process Crash Recovery
```bash
# Test: Kill process mid-protocol, restart, resume
go test -run TestProtocolCheckpointing
```

**Must pass:**
- [ ] Brainstorming protocol survives crash at round 3/5
- [ ] Consensus protocol restores pulse check state
- [ ] Adversarial protocol resumes debate position
- [ ] Session metadata includes `_protocol_state` after each step

### AC3: Memory Bounded
```bash
# Test: 1000-message session doesn't OOM
go test -run TestContextOverflowPrevention
```

**Must pass:**
- [ ] History exceeds 4K tokens → auto-summarize older messages
- [ ] Memory usage plateaus (doesn't grow linearly)
- [ ] Summarized history maintains conversation coherence

### AC4: Timeout Protection
```bash
# Test: Hung provider call doesn't block forever
go test -run TestRunTimeout
```

**Must pass:**
- [ ] Run exceeding MaxRunDuration → context cancelled
- [ ] Error includes timeout reason
- [ ] Session remains usable after timeout

### AC5: End-to-End 1-Hour Session
```bash
# Integration test: Real session with failures
go test -run TestDurableHourLongSession -timeout=70m
```

**Must pass:**
- [ ] Session runs for 60+ minutes
- [ ] Survives 3 simulated network blips
- [ ] Survives 1 process crash (manual restart)
- [ ] Final result matches expected output
- [ ] Memory usage < 500MB throughout

## Non-Goals (For Later)

- Multi-day sessions (deferred to external orchestrator integration)
- Distributed session management
- Advanced circuit breakers
- Dead letter queues
- Automatic failure alerts
- Session migration between processes

## Implementation Order

1. **Week 1:** Provider stream retry (#1)
   - Most critical, highest ROI
   - Unblocks network resilience

2. **Week 2:** Protocol checkpointing (#2)
   - Enables crash recovery
   - Foundation for durability

3. **Week 3:** Context overflow + timeouts (#3, #4)
   - Polish for production use
   - Memory safety

4. **Week 4:** Testing & documentation
   - E2E tests for all ACs
   - Update examples/README.md

## Success Metrics

**Before MVP:**
- Network blip → session fails 100%
- Process crash → session lost 100%
- 1-hour session → OOM risk

**After MVP:**
- Network blip → session recovers 95%+
- Process crash → resumable 90%+
- 1-hour session → stable memory footprint

## Open Questions

1. Should we expose retry config in SessionConfig or keep it internal?
2. Do we checkpoint after every protocol step or only at phase boundaries?
3. Should summarization be opt-in or opt-out?
4. What's the right default MaxRunDuration? (Propose: 10 minutes)

## Related

- [Full Analysis](../DURABLE_EXECUTION_ANALYSIS.md)
- [Context Management Plan](../docs/context_management_plan.md)
- [Memory Feature Analysis](../MEMORY_FEATURE_ANALYSIS.md)
