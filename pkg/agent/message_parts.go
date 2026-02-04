package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ContentPartType identifies the type of content in a message part.
type ContentPartType string

// Supported content part types.
const (
	ContentPartText     ContentPartType = "text"
	ContentPartImage    ContentPartType = "image"
	ContentPartAudio    ContentPartType = "audio"
	ContentPartVideo    ContentPartType = "video"
	ContentPartFile     ContentPartType = "file"
	ContentPartResource ContentPartType = "resource"
	ContentPartJSON     ContentPartType = "json"
)

// ContentPart represents a structured content fragment within a message.
type ContentPart struct {
	Type     ContentPartType `json:"type"`
	Text     string          `json:"text,omitempty"`
	URI      string          `json:"uri,omitempty"`
	Data     []byte          `json:"data,omitempty"`
	MIMEType string          `json:"mime_type,omitempty"`
	Name     string          `json:"name,omitempty"`
	Size     *int64          `json:"size,omitempty"`
	JSON     any             `json:"json,omitempty"`
	Detail   string          `json:"detail,omitempty"`
	Metadata map[string]any  `json:"metadata,omitempty"`
}

// Text returns the message text, combining text parts if present.
func (m Message) Text() string {
	if len(m.Parts) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, part := range m.Parts {
		if part.Type == ContentPartText && part.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(part.Text)
		}
	}

	if sb.Len() == 0 {
		return ""
	}
	return sb.String()
}

// ImageCount returns the number of image parts in the message.
func (m Message) ImageCount() int {
	count := 0
	for _, part := range m.Parts {
		if part.Type == ContentPartImage {
			count++
		}
	}
	return count
}

// Summary returns a compact, human-readable message summary.
func (m Message) Summary() string {
	text := strings.TrimSpace(m.Text())
	imageCount := m.ImageCount()
	audioCount := m.Count(ContentPartAudio)
	fileCount := m.Count(ContentPartFile) + m.Count(ContentPartResource)
	videoCount := m.Count(ContentPartVideo)
	jsonCount := m.Count(ContentPartJSON)

	switch {
	case text == "" && imageCount == 0 && audioCount == 0 && fileCount == 0 && videoCount == 0 && jsonCount == 0:
		return ""
	case text == "" && imageCount == 1 && audioCount == 0 && fileCount == 0 && videoCount == 0 && jsonCount == 0:
		return "[image]"
	case text == "" && imageCount > 1 && audioCount == 0 && fileCount == 0 && videoCount == 0 && jsonCount == 0:
		return fmt.Sprintf("[%d images]", imageCount)
	case imageCount == 0 && audioCount == 0 && fileCount == 0 && videoCount == 0 && jsonCount == 0:
		return text
	case imageCount == 1 && audioCount == 0 && fileCount == 0 && videoCount == 0 && jsonCount == 0:
		return text + " [image]"
	default:
		parts := []string{}
		if imageCount > 0 {
			parts = append(parts, fmt.Sprintf("%d images", imageCount))
		}
		if audioCount > 0 {
			parts = append(parts, fmt.Sprintf("%d audio", audioCount))
		}
		if videoCount > 0 {
			parts = append(parts, fmt.Sprintf("%d video", videoCount))
		}
		if fileCount > 0 {
			parts = append(parts, fmt.Sprintf("%d files", fileCount))
		}
		if jsonCount > 0 {
			parts = append(parts, fmt.Sprintf("%d json", jsonCount))
		}
		if text == "" {
			return "[" + strings.Join(parts, ", ") + "]"
		}
		return fmt.Sprintf("%s [%s]", text, strings.Join(parts, ", "))
	}
}

// DedupeKey returns a stable key for deduplicating messages.
func (m Message) DedupeKey() string {
	var imageKeys []string
	for _, part := range m.Parts {
		if part.Type != ContentPartImage {
			continue
		}
		if part.URI == "" {
			imageKeys = append(imageKeys, "<image>")
			continue
		}
		imageKeys = append(imageKeys, part.URI)
	}
	return fmt.Sprintf("%s:%s:%s", m.Role, m.Text(), strings.Join(imageKeys, "|"))
}

// MessageFromMap converts a map into a Message.
func MessageFromMap(data map[string]any) Message {
	msg := Message{}

	if role, ok := data["role"].(string); ok {
		msg.Role = Role(role)
	}

	if partsRaw, ok := data["parts"]; ok {
		msg.Parts = parseParts(partsRaw)
	}

	if name, ok := data["name"].(string); ok {
		msg.Name = name
	}

	if toolCallID, ok := data["tool_call_id"].(string); ok {
		msg.ToolCallID = toolCallID
	}

	if metadata, ok := data["metadata"].(map[string]any); ok {
		msg.Metadata = metadata
	}

	return msg
}

func parseParts(raw any) []ContentPart {
	switch value := raw.(type) {
	case []ContentPart:
		return value
	case []any:
		parts := make([]ContentPart, 0, len(value))
		for _, item := range value {
			if part, ok := item.(ContentPart); ok {
				parts = append(parts, part)
				continue
			}
			if partMap, ok := item.(map[string]any); ok {
				parts = append(parts, parsePartMap(partMap))
			}
		}
		return parts
	case []map[string]any:
		parts := make([]ContentPart, 0, len(value))
		for _, item := range value {
			parts = append(parts, parsePartMap(item))
		}
		return parts
	default:
		return nil
	}
}

func parsePartMap(data map[string]any) ContentPart {
	part := ContentPart{}

	if partType, ok := data["type"].(string); ok {
		part.Type = normalizePartType(partType)
	}

	if text, ok := data["text"].(string); ok {
		part.Text = text
	}

	if uri, ok := data["uri"].(string); ok {
		part.URI = uri
	}
	if name, ok := data["name"].(string); ok {
		part.Name = name
	}
	if mimeType, ok := data["mime_type"].(string); ok {
		part.MIMEType = mimeType
	}
	if detail, ok := data["detail"].(string); ok {
		part.Detail = detail
	}
	if jsonValue, ok := data["json"]; ok {
		part.JSON = jsonValue
	}

	if metadata, ok := data["metadata"].(map[string]any); ok {
		part.Metadata = metadata
	}

	return part
}

func normalizePartType(partType string) ContentPartType {
	switch strings.ToLower(partType) {
	case "input_text":
		return ContentPartText
	case "input_image":
		return ContentPartImage
	default:
		return ContentPartType(partType)
	}
}

// TextFromParts returns combined text from content parts.
func TextFromParts(parts []ContentPart) string {
	var sb strings.Builder
	for _, part := range parts {
		switch part.Type {
		case ContentPartText:
			if part.Text == "" {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(part.Text)
		case ContentPartJSON:
			if part.JSON == nil {
				continue
			}
			raw, err := json.Marshal(part.JSON)
			if err != nil {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.Write(raw)
		}
	}
	return sb.String()
}

// Count returns the number of parts of a given type.
func (m Message) Count(partType ContentPartType) int {
	count := 0
	for _, part := range m.Parts {
		if part.Type == partType {
			count++
		}
	}
	return count
}
