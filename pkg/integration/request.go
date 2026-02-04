package integration

import "time"

// Request describes a human escalation request for outbound delivery.
type Request struct {
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

	Channel string `json:"channel,omitempty"`
	Contact string `json:"contact,omitempty"`
}

// DispatchResult reports which integration handled a request.
type DispatchResult struct {
	IntegrationID string
	Channel       string
	Contact       string
}
