package engine

import (
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

// AgentPerspectiveMode controls how prior assistant messages are interpreted for the current agent turn.
type AgentPerspectiveMode string

const (
	// AgentPerspectiveSpeakerAware rewrites named assistant messages from other speakers to user-role context.
	AgentPerspectiveSpeakerAware AgentPerspectiveMode = "speaker_aware"
	// AgentPerspectiveLegacy preserves the original roles for all history messages.
	AgentPerspectiveLegacy AgentPerspectiveMode = "legacy"
)

func normalizeAgentPerspectiveMode(mode AgentPerspectiveMode) AgentPerspectiveMode {
	switch mode {
	case AgentPerspectiveLegacy, AgentPerspectiveSpeakerAware:
		return mode
	default:
		return AgentPerspectiveSpeakerAware
	}
}

func applyAgentPerspective(history []agent.Message, currentAgent string, mode AgentPerspectiveMode) []agent.Message {
	normalized := normalizeAgentPerspectiveMode(mode)
	if normalized == AgentPerspectiveLegacy {
		return history
	}
	if len(history) == 0 {
		return nil
	}
	out := make([]agent.Message, len(history))
	for i, msg := range history {
		out[i] = normalizePeerAssistantMessage(msg, currentAgent)
	}
	return out
}

func normalizePeerAssistantMessage(msg agent.Message, currentAgent string) agent.Message {
	if msg.Role != agent.RoleAssistant || strings.TrimSpace(msg.Name) == "" {
		return msg
	}
	if strings.EqualFold(strings.TrimSpace(msg.Name), strings.TrimSpace(currentAgent)) {
		return msg
	}
	msg.Role = agent.RoleUser
	return msg
}
