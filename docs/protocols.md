# Protocols

Protocols define collaboration patterns. They are loaded through registries and can be swapped per session.
For a Collaboration Kit-first view, see `docs/concepts/protocols.md`.

## Interface

```go
type Protocol interface {
    ID() string
    Participants() []protocol.Participant
    Init(ctx context.Context, sess Session) error
    OnMessage(ctx context.Context, sess Session, msg agent.Message) error
    OnEvent(ctx context.Context, sess Session, ev event.Event) error
    Shutdown(ctx context.Context, sess Session) error
}

type ResultProvider interface {
    Result() map[string]any
}

type ConfigProvider interface {
    Config() Config
}
```

The `Session` interface gives protocols a safe view of participants and the ability to run agents.

```go
type Session interface {
    ID() string
    Name() string
    Tags() []string
    Metadata() map[string]any
    ProtocolID() string
    Participants() []protocol.Participant
    Facilitator() *agent.Agent
    Groups() map[string][]protocol.Participant
    Emit(event.Event) error
    EmitWithContext(ctx context.Context, ev event.Event) error
    RunAgent(ctx context.Context, agent agent.Agent, req RunRequest) (agent.Message, error)
    RunTurn(ctx context.Context, participant protocol.Participant, req RunRequest, opts ...protocol.TurnOption) (agent.Message, error)
    AwaitInput(ctx context.Context, participant protocol.Participant, context string, resume protocol.TurnResume, opts ...protocol.InputOption) error
    RegisterTool(t any) error
    RegisterTools(tools ...any) error
    AddDefaultTools(ids ...string)
    DefaultTools() []string
}
```

Tool policy can be set at the session level (builder/config) and overridden per run via `RunRequest.ToolPolicy`. Toolkits are attached at session creation time to bundle related tools (filesystem, system, MCP, internal APIs).

Participants expose display identity and optional IDs:

```go
type Participant interface {
    Identifier() string
    DisplayName() string
    IsHuman() bool
    IsAgent() bool
    Agent() (agent.Agent, bool)
}
```

## Example: Simple Round

```go
type Round struct{}

func (Round) ID() string { return "protocol.round" }
func (Round) Init(ctx context.Context, sess protocol.Session) error { return nil }
func (Round) OnMessage(ctx context.Context, sess protocol.Session, msg agent.Message) error {
    for _, participant := range sess.Participants() {
        ag, ok := participant.Agent()
        if !ok {
            return fmt.Errorf("round protocol requires agent participants")
        }
        _, _ = sess.RunAgent(ctx, ag, protocol.RunRequest{Messages: []agent.Message{msg}})
    }
    return nil
}
func (Round) OnEvent(ctx context.Context, sess protocol.Session, ev event.Event) error { return nil }
func (Round) Shutdown(ctx context.Context, sess protocol.Session) error { return nil }
```

## Built-in Protocols

Meanwhile ships with several ready-to-use collaboration protocols in `pkg/protocol/`:

```go
import "github.com/runmeanwhile/meanwhile/pkg/protocol"

// Brainstorming - diverge, interact, and vote like a real brainstorm
brainstorm := protocol.Brainstorming()

// Adversarial - two agents debate opposing positions with optional synthesis
debate := protocol.Adversarial()

// Consensus - convergent collaboration to reach shared outcomes
consensus := protocol.Consensus()

// Breakout - split into sub-groups, work in parallel, then reconvene
breakout := protocol.Breakout()

// Handoff - simple delegation from one agent to another
handoff := protocol.Handoff(caller, callee)

// Solo - single-agent execution
solo := protocol.Solo()
```

## Breakout Groups

Breakout can use explicit session groups:

```go
sess, _ := eng.NewSession(ctx, engine.SessionConfig{
    Protocol:     protocol.Breakout(),
    Participants: participants,
    Groups: map[string][]protocol.Participant{
        "Group A": {a1, a2},
        "Group B": {b1, b2},
    },
})
```
