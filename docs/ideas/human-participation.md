# Human Participation in Protocols

## Vision

Enable end-users to participate directly in Meanwhile protocols alongside AI agents. Instead of agents autonomously completing tasks, humans can join the conversation flow, turning protocols into collaborative human-AI sessions.

## Motivation

Many real-world workflows require human-in-the-loop:
- **Brief refinement**: User provides draft, agents ask clarifying questions, user refines iteratively
- **Decision approval**: Agents propose options, user makes final call
- **Creative collaboration**: Human and agents brainstorm together
- **Supervised execution**: User steers agents mid-conversation
- **Training/validation**: Human provides examples or corrects agent outputs

Currently, Meanwhile protocols are agent-to-agent only. This limits adoption in scenarios where humans must stay involved.

## Use Cases

### 1. Brief Refinement
```
User: "Build a dashboard for customer analytics"
Moderator Agent: "What metrics are most important?"
Analyst Agent: "What's your data source?"
→ User responds with clarifications
→ Agents iterate on refined requirements
```

### 2. Approval Workflow
```
Planning Agent: "Here's the project breakdown..."
Budget Agent: "Cost estimate: $50K"
→ User approves or requests changes
→ Execution proceeds after approval
```

### 3. Collaborative Design
```
Designer Agent: "Three logo concepts..."
Brand Agent: "Concept B aligns with brand guidelines"
→ User picks favorite, suggests tweaks
→ Designer iterates
```

## Proposed API

### Core Design Principles

1. **Human as Participant**: `Human` is a first-class participant type alongside `Agent`
2. **Session-level scheduling**: Participation mode (`TurnBased`, `OnDemand`) lives on session, not human
3. **Deterministic flow**: `Session.Run()` returns status; no async channels or callbacks
4. **Protocol-agnostic**: Protocols see unified `Participant` interface
5. **Event stream optional**: Events for observability, but not required for basic flow

### Basic Usage

```go
// 1. Define human participant (identity only)
human := eng.Human("User").Build()

// 2. Create session with participation mode
sess, _ := eng.Session("Brief Refinement").
    Participant(moderator).
    Participant(analyst).
    Participant(human).
    Participation(engine.TurnBased()).  // Session-level policy
    Protocol(protocol.Consensus()).
    Start(ctx)

// 3. Run - returns StatusAwaitingInput when human turn arrives
result, _ := sess.Run(ctx, message.User("Here's my draft brief..."))

// 4. Check status and respond
if result.Status == engine.StatusAwaitingInput {
    // UI prompts user for input
    fmt.Printf("Context: %s\n", result.Context)
    fmt.Printf("Your turn: ")
    
    userInput := readUserInput()
    
    // Resume by responding
    result, _ = sess.Respond(ctx, result.RequestID, message.User(userInput))
}

// 5. Protocol completes
fmt.Println(result.Final)
```

### Return Value Pattern

```go
type RunResult struct {
    Status        RunStatus
    RequestID     string
    Context       string
    AwaitingInput *protocol.InputRequest
    Final         string
    Transcript    []agent.Message
    Events        []event.Event
    Metadata      map[string]any
}

// Run returns immediately with status
result, _ := sess.Run(ctx, msg)

// Respond resumes and returns next RunResult
result, _ = sess.Respond(ctx, result.RequestID, message.User("..."))
```

### Pause State Introspection

```go
if sess.IsPaused() {
    for _, req := range sess.PendingRequests() {
        fmt.Printf("Waiting for %s (timeout: %s)\n", req.ParticipantName, req.TimeoutAt)
    }
}
```

### Ask Human Tool

Agents can request human input directly with the `ask_human` tool.

```go
_, _ = sess.EnableAskHumanTool()

moderator := eng.Agent("Moderator").
    Prompt("If you're unsure, call ask_human with a focused question.").
    Tools(engine.AskHumanToolID).
    Build()
```

When the tool runs, the session pauses and emits `event.HumanRequestCreated`.
Resume with `sess.Respond()` when the human replies.

Set `required` to `false` in tool input to send a non-blocking request; the session continues immediately.

### Outbound Integrations (Phase 4)

Configure how humans receive requests by adding contact channels and registering integrations.

```go
human := eng.Human("Anna").
    ID("anna").
    ContactVia("slack", "@anna.chen").
    PreferredChannel("slack").
    Build()

slackClient, _ := integration.NewSlackClient(os.Getenv("SLACK_BOT_TOKEN"))
slackIntegration, _ := integration.NewSlack(slackClient)

eng, _ := engine.New(
    engine.WithProvider(provider),
    engine.WithIntegration(slackIntegration),
)
```

### Inbound Webhook (Phase 5)

Receive human responses via HTTP and resume sessions by request ID.

```go
handler := &server.HumanResponseHandler{Engine: eng}
http.Handle("/webhook/human-response", handler)
_ = http.ListenAndServe(":8080", nil)
```

### Inbox Listing (Short-Term Reliability)

The engine records human request lifecycle updates (pending → sent/failed → answered/timed_out)
in a `HumanRequestStore`. Use it to build a simple inbox UI or status dashboard.

```go
// List pending requests
requests, _ := eng.ListHumanRequests(ctx, engine.HumanRequestFilter{
    Statuses: []engine.HumanRequestStatus{engine.HumanRequestStatusPending},
    Limit:    50,
})

// Optional HTTP handler for a JSON inbox feed
handler := &server.HumanRequestInboxHandler{Engine: eng}
http.Handle("/inbox/human-requests", handler)
```

### Protocol Authoring

Protocols use unified `Participant` abstraction:

```go
func (p *myProtocol) OnMessage(ctx context.Context, sess Session, msg Message) error {
    for _, participant := range sess.Participants() {
        if participant.IsHuman() {
            _, err := sess.RunTurn(ctx, participant, RunRequest{Messages: []Message{msg}},
                WithTurnContext(msg.Summary()),
                WithTurnResume(func(ctx context.Context, resp Message) error {
                    _ = ctx
                    // Process resp...
                    return nil
                }),
            )
            return err
        }
        ag, ok := participant.Agent()
        if !ok {
            return fmt.Errorf("agent participant required")
        }
        // Agent run...
        _, err := sess.RunAgent(ctx, ag, RunRequest{Messages: []Message{msg}})
        if err != nil {
            return err
        }
    }
    return nil
}
```

### Event Stream (Optional)

```go
// Subscribe to events for UI updates
sess.Subscribe(func(ev event.Event) {
    switch ev.Type {
    case event.AwaitingUserInput:
        payload := ev.Payload.(protocol.InputRequest)
        showPrompt(payload.Context)
    case event.AgentMessageDelta:
        streamToUI(ev.Payload)
    }
})
```

## Participation Modes

### Phase 1: Turn-Based (MVP)

Human gets explicit turn in agent rotation.

```go
sess, _ := eng.Session("name").
    Participant(agent1).
    Participant(human).
    Participant(agent2).
    Participation(engine.TurnBased()).
    Start(ctx)
```

**Execution**: `agent1 → human (awaits) → agent2 → agent1 → ...`

**When to use**: Structured workflows, interviews, reviews

### Phase 2: On-Demand

Human signals readiness; protocol pauses when requested.

```go
sess, _ := eng.Session("name").
    Participant(agent1, agent2, human).
    Participation(engine.OnDemand()).
    Start(ctx)

// In separate goroutine or UI callback
sess.RequestTurn(ctx, "User")  // Pauses at next turn boundary
```

**When to use**: User wants to interject occasionally

### Phase 3: @-Mention

Human responds only when explicitly tagged.

```go
sess, _ := eng.Session("name").
    Participant(agent1, agent2, human).
    Participation(engine.OnMention()).
    Start(ctx)

// Protocol runs until agent says: "@User what do you think?"
// Then awaits input
```

**When to use**: User is advisor/consultant, not regular participant

## Architecture

### Participant Interface

```go
type Participant interface {
    Identifier() string
    DisplayName() string
    IsHuman() bool
    IsAgent() bool
    Agent() (agent.Agent, bool) // ok=false if human
}

type HumanParticipant struct {
    id   string
    name string
}
```

### Session Extensions

```go
type Session interface {
    // Existing
    Participants() []Participant  // Returns all (agents + humans)
    Run(ctx, msg) (*RunResult, error)
    
    // New
    HumanParticipants() []Participant
    Respond(ctx, requestID, msg) (*RunResult, error)
    RunTurn(ctx, participant, turn) (Message, error)
}
```

### Execution Flow

```
1. sess.Run(ctx, msg) 
   → Protocol.OnMessage()
   → sess.AwaitInput() for human turn
      → Emit AwaitingUserInput event
      → Return RunResult{Status: StatusAwaitingInput, RequestID: uuid, Context: history}
   
2. User sees AwaitingInput in UI, types response

3. sess.Respond(ctx, requestID, message.User("..."))
   → Inject message into protocol state
   → Resume execution at Turn 3
   → Protocol continues or returns next AwaitingInput
```

### Implementation Location

- **Core types**: `pkg/engine/participant.go`, `pkg/engine/interaction.go`
- **Session logic**: `pkg/engine/session.go` (Run, Respond, RunTurn)
- **Roundtable support**: `pkg/collab/roundtable/human.go`
- **Events**: `pkg/event/types.go` (add AwaitingUserInput)
- **Example**: `examples/19-human-in-loop/`

## Technical Considerations

### Why This Design

1. **No async complexity**: `Run()` returns status, caller decides what to do
2. **Testable**: `Respond()` is just another function call
3. **Deterministic**: Protocols don't need callbacks or channels
4. **UI-agnostic**: Works with CLI, web UI, or test harness
5. **Backward compatible**: Existing protocols work unchanged

### Edge Cases

- **Multiple humans**: Each gets own RequestID, tracked independently
- **Timeouts**: Optional scheduler can auto-handle with a default timeout policy
- **Concurrent sessions**: Each session has isolated state
- **Protocol compatibility**: Protocols not aware of humans see them as regular participants

### Scheduled Timeouts & Persistence

- **Timeout drivers**: Pluggable scheduler drivers (in-process or Redis) handle request deadlines
- **Request registry**: Pluggable registry (in-memory or Redis) resolves inbound responses to sessions
- **Timeout events**: `human.request.timed_out` emits when a scheduled timeout fires

### Performance

- No polling: Uses channels internally but exposes synchronous API
- Minimal overhead: Human turn just emits event and returns early
- Scales: Multiple sessions with humans run independently

## Success Metrics

1. User can join protocol with agents using 3-line code change
2. Turn-based mode works in 80%+ of target use cases
3. API feels natural to Go developers familiar with Meanwhile
4. Example demonstrates real-world brief refinement
5. Community requests on-demand/mention modes (validates phase 2)

## Open Questions

1. **Human profile/context**: Should humans have prompts like agents? (e.g., "You are a product manager...")
2. **Multi-turn human dialogue**: Can human have back-and-forth with one agent before turn passes?
3. **Human groups**: Should humans participate in breakout sessions?
4. **Typing indicators**: Emit events when human is typing?
5. **History visibility**: Do humans see full agent reasoning/tool calls or just messages?

## Next Steps

1. Validate API with 2-3 potential users
2. Build minimal prototype (participant types + turn detection)
3. Ship Turn-Based mode only
4. Gather usage data before phase 2
5. Consider tool-calling for humans (human approves tool execution)

---

**Status**: Proposed  
**Target**: Q2 2026  
**Owner**: TBD
