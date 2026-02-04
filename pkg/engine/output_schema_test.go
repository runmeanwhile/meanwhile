package engine

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/protocol"
	"github.com/runmeanwhile/meanwhile/pkg/provider"
)

type testOutputStruct struct {
	Title string   `json:"title" description:"The title of the plan"`
	Steps []string `json:"steps" description:"List of steps"`
}

func TestDeriveOutputSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  any
		wantErr bool
	}{
		{
			name:    "valid struct",
			schema:  testOutputStruct{},
			wantErr: false,
		},
		{
			name:    "valid pointer to struct",
			schema:  &testOutputStruct{},
			wantErr: false,
		},
		{
			name:    "nil schema",
			schema:  nil,
			wantErr: true,
		},
		{
			name:    "non-struct type",
			schema:  "string",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := deriveOutputSchema(tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("deriveOutputSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("Expected non-nil schema map")
				}
				if result["type"] != "object" {
					t.Errorf("Expected type 'object', got %v", result["type"])
				}
				if result["properties"] == nil {
					t.Error("Expected properties in schema")
				}
			}
		})
	}
}

func TestAgentOutputSchemaIntegration(t *testing.T) {
	var capturedParams map[string]any
	mockProvider := &mockProviderForOutputSchema{
		captureParams: func(params map[string]any) {
			capturedParams = params
		},
	}

	eng, err := New()
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	eng.ProviderRegistry().Register(mockProvider)

	testAgent := eng.Agent("Test").
		Prompt("Test").
		Model("mock://test").
		OutputSchema(testOutputStruct{}).
		Build()

	sess, err := eng.Session("test-output-schema").
		Participant(testAgent).
		Protocol(protocol.Solo()).
		Start(context.Background())
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), testAgent, protocol.RunRequest{
		Messages: []agent.Message{{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "test"}}}},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	if capturedParams == nil {
		t.Fatal("Expected params to be captured")
	}
	if capturedParams["response_format"] == nil {
		t.Fatal("Expected response_format in params")
	}

	responseFormat, ok := capturedParams["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("Expected response_format to be map[string]any, got %T", capturedParams["response_format"])
	}
	if responseFormat["type"] != "json_schema" {
		t.Errorf("Expected type 'json_schema', got %v", responseFormat["type"])
	}
}

func TestRunRequestOutputSchemaOverride(t *testing.T) {
	var capturedParams map[string]any
	mockProvider := &mockProviderForOutputSchema{
		captureParams: func(params map[string]any) {
			capturedParams = params
		},
	}

	eng, err := New()
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	eng.ProviderRegistry().Register(mockProvider)

	type AgentSchema struct {
		Field1 string `json:"field1"`
	}
	type RequestSchema struct {
		Field2 string `json:"field2"`
	}

	testAgent := eng.Agent("Test").
		Prompt("Test").
		Model("mock://test").
		OutputSchema(AgentSchema{}).
		Build()

	sess, err := eng.Session("test-override").
		Participant(testAgent).
		Protocol(protocol.Solo()).
		Start(context.Background())
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	_, err = sess.RunAgent(context.Background(), testAgent, protocol.RunRequest{
		Messages:     []agent.Message{{Role: agent.RoleUser, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "test"}}}},
		OutputSchema: RequestSchema{},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	responseFormat, ok := capturedParams["response_format"].(map[string]any)
	if !ok {
		t.Fatal("Expected response_format in params")
	}

	schemaJSON, err := json.Marshal(responseFormat["json_schema"])
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(schemaJSON), "field2") {
		t.Error("Expected request schema (field2) to be used")
	}
	if strings.Contains(string(schemaJSON), "field1") {
		t.Error("Expected agent schema (field1) to be overridden")
	}
}

type mockProviderForOutputSchema struct {
	captureParams func(map[string]any)
}

func (m *mockProviderForOutputSchema) ID() string { return "mock" }

func (m *mockProviderForOutputSchema) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	if m.captureParams != nil {
		m.captureParams(req.Params)
	}
	return &mockStreamForOutputSchema{}, nil
}

type mockStreamForOutputSchema struct {
	called bool
}

func (m *mockStreamForOutputSchema) Recv() (provider.Event, error) {
	if m.called {
		return provider.Event{}, io.EOF
	}
	m.called = true
	return provider.Event{
		Type:    provider.EventMessageCompleted,
		Message: agent.Message{Role: agent.RoleAssistant, Parts: []agent.ContentPart{{Type: agent.ContentPartText, Text: "test"}}},
	}, nil
}

func (m *mockStreamForOutputSchema) Close() error { return nil }
