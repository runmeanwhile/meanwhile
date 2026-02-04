package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
	"github.com/darkostanimirovic/meanwhile/pkg/event"
	"github.com/darkostanimirovic/meanwhile/pkg/protocol"
	"github.com/darkostanimirovic/meanwhile/pkg/tool"
)

const (
	// AskHumanToolID is the default tool ID for ask_human.
	AskHumanToolID = "ask_human"

	askHumanStatusPending = "pending"
)

// AskHumanInput captures the input for ask_human.
type AskHumanInput struct {
	Question           string   `json:"question" description:"Specific question for a human participant"`
	Context            string   `json:"context,omitempty" description:"Why this question matters"`
	Participant        string   `json:"participant,omitempty" description:"Which human to ask (ID or display name)"`
	Timeout            string   `json:"timeout,omitempty" description:"How long to wait before timing out (e.g. 6h)"`
	Required           *bool    `json:"required,omitempty" description:"Block the session until a response arrives"`
	Urgency            string   `json:"urgency,omitempty" description:"Urgency level for the request"`
	SuggestedResponses []string `json:"suggested_responses,omitempty" description:"Suggested responses for quick replies"`
}

// AskHumanOutput reports the request state.
type AskHumanOutput struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Response  string `json:"response,omitempty"`
}

// AskHumanToolOption configures the ask_human tool.
type AskHumanToolOption func(*askHumanToolConfig)

type askHumanToolConfig struct {
	id          string
	description string
}

// WithAskHumanToolName sets the tool ID.
func WithAskHumanToolName(id string) AskHumanToolOption {
	return func(cfg *askHumanToolConfig) {
		cfg.id = id
	}
}

// WithAskHumanToolDescription sets the tool description.
func WithAskHumanToolDescription(desc string) AskHumanToolOption {
	return func(cfg *askHumanToolConfig) {
		cfg.description = desc
	}
}

// AskHumanTool returns a session-scoped ask_human tool.
func (s *Session) AskHumanTool(opts ...AskHumanToolOption) (tool.Tool, error) {
	if s == nil {
		return nil, fmt.Errorf("session required")
	}
	cfg := askHumanToolConfig{
		id:          AskHumanToolID,
		description: "Request input from a human participant",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if strings.TrimSpace(cfg.id) == "" {
		return nil, fmt.Errorf("ask_human tool id required")
	}
	schema, err := askHumanSchema()
	if err != nil {
		return nil, err
	}
	return &askHumanTool{
		session:     s,
		id:          cfg.id,
		description: cfg.description,
		schema:      schema,
	}, nil
}

// EnableAskHumanTool registers ask_human as a session tool and adds it to default tools.
func (s *Session) EnableAskHumanTool(opts ...AskHumanToolOption) (tool.Tool, error) {
	askTool, err := s.AskHumanTool(opts...)
	if err != nil {
		return nil, err
	}
	if err := s.RegisterTool(askTool); err != nil {
		return nil, err
	}
	s.AddDefaultTools(askTool.ID())
	return askTool, nil
}

type askHumanTool struct {
	session     *Session
	id          string
	description string
	schema      tool.Schema
}

func (t *askHumanTool) ID() string { return t.id }

func (t *askHumanTool) Description() string { return t.description }

func (t *askHumanTool) Schema() tool.Schema { return t.schema }

func (t *askHumanTool) Run(ctx context.Context, call tool.Call, _ tool.Emitter) (tool.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t.session == nil {
		return tool.ErrorResult(call, "session required"), nil
	}

	var input AskHumanInput
	if err := json.Unmarshal(call.Arguments, &input); err != nil {
		return tool.ErrorResult(call, fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	question := strings.TrimSpace(input.Question)
	if question == "" {
		return tool.ErrorResult(call, "question required"), nil
	}

	participant, err := t.resolveParticipant(input.Participant)
	if err != nil {
		return tool.ErrorResult(call, err.Error()), nil
	}

	timeout, err := parseAskHumanTimeout(input.Timeout)
	if err != nil {
		return tool.ErrorResult(call, err.Error()), nil
	}

	required := true
	if input.Required != nil {
		required = *input.Required
	}

	contextText := strings.TrimSpace(input.Context)
	urgency := strings.TrimSpace(input.Urgency)
	suggestions := normalizeSuggestedResponses(input.SuggestedResponses)
	turnContext := formatAskHumanContext(question, contextText, urgency, suggestions)

	if required {
		resume := func(ctx context.Context, resp agent.Message) error {
			return t.session.protocol.OnMessage(ctx, t.session, resp)
		}
		inputOpts := []protocol.InputOption{}
		if timeout > 0 {
			inputOpts = append(inputOpts, protocol.WithInputTimeout(timeout))
		}

		err = t.session.AwaitInput(ctx, participant, turnContext, resume, inputOpts...)
		if err != nil {
			var awaiting *protocol.AwaitingInputError
			if !errors.As(err, &awaiting) {
				return tool.ErrorResult(call, err.Error()), nil
			}
			request := awaiting.Request
			humanRequest := buildHumanRequest(t.session, call, request, question, contextText, urgency, required, suggestions)
			_ = t.session.EmitWithContext(ctx, event.New(event.HumanRequestCreated, t.session.id, humanRequest))
			output := AskHumanOutput{RequestID: request.RequestID, Status: askHumanStatusPending}
			return tool.JSONResult(call, output), err
		}
	}

	request := buildOptionalRequest(participant, turnContext, timeout)
	humanRequest := buildHumanRequest(t.session, call, request, question, contextText, urgency, required, suggestions)
	_ = t.session.EmitWithContext(ctx, event.New(event.HumanRequestCreated, t.session.id, humanRequest))
	output := AskHumanOutput{RequestID: request.RequestID, Status: askHumanStatusPending}
	return tool.JSONResult(call, output), nil
}

func (t *askHumanTool) resolveParticipant(key string) (protocol.Participant, error) {
	if t.session == nil {
		return nil, fmt.Errorf("session required")
	}
	if trimmed := strings.TrimSpace(key); trimmed != "" {
		participant, ok := participantByKey(t.session.participants, trimmed)
		if !ok {
			return nil, fmt.Errorf("human participant not found: %s", trimmed)
		}
		if !participant.IsHuman() {
			return nil, fmt.Errorf("participant must be human: %s", trimmed)
		}
		return participant, nil
	}

	humans := t.session.HumanParticipants()
	if len(humans) == 0 {
		return nil, fmt.Errorf("no human participants in session")
	}
	if len(humans) == 1 {
		return humans[0], nil
	}
	keys := humanParticipantKeys(humans)
	return nil, fmt.Errorf("participant required (available: %s)", strings.Join(keys, ", "))
}

func askHumanSchema() (tool.Schema, error) {
	raw, err := tool.SchemaForStruct(reflect.TypeOf(AskHumanInput{}))
	if err != nil {
		return tool.Schema{}, fmt.Errorf("ask_human schema: %w", err)
	}
	return tool.Schema{JSONSchema: raw}, nil
}

func parseAskHumanTimeout(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	timeout, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout: %w", err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("timeout must be positive")
	}
	return timeout, nil
}

func formatAskHumanContext(question, context, urgency string, suggestions []string) string {
	parts := []string{question}
	if context != "" {
		parts = append(parts, context)
	}
	if urgency != "" {
		parts = append(parts, fmt.Sprintf("Urgency: %s", urgency))
	}
	if len(suggestions) > 0 {
		parts = append(parts, formatSuggestedResponses(suggestions))
	}
	return strings.Join(parts, "\n\n")
}

func formatSuggestedResponses(suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Suggested responses:")
	for _, response := range suggestions {
		sb.WriteString("\n- ")
		sb.WriteString(response)
	}
	return sb.String()
}

func normalizeSuggestedResponses(responses []string) []string {
	if len(responses) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(responses))
	for _, response := range responses {
		response = strings.TrimSpace(response)
		if response == "" {
			continue
		}
		trimmed = append(trimmed, response)
	}
	if len(trimmed) == 0 {
		return nil
	}
	return trimmed
}

func humanParticipantKeys(humans []protocol.Participant) []string {
	keys := make([]string, 0, len(humans))
	for _, participant := range humans {
		keys = append(keys, participantKey(participant))
	}
	return keys
}

func buildOptionalRequest(participant protocol.Participant, context string, timeout time.Duration) protocol.InputRequest {
	requestedAt := time.Now().UTC()
	timeoutAt := time.Time{}
	if timeout > 0 {
		timeoutAt = requestedAt.Add(timeout)
	}
	return protocol.InputRequest{
		RequestID:       event.NewID(),
		ParticipantID:   participantKey(participant),
		ParticipantName: participant.DisplayName(),
		Context:         context,
		RequestedAt:     requestedAt,
		TimeoutAt:       timeoutAt,
	}
}

func buildHumanRequest(sess *Session, call tool.Call, req protocol.InputRequest, question, context, urgency string, required bool, suggestions []string) HumanRequest {
	var sessionID string
	var protocolID string
	if sess != nil {
		sessionID = sess.id
		if sess.protocol != nil {
			protocolID = sess.protocol.ID()
		}
	}
	return HumanRequest{
		RequestID:          req.RequestID,
		SessionID:          sessionID,
		ProtocolID:         protocolID,
		ParticipantID:      req.ParticipantID,
		ParticipantName:    req.ParticipantName,
		Question:           question,
		Context:            context,
		Urgency:            urgency,
		Required:           required,
		SuggestedResponses: suggestions,
		RequestedAt:        req.RequestedAt,
		TimeoutAt:          req.TimeoutAt,
		ToolCallID:         call.ID,
		ToolID:             call.ToolID,
		AgentID:            call.AgentID,
	}
}
