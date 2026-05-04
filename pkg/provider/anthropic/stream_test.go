package anthropic

import (
	"io"
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

func TestStreamAccumulatesToolInputJSONDeltas(t *testing.T) {
	body := strings.NewReader(strings.Join([]string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"inspect_image","input":{}}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"question\":\""}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"look now\"}"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
	}, "\n"))

	stream := newStream(io.NopCloser(body))
	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if ev.Type != provider.EventToolCall {
		t.Fatalf("expected tool call, got %s", ev.Type)
	}
	if len(ev.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(ev.ToolCalls))
	}
	call := ev.ToolCalls[0]
	if call.ID != "toolu_1" || call.ToolID != "inspect_image" {
		t.Fatalf("unexpected tool call: %#v", call)
	}
	if string(call.Arguments) != `{"question":"look now"}` {
		t.Fatalf("unexpected arguments: %s", call.Arguments)
	}
}
