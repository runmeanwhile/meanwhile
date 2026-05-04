package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

type stream struct {
	decoder *sseDecoder
	body    io.Closer
	blocks  map[int]*contentBlock
	text    string
}

type contentBlock struct {
	Type  string
	ID    string
	Name  string
	Input string
}

func newStream(body io.ReadCloser) provider.Stream {
	return &stream{decoder: newSSEDecoder(body), body: body, blocks: make(map[int]*contentBlock)}
}

func (s *stream) Recv() (provider.Event, error) {
	for {
		data, err := s.decoder.Next()
		if err != nil {
			return provider.Event{}, err
		}
		if len(data) == 0 {
			return provider.Event{}, io.EOF
		}
		ev, ok, err := s.decode(data)
		if err != nil || ok {
			return ev, err
		}
	}
}

func (s *stream) Close() error { return s.body.Close() }

type envelope struct {
	Type         string          `json:"type"`
	Index        int             `json:"index"`
	ContentBlock json.RawMessage `json:"content_block"`
	Delta        json.RawMessage `json:"delta"`
	Error        *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type blockStart struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Text  string          `json:"text"`
}

type blockDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	Thinking    string `json:"thinking"`
}

func (s *stream) decode(data []byte) (provider.Event, bool, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return provider.Event{}, true, fmt.Errorf("decode anthropic event: %w", err)
	}
	if env.Error != nil {
		return provider.Event{Type: provider.EventError, Err: errors.New(env.Error.Message)}, true, nil
	}
	switch env.Type {
	case "content_block_start":
		var start blockStart
		if err := json.Unmarshal(env.ContentBlock, &start); err != nil {
			return provider.Event{}, true, fmt.Errorf("decode content block: %w", err)
		}
		block := &contentBlock{Type: start.Type, ID: start.ID, Name: start.Name}
		if initial := strings.TrimSpace(string(start.Input)); initial != "" && initial != "{}" {
			block.Input = initial
		}
		s.blocks[env.Index] = block
		if start.Type == "text" && start.Text != "" {
			s.text += start.Text
			return provider.Event{Type: provider.EventMessageDelta, Delta: start.Text}, true, nil
		}
	case "content_block_delta":
		var delta blockDelta
		if err := json.Unmarshal(env.Delta, &delta); err != nil {
			return provider.Event{}, true, fmt.Errorf("decode content delta: %w", err)
		}
		block := s.blocks[env.Index]
		switch delta.Type {
		case "text_delta":
			s.text += delta.Text
			return provider.Event{Type: provider.EventMessageDelta, Delta: delta.Text}, true, nil
		case "input_json_delta":
			if block != nil {
				block.Input += delta.PartialJSON
			}
		case "thinking_delta":
			return provider.Event{Type: provider.EventReasoningDelta, Delta: delta.Thinking}, true, nil
		}
	case "content_block_stop":
		block := s.blocks[env.Index]
		if block != nil && block.Type == "tool_use" {
			args := block.Input
			if args == "" {
				args = "{}"
			}
			return provider.Event{Type: provider.EventToolCall, ToolCalls: []provider.ToolCall{{
				ID:        block.ID,
				ToolID:    block.Name,
				Arguments: json.RawMessage(args),
			}}}, true, nil
		}
	case "message_stop":
		if s.text != "" {
			return provider.Event{Type: provider.EventMessageCompleted, Message: modelruntime.Message{
				Role:  modelruntime.RoleAssistant,
				Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: s.text}},
			}}, true, nil
		}
		return provider.Event{}, true, io.EOF
	case "ping", "message_start", "message_delta":
		return provider.Event{Type: provider.EventRaw, Raw: append([]byte(nil), data...)}, true, nil
	}
	return provider.Event{}, false, nil
}
