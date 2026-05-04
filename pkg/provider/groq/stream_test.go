package groq

import (
	"io"
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

func TestStreamAccumulatesFragmentedToolCallArguments(t *testing.T) {
	body := strings.NewReader(strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"inspect_image","arguments":""}}]},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"question\":\""}}]},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"look now\"}"}}]},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		``,
	}, "\n"))

	stream := newStream(io.NopCloser(body))
	for {
		ev, err := stream.Recv()
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		if ev.Type != provider.EventToolCall {
			continue
		}
		if len(ev.ToolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(ev.ToolCalls))
		}
		call := ev.ToolCalls[0]
		if call.ID != "call_1" || call.ToolID != "inspect_image" {
			t.Fatalf("unexpected tool call: %#v", call)
		}
		if string(call.Arguments) != `{"question":"look now"}` {
			t.Fatalf("unexpected arguments: %s", call.Arguments)
		}
		return
	}
}
