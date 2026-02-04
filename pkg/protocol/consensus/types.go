package consensus

import (
	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/collab/pulse"
)

// Position represents an agent's stance on the consensus topic.
type Position = pulse.Position

const (
	// PositionAgree indicates full agreement without reservations.
	PositionAgree = pulse.PositionAgree
	// PositionConditional indicates agreement with specific conditions.
	PositionConditional = pulse.PositionConditional
	// PositionBlock indicates objection preventing agreement.
	PositionBlock = pulse.PositionBlock
	// PositionAbstain indicates choosing not to take a position.
	PositionAbstain = pulse.PositionAbstain
	// PositionPending indicates no position has been signaled yet.
	PositionPending = pulse.PositionPending
)

// AgentPosition captures an agent's position and reasoning.
type AgentPosition = pulse.AgentPosition

// State represents the outcome state of the consensus process.
type State = pulse.State

const (
	// StateInProgress indicates consensus is still being negotiated.
	StateInProgress = pulse.StateInProgress
	// StateFullAgreement indicates all agents agree without conditions.
	StateFullAgreement = pulse.StateFullAgreement
	// StateConditional indicates agreement with conditions that must be met.
	StateConditional = pulse.StateConditional
	// StateBlocked indicates one or more agents have blocking objections.
	StateBlocked = pulse.StateBlocked
	// StateUnresolved indicates time budget exhausted before reaching consensus.
	StateUnresolved = pulse.StateUnresolved
)

// Result contains the structured outcome of a consensus session.
type Result struct {
	State          State           `json:"state"`
	Reasoning      string          `json:"reasoning"`
	Positions      []AgentPosition `json:"positions"`
	Conditions     []string        `json:"conditions,omitempty"`
	BlockingIssues []string        `json:"blocking_issues,omitempty"`
	RoundsUsed     int             `json:"rounds_used"`
	MaxRounds      int             `json:"max_rounds"`
	MessageThread  []agent.Message `json:"message_thread"`
}
