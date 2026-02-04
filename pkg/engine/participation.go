package engine

// RunStatus represents the outcome status of a session run.
type RunStatus string

const (
	StatusCompleted     RunStatus = "completed"
	StatusAwaitingInput RunStatus = "awaiting_input"
	StatusAwaitingTool  RunStatus = "awaiting_tool"
)

// Participation identifies how humans participate in a session.
type Participation string

const (
	ParticipationTurnBased Participation = "turn_based"
)

// ParticipationMode defines the session-level participation strategy.
type ParticipationMode interface {
	Mode() Participation
}

type turnBasedMode struct{}

// TurnBased enables turn-based human participation.
func TurnBased() ParticipationMode {
	return turnBasedMode{}
}

func (turnBasedMode) Mode() Participation {
	return ParticipationTurnBased
}
