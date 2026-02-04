package tool

import (
	"context"
	"testing"
)

type testArgs struct {
	Message string `json:"message"`
}

func TestWithDescription(t *testing.T) {
	handler := func(ctx context.Context, args testArgs) (string, error) {
		return "ok", nil
	}

	toolBase, err := New("test_tool", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool := toolBase.WithDescription("A test tool")

	if tool.Description() != "A test tool" {
		t.Errorf("expected description 'A test tool', got '%s'", tool.Description())
	}
}

func TestWithDescriptionChaining(t *testing.T) {
	handler := func(ctx context.Context, args testArgs) (string, error) {
		return "ok", nil
	}

	// Test fluent chaining
	toolBase, err := New("test_tool", handler)
	if err != nil {
		t.Fatalf("unexpected error creating tool: %v", err)
	}

	tool := toolBase.WithDescription("Does something useful")

	if tool.ID() != "test_tool" {
		t.Errorf("expected ID 'test_tool', got '%s'", tool.ID())
	}
	if tool.Description() != "Does something useful" {
		t.Errorf("expected description, got '%s'", tool.Description())
	}
}

func TestDescriptionInDefinition(t *testing.T) {
	handler := func(ctx context.Context, args testArgs) (string, error) {
		return "ok", nil
	}

	toolBase, err := New("test_tool", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tool := toolBase.WithDescription("My description")

	def := DefinitionFromTool(tool)
	if def.Description != "My description" {
		t.Errorf("expected definition description 'My description', got '%s'", def.Description)
	}
}
