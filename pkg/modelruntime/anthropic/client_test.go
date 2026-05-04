package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
)

func TestClientStream(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/messages" {
			t.Fatalf("request.URL.Path = %q, want /v1/messages", request.URL.Path)
		}
		if request.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key = %q, want test-key", request.Header.Get("x-api-key"))
		}

		var payload anthropicRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload.Model != "claude-test" {
			t.Fatalf("payload.Model = %q, want claude-test", payload.Model)
		}
		if len(payload.Messages) != 1 || len(payload.Messages[0].Content) != 1 {
			t.Fatalf("unexpected payload shape: %#v", payload)
		}
		if !strings.Contains(payload.Messages[0].Content[0].Text, "checkout.md") {
			t.Fatalf("payload text = %q, want grounded prompt", payload.Messages[0].Content[0].Text)
		}

		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": "Checkout requires 3DS per checkout.md."},
			},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	stream, err := client.Stream(context.Background(), modelruntime.Request{
		Model: "claude-test",
		Messages: []modelruntime.Message{{
			Role:  modelruntime.RoleUser,
			Parts: []modelruntime.Part{{Type: modelruntime.PartText, Text: "Grounded evidence: checkout.md"}},
		}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	event, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if event.Type != modelruntime.EventMessageCompleted {
		t.Fatalf("event.Type = %q, want message.completed", event.Type)
	}
	if event.Message.Text() != "Checkout requires 3DS per checkout.md." {
		t.Fatalf("event.Message.Text() = %q, want Anthropic output", event.Message.Text())
	}
	if _, err := stream.Recv(); err == nil || err != io.EOF {
		t.Fatalf("second Recv() error = %v, want io.EOF", err)
	}
}
