package engine

import (
	"strings"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

// HumanParticipant represents a real human participant in a session.
type HumanParticipant struct {
	id               string
	name             string
	contacts         map[string]string
	preferredChannel string
}

// Identifier returns the participant ID (optional).
func (h HumanParticipant) Identifier() string { return h.id }

// DisplayName returns the participant display name.
func (h HumanParticipant) DisplayName() string { return h.name }

// IsHuman reports that this participant is a human.
func (h HumanParticipant) IsHuman() bool { return true }

// IsAgent reports that this participant is not an agent.
func (h HumanParticipant) IsAgent() bool { return false }

// Agent always returns false for human participants.
func (h HumanParticipant) Agent() (agent.Agent, bool) {
	return agent.Agent{}, false
}

// Contact returns the contact value for a channel.
func (h HumanParticipant) Contact(channel string) (string, bool) {
	if channel == "" || len(h.contacts) == 0 {
		return "", false
	}
	value, ok := h.contacts[channel]
	return value, ok
}

// Contacts returns a copy of contact values keyed by channel.
func (h HumanParticipant) Contacts() map[string]string {
	if len(h.contacts) == 0 {
		return nil
	}
	out := make(map[string]string, len(h.contacts))
	for key, value := range h.contacts {
		out[key] = value
	}
	return out
}

// PreferredChannel returns the preferred contact channel.
func (h HumanParticipant) PreferredChannel() string { return h.preferredChannel }

// HumanBuilder constructs a human participant.
type HumanBuilder struct {
	id               string
	name             string
	contacts         map[string]string
	preferredChannel string
}

// Human creates a builder for a human participant.
func (e *Engine) Human(name string) *HumanBuilder {
	return &HumanBuilder{name: name}
}

// ID sets a stable identifier for the human participant.
func (b *HumanBuilder) ID(id string) *HumanBuilder {
	b.id = id
	return b
}

// ContactVia adds a contact channel for the human.
func (b *HumanBuilder) ContactVia(channel, contact string) *HumanBuilder {
	channel = normalizeChannel(channel)
	contact = strings.TrimSpace(contact)
	if channel == "" || contact == "" {
		return b
	}
	if b.contacts == nil {
		b.contacts = make(map[string]string)
	}
	b.contacts[channel] = contact
	return b
}

// PreferredChannel sets the preferred contact channel.
func (b *HumanBuilder) PreferredChannel(channel string) *HumanBuilder {
	b.preferredChannel = normalizeChannel(channel)
	return b
}

// Build creates the human participant.
func (b *HumanBuilder) Build() HumanParticipant {
	return HumanParticipant{
		id:               b.id,
		name:             b.name,
		contacts:         cloneContacts(b.contacts),
		preferredChannel: b.preferredChannel,
	}
}

func normalizeChannel(channel string) string {
	return strings.ToLower(strings.TrimSpace(channel))
}

func cloneContacts(contacts map[string]string) map[string]string {
	if len(contacts) == 0 {
		return nil
	}
	out := make(map[string]string, len(contacts))
	for key, value := range contacts {
		out[key] = value
	}
	return out
}
