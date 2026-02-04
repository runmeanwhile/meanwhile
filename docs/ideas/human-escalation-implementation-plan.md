# Human Escalation Implementation Plan

## Overview

This plan outlines the phased implementation of human-in-the-loop capabilities in Meanwhile, progressing from basic human participation to full human escalation with async response handling and connected personas.

**End Goal**: Agents can recognize uncertainty, call `ask_human`, send questions via Slack/Email/Webhook, pause the session, and resume when humans respond asynchronously.

**Timeline**: 8 phases over ~6-8 months  
**Status**: Proposed  
**Last Updated**: 2026-02-02

---

## Phase 1: Human Participants (Foundation)

**Goal**: Enable humans to participate directly in protocols alongside agents using turn-based flow.

**Duration**: 3-4 weeks

### Capabilities

- Human as first-class participant type
- Session-level participation mode (Turn-Based only)
- `Session.Run()` returns `AwaitingInput` status when human turn arrives
- `Session.Respond()` to inject human response and resume
- Event stream for `AwaitingUserInput`

### API Surface

```go
// Define human participant
human := eng.Human("User").Build()

// Create session with turn-based participation
sess, _ := eng.Session("Brief Refinement").
    Participant(moderator).
    Participant(human).
    Participant(analyst).
    Participation(engine.TurnBased()).
    Protocol(protocol.Consensus()).
    Start(ctx)

// Run - returns AwaitingInput when human turn arrives
result, _ := sess.Run(ctx, message.User("Draft brief..."))

if result.Status == engine.AwaitingInput {
    userInput := readUserInput()
    result, _ = sess.Respond(ctx, result.RequestID, message.User(userInput))
}
```

### Implementation

**Files to create/modify**:
- `pkg/engine/participant.go` - `Participant` interface, `HumanParticipant`, `AgentParticipant`
- `pkg/engine/interaction.go` - `ParticipationMode`, `TurnBased()`, `RunResult`, `Status`
- `pkg/engine/session.go` - `Respond()`, `RunTurn()`, `HumanParticipants()`
- `pkg/collab/roundtable/human.go` - `Human()` turn constructor, detection logic
- `pkg/event/types.go` - `AwaitingUserInput` event
- `examples/20-human-turn-based/` - Example demonstrating basic human participation

**Key Design Decisions**:
- No async channels/callbacks - synchronous API only
- Participation mode lives on session, not human
- Protocols use unified `Participant` abstraction
- RequestID for correlation between Run/Respond calls

### Success Criteria

- [ ] User can join protocol with 3-line code change
- [ ] Turn-based mode works with Consensus, Brainstorming, Handoff protocols
- [ ] Example demonstrates brief refinement use case
- [ ] Unit tests for Respond() correlation
- [ ] Events emitted correctly for UI observability

---

## Phase 2: Pause/Resume State Management

**Goal**: Robust session state persistence and resume logic for async human responses.

**Duration**: 2-3 weeks

### Capabilities

- Session state serialization/deserialization
- RequestID-based state tracking
- Multiple concurrent pause points (for multiple humans)
- Context preservation across pause/resume
- Timeout detection (no scheduling yet, just detection)

### API Additions

```go
// Check if session is paused
if sess.IsPaused() {
    pending := sess.PendingRequests()
    for _, req := range pending {
        fmt.Printf("Waiting for %s (timeout: %s)\n", 
            req.ParticipantID, req.TimeoutAt)
    }
}

// Resume with timeout information
result, _ := sess.Respond(ctx, requestID, msg, 
    engine.WithTimeoutHandling(engine.ContinueWithNote))
```

### Implementation

**Files to create/modify**:
- `pkg/engine/session_state.go` - State serialization, pause tracking
- `pkg/engine/session_store.go` - Extend interface for state persistence
- `pkg/engine/timeout.go` - Timeout policies (continue, retry, fail)
- `pkg/event/types.go` - `SessionPaused`, `SessionResumed` events
- Unit tests for concurrent pause points

**Key Design Decisions**:
- State stored as JSON-serializable struct
- Default store remains in-memory; persistence is opt-in
- Multiple RequestIDs tracked independently
- Timeout policies configurable per request

### Success Criteria

- [ ] Session can pause with 3 concurrent human requests
- [ ] State survives serialization round-trip
- [ ] Timeout detection works correctly
- [ ] Resume from arbitrary pause point
- [ ] Events track full pause/resume lifecycle

---

## Phase 3: Ask Human Tool

**Goal**: Agents can programmatically request human input via tool call.

**Duration**: 2 weeks

### Capabilities

- `ask_human` tool with structured schema
- Tool handler emits `HumanRequest` and pauses session
- Integration with Phase 1 pause/resume flow
- Request context and urgency levels
- Optional vs required human input

### Tool Schema

```go
type AskHumanInput struct {
    Question     string   `json:"question" description:"Specific question for human"`
    Context      string   `json:"context" description:"Why this question matters"`
    Participant  string   `json:"participant" description:"Which human to ask"`
    Timeout      Duration `json:"timeout,omitempty" description:"How long to wait"`
    Required     bool     `json:"required" description:"Block or continue if no response"`
    SuggestedResponses []string `json:"suggested_responses,omitempty"`
}

type AskHumanOutput struct {
    RequestID    string   `json:"request_id"`
    Status       string   `json:"status"` // "pending", "timeout", "answered"
    Response     string   `json:"response,omitempty"`
}
```

### Example Usage

```go
// Agent's perspective
askHuman := tool.New("ask_human", handleAskHuman).
    WithInput(AskHumanInput{}).
    WithOutput(AskHumanOutput{}).
    WithDescription("Request input from human participant")

agent := eng.Agent("Moderator").
    Tool(askHuman).
    Prompt("When uncertain, use ask_human to get clarification").
    Build()

// Tool handler
func handleAskHuman(ctx context.Context, input AskHumanInput) (AskHumanOutput, error) {
    reqID := uuid.New().String()
    
    // Emit request event (integrations listen to this)
    sess.EmitEvent(event.HumanRequestCreated{
        RequestID: reqID,
        Question: input.Question,
        Participant: input.Participant,
        Timeout: input.Timeout,
    })
    
    // Pause session
    return AskHumanOutput{
        RequestID: reqID,
        Status: "pending",
    }, ErrAwaitingHuman{RequestID: reqID}
}
```

### Implementation

**Files to create/modify**:
- `pkg/tool/ask_human.go` - Tool definition and handler
- `pkg/engine/human_request.go` - `HumanRequest` model
- `pkg/event/types.go` - `HumanRequestCreated`, `HumanResponseReceived` events
- `examples/21-ask-human-tool/` - Agent using ask_human in decision-making

**Key Design Decisions**:
- Tool returns immediately with "pending" status
- `ErrAwaitingHuman` signals session to pause
- Events are the integration point (no direct Slack code in tool)
- Timeout and required flags control fallback behavior

### Success Criteria

- [ ] Agent can call ask_human during protocol execution
- [ ] Session pauses correctly on tool invocation
- [ ] Request events include all context needed for integrations
- [ ] Required vs optional handling works
- [ ] Example shows realistic uncertainty detection

---

## Phase 4: Outbound Integrations

**Goal**: Send human requests to Slack, Email, or Webhook endpoints.

**Duration**: 3-4 weeks

### Capabilities

- Event listener pattern for `HumanRequestCreated`
- Slack adapter (DM sending, formatting)
- Email adapter (SMTP, HTML templates)
- Webhook adapter (generic POST)
- Per-human routing configuration
- Message templates with context injection

### Architecture

```go
// Integration registry
type Integration interface {
    Send(ctx context.Context, req HumanRequest) error
    SupportsHuman(humanID string) bool
}

// Slack integration
slackIntegration := integration.NewSlack(slackClient).
    WithTemplate(slackMessageTemplate).
    WithFallback(emailIntegration)

eng.RegisterIntegration(slackIntegration)

// Human configuration
human := eng.Human("Anna").
    ContactVia("slack", "@anna.chen").
    ContactVia("email", "anna@company.com").
    PreferredChannel("slack").
    Build()
```

### Slack Message Format

```
Meanwhile (@meanwhile)

Hey Anna! Quick question from a planning session:

────────────────────────────────────────────────────
QUESTION
"For Q2, would you rather ship a scoped-down dashboard 
on time, or take more time for the full version?"

CONTEXT
We're debating priorities for the roadmap review. Your 
persona is uncertain about your timeline risk tolerance.

SESSION
"Q2 Roadmap Review" started by Darko
Participants: Anna (you), Marcus, Chen
────────────────────────────────────────────────────

Reply here to respond, or ignore (session will continue 
with best guess after 6 hours).

Request ID: req_abc123
Respond: /meanwhile respond req_abc123 [your answer]
```

### Implementation

**Files to create/modify**:
- `pkg/integration/` - New package for outbound integrations
- `pkg/integration/slack.go` - Slack client wrapper, formatting
- `pkg/integration/email.go` - SMTP sender, HTML templates
- `pkg/integration/webhook.go` - Generic HTTP POST
- `pkg/integration/router.go` - Route requests based on human config
- `pkg/engine/human.go` - Add contact configuration to Human builder
- `examples/22-slack-integration/` - Complete Slack roundtrip

**Dependencies**:
- `github.com/slack-go/slack` for Slack API
- Standard library `net/smtp` for email
- Environment variables for credentials

**Key Design Decisions**:
- Integrations are event listeners, not part of core engine
- Humans specify preferred channels in builder
- Fallback chain if primary channel fails
- Templates support Markdown and HTML formatting

### Success Criteria

- [ ] Slack DM sent when ask_human called
- [ ] Email sent as fallback
- [ ] Webhook POST with JSON payload works
- [ ] Message formatting is clear and actionable
- [ ] Rate limiting prevents spam
- [ ] Example demonstrates end-to-end question sending

---

## Phase 5: Inbound Response Handling

**Goal**: Receive human responses and inject them back into paused sessions.

**Duration**: 3-4 weeks

### Capabilities

- Webhook HTTP server for inbound responses
- Correlation ID matching (RequestID → Session)
- Response validation and authentication
- Session resume with human message injection
- Slack slash command handler (`/meanwhile respond`)
- Email reply parsing

### Webhook API

```
POST /webhook/human-response
Content-Type: application/json

{
  "request_id": "req_abc123",
  "response": "Scoped-down, definitely. Ship something solid.",
  "responder": "anna@company.com",
  "source": "slack",
  "timestamp": "2026-02-02T14:30:00Z",
  "signature": "hmac_sha256..."
}
```

### Integration Flow

```
1. Slack user clicks "Reply" or uses slash command
   /meanwhile respond req_abc123 Scoped-down version

2. Slack app backend posts to Meanwhile webhook
   POST /webhook/human-response

3. Webhook validates signature and correlation ID
   
4. Webhook looks up session by RequestID
   
5. Webhook calls sess.Respond(ctx, requestID, message.Human(response))
   
6. Session resumes, response attributed as "Anna (real)"
   
7. Protocol continues with human input injected
```

### Implementation

**Files to create/modify**:
- `pkg/server/` - New package for HTTP webhook server
- `pkg/server/webhook.go` - Handler for `/webhook/human-response`
- `pkg/server/auth.go` - HMAC signature validation
- `pkg/integration/slack_commands.go` - Slash command handlers
- `pkg/engine/session_registry.go` - RequestID → Session lookup
- `cmd/meanwhile-server/` - Optional standalone server binary
- `examples/23-webhook-receiver/` - Complete webhook integration

**Key Design Decisions**:
- Webhook server is optional (for serverless, use SDK directly)
- HMAC signature prevents spoofing
- RequestID lookup requires session registry (in-memory or Redis)
- Responses include source attribution ("via Slack")

### Success Criteria

- [ ] Webhook receives Slack responses correctly
- [ ] Session resumes with human message injected
- [ ] Signature validation prevents unauthorized responses
- [ ] Multiple concurrent sessions handled correctly
- [ ] Email reply parsing works for common clients
- [ ] Example demonstrates full roundtrip (send → receive → resume)

---

## Phase 6: Timeout & Async Scheduling

**Goal**: Handle timeouts gracefully when humans don't respond in time.

**Duration**: 2-3 weeks

### Capabilities

- Timeout scheduler (cron-style or delay queue)
- Configurable timeout policies (continue, retry, fail)
- Timeout warnings (remind human before expiry)
- Fallback to backup human
- Session continuation with uncertainty note

### Timeout Policies

```go
// Policy 1: Continue without input
sess.Respond(ctx, requestID, nil, 
    engine.OnTimeout(engine.ContinueWithNote(
        "Anna didn't respond; proceeding with best guess")))

// Policy 2: Retry with backup human
sess.Respond(ctx, requestID, nil,
    engine.OnTimeout(engine.RetryWith("Marcus")))

// Policy 3: Mark session incomplete
sess.Respond(ctx, requestID, nil,
    engine.OnTimeout(engine.MarkIncomplete()))
```

### Scheduler Architecture

```go
// Option A: In-process scheduler (simple)
scheduler := scheduler.NewInProcess()
scheduler.Schedule(time.Now().Add(6*time.Hour), func() {
    sess.HandleTimeout(requestID)
})

// Option B: External job queue (production)
scheduler := scheduler.NewRedis(redisClient)
scheduler.Schedule(requestID, 6*time.Hour, engine.ContinueWithNote("..."))
```

### Implementation

**Files to create/modify**:
- `pkg/scheduler/` - New package for timeout scheduling
- `pkg/scheduler/in_process.go` - Simple in-memory scheduler
- `pkg/scheduler/redis.go` - Redis-backed scheduler (optional)
- `pkg/engine/timeout_policy.go` - Policy definitions
- `pkg/engine/session.go` - `HandleTimeout()` method
- `pkg/integration/slack.go` - Warning messages before timeout
- `examples/24-timeout-handling/` - Timeout policy demonstration

**Dependencies**:
- Optional: `github.com/go-redis/redis` for distributed scheduling

**Key Design Decisions**:
- Default policy is "continue with note" (non-blocking)
- In-process scheduler sufficient for single-instance deployments
- Redis scheduler for multi-instance/production
- Warning sent at 80% of timeout duration

### Success Criteria

- [ ] Timeout triggers after configured duration
- [ ] Continue-with-note policy works correctly
- [ ] Retry-with-backup sends to second human
- [ ] Warning messages sent before timeout
- [ ] Redis scheduler works in distributed setup
- [ ] Example shows all three timeout policies

---

## Phase 7: Advanced Participation Modes

**Goal**: On-demand and @-mention participation modes beyond turn-based.

**Duration**: 3-4 weeks

### Capabilities

- **On-Demand**: Human signals readiness, session pauses at next boundary
- **@-Mention**: Human responds only when explicitly tagged by agents
- Dynamic mode switching during session
- UI affordances for requesting turn

### On-Demand Mode

```go
sess, _ := eng.Session("Review").
    Participant(agent1, agent2, human).
    Participation(engine.OnDemand()).
    Start(ctx)

// In separate goroutine or UI callback
sess.RequestTurn(ctx, "User")  // Pauses at next turn boundary

// Session run loop
result, _ := sess.Run(ctx, msg)
if result.Status == engine.AwaitingInput {
    // User requested turn - now waiting for input
}
```

### @-Mention Mode

```go
sess, _ := eng.Session("Brainstorm").
    Participant(agent1, agent2, human).
    Participation(engine.OnMention()).
    Start(ctx)

// Agent mentions human in message
agent.SendMessage("@User, what do you think about this approach?")

// Triggers AwaitingInput for mentioned human
```

### Implementation

**Files to create/modify**:
- `pkg/engine/interaction.go` - `OnDemand()`, `OnMention()` modes
- `pkg/engine/session.go` - `RequestTurn()`, mention detection
- `pkg/collab/roundtable/on_demand.go` - Turn injection logic
- `pkg/event/types.go` - `TurnRequested`, `HumanMentioned` events
- `examples/25-on-demand-mode/` - On-demand participation demo
- `examples/26-mention-mode/` - @-mention participation demo

**Key Design Decisions**:
- On-demand pauses at next protocol boundary (not mid-turn)
- Mention detection uses simple `@ParticipantID` syntax
- Modes can switch mid-session via `sess.SetParticipationMode()`
- UI can subscribe to events for "request turn" button state

### Success Criteria

- [ ] On-demand mode pauses correctly when requested
- [ ] @-mention detection triggers AwaitingInput
- [ ] Mode switching mid-session works
- [ ] Events enable UI affordances
- [ ] Examples demonstrate both modes

---

## Phase 8: Connected Personas

**Goal**: Map personas to real humans with escalation intelligence and preferences.

**Duration**: 4-5 weeks

### Capabilities

- Persona-to-human connection with consent
- Escalation intelligence (when to ask vs simulate)
- Human preference configuration (availability, rate limits, filters)
- Persona learning from human responses
- Drift detection and persona updates

### Connection Flow

```bash
# Create persona
meanwhile persona create "Anna Chen" \
  --from-slack ./anna-slack.json \
  --role "Head of Engineering"

# Connect to real human (requires consent)
meanwhile persona connect "Anna Chen" \
  --slack-user "@anna.chen" \
  --email "anna@company.com"

# Anna receives consent request in Slack
# Upon acceptance, preferences configured
```

### Escalation Intelligence

```go
type EscalationPolicy struct {
    // When to escalate
    MissingCriticalInfo bool   // Default: true
    ConflictingSignals  bool   // Default: true  
    HighStakes          bool   // Default: true
    LowConfidence       float64 // Threshold: 0.6
    
    // How to escalate
    QuestionQuality     QualityCheck
    MaxPerSession       int     // Default: 2
    MaxPerDay           int     // Default: 5
    CooldownMinutes     int     // Default: 30
}

// Agent checks before escalating
if persona.ShouldEscalate(ctx, uncertainty) {
    askHuman.Call(ctx, AskHumanInput{
        Question: formulateQuestion(uncertainty),
        Participant: persona.ConnectedHuman(),
    })
}
```

### Human Preferences

```go
human := eng.Human("Anna").
    ContactVia("slack", "@anna.chen").
    Availability(humanpref.WorkHours("Mon-Fri 9am-6pm PST")).
    QuestionFilters(
        humanpref.Allow("technical_decisions"),
        humanpref.Allow("timeline_capacity"),
        humanpref.Deny("budget_financial"),
    ).
    RateLimits(
        humanpref.MaxPerDay(5),
        humanpref.MaxPerSession(2),
        humanpref.CooldownMinutes(30),
    ).
    ApprovalMode(humanpref.QueueForApproval()).
    Build()
```

### Persona Learning

```go
// After human response, update persona
persona.Learn(ctx, LearningData{
    Question: "Scoped-down vs full version?",
    Response: "Scoped-down, definitely. Ship something solid.",
    Context: "Q2 planning, dashboard decision",
    Confidence: 0.95,
})

// Drift detection
if persona.DetectDrift() > 0.3 {
    suggested := persona.SuggestedUpdates()
    // Prompt owner to review and apply
}
```

### Implementation

**Files to create/modify**:
- `pkg/persona/` - New package for persona management
- `pkg/persona/connection.go` - Persona-to-human mapping
- `pkg/persona/escalation.go` - Escalation intelligence
- `pkg/persona/preferences.go` - Human preference models
- `pkg/persona/learning.go` - Response-based learning
- `pkg/persona/drift.go` - Drift detection algorithms
- `pkg/integration/slack.go` - Consent flow, preference UI
- `cmd/meanwhile/persona.go` - CLI commands for persona management
- `examples/27-connected-personas/` - Complete connected persona demo

**Key Design Decisions**:
- Personas stored as JSON with optional human connection
- Escalation policy per persona, overridable per session
- Learning is opt-in (default off for privacy)
- Drift detection uses response divergence metrics
- Consent requires mutual agreement (requester + human)

### Success Criteria

- [ ] Persona can be connected to real human with consent
- [ ] Escalation policy prevents over-pinging
- [ ] Preferences enforced (rate limits, filters, availability)
- [ ] Question quality check rejects vague questions
- [ ] Learning updates persona model correctly
- [ ] Drift detection triggers review prompts
- [ ] Example demonstrates full lifecycle (create → connect → escalate → learn)

---

## Phase Summary

| Phase | Duration | Key Deliverable | Dependencies |
|-------|----------|----------------|--------------|
| 1. Human Participants | 3-4 weeks | Turn-based human participation | None |
| 2. Pause/Resume State | 2-3 weeks | Session state persistence | Phase 1 |
| 3. Ask Human Tool | 2 weeks | `ask_human` tool for agents | Phase 1-2 |
| 4. Outbound Integrations | 3-4 weeks | Slack/Email sending | Phase 3 |
| 5. Inbound Response | 3-4 weeks | Webhook receiver, resume | Phase 2-4 |
| 6. Timeout & Scheduling | 2-3 weeks | Timeout handling | Phase 5 |
| 7. Advanced Modes | 3-4 weeks | On-demand, @-mention | Phase 1 |
| 8. Connected Personas | 4-5 weeks | Persona-human mapping | Phase 3-6 |

**Total Duration**: ~24-30 weeks (~6-8 months)

---

## Testing Strategy

### Per-Phase Testing

Each phase includes:
- **Unit tests**: Core logic (pause/resume, correlation, etc.)
- **Integration tests**: Cross-component flows (e.g., webhook → session resume)
- **Example programs**: Demonstrable end-to-end scenarios
- **Documentation**: Updated docs/guides with new capabilities

### End-to-End Testing

After Phase 6 completion:
- **Scenario 1**: Agent asks question → Slack sent → Human responds → Session resumes
- **Scenario 2**: Agent asks question → No response → Timeout → Session continues with note
- **Scenario 3**: Multiple humans asked concurrently → All respond → Session continues
- **Scenario 4**: Human outside availability hours → Queued → Sent during next window

After Phase 8 completion:
- **Scenario 5**: Persona uncertainty → Escalation check → Human asked (within limits)
- **Scenario 6**: Human response → Persona learns → Updated model used in next session
- **Scenario 7**: Drift detected → Human reviews → Approves persona update

---

## Migration & Backward Compatibility

### Phases 1-2: Breaking Changes Expected

- New `Participant` abstraction requires protocol updates
- Existing protocols continue working with agents only
- Human participation is opt-in

### Phases 3-8: Fully Backward Compatible

- Existing sessions without humans unaffected
- `ask_human` tool is optional
- Integrations are optional (can run without Slack/Email)
- Connected personas are optional enhancement

### Migration Guide

For each phase, provide:
1. **What changed**: API differences
2. **Migration steps**: Code changes required
3. **Deprecation timeline**: If applicable
4. **Compatibility matrix**: Which versions work together

---

## Open Questions & Decisions Needed

### Phase 1-2
- [ ] Should humans have prompts/context like agents? (e.g., "You are a product manager...")
- [ ] Can humans participate in breakout sessions?
- [ ] Do humans see full agent reasoning/tool calls or just messages?

### Phase 3-4
- [ ] Default timeout duration? (Suggest: 6 hours)
- [ ] Should ask_human support multi-choice responses? (Yes/No/Maybe vs free text)
- [ ] Rate limiting per human or per session?

### Phase 5-6
- [ ] Webhook authentication: HMAC only or support OAuth2?
- [ ] Session registry: Redis required for production or optional?
- [ ] Email reply parsing: Plain text only or HTML stripping?

### Phase 7-8
- [ ] Persona learning: Manual approval required or auto-apply?
- [ ] Drift threshold: What divergence percentage triggers review?
- [ ] Multi-tenant isolation: How to separate org/workspace personas?

---

## Success Metrics

### Phase 1-3 Success (Foundation)
- 5+ community examples using human participation
- 80%+ positive feedback on API ergonomics
- Zero P0 bugs in pause/resume flow

### Phase 4-6 Success (Core Workflow)
- Slack integration used in 3+ production scenarios
- Average response time < 4 hours for human requests
- Timeout handling prevents session deadlock in 100% of cases

### Phase 7-8 Success (Advanced Capabilities)
- 10+ connected personas in active use
- Escalation intelligence reduces unnecessary pings by 60%+
- Persona drift detection accuracy > 80%

### Overall Adoption
- 25% of Meanwhile users adopt human participation by end of Phase 8
- Documentation cited as clear and comprehensive
- Community contributes 2+ new integration adapters

---

## Resources & Roles

### Engineering
- **Phase 1-2**: 1 engineer, core session/protocol expertise
- **Phase 3-5**: 1-2 engineers, integration and tooling focus
- **Phase 6-8**: 1 engineer, persona/learning expertise

### Design/UX
- **Phase 1**: API design review
- **Phase 4**: Slack/Email message templates
- **Phase 7**: UI affordances for on-demand mode
- **Phase 8**: Persona preference configuration UX

### Documentation
- Per-phase guides and examples
- Migration guides for breaking changes
- Video walkthrough after Phase 6

### Community
- Phase 1 preview for early feedback
- Phase 4 beta with Slack-first users
- Phase 8 open beta for connected personas

---

## Related Documents

- `docs/ideas/human-participation.md` - Detailed human participant design
- `docs/ideas/connected-personas.md` - Persona-human connection vision
- `docs/ideas/human-escalation-workflow.md` - End-to-end workflow requirements
- `docs/concepts/hooks.md` - Interruption mechanisms
- `docs/guides/build-a-protocol.md` - Protocol authoring guide

---

**Next Steps**:
1. Review and validate plan with 2-3 potential users
2. Prototype Phase 1 API surface (participant types + turn detection)
3. Build minimal example for feedback
4. Finalize Phase 1 scope and timeline
5. Begin implementation

**Owner**: TBD  
**Reviewers**: TBD
