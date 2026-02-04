package integration

import (
	"fmt"
	"strings"
	"time"
)

// FormatPlainText renders a human request as plain text.
func FormatPlainText(req Request) string {
	parts := make([]string, 0, 8)
	parts = append(parts, "Meanwhile")
	parts = append(parts, "")
	parts = append(parts, "QUESTION")
	parts = append(parts, req.Question)
	if req.Context != "" {
		parts = append(parts, "", "CONTEXT", req.Context)
	}
	if req.Urgency != "" {
		parts = append(parts, "", "URGENCY", req.Urgency)
	}
	if len(req.SuggestedResponses) > 0 {
		parts = append(parts, "", "SUGGESTED RESPONSES", formatSuggestions(req.SuggestedResponses))
	}
	if req.TimeoutAt.IsZero() {
		parts = append(parts, "", "TIMEOUT", "None")
	} else {
		parts = append(parts, "", "TIMEOUT", req.TimeoutAt.UTC().Format(time.RFC3339))
	}
	parts = append(parts, "", fmt.Sprintf("Request ID: %s", req.RequestID))
	return strings.Join(parts, "\n")
}

// FormatMarkdown renders a human request with simple Markdown headings.
func FormatMarkdown(req Request) string {
	parts := make([]string, 0, 8)
	parts = append(parts, "*Meanwhile*")
	parts = append(parts, "", "*Question*", req.Question)
	if req.Context != "" {
		parts = append(parts, "", "*Context*", req.Context)
	}
	if req.Urgency != "" {
		parts = append(parts, "", "*Urgency*", req.Urgency)
	}
	if len(req.SuggestedResponses) > 0 {
		parts = append(parts, "", "*Suggested responses*", formatSuggestionsMarkdown(req.SuggestedResponses))
	}
	if req.TimeoutAt.IsZero() {
		parts = append(parts, "", "*Timeout*", "None")
	} else {
		parts = append(parts, "", "*Timeout*", req.TimeoutAt.UTC().Format(time.RFC3339))
	}
	parts = append(parts, "", fmt.Sprintf("Request ID: `%s`", req.RequestID))
	return strings.Join(parts, "\n")
}

func formatSuggestions(responses []string) string {
	var sb strings.Builder
	for _, response := range responses {
		response = strings.TrimSpace(response)
		if response == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(response)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatSuggestionsMarkdown(responses []string) string {
	return formatSuggestions(responses)
}
