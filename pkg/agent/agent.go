package agent

import "fmt"

// Agent represents an AI participant in a session.
type Agent struct {
	ID           string
	Name         string
	Model        string
	Profile      *Profile
	ProfileID    string
	ProviderID   string
	Tools        []string
	Params       map[string]any
	OutputSchema any // Optional: constrains agent output to this type
}

// Validate ensures the agent has required fields.
func (a Agent) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("agent name required")
	}
	// Model is optional - will be resolved by engine
	return nil
}

// Role is a message role identifier.
type Role string

// Supported message roles.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a conversational message.
type Message struct {
	Role       Role           `json:"role"`
	Parts      []ContentPart  `json:"parts,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
