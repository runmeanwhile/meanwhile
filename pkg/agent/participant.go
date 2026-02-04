package agent

// Identifier returns the agent identifier (optional).
func (a Agent) Identifier() string { return a.ID }

// DisplayName returns the agent display name.
func (a Agent) DisplayName() string { return a.Name }

// IsHuman reports that this participant is not a human.
func (a Agent) IsHuman() bool { return false }

// IsAgent reports that this participant is an agent.
func (a Agent) IsAgent() bool { return true }

// Agent returns the agent value for participant use.
func (a Agent) Agent() (Agent, bool) { return a, true }
