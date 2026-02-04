package message

import "github.com/darkostanimirovic/meanwhile/pkg/agent"

// System constructs a system message.
func System(content string) agent.Message {
	return Parts(agent.RoleSystem, TextPart(content))
}

// User constructs a user message.
func User(content string) agent.Message {
	return Parts(agent.RoleUser, TextPart(content))
}

// Assistant constructs an assistant message.
func Assistant(content string) agent.Message {
	return Parts(agent.RoleAssistant, TextPart(content))
}

// Parts constructs a message with structured content parts.
func Parts(role agent.Role, parts ...agent.ContentPart) agent.Message {
	return agent.Message{
		Role:  role,
		Parts: parts,
	}
}

// UserParts constructs a user message with structured content parts.
func UserParts(parts ...agent.ContentPart) agent.Message {
	return Parts(agent.RoleUser, parts...)
}

// AssistantParts constructs an assistant message with structured content parts.
func AssistantParts(parts ...agent.ContentPart) agent.Message {
	return Parts(agent.RoleAssistant, parts...)
}

// SystemParts constructs a system message with structured content parts.
func SystemParts(parts ...agent.ContentPart) agent.Message {
	return Parts(agent.RoleSystem, parts...)
}

// TextPart constructs a text content part.
func TextPart(text string) agent.ContentPart {
	return agent.ContentPart{Type: agent.ContentPartText, Text: text}
}

// ImagePart constructs an image content part with a URL.
func ImagePart(uri string) agent.ContentPart {
	return agent.ContentPart{Type: agent.ContentPartImage, URI: uri}
}

// JSONPart constructs a JSON content part.
func JSONPart(value any) agent.ContentPart {
	return agent.ContentPart{Type: agent.ContentPartJSON, JSON: value}
}

// ResourcePart constructs a resource content part with a URI.
func ResourcePart(uri, name string) agent.ContentPart {
	return agent.ContentPart{Type: agent.ContentPartResource, URI: uri, Name: name}
}

// FilePart constructs a file content part with a URI and MIME type.
func FilePart(uri, name, mimeType string) agent.ContentPart {
	return agent.ContentPart{Type: agent.ContentPartFile, URI: uri, Name: name, MIMEType: mimeType}
}

// Tool constructs a tool message with a call ID.
func Tool(callID string, parts ...agent.ContentPart) agent.Message {
	return agent.Message{Role: agent.RoleTool, ToolCallID: callID, Parts: parts}
}
