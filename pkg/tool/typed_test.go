package tool

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTypedTool(t *testing.T) {
	type Args struct {
		Task   string `json:"task" description:"The task to perform"`
		Reason string `json:"reason,omitempty" description:"Why this task is needed"`
	}

	callCount := 0
	tool, err := New[Args, string]("test_tool", func(_ context.Context, args Args) (string, error) {
		callCount++
		return "result: " + args.Task, nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if tool.ID() != "test_tool" {
		t.Errorf("expected ID 'test_tool', got %q", tool.ID())
	}

	// Test schema generation
	var schemaMap map[string]any
	if err := json.Unmarshal(tool.Schema().JSONSchema, &schemaMap); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	if schemaMap["type"] != "object" {
		t.Errorf("expected type=object, got %v", schemaMap["type"])
	}

	properties, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties is not a map")
	}

	taskProp, ok := properties["task"].(map[string]any)
	if !ok {
		t.Fatal("task property not found")
	}
	if taskProp["type"] != "string" {
		t.Errorf("expected task type=string, got %v", taskProp["type"])
	}
	if taskProp["description"] != "The task to perform" {
		t.Errorf("unexpected task description: %v", taskProp["description"])
	}

	reasonProp, ok := properties["reason"].(map[string]any)
	if !ok {
		t.Fatal("reason property not found")
	}
	if reasonProp["description"] != "Why this task is needed" {
		t.Errorf("unexpected reason description: %v", reasonProp["description"])
	}

	// Check required fields
	required, ok := schemaMap["required"].([]any)
	if !ok {
		t.Fatal("required is not an array")
	}
	if len(required) != 1 || required[0] != "task" {
		t.Errorf("expected required=[task], got %v", required)
	}

	// Test execution
	argsJSON, _ := json.Marshal(Args{Task: "test task", Reason: "testing"})
	result, err := tool.Run(context.Background(), Call{
		ID:        "call-1",
		ToolID:    "test_tool",
		Arguments: argsJSON,
	}, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %s", result.Error.Message)
	}
	if result.Text() != "result: test task" {
		t.Errorf("unexpected content: %s", result.Text())
	}
	if callCount != 1 {
		t.Errorf("expected callCount=1, got %d", callCount)
	}
}

func TestTypedToolInvalidArgs(t *testing.T) {
	type Args struct {
		Count int `json:"count"`
	}

	tool, err := New[Args, string]("test", func(_ context.Context, _ Args) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Pass invalid JSON
	result, err := tool.Run(context.Background(), Call{
		ID:        "call-1",
		ToolID:    "test",
		Arguments: []byte(`{"count": "not a number"}`),
	}, nil)

	if err != nil {
		t.Fatalf("Run should not return error, got: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error in result.Error")
	}
}

func TestTypedToolPointerArgs(t *testing.T) {
	type Args struct {
		Task string `json:"task"`
	}

	tool, err := New[*Args, string]("ptr_tool", func(_ context.Context, args *Args) (string, error) {
		if args == nil {
			return "nil", nil
		}
		return args.Task, nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(tool.Schema().JSONSchema, &schemaMap); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schemaMap["type"] != "object" {
		t.Errorf("expected type=object, got %v", schemaMap["type"])
	}
}

func TestTypedToolComplexTypes(t *testing.T) {
	type Args struct {
		Name    string   `json:"name"`
		Age     int      `json:"age"`
		Active  bool     `json:"active"`
		Score   float64  `json:"score"`
		Tags    []string `json:"tags"`
		Enabled *bool    `json:"enabled,omitempty"`
	}

	tool, err := New[Args, string]("complex", func(_ context.Context, _ Args) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(tool.Schema().JSONSchema, &schemaMap); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	properties := schemaMap["properties"].(map[string]any)

	nameProp := properties["name"].(map[string]any)
	if nameProp["type"] != "string" {
		t.Errorf("name type: got %v", nameProp["type"])
	}

	ageProp := properties["age"].(map[string]any)
	if ageProp["type"] != "integer" {
		t.Errorf("age type: got %v", ageProp["type"])
	}

	activeProp := properties["active"].(map[string]any)
	if activeProp["type"] != "boolean" {
		t.Errorf("active type: got %v", activeProp["type"])
	}

	scoreProp := properties["score"].(map[string]any)
	if scoreProp["type"] != "number" {
		t.Errorf("score type: got %v", scoreProp["type"])
	}

	tagsProp := properties["tags"].(map[string]any)
	if tagsProp["type"] != "array" {
		t.Errorf("tags type: got %v", tagsProp["type"])
	}
	tagsItems := tagsProp["items"].(map[string]any)
	if tagsItems["type"] != "string" {
		t.Errorf("tags items type: got %v", tagsItems["type"])
	}

	// Check required fields (enabled should not be required due to omitempty)
	required := schemaMap["required"].([]any)
	if len(required) != 5 {
		t.Errorf("expected 5 required fields, got %d: %v", len(required), required)
	}
}

func TestTypedToolNestedStructSchema(t *testing.T) {
	type Inner struct {
		Code string `json:"code"`
	}
	type Args struct {
		Name  string `json:"name"`
		Inner Inner  `json:"inner"`
	}

	tool, err := New[Args, string]("nested", func(_ context.Context, _ Args) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(tool.Schema().JSONSchema, &schemaMap); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	properties := schemaMap["properties"].(map[string]any)
	innerProp := properties["inner"].(map[string]any)
	if innerProp["type"] != "object" {
		t.Fatalf("expected inner type object, got %v", innerProp["type"])
	}
	innerProps, ok := innerProp["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected inner properties to be expanded")
	}
	codeProp := innerProps["code"].(map[string]any)
	if codeProp["type"] != "string" {
		t.Fatalf("expected inner.code type string, got %v", codeProp["type"])
	}
}
