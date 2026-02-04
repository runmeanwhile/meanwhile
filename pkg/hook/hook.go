package hook

import (
	"context"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Decision determines how the engine should proceed after a hook.
type Decision int

// Decision constants define hook outcomes.
const (
	Allow Decision = iota
	Block
	Modify
)

// Hook is the base interface for all hooks.
type Hook interface {
	ID() string
	Priority() int
}

// SessionMeta is minimal session information for hooks.
type SessionMeta struct {
	SessionID  string
	ProtocolID string
}

// StopReason describes why a stop was requested.
type StopReason string

// Stop reason constants.
const (
	StopReasonSessionClose StopReason = "session.close"
)

// PreMessageHook runs before a message is processed.
type PreMessageHook interface {
	Hook
	OnPreMessage(ctx context.Context, meta SessionMeta, msg agent.Message) (Decision, agent.Message, error)
}

// Turn represents a single agent run request.
type Turn struct {
	Agent   agent.Agent
	Request protocol.RunRequest
}

// TurnResult represents the outcome of a single agent run.
type TurnResult struct {
	Agent    agent.Agent
	Request  protocol.RunRequest
	Response agent.Message
}

// Interrupt describes a hook-driven interjection.
type Interrupt struct {
	Message agent.Message
	Reason  string
}

// PreTurnHook runs before an agent turn executes.
type PreTurnHook interface {
	Hook
	OnPreTurn(ctx context.Context, meta SessionMeta, turn Turn) (Decision, Turn, []Interrupt, error)
}

// PostTurnHook runs after an agent turn executes.
type PostTurnHook interface {
	Hook
	OnPostTurn(ctx context.Context, meta SessionMeta, result TurnResult) (Decision, TurnResult, []Interrupt, error)
}

// PreToolHook runs before a tool executes.
type PreToolHook interface {
	Hook
	OnPreToolUse(ctx context.Context, meta SessionMeta, call tool.Call) (Decision, tool.Call, error)
}

// PostToolHook runs after a tool executes.
type PostToolHook interface {
	Hook
	OnPostToolUse(ctx context.Context, meta SessionMeta, result tool.Result) (Decision, tool.Result, error)
}

// StopHook runs when the engine is about to stop a loop.
type StopHook interface {
	Hook
	OnStop(ctx context.Context, meta SessionMeta, reason StopReason) (Decision, error)
}
