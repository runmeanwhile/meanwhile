package pulse

import "time"

// Position represents an agent's stance on a topic.
type Position string

const (
	// PositionAgree indicates full agreement without reservations.
	PositionAgree Position = "agree"
	// PositionConditional indicates agreement with specific conditions.
	PositionConditional Position = "conditional"
	// PositionBlock indicates objection preventing agreement.
	PositionBlock Position = "block"
	// PositionAbstain indicates choosing not to take a position.
	PositionAbstain Position = "abstain"
	// PositionPending indicates no position has been signaled yet.
	PositionPending Position = "pending"
)

// AgentPosition captures an agent's position and reasoning.
type AgentPosition struct {
	Agent      string    `json:"agent"`
	Position   Position  `json:"position"`
	Reasoning  string    `json:"reasoning"`
	Conditions []string  `json:"conditions,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// State represents the outcome state of consensus-like processes.
type State string

const (
	// StateInProgress indicates consensus is still being negotiated.
	StateInProgress State = "in_progress"
	// StateFullAgreement indicates all agents agree without conditions.
	StateFullAgreement State = "full_agreement"
	// StateConditional indicates agreement with conditions that must be met.
	StateConditional State = "conditional_agreement"
	// StateBlocked indicates one or more agents have blocking objections.
	StateBlocked State = "blocked"
	// StateUnresolved indicates time budget exhausted before reaching consensus.
	StateUnresolved State = "unresolved"
)
