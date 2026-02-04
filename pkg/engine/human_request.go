package engine

import (
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

// HumanRequest captures a human input request emitted by ask_human.
type HumanRequest struct {
	RequestID          string    `json:"request_id"`
	SessionID          string    `json:"session_id"`
	ProtocolID         string    `json:"protocol_id"`
	ParticipantID      string    `json:"participant_id"`
	ParticipantName    string    `json:"participant_name"`
	Question           string    `json:"question"`
	Context            string    `json:"context,omitempty"`
	Urgency            string    `json:"urgency,omitempty"`
	Required           bool      `json:"required"`
	SuggestedResponses []string  `json:"suggested_responses,omitempty"`
	RequestedAt        time.Time `json:"requested_at"`
	TimeoutAt          time.Time `json:"timeout_at,omitempty"`
	ToolCallID         string    `json:"tool_call_id,omitempty"`
	ToolID             string    `json:"tool_id,omitempty"`
	AgentID            string    `json:"agent_id,omitempty"`
}

// HumanResponse captures a human response for a pending request.
type HumanResponse struct {
	RequestID       string        `json:"request_id"`
	SessionID       string        `json:"session_id"`
	ProtocolID      string        `json:"protocol_id"`
	ParticipantID   string        `json:"participant_id"`
	ParticipantName string        `json:"participant_name"`
	Response        agent.Message `json:"response"`
	ReceivedAt      time.Time     `json:"received_at"`
}
