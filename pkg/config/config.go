package config

import "time"

// Config is the root configuration structure.
type Config struct {
	Global   GlobalConfig             `json:"global" yaml:"global"`
	Agents   map[string]AgentConfig   `json:"agents" yaml:"agents"`
	Tools    map[string]ToolConfig    `json:"tools" yaml:"tools"`
	Sessions map[string]SessionConfig `json:"sessions" yaml:"sessions"`
}

// GlobalConfig contains global defaults and shared providers.
type GlobalConfig struct {
	Providers map[string]ProviderConfig `json:"providers" yaml:"providers"`
	Defaults  AgentConfig               `json:"defaults" yaml:"defaults"`
	Memory    MemoryConfig              `json:"memory" yaml:"memory"`
	// ProviderRetry configures retry behavior for provider streams.
	ProviderRetry ProviderRetryConfig `json:"provider_retry" yaml:"provider_retry"`
	// MemoryAutomation configures automatic session memory capture.
	MemoryAutomation MemoryAutomationConfig `json:"memory_automation" yaml:"memory_automation"`
	Context          ContextConfig          `json:"context" yaml:"context"`
	Telemetry        TelemetryConfig        `json:"telemetry" yaml:"telemetry"`
	// RunTimeoutSeconds sets the default timeout for session runs.
	RunTimeoutSeconds int `json:"run_timeout_seconds" yaml:"run_timeout_seconds"`
}

// ProviderConfig defines a provider configuration.
type ProviderConfig struct {
	Type    string         `json:"type" yaml:"type"`
	APIKey  string         `json:"api_key" yaml:"api_key"`
	BaseURL string         `json:"base_url" yaml:"base_url"`
	Params  map[string]any `json:"params" yaml:"params"`
}

// AgentConfig configures an agent instance.
type AgentConfig struct {
	Name       string         `json:"name" yaml:"name"`
	ProfileID  string         `json:"profile_id" yaml:"profile_id"`
	ProviderID string         `json:"provider_id" yaml:"provider_id"`
	Tools      []string       `json:"tools" yaml:"tools"`
	Params     map[string]any `json:"params" yaml:"params"`
}

// ToolConfig configures a tool.
type ToolConfig struct {
	Params map[string]any `json:"params" yaml:"params"`
}

// SessionConfig is a named session configuration.
type SessionConfig struct {
	Name          string               `json:"name" yaml:"name"`
	Tags          []string             `json:"tags" yaml:"tags"`
	Metadata      map[string]any       `json:"metadata" yaml:"metadata"`
	ProtocolID    string               `json:"protocol_id" yaml:"protocol_id"`
	Participants  []string             `json:"participants" yaml:"participants"`
	Facilitator   string               `json:"facilitator" yaml:"facilitator"`
	Params        map[string]any       `json:"params" yaml:"params"`
	Groups        map[string][]string  `json:"groups" yaml:"groups"`
	TimeoutPolicy *TimeoutPolicyConfig `json:"timeout_policy" yaml:"timeout_policy"`
}

// TimeoutPolicyConfig configures how a session handles human request timeouts.
type TimeoutPolicyConfig struct {
	Strategy         string `json:"strategy" yaml:"strategy"`
	Note             string `json:"note" yaml:"note"`
	RetryParticipant string `json:"retry_participant" yaml:"retry_participant"`
}

// MemoryConfig configures memory policies.
type MemoryConfig struct {
	Store  string         `json:"store" yaml:"store"`
	Params map[string]any `json:"params" yaml:"params"`
}

// MemoryAutomationConfig configures automatic memory capture on session close.
type MemoryAutomationConfig struct {
	Enabled        bool                    `json:"enabled" yaml:"enabled"`
	ProviderID     string                  `json:"provider_id" yaml:"provider_id"`
	Model          string                  `json:"model" yaml:"model"`
	Prompt         string                  `json:"prompt" yaml:"prompt"`
	Params         map[string]any          `json:"params" yaml:"params"`
	Context        MemoryAutomationContext `json:"context" yaml:"context"`
	TimeoutSeconds int                     `json:"timeout_seconds" yaml:"timeout_seconds"`
	StoreEvent     string                  `json:"store_event" yaml:"store_event"`
}

// MemoryAutomationContext configures how conversation context is built.
type MemoryAutomationContext struct {
	RecentMessages     int  `json:"recent_messages" yaml:"recent_messages"`
	TokenLimit         int  `json:"token_limit" yaml:"token_limit"`
	IncludeToolResults bool `json:"include_tool_results" yaml:"include_tool_results"`
}

// TelemetryConfig configures telemetry.
type TelemetryConfig struct {
	Provider string         `json:"provider" yaml:"provider"`
	Params   map[string]any `json:"params" yaml:"params"`
}

// ContextConfig configures prompt context handling.
type ContextConfig struct {
	MaxPromptTokens    int                 `json:"max_prompt_tokens" yaml:"max_prompt_tokens"`
	RollingWindow      int                 `json:"rolling_window" yaml:"rolling_window"`
	MaxToolOutputChars int                 `json:"max_tool_output_chars" yaml:"max_tool_output_chars"`
	Summarization      SummarizationConfig `json:"summarization" yaml:"summarization"`
	AutoSummarize      AutoSummarizeConfig `json:"auto_summarize" yaml:"auto_summarize"`
}

// SummarizationConfig configures context summarization.
type SummarizationConfig struct {
	Enabled         bool `json:"enabled" yaml:"enabled"`
	ThresholdTokens int  `json:"threshold_tokens" yaml:"threshold_tokens"`
}

// AutoSummarizeConfig configures automatic summarization.
type AutoSummarizeConfig struct {
	Enabled           *bool `json:"enabled" yaml:"enabled"`
	SummarizeAtTokens int   `json:"summarize_at_tokens" yaml:"summarize_at_tokens"`
	MinKeepMessages   int   `json:"min_keep_messages" yaml:"min_keep_messages"`
}

// ProviderRetryConfig configures provider stream retry behavior.
type ProviderRetryConfig struct {
	Enabled         *bool         `json:"enabled" yaml:"enabled"`
	MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
	InitialInterval time.Duration `json:"initial_interval" yaml:"initial_interval"`
	MaxInterval     time.Duration `json:"max_interval" yaml:"max_interval"`
	Multiplier      float64       `json:"multiplier" yaml:"multiplier"`
}
