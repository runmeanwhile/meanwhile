package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
)

func validateGroups(participants []protocol.Participant, groups map[string][]protocol.Participant) error {
	if len(groups) == 0 {
		// Still validate unique participant names.
		return validateUniqueParticipants(participants)
	}

	participantNames := make(map[string]struct{}, len(participants))
	for _, participant := range participants {
		key := participantKey(participant)
		if key == "" {
			return fmt.Errorf("participant name required")
		}
		if _, ok := participantNames[key]; ok {
			return fmt.Errorf("duplicate participant identifier %q", key)
		}
		participantNames[key] = struct{}{}
	}

	for name, members := range groups {
		if name == "" {
			return fmt.Errorf("group name required")
		}
		if len(members) == 0 {
			return fmt.Errorf("group %q has no members", name)
		}
		for _, member := range members {
			key := participantKey(member)
			if key == "" {
				return fmt.Errorf("group %q member missing name", name)
			}
			if _, ok := participantNames[key]; !ok {
				return fmt.Errorf("group %q member %s not in participants", name, key)
			}
		}
	}

	return nil
}

func validateUniqueParticipants(participants []protocol.Participant) error {
	seen := make(map[string]struct{}, len(participants))
	for _, participant := range participants {
		key := participantKey(participant)
		if key == "" {
			return fmt.Errorf("participant name required")
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate participant identifier %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func cloneGroups(groups map[string][]protocol.Participant) map[string][]protocol.Participant {
	if len(groups) == 0 {
		return nil
	}
	out := make(map[string][]protocol.Participant, len(groups))
	for name, members := range groups {
		out[name] = append([]protocol.Participant(nil), members...)
	}
	return out
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func validateParticipants(participants []protocol.Participant) error {
	for _, participant := range participants {
		if participant == nil {
			return fmt.Errorf("participant required")
		}
		if strings.TrimSpace(participant.DisplayName()) == "" {
			return fmt.Errorf("participant name required")
		}
		if participant.IsAgent() {
			if ag, ok := participant.Agent(); ok {
				if err := ag.Validate(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func participantKey(participant protocol.Participant) string {
	if participant == nil {
		return ""
	}
	if id := strings.TrimSpace(participant.Identifier()); id != "" {
		return id
	}
	return strings.TrimSpace(participant.DisplayName())
}

func participantByKey(participants []protocol.Participant, key string) (protocol.Participant, bool) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return nil, false
	}
	for _, participant := range participants {
		if participantKey(participant) == trimmed {
			return participant, true
		}
	}
	return nil, false
}

func (s *Session) clearPendingRequest(ctx context.Context, requestID string) {
	if s == nil || requestID == "" {
		return
	}
	remaining, removed := s.removePending(requestID)
	if !removed {
		return
	}
	if remaining == 0 {
		_ = s.EmitWithContext(ctx, event.New(event.SessionResumed, s.id, s.State()))
	}
	if s.engine != nil {
		_ = s.engine.persistSessionState(ctx, s)
	}
}

func (s *Session) scheduleTimeout(ctx context.Context, request protocol.InputRequest) error {
	if s == nil || s.engine == nil || s.engine.timeoutScheduler == nil {
		return nil
	}
	if request.TimeoutAt.IsZero() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.engine.timeoutScheduler.ScheduleTimeout(ctx, TimeoutRequest{
		SessionID: s.id,
		RequestID: request.RequestID,
		TimeoutAt: request.TimeoutAt,
	})
}

func (s *Session) registerConfiguredToolkits(ctx context.Context, toolkitIDs []string) error {
	if len(toolkitIDs) == 0 {
		return nil
	}
	if s.engine == nil || s.engine.toolkits == nil {
		return fmt.Errorf("toolkit registry required")
	}
	for _, id := range toolkitIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		tk, ok := s.engine.toolkits.Get(id)
		if !ok {
			return fmt.Errorf("toolkit not found: %s", id)
		}
		if err := s.RegisterToolkit(ctx, tk); err != nil {
			return err
		}
	}
	return nil
}
