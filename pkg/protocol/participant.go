package protocol

import "github.com/runmeanwhile/meanwhile/pkg/agent"

// Participant represents a session participant (agent or human).
type Participant interface {
	Identifier() string
	DisplayName() string
	IsHuman() bool
	IsAgent() bool
	Agent() (agent.Agent, bool)
}
