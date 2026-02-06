package protocol

import (
	"context"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Config is a protocol configuration map.
type Config map[string]any

// Protocol defines collaboration behavior for a session.
type Protocol interface {
	ID() string
	Participants() []Participant
	Init(ctx context.Context, sess Session) error
	OnMessage(ctx context.Context, sess Session, msg agent.Message) error
	OnEvent(ctx context.Context, sess Session, ev event.Event) error
	Shutdown(ctx context.Context, sess Session) error
}

// ResultProvider exposes structured results for a protocol run.
type ResultProvider interface {
	Result() map[string]any
}

// StatefulProtocol exposes protocol state for checkpointing.
type StatefulProtocol interface {
	Protocol
	GetState() (map[string]any, error)
	SetState(state map[string]any) error
}

// TimeoutProvider supplies a default timeout for session runs.
type TimeoutProvider interface {
	DefaultTimeout() time.Duration
}

// ConfigProvider exposes protocol configuration for persistence.
type ConfigProvider interface {
	Config() Config
}

// Session exposes minimal session capabilities to protocols.
type Session interface {
	ID() string
	Name() string
	Tags() []string
	Metadata() map[string]any
	ProtocolID() string
	Participants() []Participant
	Facilitator() *agent.Agent
	Groups() map[string][]Participant
	Emit(event.Event) error
	EmitWithContext(ctx context.Context, ev event.Event) error
	RunAgent(ctx context.Context, agent agent.Agent, req RunRequest) (agent.Message, error)
	RunTurn(ctx context.Context, participant Participant, req RunRequest, opts ...TurnOption) (agent.Message, error)
	AwaitInput(ctx context.Context, participant Participant, turnContext string, resume TurnResume, opts ...InputOption) error
	// Tool registration for session-scoped tools
	RegisterTool(t any) error
	RegisterTools(tools ...any) error
	AddDefaultTools(ids ...string)
	DefaultTools() []string
}

// RunRequest configures an agent execution.
type RunRequest struct {
	Messages          []agent.Message
	SystemMessages    []agent.Message
	Params            map[string]any
	MaxToolIterations int
	MaxRunDuration    time.Duration
	Tools             []string // Tool IDs to add for this run
	ToolPolicy        tool.Policy
	Context           ContextConfig
	OutputSchema      any // Optional: constrains output to this type (overrides agent-level schema)
	Silent            bool
}

// ContextConfig configures context selection.
type ContextConfig struct {
	MaxPromptTokens    int                 `json:"max_prompt_tokens" yaml:"max_prompt_tokens"`
	RollingWindow      int                 `json:"rolling_window" yaml:"rolling_window"`
	MaxToolOutputChars int                 `json:"max_tool_output_chars" yaml:"max_tool_output_chars"`
	Summarization      SummarizationConfig `json:"summarization" yaml:"summarization"`
}

// SummarizationConfig controls summary behavior.
type SummarizationConfig struct {
	Enabled         bool `json:"enabled" yaml:"enabled"`
	ThresholdTokens int  `json:"threshold_tokens" yaml:"threshold_tokens"`
}
