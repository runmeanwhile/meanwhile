# Planning Mode Design for Meanwhile Framework

## Vision (Simplified)

Planning should be a **primitive for creating structured plans**, not orchestrating execution. It's a Collab Kit component that:

1. **Creates structured plans**: Invoke a planning agent to produce a plan document
2. **Flexible storage**: Session-scoped, memory store, or just return to caller
3. **Optional approval**: Hook-based interruption for plan confirmation
4. **Plan becomes context**: Main agent/protocol uses plan for subsequent execution

**Key insight from Claude Code:** The Plan subagent creates the plan and exits. The main agent/session continues execution using that plan as context.

---

## Architectural Placement

**Location:** `pkg/collab/planning/` (Collab Kit, not Protocol or Core)

**Rationale:**
- Reusable primitive for any protocol
- Just creates plans - doesn't execute them
- Client decides how to use the plan (execute, store, modify, etc.)
- Composes with other collab kit components

---

## Core Concepts

### 1. Plan Document
A structured representation of a plan - flexible schema:

```go
type Plan struct {
    ID        string
    Title     string
    Summary   string
    Steps     []Step
    Metadata  map[string]any
    CreatedAt time.Time
}

type Step struct {
    ID           string
    Title        string
    Description  string
    Dependencies []string // Step IDs this depends on
    Metadata     map[string]any
}
```

**Note:** Deliberately minimal. Clients can use Metadata for custom fields (estimates, status, etc.)

### 2. Planning Agent
The agent that creates plans:

```go
type Planner struct {
    Agent         agent.Agent     // The planning agent (typically Sonnet)
    Questions     []Question      // Pre-configured questions
    AskQuestions  bool            // Whether to ask clarifying questions
    MaxQuestions  int             // Max interactive questions
    SystemPrompt  string          // Custom system prompt for planning
}

type Question struct {
    ID     string
    Text   string
    Answer string  // Populated after asking
}
```

### 3. Storage Strategy
Where to put the plan:

```go
type Storage string

const (
    StorageReturn  Storage = "return"   // Just return plan to caller
    StorageSession Storage = "session"  // Store in session metadata
    StorageMemory  Storage = "memory"   // Persist in memory store
    StorageCustom  Storage = "custom"   // Use custom storage function
)

type StorageConfig struct {
    Strategy Storage
    Key      string // Metadata key for session storage
    Custom   func(context.Context, protocol.Session, Plan) error
}
```

### 4. Approval (Optional)
Hook-based interruption for confirmation:

```go
type ApprovalConfig struct {
    Required      bool
    Timeout       time.Duration
    AutoApprove   func(Plan) bool
    OnApprove     func(Plan) error
    OnReject      func(Plan) error
    OnModify      func(Plan, Plan) error  // original, modified
}

type ApprovalResponse struct {
    Action   ApprovalAction  // approve, reject, modify
    Modified *Plan           // Non-nil if modifying
    Reason   string
}
```
```

---

## Fluent API Design (Meanwhile Style)

### Simple Planning Primitive

```go
package planning

// Planner creates a planning configuration
type Planner struct {
    agent        agent.Agent
    questions    []Question
    askQuestions bool
    maxQuestions int
    systemPrompt string
    storage      StorageConfig
    approval     *ApprovalConfig
}

// New creates a new planner
func New(agent agent.Agent, opts ...Option) *Planner {
    p := &Planner{
        agent: agent,
        storage: StorageConfig{Strategy: StorageReturn},
    }
    for _, opt := range opts {
        opt(p)
    }
    return p
}

type Option func(*Planner)

// WithQuestions configures interactive questions
func WithQuestions(max int) Option {
    return func(p *Planner) {
        p.askQuestions = true
        p.maxQuestions = max
    }
}

// WithQuestion adds a pre-configured question
func WithQuestion(q Question) Option {
    return func(p *Planner) {
        p.questions = append(p.questions, q)
    }
}

// WithSystemPrompt sets custom planning prompt
func WithSystemPrompt(prompt string) Option {
    return func(p *Planner) {
        p.systemPrompt = prompt
    }
}

// WithStorage configures plan storage
func WithStorage(strategy Storage, key string) Option {
    return func(p *Planner) {
        p.storage = StorageConfig{
            Strategy: strategy,
            Key:      key,
        }
    }
}

// WithCustomStorage sets custom storage function
func WithCustomStorage(fn func(context.Context, protocol.Session, Plan) error) Option {
    return func(p *Planner) {
        p.storage = StorageConfig{
            Strategy: StorageCustom,
            Custom:   fn,
        }
    }
}

// WithApproval configures approval requirement
func WithApproval(opts ...ApprovalOption) Option {
    cfg := &ApprovalConfig{Required: true}
    for _, opt := range opts {
        opt(cfg)
    }
    return func(p *Planner) {
        p.approval = cfg
    }
}

type ApprovalOption func(*ApprovalConfig)

func WithTimeout(d time.Duration) ApprovalOption {
    return func(a *ApprovalConfig) { a.Timeout = d }
}

func WithAutoApprove(fn func(Plan) bool) ApprovalOption {
    return func(a *ApprovalConfig) { a.AutoApprove = fn }
}
```

### Usage Examples

#### Example 1: Simple - Create and Return Plan

```go
// Most basic: create plan, return it
planner := planning.New(
    eng.Agent("Planner").
        Prompt("Create detailed implementation plans.").
        Model("claude-sonnet-4").
        Build(),
)

plan, err := planner.CreatePlan(ctx, sess, message.User("Build REST API"))
if err != nil {
    log.Fatal(err)
}

// Use plan however you want
fmt.Println(plan.Summary)
for _, step := range plan.Steps {
    fmt.Printf("- %s\n", step.Title)
}
```

#### Example 2: Session-Scoped Plan

```go
// Store plan in session metadata
planner := planning.New(
    plannerAgent,
    planning.WithStorage(planning.StorageSession, "current_plan"),
)

plan, _ := planner.CreatePlan(ctx, sess, message.User("Add user auth"))

// Later, main agent execution references the plan
result, _ := eng.Run(ctx, sess.ID(), message.User("Execute step 1"))

// Agent sees plan in its context (via protocol/hooks)
```

#### Example 3: With Approval

```go
// Plan requires approval before proceeding
planner := planning.New(
    plannerAgent,
    planning.WithStorage(planning.StorageSession, "plan"),
    planning.WithApproval(
        planning.WithTimeout(30*time.Second),
        planning.WithAutoApprove(func(p planning.Plan) bool {
            return len(p.Steps) < 3 // Auto-approve simple plans
        }),
    ),
)

plan, err := planner.CreatePlan(ctx, sess, message.User("Refactor auth"))
if err != nil {
    if errors.Is(err, planning.ErrAwaitingApproval) {
        // Plan is pending, need to approve
        // In a real app, show plan to user and get confirmation
        
        // Approve it
        err = planner.Approve(ctx, sess, planning.ApprovalResponse{
            Action: planning.ApproveAction,
        })
    }
}
```

#### Example 4: With Memory Persistence

```go
// Store plan in memory for cross-session access
planner := planning.New(
    plannerAgent,
    planning.WithStorage(planning.StorageMemory, ""),
    planning.WithQuestions(2), // Ask up to 2 clarifying questions
)

// Session 1: Create plan
plan, _ := planner.CreatePlan(ctx, sess1, message.User("Build feature X"))

// Session 2: Load and use plan
loadedPlan, _ := planning.LoadPlan(ctx, memStore, sess1.ID(), plan.ID)
// Continue with loaded plan
```

#### Example 5: Custom Storage

```go
// Use custom storage (e.g., external database)
planner := planning.New(
    plannerAgent,
    planning.WithCustomStorage(func(ctx context.Context, sess protocol.Session, p planning.Plan) error {
        // Store in your database
        return db.SavePlan(ctx, p)
    }),
)

plan, _ := planner.CreatePlan(ctx, sess, message.User("Task"))
```

#### Example 6: Protocol Integration

```go
// Use planning within a protocol
type PlanningConsensus struct {
    consensus *consensus.Consensus
    planner   *planning.Planner
}

func (p *PlanningConsensus) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
    // First, create a plan
    plan, err := p.planner.CreatePlan(ctx, sess, msg)
    if err != nil {
        return err
    }
    
    // Then run consensus on the plan
    planMsg := message.User(fmt.Sprintf("Review this plan:\n%s", plan.Summary))
    return p.consensus.OnMessage(ctx, sess, planMsg)
}
```

---

## Implementation Architecture

### Component Structure

```
pkg/collab/planning/
├── doc.go              # Package documentation
├── planning.go         # Planner type and CreatePlan()
├── plan.go             # Plan, Step, Question types
├── approval.go         # Approval hooks and flow
├── storage.go          # Storage strategies
├── parser.go           # Parse plan from agent response
├── events.go           # Planning events
└── planning_test.go
```

### Core Type

```go
package planning

type Planner struct {
    agent        agent.Agent
    questions    []Question
    askQuestions bool
    maxQuestions int
    systemPrompt string
    storage      StorageConfig
    approval     *ApprovalConfig
    
    mu    sync.RWMutex
    state State
}

type State string
const (
    StateIdle             State = "idle"
    StatePlanning         State = "planning"
    StateAwaitingApproval State = "awaiting_approval"
    StateApproved         State = "approved"
    StateRejected         State = "rejected"
)

// CreatePlan invokes the planning agent to create a plan
func (p *Planner) CreatePlan(ctx context.Context, sess protocol.Session, msg agent.Message) (*Plan, error) {
    p.setState(StatePlanning)
    
    // Build planning prompt
    prompt := p.buildPlanningPrompt(msg)
    
    // Ask clarifying questions if configured
    if p.askQuestions {
        answers, err := p.askQuestions(ctx, sess, msg)
        if err != nil {
            return nil, fmt.Errorf("ask questions: %w", err)
        }
        prompt = p.appendAnswers(prompt, answers)
    }
    
    // Run planning agent
    resp, err := sess.RunAgent(ctx, p.agent, protocol.RunRequest{
        Messages: []agent.Message{prompt},
        SystemMessages: p.systemMessages(),
        MaxToolIterations: 1,
    })
    if err != nil {
        return nil, fmt.Errorf("run planning agent: %w", err)
    }
    
    // Parse plan from response
    plan, err := ParsePlan(resp.Text())
    if err != nil {
        return nil, fmt.Errorf("parse plan: %w", err)
    }
    
    // Emit plan created event
    sess.Emit(event.New("planning.plan_created", sess.ID(), map[string]any{
        "plan": plan,
    }))
    
    // Handle approval if required
    if p.approval != nil && p.approval.Required {
        if !p.shouldAutoApprove(plan) {
            p.setState(StateAwaitingApproval)
            if err := p.awaitApproval(ctx, sess, plan); err != nil {
                return nil, err
            }
        }
    }
    
    // Store plan
    if err := p.storePlan(ctx, sess, plan); err != nil {
        return nil, fmt.Errorf("store plan: %w", err)
    }
    
    p.setState(StateApproved)
    return plan, nil
}

// buildPlanningPrompt constructs the prompt for planning
func (p *Planner) buildPlanningPrompt(msg agent.Message) agent.Message {
    text := msg.Text()
    if p.askQuestions && len(p.questions) > 0 {
        var sb strings.Builder
        sb.WriteString(text)
        sb.WriteString("\n\nClarifying information:\n")
        for _, q := range p.questions {
            sb.WriteString(fmt.Sprintf("- %s: %s\n", q.Text, q.Answer))
        }
        text = sb.String()
    }
    
    return message.User(text)
}

// systemMessages returns system messages for planning
func (p *Planner) systemMessages() []agent.Message {
    prompt := p.systemPrompt
    if prompt == "" {
        prompt = defaultPlanningPrompt
    }
    return []agent.Message{message.System(prompt)}
}

const defaultPlanningPrompt = `You are a planning specialist. Create a detailed, structured implementation plan.

Output the plan as a JSON object with this structure:
{
    "title": "Plan title",
    "summary": "Brief overview",
    "steps": [
        {
            "id": "step-1",
            "title": "Step title",
            "description": "What to do",
            "dependencies": []
        }
    ]
}

Be specific and actionable. Break down complex tasks into clear steps.`
```

---

## Approval Implementation

### Hook-Based Approval Flow

```go
// ApprovalHook intercepts and blocks until approval received
type ApprovalHook struct {
    planner  *Planner
    response chan ApprovalResponse
}

func (h *ApprovalHook) ID() string { return "planning.approval" }
func (h *ApprovalHook) Priority() int { return 1000 } // High priority

func (h *ApprovalHook) OnPreTurn(ctx context.Context, meta hook.SessionMeta, turn hook.Turn) (hook.Decision, hook.Turn, []hook.Interrupt, error) {
    if h.planner.State() != StateAwaitingApproval {
        return hook.Allow, turn, nil, nil
    }
    
    // Block execution and request approval
    plan := h.planner.CurrentPlan()
    return hook.Block, turn, []hook.Interrupt{{
        Message: agent.Message{
            Role: agent.RoleSystem,
            Parts: []agent.ContentPart{
                {Text: fmt.Sprintf("Plan requires approval:\n\n%s", plan.Format())},
            },
        },
        Reason: "planning.approval_required",
    }}, nil
}

// Approve processes approval response
func (p *Planner) Approve(ctx context.Context, sess protocol.Session, resp ApprovalResponse) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if p.state != StateAwaitingApproval {
        return fmt.Errorf("not awaiting approval")
    }
    
    plan := p.currentPlan
    
    switch resp.Action {
    case ApproveAction:
        p.state = StateApproved
        sess.Emit(event.New("planning.plan_approved", sess.ID(), map[string]any{
            "plan": plan,
        }))
        if p.approval.OnApprove != nil {
            return p.approval.OnApprove(*plan)
        }
        
    case RejectAction:
        p.state = StateRejected
        sess.Emit(event.New("planning.plan_rejected", sess.ID(), map[string]any{
            "plan":   plan,
            "reason": resp.Reason,
        }))
        if p.approval.OnReject != nil {
            return p.approval.OnReject(*plan)
        }
        return fmt.Errorf("plan rejected: %s", resp.Reason)
        
    case ModifyAction:
        if resp.Modified == nil {
            return fmt.Errorf("modified plan required")
        }
        originalPlan := *plan
        p.currentPlan = resp.Modified
        p.state = StateApproved
        sess.Emit(event.New("planning.plan_modified", sess.ID(), map[string]any{
            "original": originalPlan,
            "modified": resp.Modified,
        }))
        if p.approval.OnModify != nil {
            return p.approval.OnModify(originalPlan, *resp.Modified)
        }
        
    default:
        return fmt.Errorf("unknown approval action: %s", resp.Action)
    }
    
    // Unblock execution
    select {
    case p.approvalDone <- struct{}{}:
    default:
    }
    
    return nil
}

// awaitApproval blocks until approval received or timeout
func (p *Planner) awaitApproval(ctx context.Context, sess protocol.Session, plan *Plan) error {
    sess.Emit(event.New("planning.approval_required", sess.ID(), map[string]any{
        "plan": plan,
    }))
    
    timeout := p.approval.Timeout
    if timeout == 0 {
        timeout = 30 * time.Second
    }
    
    timer := time.NewTimer(timeout)
    defer timer.Stop()
    
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-timer.C:
        // Timeout - handle default action
        return fmt.Errorf("approval timeout after %s", timeout)
    case <-p.approvalDone:
        // Approval processed
        if p.state == StateRejected {
            return fmt.Errorf("plan rejected")
        }
        return nil
    }
}
```

---

## Storage Implementation

```go
// storePlan stores the plan based on configured strategy
func (p *Planner) storePlan(ctx context.Context, sess protocol.Session, plan *Plan) error {
    switch p.storage.Strategy {
    case StorageReturn:
        // No storage, just return
        return nil
        
    case StorageSession:
        // Store in session metadata
        key := p.storage.Key
        if key == "" {
            key = "plan"
        }
        sess.Metadata()[key] = plan
        return nil
        
    case StorageMemory:
        // Persist in memory store as event
        return sess.EmitWithContext(ctx, event.New("planning.plan_stored", sess.ID(), map[string]any{
            "plan": plan,
            "type": "plan_artifact",
        }))
        
    case StorageCustom:
        // Use custom storage function
        if p.storage.Custom == nil {
            return fmt.Errorf("custom storage function not configured")
        }
        return p.storage.Custom(ctx, sess, *plan)
        
    default:
        return fmt.Errorf("unknown storage strategy: %s", p.storage.Strategy)
    }
}

// LoadPlan retrieves a plan from memory store
func LoadPlan(ctx context.Context, store memory.Store, sessionID, planID string) (*Plan, error) {
    items, err := store.Query(ctx, memory.Query{
        SessionID: sessionID,
        Types:     []event.Type{"planning.plan_stored"},
    })
    if err != nil {
        return nil, err
    }
    
    for _, item := range items {
        if payload, ok := item.Event.Payload.(map[string]any); ok {
            if p, ok := payload["plan"].(Plan); ok && p.ID == planID {
                return &p, nil
            }
        }
    }
    
    return nil, fmt.Errorf("plan not found: %s", planID)
}

// GetPlan retrieves the current plan from session metadata
func GetPlan(sess protocol.Session, key string) (*Plan, bool) {
    if key == "" {
        key = "plan"
    }
    p, ok := sess.Metadata()[key].(*Plan)
    return p, ok
}
```
```

---

## Events

Planning emits events for observability:

```go
const (
    EventPlanningStarted   event.Type = "planning.started"
    EventQuestionsAsked    event.Type = "planning.questions_asked"
    EventPlanCreated       event.Type = "planning.plan_created"
    EventPlanStored        event.Type = "planning.plan_stored"
    EventApprovalRequired  event.Type = "planning.approval_required"
    EventPlanApproved      event.Type = "planning.plan_approved"
    EventPlanRejected      event.Type = "planning.plan_rejected"
    EventPlanModified      event.Type = "planning.plan_modified"
)
```

---

## How Execution Works

**Key insight:** Planning doesn't execute - it creates plans. The main agent/protocol executes using the plan.

### Pattern 1: Manual Execution with Plan Context

```go
// Create plan
planner := planning.New(plannerAgent)
plan, _ := planner.CreatePlan(ctx, sess, message.User("Build feature X"))

// Now main agent executes, referencing the plan
for _, step := range plan.Steps {
    result, _ := eng.Run(ctx, sess.ID(), message.User(fmt.Sprintf(
        "Execute this step from the plan:\n%s\n\nPlan context:\n%s",
        step.Description,
        plan.Summary,
    )))
    fmt.Printf("Step %s complete: %s\n", step.Title, result.Final)
}
```

### Pattern 2: Protocol with Plan-Aware Execution

```go
// Custom protocol that uses plans
type Planned struct {
    planner  *planning.Planner
    executor agent.Agent
}

func (p *Planned) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
    // Create plan
    plan, err := p.planner.CreatePlan(ctx, sess, msg)
    if err != nil {
        return err
    }
    
    // Store in session for subsequent turns
    sess.Metadata()["plan"] = plan
    
    // Execute with plan in context
    planContext := message.System(fmt.Sprintf("Execute this plan:\n%s", plan.Format()))
    resp, err := sess.RunAgent(ctx, p.executor, protocol.RunRequest{
        Messages: []agent.Message{msg},
        SystemMessages: []agent.Message{planContext},
    })
    if err != nil {
        return err
    }
    
    sess.Emit(event.New(event.ProtocolAction, sess.ID(), map[string]any{
        "plan":   plan,
        "result": resp.Text(),
    }))
    
    return nil
}
```

### Pattern 3: Execution with Memory Context

```go
// Plan stored in memory, main agent loads it
planner := planning.New(
    plannerAgent,
    planning.WithStorage(planning.StorageMemory, ""),
)

// Session 1: Planning
plan, _ := planner.CreatePlan(ctx, sess1, message.User("Task"))

// Session 2: Execution with plan context
history, _ := memory.BuildConversationContext(ctx, memStore, sess1.ID())
// history includes the plan as an event

result, _ := eng.Run(ctx, sess2.ID(), message.User("Continue with the plan"))
// Agent automatically has plan in conversation history
```

---

## Integration with Meanwhile Patterns

### 1. Composing with Agenda

```go
// Agenda refines scope, then planning creates detailed plan
type AgendaPlanning struct {
    agenda  *agenda.Agenda
    planner *planning.Planner
}

func (a *AgendaPlanning) Process(ctx context.Context, sess protocol.Session, msg agent.Message) (*planning.Plan, error) {
    // Refine scope with agenda
    scope, _ := a.agenda.RefineScope(ctx, sess, msg)
    
    // Create plan based on refined scope
    scopeMsg := message.User(scope)
    return a.planner.CreatePlan(ctx, sess, scopeMsg)
}
```

### 2. Composing with Chair

```go
// Chair intervenes during planning
type ChairPlanning struct {
    chair   *chair.Chair
    planner *planning.Planner
}

func (c *ChairPlanning) CreatePlanWithFacilitation(ctx context.Context, sess protocol.Session, msg agent.Message, maxRounds int) (*planning.Plan, error) {
    currentRound := 1
    var plan *planning.Plan
    
    for currentRound <= maxRounds {
        // Check if chair should interject
        if should, _ := c.chair.ShouldInterject(currentRound, maxRounds); should {
            facilitator := sess.Facilitator()
            if facilitator != nil {
                // Chair refines planning prompt
                prompt := chair.Prompt{
                    System: "Refine this planning request",
                    User:   msg.Text(),
                }
                interjection, _ := c.chair.Interject(ctx, sess, *facilitator, prompt)
                msg = message.User(interjection.Text())
            }
        }
        
        // Create plan
        plan, _ = c.planner.CreatePlan(ctx, sess, msg)
        
        // Check if plan is satisfactory
        if len(plan.Steps) > 0 {
            break
        }
        
        currentRound++
    }
    
    return plan, nil
}
```

### 3. Tool Integration

```go
// Expose planning as a tool for agents
func (p *Planner) AsTool() any {
    type Args struct {
        Task string `json:"task" description:"Task to create a plan for"`
    }
    
    tool, _ := tool.New[Args, string]("create_plan", func(ctx context.Context, args Args) (string, error) {
        // Get session from context
        sess := protocol.SessionFromContext(ctx)
        
        plan, err := p.CreatePlan(ctx, sess, message.User(args.Task))
        if err != nil {
            return "", err
        }
        
        return plan.Format(), nil
    })
    
    return tool
}

// Register and use
planTool := planner.AsTool()
eng.ToolRegistry().Register(planTool)

// Agent can now call create_plan tool
coordinator := eng.Agent("Coordinator").
    Prompt("You coordinate work. Use create_plan tool for complex tasks.").
    Tools("create_plan").
    Build()
```

---

## Example: End-to-End Workflow

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/runmeanwhile/meanwhile/pkg/collab/planning"
    "github.com/runmeanwhile/meanwhile/pkg/engine"
    "github.com/runmeanwhile/meanwhile/pkg/message"
    "github.com/runmeanwhile/meanwhile/pkg/protocol"
)

func main() {
    ctx := context.Background()
    eng, _ := engine.New(...)
    
    // Planning agent (Sonnet)
    planner := eng.Agent("Architect").
        Prompt("You create detailed, structured implementation plans.").
        Model("claude-sonnet-4").
        Build()
    
    // Main execution agent (Haiku)
    developer := eng.Agent("Developer").
        Prompt("You execute implementation steps methodically.").
        Model("claude-haiku-4").
        Build()
    
    // Create planner with approval
    p := planning.New(
        planner,
        planning.WithStorage(planning.StorageSession, "plan"),
        planning.WithApproval(
            planning.WithTimeout(30*time.Second),
            planning.WithAutoApprove(func(plan planning.Plan) bool {
                return len(plan.Steps) < 3
            }),
        ),
    )
    
    // Session for planning
    sess, _ := eng.Session("Feature Dev").
        Participant(planner).
        Participant(developer).
        Protocol(protocol.Solo()).
        Start(ctx)
    
    // Phase 1: Create plan
    fmt.Println("=== Creating Plan ===")
    plan, err := p.CreatePlan(ctx, sess, message.User("Build user authentication"))
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Plan: %s\n", plan.Title)
    for i, step := range plan.Steps {
        fmt.Printf("  %d. %s\n", i+1, step.Title)
    }
    
    // Phase 2: Execute using plan context
    fmt.Println("\n=== Executing Plan ===")
    planContext := message.System(fmt.Sprintf(
        "Follow this implementation plan:\n%s",
        plan.Format(),
    ))
    
    result, _ := eng.Run(ctx, sess.ID(), message.User("Execute the plan"))
    fmt.Println(result.Final)
}
```

---

## Implementation Phases

### Phase 1: Core Planning Primitive (MVP)
- [ ] `pkg/collab/planning/` package structure
- [ ] Plan, Step, Question types
- [ ] Planner type with `CreatePlan()`
- [ ] JSON plan parser
- [ ] Storage strategies (return, session, memory)
- [ ] Basic events (plan_created, plan_stored)
- [ ] Tests

### Phase 2: Approval Mechanism
- [ ] ApprovalConfig and options
- [ ] ApprovalHook for blocking
- [ ] `Approve()` API
- [ ] Approval events and flow
- [ ] Auto-approval predicates
- [ ] Timeout handling
- [ ] Tests

### Phase 3: Interactive Questions
- [ ] Question asking flow
- [ ] Pre-configured questions
- [ ] Dynamic question generation
- [ ] Answer collection
- [ ] Integrate with planning prompt
- [ ] Tests

### Phase 4: Integration & Polish
- [ ] Tool integration (`AsTool()`)
- [ ] Composition helpers (with agenda, chair)
- [ ] Plan formatting utilities
- [ ] Example: `17-planning-mode/`
- [ ] Documentation: `docs/recipes/planning.md`
- [ ] Update `docs/guides/build-a-protocol.md`

---

## Open Questions

1. **Should planner be session-attached or freestanding?**
   - Freestanding: `planner.CreatePlan(ctx, sess, msg)` (current design)
   - Session-attached: `sess.Planning().CreatePlan(ctx, msg)`
   - **Recommendation:** Freestanding - more flexible, reusable across sessions

2. **How should interactive questions work?**
   - Block with hook and wait for responses via API?
   - Use tool calling pattern for questions?
   - Emit events and collect responses asynchronously?
   - **Recommendation:** Start simple with pre-configured questions, add interactive later

3. **Plan parsing: Strict or flexible?**
   - Strict JSON schema validation
   - Flexible parsing with fallbacks
   - **Recommendation:** Flexible - extract what's there, allow custom fields in Metadata

4. **Should plans have execution tracking built-in?**
   - Add `Status`, `Result` fields to Step?
   - Keep Plan immutable, track execution separately?
   - **Recommendation:** Keep Plan immutable - execution tracking is separate concern

5. **How to handle plan modifications?**
   - Edit plan in place (mutable)
   - Create new plan version (immutable)
   - **Recommendation:** Immutable - modified plan is new Plan with incremented version

---

## Success Criteria

1. **Simple primitive**: Just creates plans, doesn't orchestrate execution
2. **Flexible storage**: Works with any storage strategy (return, session, memory, custom)
3. **Approval pattern**: Hook-based interruption for confirmation
4. **Composable**: Works with agenda, chair, protocols
5. **Observable**: Emits events for all operations
6. **Meanwhile style**: Fluent builder API matching existing components

---

## Key Simplifications from Original Design

**Removed:**
- ❌ ExecutionPhase - execution is separate concern
- ❌ Step status tracking - not part of planning primitive
- ❌ Resumption/checkpoints - execution concern, not planning
- ❌ Parallel execution - execution concern, not planning
- ❌ Plan format enum - just parse JSON flexibly
- ❌ Complex state machine - simplified to idle/planning/awaiting/approved/rejected

**Added:**
- ✅ Storage strategies - flexible plan persistence
- ✅ Simpler Plan structure - just ID, title, summary, steps
- ✅ Clear separation: planning creates plans, agents execute them
- ✅ Examples showing execution patterns with plans

**Result:** Planning is now a focused primitive that does one thing well: create structured plans.
