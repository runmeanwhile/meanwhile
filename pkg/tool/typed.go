package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

// TypedTool wraps a typed function as a Tool.
type TypedTool[In, Out any] struct {
	id          string
	description string
	schema      Schema
	fn          func(context.Context, In) (Out, error)
}

// New creates a typed tool from a function.
// The function signature must be: func(context.Context, In) (Out, error)
// In must be a struct with json tags for field names and optional description tags.
func New[In, Out any](id string, fn func(context.Context, In) (Out, error)) (*TypedTool[In, Out], error) {
	if fn == nil {
		return nil, fmt.Errorf("tool function required")
	}
	var zero In
	typ := reflect.TypeOf(zero)
	if typ == nil {
		typ = reflect.TypeOf((*In)(nil)).Elem()
	}
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("tool argument type must be a struct, got %s", typ.Kind())
	}

	schema, err := deriveSchema(typ)
	if err != nil {
		return nil, fmt.Errorf("derive schema: %w", err)
	}

	return &TypedTool[In, Out]{
		id:          id,
		description: "",
		schema:      schema,
		fn:          fn,
	}, nil
}

// WithDescription sets the tool description and returns the tool for chaining.
// This enables fluent tool creation:
//
//	tool.New("my_tool", handler).WithDescription("Does something useful")
func (t *TypedTool[In, Out]) WithDescription(desc string) *TypedTool[In, Out] {
	t.description = desc
	return t
}

// NewResult creates a typed tool whose handler returns a full Result.
func NewResult[In any](id string, fn func(context.Context, In) (Result, error)) (*TypedTool[In, Result], error) {
	return New[In, Result](id, fn)
}

// ID returns the tool ID.
func (t *TypedTool[In, Out]) ID() string { return t.id }

// Description returns the tool description.
func (t *TypedTool[In, Out]) Description() string { return t.description }

// Schema returns the tool schema.
func (t *TypedTool[In, Out]) Schema() Schema { return t.schema }

// Run executes the tool with typed arguments.
func (t *TypedTool[In, Out]) Run(ctx context.Context, call Call, _ Emitter) (Result, error) {
	var args In
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return Result{
			ID:     call.ID,
			ToolID: call.ToolID,
			Error:  &Error{Message: fmt.Sprintf("invalid arguments: %v", err)},
		}, nil
	}

	output, err := t.fn(ctx, args)
	if err != nil {
		return Result{
			ID:     call.ID,
			ToolID: call.ToolID,
			Error:  &Error{Message: err.Error()},
		}, nil
	}

	return resultFromOutput(call, output), nil
}

func resultFromOutput(call Call, output any) Result {
	switch value := output.(type) {
	case Result:
		res := value
		if res.ID == "" {
			res.ID = call.ID
		}
		if res.ToolID == "" {
			res.ToolID = call.ToolID
		}
		return res
	case *Result:
		if value == nil {
			return ResultForCall(call)
		}
		res := *value
		if res.ID == "" {
			res.ID = call.ID
		}
		if res.ToolID == "" {
			res.ToolID = call.ToolID
		}
		return res
	case string:
		return TextResult(call, value)
	default:
		res := ResultForCall(call)
		if output != nil {
			res.Output = output
			res.Parts = []agent.ContentPart{{Type: agent.ContentPartJSON, JSON: output}}
		}
		return res
	}
}

// deriveSchema generates a JSON schema from a struct type.
func deriveSchema(typ reflect.Type) (Schema, error) {
	schemaMap, err := schemaForStruct(typ)
	if err != nil {
		return Schema{}, err
	}
	return Schema{JSONSchema: schemaMap}, nil
}

// buildFieldSchema creates a JSON schema for a field type.
func buildFieldSchema(typ reflect.Type, description string) map[string]any {
	schema := make(map[string]any)

	// Handle pointer types
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Map Go types to JSON schema types
	switch typ.Kind() {
	case reflect.String:
		schema["type"] = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		schema["type"] = "number"
	case reflect.Bool:
		schema["type"] = "boolean"
	case reflect.Slice, reflect.Array:
		schema["type"] = "array"
		schema["items"] = buildFieldSchema(typ.Elem(), "")
	case reflect.Map:
		schema["type"] = "object"
	case reflect.Struct:
		nestedSchema, err := schemaForStruct(typ)
		if err == nil {
			if err := json.Unmarshal(nestedSchema, &schema); err != nil {
				schema["type"] = "object"
			}
		} else {
			schema["type"] = "object"
		}
	default:
		schema["type"] = "string"
	}

	if description != "" {
		schema["description"] = description
	}

	return schema
}

// SchemaForStruct generates a JSON schema from a struct type.
// This is exported for use by the engine when deriving output schemas.
func SchemaForStruct(typ reflect.Type) (json.RawMessage, error) {
	return schemaForStruct(typ)
}

func schemaForStruct(typ reflect.Type) (json.RawMessage, error) {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("schema expects struct, got %s", typ.Kind())
	}

	properties := make(map[string]any)
	required := make([]string, 0)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]
		omitempty := false
		for _, part := range parts[1:] {
			if part == "omitempty" {
				omitempty = true
			}
		}

		description := field.Tag.Get("description")
		fieldSchema := buildFieldSchema(field.Type, description)
		properties[fieldName] = fieldSchema

		if !omitempty {
			required = append(required, fieldName)
		}
	}

	schemaMap := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schemaMap["required"] = required
	}

	schemaJSON, err := json.Marshal(schemaMap)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	return schemaJSON, nil
}
