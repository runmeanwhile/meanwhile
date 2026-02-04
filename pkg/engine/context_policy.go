package engine

import (
	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/contextpolicy"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

type resolvedContextConfig struct {
	MaxPromptTokens        int
	RollingWindow          int
	MaxToolOutputChars     int
	SummarizationEnabled   bool
	SummarizationThreshold int
}

func resolveContextConfig(global config.ContextConfig, override protocol.ContextConfig) resolvedContextConfig {
	out := resolvedContextConfig{
		MaxPromptTokens:        global.MaxPromptTokens,
		RollingWindow:          global.RollingWindow,
		MaxToolOutputChars:     global.MaxToolOutputChars,
		SummarizationEnabled:   global.Summarization.Enabled,
		SummarizationThreshold: global.Summarization.ThresholdTokens,
	}

	if override.MaxPromptTokens > 0 {
		out.MaxPromptTokens = override.MaxPromptTokens
	}
	if override.RollingWindow > 0 {
		out.RollingWindow = override.RollingWindow
	}
	if override.MaxToolOutputChars > 0 {
		out.MaxToolOutputChars = override.MaxToolOutputChars
	}
	if override.Summarization.Enabled {
		out.SummarizationEnabled = true
	}
	if override.Summarization.ThresholdTokens > 0 {
		out.SummarizationThreshold = override.Summarization.ThresholdTokens
	}

	return out
}

func buildSystemMessages(profile agent.Profile, systemMessages []agent.Message) []agent.Message {
	out := make([]agent.Message, 0, len(systemMessages)+1)
	if profile.Prompt != "" {
		out = append(out, message.System(profile.Prompt))
	}
	out = append(out, systemMessages...)
	return out
}

func buildContextInput(model string, systemMessages []agent.Message, history []agent.Message, cfg resolvedContextConfig, summarizer contextpolicy.Summarizer, estimator contextpolicy.TokenEstimator) contextpolicy.Input {
	return contextpolicy.Input{
		Model:                  model,
		SystemMessages:         systemMessages,
		Messages:               history,
		MaxTokens:              cfg.MaxPromptTokens,
		RollingWindow:          cfg.RollingWindow,
		MaxToolOutputChars:     cfg.MaxToolOutputChars,
		SummarizationEnabled:   cfg.SummarizationEnabled,
		SummarizationThreshold: cfg.SummarizationThreshold,
		Summarizer:             summarizer,
		TokenEstimator:         estimator,
	}
}
