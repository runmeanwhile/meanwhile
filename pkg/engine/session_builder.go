package engine

import (
	"context"
	"fmt"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/mcp"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
	"github.com/darkostanimirovic/meanwhile/pkg/toolkit"
)

// SessionBuilder provides a fluent API for creating sessions.
// Use eng.Session(name) to create a builder, then chain configuration
// methods before calling Build() to create the session.
//
// Example:
//
//	sess, _ := eng.Session("Planning Meeting").
//	    Participant(alice).
//	    Participant(bob).
//	    Protocol(protocol.Consensus()).
//	    Build(ctx)
//
// For ephemeral sessions that run immediately, use Run():
//
//	result, _ := eng.Session("Quick Task").
//	    Participant(alice).
//	    Protocol(protocol.Solo()).
//	    Run(ctx, message.User("hello"))
//
// Defaults to solo protocol if none specified.
// Participants are auto-extracted from protocol if not set explicitly.
// Meanwhile... sessions feel like meetings, not infrastructure.
type SessionBuilder struct {
	engine        *Engine
	name          string
	tags          []string
	metadata      map[string]any
	protocol      protocol.Protocol
	participants  []protocol.Participant
	facilitator   *agent.Agent
	groups        map[string][]protocol.Participant
	participation ParticipationMode
	timeoutPolicy *TimeoutPolicy
	tools         []any
	toolkits      []any
	toolPolicy    *tool.Policy
}

// Session creates a new session builder.
func (e *Engine) Session(name string) *SessionBuilder {
	return &SessionBuilder{
		engine:   e,
		name:     name,
		metadata: make(map[string]any),
		groups:   make(map[string][]protocol.Participant),
	}
}

// Tags adds tags to the session.
func (sb *SessionBuilder) Tags(tags ...string) *SessionBuilder {
	sb.tags = append(sb.tags, tags...)
	return sb
}

// Metadata adds metadata to the session.
func (sb *SessionBuilder) Metadata(key string, value any) *SessionBuilder {
	sb.metadata[key] = value
	return sb
}

// Protocol sets the session protocol.
func (sb *SessionBuilder) Protocol(proto protocol.Protocol) *SessionBuilder {
	sb.protocol = proto
	return sb
}

// Participants sets the session participants.
func (sb *SessionBuilder) Participants(participants ...protocol.Participant) *SessionBuilder {
	sb.participants = participants
	return sb
}

// Participant adds a single participant (convenience method).
func (sb *SessionBuilder) Participant(participant protocol.Participant) *SessionBuilder {
	sb.participants = append(sb.participants, participant)
	return sb
}

// Facilitator sets the session facilitator.
func (sb *SessionBuilder) Facilitator(facilitator agent.Agent) *SessionBuilder {
	sb.facilitator = &facilitator
	return sb
}

// Groups sets participant groups for protocols that support them.
func (sb *SessionBuilder) Groups(groups map[string][]protocol.Participant) *SessionBuilder {
	sb.groups = groups
	return sb
}

// Group adds a single group (convenience method).
func (sb *SessionBuilder) Group(name string, members ...protocol.Participant) *SessionBuilder {
	sb.groups[name] = members
	return sb
}

// Participation sets the session participation mode.
func (sb *SessionBuilder) Participation(mode ParticipationMode) *SessionBuilder {
	sb.participation = mode
	return sb
}

// TimeoutPolicy sets the default timeout policy for human requests.
func (sb *SessionBuilder) TimeoutPolicy(policy TimeoutPolicy) *SessionBuilder {
	sb.timeoutPolicy = &policy
	return sb
}

// Tools registers tools for this session and adds them to default tool IDs.
// Accepts tool IDs (string) or tool instances.
func (sb *SessionBuilder) Tools(tools ...any) *SessionBuilder {
	sb.tools = append(sb.tools, tools...)
	return sb
}

// Toolkits registers toolkits by instance or ID.
func (sb *SessionBuilder) Toolkits(kits ...any) *SessionBuilder {
	sb.toolkits = append(sb.toolkits, kits...)
	return sb
}

// ToolPolicy sets the tool policy for the session.
func (sb *SessionBuilder) ToolPolicy(policy tool.Policy) *SessionBuilder {
	sb.toolPolicy = &policy
	return sb
}

// MCP attaches an MCP server as a toolkit for this session.
func (sb *SessionBuilder) MCP(server *mcp.Server, toolkitID ...string) *SessionBuilder {
	if server == nil {
		return sb
	}
	id := ""
	if len(toolkitID) > 0 {
		id = toolkitID[0]
	}
	sb.toolkits = append(sb.toolkits, mcp.NewToolkit(id, server))
	return sb
}

// Build creates and initializes the session.
// If participants are not set, they are extracted from protocol.Participants().
func (sb *SessionBuilder) Build(ctx context.Context) (*Session, error) {
	// Default to solo protocol if none specified
	if sb.protocol == nil {
		sb.protocol = protocol.Solo()
	}

	// Auto-extract participants from protocol if not explicitly set
	participants := sb.participants
	if len(participants) == 0 && sb.protocol != nil {
		if protoParticipants := sb.protocol.Participants(); len(protoParticipants) > 0 {
			participants = protoParticipants
		}
	}

	// Validate groups against participants
	if err := validateGroups(participants, sb.groups); err != nil {
		return nil, err
	}

	sess, err := sb.engine.NewSession(ctx, SessionConfig{
		Name:          sb.name,
		Tags:          sb.tags,
		Metadata:      sb.metadata,
		Protocol:      sb.protocol,
		Participants:  participants,
		Facilitator:   sb.facilitator,
		Groups:        sb.groups,
		Participation: sb.participation,
		TimeoutPolicy: sb.timeoutPolicy,
		DefaultTools:  extractToolIDs(sb.tools),
		ToolPolicy:    resolveToolPolicy(sb.toolPolicy),
		Toolkits:      extractToolkitIDs(sb.toolkits),
	})
	if err != nil {
		return nil, err
	}

	if err := sb.registerSessionTools(sess, sb.tools); err != nil {
		return nil, err
	}

	if err := sb.registerSessionToolkits(ctx, sess, sb.toolkits); err != nil {
		return nil, err
	}

	return sess, nil
}

// Start creates and initializes the session.
// Deprecated: Use Build() instead for clarity. Start() remains for backwards compatibility.
func (sb *SessionBuilder) Start(ctx context.Context) (*Session, error) {
	return sb.Build(ctx)
}

// Run creates an ephemeral session, runs it with the given message, and closes it.
// Useful for one-off protocol executions without managing session lifecycle.
func (sb *SessionBuilder) Run(ctx context.Context, msg agent.Message) (*RunResult, error) {
	sess, err := sb.Build(ctx)
	if err != nil {
		return nil, err
	}
	defer sess.engine.CloseSession(context.Background(), sess.ID())
	return sess.Run(ctx, msg)
}

func (sb *SessionBuilder) registerSessionTools(sess *Session, tools []any) error {
	for _, t := range tools {
		if _, ok := t.(string); ok {
			continue
		}
		if toolInst, ok := t.(tool.Tool); ok {
			if err := sess.RegisterTool(toolInst); err != nil {
				return err
			}
			continue
		}
		return fmt.Errorf("invalid tool: %T", t)
	}
	return nil
}

func (sb *SessionBuilder) registerSessionToolkits(ctx context.Context, sess *Session, kits []any) error {
	for _, entry := range kits {
		switch value := entry.(type) {
		case toolkit.Toolkit:
			if err := sess.RegisterToolkit(ctx, value); err != nil {
				return err
			}
		case string:
			id := value
			if id == "" {
				continue
			}
			if sb.engine == nil || sb.engine.toolkits == nil {
				return fmt.Errorf("toolkit registry required")
			}
			tk, ok := sb.engine.toolkits.Get(id)
			if !ok {
				return fmt.Errorf("toolkit not found: %s", id)
			}
			if err := sess.RegisterToolkit(ctx, tk); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid toolkit: %T", entry)
		}
	}
	return nil
}

func extractToolIDs(tools []any) []string {
	if len(tools) == 0 {
		return nil
	}
	ids := make([]string, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		switch value := t.(type) {
		case string:
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			ids = append(ids, value)
		case interface{ ID() string }:
			id := value.ID()
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func extractToolkitIDs(kits []any) []string {
	if len(kits) == 0 {
		return nil
	}
	ids := make([]string, 0, len(kits))
	seen := make(map[string]struct{}, len(kits))
	for _, entry := range kits {
		switch value := entry.(type) {
		case string:
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			ids = append(ids, value)
		case toolkit.Toolkit:
			id := value.ID()
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func resolveToolPolicy(policy *tool.Policy) tool.Policy {
	if policy == nil {
		return tool.Policy{}
	}
	return *policy
}
