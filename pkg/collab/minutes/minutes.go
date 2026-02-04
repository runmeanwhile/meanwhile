package minutes

import (
	"strings"
	"sync"
)

// Minutes captures structured outputs from a protocol run.
type Minutes struct {
	mu          sync.RWMutex
	payload     map[string]any
	summary     string
	decisions   []Decision
	actions     []ActionItem
	questions   []OpenQuestion
	risks       []Risk
	assumptions []Assumption
	notes       []string
}

// Decision captures a recorded decision.
type Decision struct {
	// Statement is the decision made.
	Statement string `json:"statement"`
	// Rationale captures why the decision was made.
	Rationale string `json:"rationale,omitempty"`
	// Conditions capture any requirements to keep the decision valid.
	Conditions []string `json:"conditions,omitempty"`
	// Owner identifies who is responsible for carrying the decision forward.
	Owner string `json:"owner,omitempty"`
}

// ActionItem captures a follow-up action.
type ActionItem struct {
	// Description describes the work to complete.
	Description string `json:"description"`
	// Owner identifies who is responsible.
	Owner string `json:"owner,omitempty"`
	// Due captures a due date or timebox hint.
	Due string `json:"due,omitempty"`
	// Status captures optional progress information.
	Status string `json:"status,omitempty"`
}

// OpenQuestion captures an unresolved question.
type OpenQuestion struct {
	// Question describes what remains open.
	Question string `json:"question"`
	// Owner identifies who will resolve it.
	Owner string `json:"owner,omitempty"`
	// Notes provides supporting detail.
	Notes string `json:"notes,omitempty"`
}

// Risk captures a risk or concern raised in the session.
type Risk struct {
	// Description describes the risk.
	Description string `json:"description"`
	// Severity notes impact or urgency.
	Severity string `json:"severity,omitempty"`
	// Mitigation captures a proposed mitigation.
	Mitigation string `json:"mitigation,omitempty"`
}

// Assumption captures an explicit assumption used during discussion.
type Assumption struct {
	// Statement is the assumption being made.
	Statement string `json:"statement"`
	// Source notes who or what introduced the assumption.
	Source string `json:"source,omitempty"`
}

// New creates empty minutes.
func New() *Minutes {
	return &Minutes{payload: make(map[string]any)}
}

// Add stores a key/value in the minutes payload.
func (m *Minutes) Add(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payload[key] = value
}

// AddDecision stores a decision in minutes.
func (m *Minutes) AddDecision(decision Decision) {
	if strings.TrimSpace(decision.Statement) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisions = append(m.decisions, decision)
}

// Decisions returns a copy of recorded decisions.
func (m *Minutes) Decisions() []Decision {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Decision, len(m.decisions))
	copy(out, m.decisions)
	return out
}

// AddActionItem stores a follow-up action in minutes.
func (m *Minutes) AddActionItem(item ActionItem) {
	if strings.TrimSpace(item.Description) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actions = append(m.actions, item)
}

// ActionItems returns a copy of recorded actions.
func (m *Minutes) ActionItems() []ActionItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ActionItem, len(m.actions))
	copy(out, m.actions)
	return out
}

// AddOpenQuestion stores an unresolved question.
func (m *Minutes) AddOpenQuestion(question OpenQuestion) {
	if strings.TrimSpace(question.Question) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.questions = append(m.questions, question)
}

// OpenQuestions returns a copy of unresolved questions.
func (m *Minutes) OpenQuestions() []OpenQuestion {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]OpenQuestion, len(m.questions))
	copy(out, m.questions)
	return out
}

// AddRisk stores a risk or concern.
func (m *Minutes) AddRisk(risk Risk) {
	if strings.TrimSpace(risk.Description) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.risks = append(m.risks, risk)
}

// Risks returns a copy of recorded risks.
func (m *Minutes) Risks() []Risk {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Risk, len(m.risks))
	copy(out, m.risks)
	return out
}

// AddAssumption stores an explicit assumption.
func (m *Minutes) AddAssumption(assumption Assumption) {
	if strings.TrimSpace(assumption.Statement) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assumptions = append(m.assumptions, assumption)
}

// Assumptions returns a copy of recorded assumptions.
func (m *Minutes) Assumptions() []Assumption {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Assumption, len(m.assumptions))
	copy(out, m.assumptions)
	return out
}

// AddNote stores a free-form note.
func (m *Minutes) AddNote(note string) {
	if strings.TrimSpace(note) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notes = append(m.notes, note)
}

// Notes returns a copy of free-form notes.
func (m *Minutes) Notes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.notes...)
}

// SetSummary sets a human-readable summary.
func (m *Minutes) SetSummary(summary string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summary = summary
}

// Summary returns the summary text.
func (m *Minutes) Summary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.summary
}

// Payload returns a copy of the minutes payload.
func (m *Minutes) Payload() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]any, len(m.payload))
	for key, value := range m.payload {
		out[key] = value
	}
	if m.summary != "" {
		out["summary"] = m.summary
	}
	if len(m.decisions) > 0 {
		out["decisions"] = append([]Decision(nil), m.decisions...)
	}
	if len(m.actions) > 0 {
		out["actions"] = append([]ActionItem(nil), m.actions...)
	}
	if len(m.questions) > 0 {
		out["open_questions"] = append([]OpenQuestion(nil), m.questions...)
	}
	if len(m.risks) > 0 {
		out["risks"] = append([]Risk(nil), m.risks...)
	}
	if len(m.assumptions) > 0 {
		out["assumptions"] = append([]Assumption(nil), m.assumptions...)
	}
	if len(m.notes) > 0 {
		out["notes"] = append([]string(nil), m.notes...)
	}
	return out
}
