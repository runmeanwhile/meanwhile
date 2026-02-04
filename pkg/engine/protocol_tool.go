package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/message"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// ProtocolToolOption configures a protocol-as-tool wrapper.
type ProtocolToolOption func(*protocolTool)

// WithToolName sets the tool name (ID).
func WithToolName(name string) ProtocolToolOption {
	return func(pt *protocolTool) {
		pt.name = name
	}
}

// WithToolDescription sets the tool description.
func WithToolDescription(desc string) ProtocolToolOption {
	return func(pt *protocolTool) {
		pt.description = desc
	}
}

// WithToolParticipants sets the participants for the nested session.
func WithToolParticipants(participants ...protocol.Participant) ProtocolToolOption {
	return func(pt *protocolTool) {
		pt.participants = participants
	}
}

// WithToolFacilitator sets the facilitator for the nested session.
func WithToolFacilitator(facilitator agent.Agent) ProtocolToolOption {
	return func(pt *protocolTool) {
		pt.facilitator = &facilitator
	}
}

// WithToolTags sets tags for the nested session.
func WithToolTags(tags ...string) ProtocolToolOption {
	return func(pt *protocolTool) {
		pt.tags = tags
	}
}

// AsTool wraps any protocol as a callable tool.
// The tool creates a nested session with the protocol when invoked.
func (e *Engine) AsTool(proto protocol.Protocol, opts ...ProtocolToolOption) tool.Tool {
	pt := &protocolTool{
		engine:      e,
		protocol:    proto,
		name:        "protocol_tool",
		description: "Delegates to a nested session",
	}
	for _, opt := range opts {
		opt(pt)
	}
	return pt
}

// protocolTool implements tool.Tool by wrapping a protocol.
type protocolTool struct {
	engine       *Engine
	protocol     protocol.Protocol
	name         string
	description  string
	participants []protocol.Participant
	facilitator  *agent.Agent
	tags         []string
}

// ID returns the tool name.
func (pt *protocolTool) ID() string {
	return pt.name
}

// Schema returns the tool schema.
func (pt *protocolTool) Schema() tool.Schema {
	schemaJSON := fmt.Sprintf(`{
		"type": "object",
		"description": %q,
		"properties": {
			"task": {
				"type": "string",
				"description": "The task or question for the nested session"
			},
			"context": {
				"type": "string",
				"description": "Additional context or background information"
			}
		},
		"required": ["task"]
	}`, pt.description)

	return tool.Schema{
		JSONSchema: []byte(schemaJSON),
	}
}

// Run executes the tool by creating a nested session.
func (pt *protocolTool) Run(ctx context.Context, call tool.Call, _ tool.Emitter) (tool.Result, error) {
	// Parse arguments
	var args struct {
		Task    string `json:"task"`
		Context string `json:"context,omitempty"`
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return tool.ErrorResult(call, fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	// Build user message
	userMsg := args.Task
	if args.Context != "" {
		userMsg = fmt.Sprintf("%s\n\nContext: %s", args.Task, args.Context)
	}

	// Create nested session
	sessionName := fmt.Sprintf("Tool: %s", pt.name)
	builder := pt.engine.Session(sessionName).Protocol(pt.protocol)
	if len(pt.tags) > 0 {
		builder.Tags(pt.tags...)
	}
	if len(pt.participants) > 0 {
		builder.Participants(pt.participants...)
	}
	if pt.facilitator != nil {
		builder.Facilitator(*pt.facilitator)
	}

	sess, err := builder.Build(ctx)
	if err != nil {
		return tool.ErrorResult(call, fmt.Sprintf("failed to create nested session: %v", err)), nil
	}
	defer func() {
		_ = pt.engine.CloseSession(context.Background(), sess.ID())
	}()

	// Run the nested session
	result, err := pt.engine.Run(ctx, sess.ID(), message.User(userMsg))
	if err != nil {
		return tool.ErrorResult(call, fmt.Sprintf("nested session failed: %v", err)), nil
	}

	// Return the final result
	return tool.TextResult(call, result.Final), nil
}
