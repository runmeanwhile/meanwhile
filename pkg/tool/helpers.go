package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

// Func is a helper for defining tools with a single function.
type Func struct {
	IDValue          string
	DescriptionValue string
	SchemaValue      Schema
	RunFunc          func(ctx context.Context, call Call, emit Emitter) (Result, error)
}

// ID returns the tool ID.
func (f Func) ID() string { return f.IDValue }

// Description returns the tool description, if provided.
func (f Func) Description() string { return f.DescriptionValue }

// Schema returns the tool schema.
func (f Func) Schema() Schema { return f.SchemaValue }

// Run executes the tool function.
func (f Func) Run(ctx context.Context, call Call, emit Emitter) (Result, error) {
	if f.RunFunc == nil {
		return Result{}, fmt.Errorf("tool run function required")
	}
	return f.RunFunc(ctx, call, emit)
}

// SchemaFrom encodes an object into a JSON Schema definition.
func SchemaFrom(schema any) (Schema, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return Schema{}, fmt.Errorf("encode schema: %w", err)
	}
	return Schema{JSONSchema: raw}, nil
}

// ResultForCall creates a new Result with IDs populated from the call.
func ResultForCall(call Call) Result {
	return Result{ID: call.ID, ToolID: call.ToolID}
}

// TextResult creates a text-only result for the given call.
func TextResult(call Call, text string) Result {
	res := ResultForCall(call)
	if text != "" {
		res.Parts = []agent.ContentPart{{Type: agent.ContentPartText, Text: text}}
	}
	return res
}

// JSONResult creates a structured JSON result for the given call.
func JSONResult(call Call, output any) Result {
	res := ResultForCall(call)
	if output != nil {
		res.Output = output
		res.Parts = []agent.ContentPart{{Type: agent.ContentPartJSON, JSON: output}}
	}
	return res
}

// ErrorResult creates an error result for the given call.
func ErrorResult(call Call, message string) Result {
	res := ResultForCall(call)
	if message != "" {
		res.Error = &Error{Message: message}
	}
	return res
}
