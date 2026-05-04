package groq

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

const doneToken = "[DONE]"

type stream struct {
	decoder *sseDecoder
	body    io.Closer
	text    string
	calls   map[int]*toolCallBuilder
}

func newStream(body io.ReadCloser) provider.Stream {
	return &stream{decoder: newSSEDecoder(body), body: body, calls: make(map[int]*toolCallBuilder)}
}

func (s *stream) Recv() (provider.Event, error) {
	data, err := s.decoder.Next()
	if err != nil {
		return provider.Event{}, err
	}
	if len(data) == 0 || string(data) == doneToken {
		return provider.Event{}, io.EOF
	}
	return s.decodeChunk(data)
}

func (s *stream) Close() error { return s.body.Close() }

type chunk struct {
	Choices []struct {
		Delta struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"delta"`
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type toolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type toolCallBuilder struct {
	ID        string
	ToolID    string
	Arguments string
}

func (s *stream) decodeChunk(data []byte) (provider.Event, error) {
	var ch chunk
	if err := json.Unmarshal(data, &ch); err != nil {
		return provider.Event{}, fmt.Errorf("decode groq chunk: %w", err)
	}
	if ch.Error != nil {
		return provider.Event{Type: provider.EventError, Err: errors.New(ch.Error.Message)}, nil
	}
	if len(ch.Choices) == 0 {
		return provider.Event{Type: provider.EventRaw, Raw: append([]byte(nil), data...)}, nil
	}
	choice := ch.Choices[0]
	if choice.Delta.Content != "" {
		s.text += choice.Delta.Content
		return provider.Event{Type: provider.EventMessageDelta, Delta: choice.Delta.Content}, nil
	}
	s.ingestToolCalls(choice.Delta.ToolCalls)
	if calls := toolCalls(choice.Message.ToolCalls); len(calls) > 0 {
		return provider.Event{Type: provider.EventToolCall, ToolCalls: calls}, nil
	}
	if choice.Message.Content != "" {
		return provider.Event{
			Type: provider.EventMessageCompleted,
			Message: modelruntime.Message{
				Role:  modelruntime.RoleAssistant,
				Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: choice.Message.Content}},
			},
		}, nil
	}
	if choice.FinishReason != "" {
		if calls := s.completedToolCalls(); len(calls) > 0 {
			return provider.Event{Type: provider.EventToolCall, ToolCalls: calls}, nil
		}
		if s.text != "" {
			return provider.Event{
				Type: provider.EventMessageCompleted,
				Message: modelruntime.Message{
					Role:  modelruntime.RoleAssistant,
					Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: s.text}},
				},
			}, nil
		}
		return provider.Event{Type: provider.EventMessageCompleted}, nil
	}
	return provider.Event{Type: provider.EventRaw, Raw: append([]byte(nil), data...)}, nil
}

func (s *stream) ingestToolCalls(values []toolCall) {
	for _, value := range values {
		index := value.Index
		builder := s.calls[index]
		if builder == nil {
			builder = &toolCallBuilder{}
			s.calls[index] = builder
		}
		if value.ID != "" {
			builder.ID = value.ID
		}
		if value.Function.Name != "" {
			builder.ToolID = value.Function.Name
		}
		builder.Arguments += value.Function.Arguments
	}
}

func (s *stream) completedToolCalls() []provider.ToolCall {
	if len(s.calls) == 0 {
		return nil
	}
	out := make([]provider.ToolCall, 0, len(s.calls))
	for i := 0; i < len(s.calls); i++ {
		builder := s.calls[i]
		if builder == nil || builder.ID == "" || builder.ToolID == "" {
			continue
		}
		args := builder.Arguments
		if args == "" {
			args = "{}"
		}
		out = append(out, provider.ToolCall{
			ID:        builder.ID,
			ToolID:    builder.ToolID,
			Arguments: json.RawMessage(args),
		})
	}
	return out
}

func toolCalls(values []toolCall) []provider.ToolCall {
	out := make([]provider.ToolCall, 0, len(values))
	for _, value := range values {
		if value.ID == "" || value.Function.Name == "" {
			continue
		}
		out = append(out, provider.ToolCall{
			ID:        value.ID,
			ToolID:    value.Function.Name,
			Arguments: json.RawMessage(value.Function.Arguments),
		})
	}
	return out
}
