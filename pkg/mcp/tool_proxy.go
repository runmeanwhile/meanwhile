package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	"github.com/runmeanwhile/meanwhile/pkg/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProxyTool exposes a remote MCP tool as a local tool.Tool.
type ProxyTool struct {
	server      *Server
	toolName    string
	toolID      string
	schema      tool.Schema
	description string
}

// ID returns the tool ID.
func (p *ProxyTool) ID() string { return p.toolID }

// Schema returns the tool schema.
func (p *ProxyTool) Schema() tool.Schema { return p.schema }

// Description returns a human-readable tool description when available.
func (p *ProxyTool) Description() string { return p.description }

// Run executes the remote tool via MCP.
func (p *ProxyTool) Run(ctx context.Context, call tool.Call, _ tool.Emitter) (tool.Result, error) {
	result := tool.Result{ID: call.ID, ToolID: call.ToolID}

	var args json.RawMessage
	if len(call.Arguments) > 0 {
		args = json.RawMessage(call.Arguments)
	}

	res, err := p.server.CallTool(ctx, p.toolName, args)
	if err != nil {
		result.Error = &tool.Error{Message: err.Error()}
		return result, nil
	}
	parts := partsFromMCP(res)
	result.Parts = parts
	if res.StructuredContent != nil {
		result.Output = res.StructuredContent
		if !hasPartType(parts, agent.ContentPartJSON) {
			result.Parts = append(result.Parts, agent.ContentPart{Type: agent.ContentPartJSON, JSON: res.StructuredContent})
		}
	}
	if res.IsError {
		message := toolResultText(res)
		if message == "" {
			message = "tool error"
		}
		result.Error = &tool.Error{Message: message}
		return result, nil
	}
	return result, nil
}

func toolResultText(res *sdkmcp.CallToolResult) string {
	if res == nil {
		return ""
	}

	if len(res.Content) > 0 {
		items := make([]string, 0, len(res.Content))
		for _, content := range res.Content {
			if text := contentText(content); text != "" {
				items = append(items, text)
			}
		}
		if len(items) > 0 {
			return joinLines(items)
		}
	}

	if res.StructuredContent != nil {
		if raw, err := json.Marshal(res.StructuredContent); err == nil {
			return string(raw)
		}
	}

	return ""
}

func contentText(content sdkmcp.Content) string {
	switch c := content.(type) {
	case *sdkmcp.TextContent:
		return c.Text
	case *sdkmcp.ImageContent:
		if c.MIMEType != "" {
			return fmt.Sprintf("[image:%s]", c.MIMEType)
		}
		return "[image]"
	case *sdkmcp.AudioContent:
		if c.MIMEType != "" {
			return fmt.Sprintf("[audio:%s]", c.MIMEType)
		}
		return "[audio]"
	case *sdkmcp.ResourceLink:
		if c.Name != "" && c.URI != "" {
			return fmt.Sprintf("%s (%s)", c.Name, c.URI)
		}
		if c.Title != "" && c.URI != "" {
			return fmt.Sprintf("%s (%s)", c.Title, c.URI)
		}
		if c.URI != "" {
			return c.URI
		}
		return "[resource]"
	case *sdkmcp.EmbeddedResource:
		if c.Resource == nil {
			return "[resource]"
		}
		if c.Resource.Text != "" {
			return c.Resource.Text
		}
		if c.Resource.URI != "" {
			return c.Resource.URI
		}
		return "[resource]"
	default:
		if raw, err := json.Marshal(content); err == nil {
			return string(raw)
		}
	}
	return ""
}

func partsFromMCP(res *sdkmcp.CallToolResult) []agent.ContentPart {
	if res == nil || len(res.Content) == 0 {
		return nil
	}
	parts := make([]agent.ContentPart, 0, len(res.Content))
	for _, content := range res.Content {
		switch c := content.(type) {
		case *sdkmcp.TextContent:
			if c.Text != "" {
				parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: c.Text})
			}
		case *sdkmcp.ImageContent:
			parts = append(parts, agent.ContentPart{Type: agent.ContentPartImage, MIMEType: c.MIMEType, Data: c.Data})
			if text := contentText(content); text != "" {
				parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: text})
			}
		case *sdkmcp.AudioContent:
			parts = append(parts, agent.ContentPart{Type: agent.ContentPartAudio, MIMEType: c.MIMEType, Data: c.Data})
			if text := contentText(content); text != "" {
				parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: text})
			}
		case *sdkmcp.ResourceLink:
			part := agent.ContentPart{Type: agent.ContentPartResource, URI: c.URI, Name: c.Name, MIMEType: c.MIMEType}
			if c.Title != "" {
				part.Metadata = map[string]any{"title": c.Title}
			}
			parts = append(parts, part)
			if text := contentText(content); text != "" {
				parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: text})
			}
		case *sdkmcp.EmbeddedResource:
			if c.Resource != nil {
				part := agent.ContentPart{Type: agent.ContentPartResource, URI: c.Resource.URI, MIMEType: c.Resource.MIMEType}
				if c.Resource.Text != "" {
					part.Text = c.Resource.Text
				}
				if len(c.Resource.Blob) > 0 {
					part.Data = c.Resource.Blob
				}
				parts = append(parts, part)
			}
			if text := contentText(content); text != "" {
				parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: text})
			}
		default:
			if raw, err := json.Marshal(content); err == nil {
				parts = append(parts, agent.ContentPart{Type: agent.ContentPartJSON, JSON: json.RawMessage(raw)})
				parts = append(parts, agent.ContentPart{Type: agent.ContentPartText, Text: string(raw)})
			}
		}
	}
	return parts
}
func hasPartType(parts []agent.ContentPart, partType agent.ContentPartType) bool {
	for _, part := range parts {
		if part.Type == partType {
			return true
		}
	}
	return false
}

func joinLines(values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for i := 1; i < len(values); i++ {
		out += "\n" + values[i]
	}
	return out
}
