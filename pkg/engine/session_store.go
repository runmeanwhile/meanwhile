package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

// SessionStore persists session definitions for rehydration.
// Implementations should be safe for concurrent use.
type SessionStore interface {
	Save(ctx context.Context, record SessionRecord) error
	Load(ctx context.Context, id string) (SessionRecord, error)
	Delete(ctx context.Context, id string) error
}

// SessionStateStore persists runtime session state for pause/resume.
type SessionStateStore interface {
	SaveState(ctx context.Context, state SessionState) error
	LoadState(ctx context.Context, id string) (SessionState, error)
	DeleteState(ctx context.Context, id string) error
}

// SessionRecord captures the minimum information needed to recreate a session.
type SessionRecord struct {
	ID             string
	Name           string
	Tags           []string
	Metadata       map[string]any
	ProtocolID     string
	ProtocolConfig protocol.Config
	Participants   []ParticipantRecord
	Facilitator    *agent.Agent
	Participation  string
	TimeoutPolicy  *TimeoutPolicy
	// Groups reference participants by identifier (ID if set, else name).
	Groups       map[string][]string
	DefaultTools []string
	ToolPolicy   tool.Policy
	Toolkits     []string
}

// ParticipantRecord captures a persisted participant.
type ParticipantRecord struct {
	Kind  string
	Agent *agent.Agent
	Human *HumanRecord
}

// HumanRecord captures a human participant.
type HumanRecord struct {
	ID               string
	Name             string
	Contacts         map[string]string
	PreferredChannel string
}

// InMemorySessionStore stores session records in memory.
type InMemorySessionStore struct {
	mu      sync.RWMutex
	records map[string]SessionRecord
	states  map[string]SessionState
}

// NewInMemorySessionStore creates an in-memory session store.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		records: make(map[string]SessionRecord),
		states:  make(map[string]SessionState),
	}
}

// Save upserts a session record.
func (s *InMemorySessionStore) Save(_ context.Context, record SessionRecord) error {
	if record.ID == "" {
		return fmt.Errorf("session id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.ID] = cloneSessionRecord(record)
	return nil
}

// Load retrieves a session record by ID.
func (s *InMemorySessionStore) Load(_ context.Context, id string) (SessionRecord, error) {
	if id == "" {
		return SessionRecord{}, fmt.Errorf("session id required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[id]
	if !ok {
		return SessionRecord{}, ErrSessionNotFound
	}
	return cloneSessionRecord(record), nil
}

// Delete removes a session record by ID.
func (s *InMemorySessionStore) Delete(_ context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("session id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[id]; !ok {
		return ErrSessionNotFound
	}
	delete(s.records, id)
	return nil
}

// SaveState upserts a session state record.
func (s *InMemorySessionStore) SaveState(_ context.Context, state SessionState) error {
	if state.SessionID == "" {
		return fmt.Errorf("session id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state.SessionID] = cloneSessionState(state)
	return nil
}

// LoadState retrieves session state by ID.
func (s *InMemorySessionStore) LoadState(_ context.Context, id string) (SessionState, error) {
	if id == "" {
		return SessionState{}, fmt.Errorf("session id required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.states[id]
	if !ok {
		return SessionState{}, ErrSessionNotFound
	}
	return cloneSessionState(state), nil
}

// DeleteState removes session state by ID.
func (s *InMemorySessionStore) DeleteState(_ context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("session id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.states[id]; !ok {
		return ErrSessionNotFound
	}
	delete(s.states, id)
	return nil
}

func cloneSessionRecord(record SessionRecord) SessionRecord {
	out := record
	out.Tags = append([]string(nil), record.Tags...)
	out.Metadata = cloneMetadata(record.Metadata)
	out.ProtocolConfig = cloneMetadata(record.ProtocolConfig)
	out.Participants = cloneParticipants(record.Participants)
	out.DefaultTools = append([]string(nil), record.DefaultTools...)
	out.ToolPolicy = cloneToolPolicy(record.ToolPolicy)
	out.Toolkits = append([]string(nil), record.Toolkits...)
	if record.Facilitator != nil {
		facilitator := cloneAgent(*record.Facilitator)
		out.Facilitator = &facilitator
	}
	if record.TimeoutPolicy != nil {
		policy := *record.TimeoutPolicy
		out.TimeoutPolicy = &policy
	}
	out.Groups = cloneGroupNames(record.Groups)
	return out
}

func cloneSessionState(state SessionState) SessionState {
	out := state
	if len(state.Pending) > 0 {
		out.Pending = append([]protocol.InputRequest(nil), state.Pending...)
	}
	if len(state.PendingTools) > 0 {
		out.PendingTools = append([]ToolRunState(nil), state.PendingTools...)
	}
	return out
}

func cloneGroupNames(groups map[string][]string) map[string][]string {
	if len(groups) == 0 {
		return nil
	}
	out := make(map[string][]string, len(groups))
	for name, members := range groups {
		out[name] = append([]string(nil), members...)
	}
	return out
}

func cloneParticipants(participants []ParticipantRecord) []ParticipantRecord {
	if len(participants) == 0 {
		return nil
	}
	out := make([]ParticipantRecord, 0, len(participants))
	for _, participant := range participants {
		out = append(out, cloneParticipant(participant))
	}
	return out
}

func cloneToolPolicy(policy tool.Policy) tool.Policy {
	out := policy
	out.AllowIDs = append([]string(nil), policy.AllowIDs...)
	out.DenyIDs = append([]string(nil), policy.DenyIDs...)
	out.AllowTags = append([]string(nil), policy.AllowTags...)
	out.DenyTags = append([]string(nil), policy.DenyTags...)
	return out
}

func cloneParticipant(participant ParticipantRecord) ParticipantRecord {
	out := participant
	if participant.Agent != nil {
		agentCopy := cloneAgent(*participant.Agent)
		out.Agent = &agentCopy
	}
	if participant.Human != nil {
		humanCopy := *participant.Human
		humanCopy.Contacts = cloneContacts(participant.Human.Contacts)
		out.Human = &humanCopy
	}
	return out
}

func cloneAgent(a agent.Agent) agent.Agent {
	out := a
	out.Tools = append([]string(nil), a.Tools...)
	out.Params = cloneMetadata(a.Params)
	if a.Profile != nil {
		profile := *a.Profile
		profile.Tools = append([]string(nil), a.Profile.Tools...)
		out.Profile = &profile
	}
	return out
}

const (
	participantKindAgent = "agent"
	participantKindHuman = "human"
)

func participantRecordsFromParticipants(participants []protocol.Participant) ([]ParticipantRecord, error) {
	if len(participants) == 0 {
		return nil, nil
	}
	out := make([]ParticipantRecord, 0, len(participants))
	for _, participant := range participants {
		record, err := participantRecord(participant)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, nil
}

func participantRecord(participant protocol.Participant) (ParticipantRecord, error) {
	if participant == nil {
		return ParticipantRecord{}, fmt.Errorf("participant required")
	}
	if participant.IsHuman() {
		var contacts map[string]string
		var preferred string
		if provider, ok := participant.(contactProvider); ok {
			contacts = provider.Contacts()
			preferred = provider.PreferredChannel()
		}
		return ParticipantRecord{
			Kind: participantKindHuman,
			Human: &HumanRecord{
				ID:               participant.Identifier(),
				Name:             participant.DisplayName(),
				Contacts:         cloneContacts(contacts),
				PreferredChannel: preferred,
			},
		}, nil
	}
	if participant.IsAgent() {
		ag, ok := participant.Agent()
		if !ok {
			return ParticipantRecord{}, fmt.Errorf("agent participant required")
		}
		agentCopy := cloneAgent(ag)
		return ParticipantRecord{
			Kind:  participantKindAgent,
			Agent: &agentCopy,
		}, nil
	}
	return ParticipantRecord{}, fmt.Errorf("participant kind required")
}

func participantsFromRecords(records []ParticipantRecord) ([]protocol.Participant, error) {
	if len(records) == 0 {
		return nil, nil
	}
	out := make([]protocol.Participant, 0, len(records))
	for _, record := range records {
		participant, err := participantFromRecord(record)
		if err != nil {
			return nil, err
		}
		out = append(out, participant)
	}
	return out, nil
}

func participantFromRecord(record ParticipantRecord) (protocol.Participant, error) {
	switch record.Kind {
	case participantKindAgent:
		if record.Agent == nil {
			return nil, fmt.Errorf("agent participant missing")
		}
		agentCopy := cloneAgent(*record.Agent)
		return agentCopy, nil
	case participantKindHuman:
		if record.Human == nil {
			return nil, fmt.Errorf("human participant missing")
		}
		return HumanParticipant{
			id:               record.Human.ID,
			name:             record.Human.Name,
			contacts:         cloneContacts(record.Human.Contacts),
			preferredChannel: record.Human.PreferredChannel,
		}, nil
	default:
		if record.Agent != nil {
			agentCopy := cloneAgent(*record.Agent)
			return agentCopy, nil
		}
		if record.Human != nil {
			return HumanParticipant{
				id:               record.Human.ID,
				name:             record.Human.Name,
				contacts:         cloneContacts(record.Human.Contacts),
				preferredChannel: record.Human.PreferredChannel,
			}, nil
		}
		return nil, fmt.Errorf("unknown participant kind %q", record.Kind)
	}
}

func groupNames(groups map[string][]protocol.Participant) map[string][]string {
	if len(groups) == 0 {
		return nil
	}
	out := make(map[string][]string, len(groups))
	for name, members := range groups {
		if len(members) == 0 {
			out[name] = nil
			continue
		}
		names := make([]string, 0, len(members))
		for _, member := range members {
			names = append(names, participantKey(member))
		}
		out[name] = names
	}
	return out
}

func groupsFromNames(participants []protocol.Participant, groups map[string][]string) (map[string][]protocol.Participant, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	index := make(map[string]protocol.Participant, len(participants))
	for _, participant := range participants {
		key := participantKey(participant)
		if key == "" {
			return nil, fmt.Errorf("participant name required")
		}
		if _, ok := index[key]; ok {
			return nil, fmt.Errorf("duplicate participant identifier %q", key)
		}
		index[key] = participant
	}

	out := make(map[string][]protocol.Participant, len(groups))
	for name, memberNames := range groups {
		if name == "" {
			return nil, fmt.Errorf("group name required")
		}
		members := make([]protocol.Participant, 0, len(memberNames))
		for _, memberName := range memberNames {
			participant, ok := index[memberName]
			if !ok {
				return nil, fmt.Errorf("group %q member %s not in participants", name, memberName)
			}
			members = append(members, participant)
		}
		out[name] = members
	}
	return out, nil
}

func (e *Engine) persistSession(ctx context.Context, sess *Session) error {
	if e.sessionStore == nil {
		return nil
	}
	record, err := sessionRecordFromSession(sess)
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return e.sessionStore.Save(ctx, record)
}

func (e *Engine) persistSessionState(ctx context.Context, sess *Session) error {
	if e.sessionStore == nil {
		return nil
	}
	stateStore, ok := e.sessionStore.(SessionStateStore)
	if !ok {
		return nil
	}
	if sess == nil {
		return fmt.Errorf("session required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	state := sess.State()
	if len(state.Pending) == 0 {
		if err := stateStore.DeleteState(ctx, sess.id); err != nil && err != ErrSessionNotFound {
			return err
		}
		return nil
	}
	return stateStore.SaveState(ctx, state)
}

func sessionRecordFromSession(sess *Session) (SessionRecord, error) {
	if sess == nil {
		return SessionRecord{}, fmt.Errorf("session required")
	}
	record := SessionRecord{
		ID:           sess.id,
		Name:         sess.name,
		Tags:         append([]string(nil), sess.tags...),
		Metadata:     cloneMetadata(sess.metadata),
		ProtocolID:   "",
		Participants: nil,
		Groups:       groupNames(sess.groups),
	}
	record.DefaultTools = append([]string(nil), sess.defaultToolIDs...)
	if sess.toolPolicySet {
		record.ToolPolicy = sess.toolPolicy
	}
	record.Toolkits = append([]string(nil), sess.toolkitIDs...)
	if sess.participation != nil {
		record.Participation = string(sess.participation.Mode())
	}
	if sess.timeoutPolicy != nil {
		policy := *sess.timeoutPolicy
		record.TimeoutPolicy = &policy
	}
	participants, err := participantRecordsFromParticipants(sess.participants)
	if err != nil {
		return SessionRecord{}, err
	}
	record.Participants = participants
	if sess.facilitator != nil {
		facilitator := cloneAgent(*sess.facilitator)
		record.Facilitator = &facilitator
	}
	if sess.protocol != nil {
		record.ProtocolID = sess.protocol.ID()
		if cp, ok := sess.protocol.(protocol.ConfigProvider); ok {
			record.ProtocolConfig = cloneMetadata(cp.Config())
		}
	}
	if record.ProtocolID == "" {
		return SessionRecord{}, fmt.Errorf("protocol id required")
	}
	return record, nil
}

func (e *Engine) sessionFromRecord(ctx context.Context, record SessionRecord) (*Session, error) {
	if record.ProtocolID == "" {
		return nil, fmt.Errorf("protocol id required")
	}
	factory, ok := e.protocols.Get(record.ProtocolID)
	if !ok {
		return nil, fmt.Errorf("protocol not found: %s", record.ProtocolID)
	}
	proto := factory(record.ProtocolConfig)

	participants, err := participantsFromRecords(record.Participants)
	if err != nil {
		return nil, err
	}
	groups, err := groupsFromNames(participants, record.Groups)
	if err != nil {
		return nil, err
	}

	var facilitator *agent.Agent
	if record.Facilitator != nil {
		f := cloneAgent(*record.Facilitator)
		facilitator = &f
	}

	participation, err := participationFromRecord(record.Participation)
	if err != nil {
		return nil, err
	}

	sess, err := e.NewSession(ctx, SessionConfig{
		ID:            record.ID,
		Name:          record.Name,
		Tags:          append([]string(nil), record.Tags...),
		Metadata:      cloneMetadata(record.Metadata),
		Protocol:      proto,
		Participants:  participants,
		Facilitator:   facilitator,
		Groups:        groups,
		Participation: participation,
		TimeoutPolicy: record.TimeoutPolicy,
		DefaultTools:  append([]string(nil), record.DefaultTools...),
		ToolPolicy:    record.ToolPolicy,
		Toolkits:      append([]string(nil), record.Toolkits...),
	})
	if err != nil {
		return nil, err
	}
	if state, ok := record.Metadata[protocolStateMetadataKey].(map[string]any); ok {
		if stateful, ok := sess.protocol.(protocol.StatefulProtocol); ok {
			_ = stateful.SetState(state)
		}
	}
	if err := e.restoreSessionState(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (e *Engine) restoreSessionState(ctx context.Context, sess *Session) error {
	if e == nil || sess == nil || e.sessionStore == nil {
		return nil
	}
	stateStore, ok := e.sessionStore.(SessionStateStore)
	if !ok {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	state, err := stateStore.LoadState(ctx, sess.id)
	if err != nil {
		if err == ErrSessionNotFound {
			return nil
		}
		return err
	}
	restored := sess.restorePending(state.Pending)
	if len(restored) == 0 {
		if len(state.PendingTools) == 0 {
			return nil
		}
	}
	for _, req := range restored {
		if e.requestRegistry != nil {
			if err := e.requestRegistry.Register(ctx, req.RequestID, sess.id); err != nil {
				_ = sess.EmitWithContext(ctx, event.New(event.HumanRequestRegistryFailed, sess.id, map[string]any{
					"request_id": req.RequestID,
					"error":      err.Error(),
				}))
			}
		}
		if e.timeoutScheduler != nil && !req.TimeoutAt.IsZero() {
			if err := e.timeoutScheduler.ScheduleTimeout(ctx, TimeoutRequest{
				SessionID: sess.id,
				RequestID: req.RequestID,
				TimeoutAt: req.TimeoutAt,
			}); err != nil {
				_ = sess.EmitWithContext(ctx, event.New(event.HumanRequestTimeoutScheduleFailed, sess.id, map[string]any{
					"request_id": req.RequestID,
					"error":      err.Error(),
				}))
			}
		}
	}
	if len(state.PendingTools) > 0 {
		_ = sess.restorePendingTools(state.PendingTools)
	}
	return nil
}

func participationFromRecord(value string) (ParticipationMode, error) {
	if value == "" || value == string(ParticipationTurnBased) {
		return TurnBased(), nil
	}
	return nil, fmt.Errorf("unsupported participation mode: %s", value)
}
