package discovery

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var sourceRefPattern = regexp.MustCompile(`\[([A-Za-z0-9._:-]+)\]`)

// Note is a normalized discovery turn artifact.
type Note struct {
	Question    string   `json:"question,omitempty"`
	Evidence    string   `json:"evidence,omitempty"`
	SourceIDs   []string `json:"source_ids,omitempty"`
	Uncertainty string   `json:"uncertainty,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	Raw         string   `json:"raw,omitempty"`
}

// Requirement defines quality gates for discovery notes.
type Requirement struct {
	RequireQuestion    bool
	RequireSources     bool
	RequireUncertainty bool
	MinSources         int
	AllowedSourceIDs   []string
}

// Issue describes a requirement miss.
type Issue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// DefaultRequirement returns a strict baseline for discovery phases.
func DefaultRequirement() Requirement {
	return Requirement{
		RequireQuestion:    true,
		RequireSources:     true,
		RequireUncertainty: true,
		MinSources:         1,
	}
}

// ParseNote extracts a discovery note from free text.
func ParseNote(text string) Note {
	note := Note{Raw: strings.TrimSpace(text)}
	if note.Raw == "" {
		return note
	}

	lines := strings.Split(note.Raw, "\n")
	var evidenceLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "question:"):
			note.Question = strings.TrimSpace(trimmed[len("question:"):])
		case strings.HasPrefix(lower, "evidence:"):
			value := strings.TrimSpace(trimmed[len("evidence:"):])
			if value != "" {
				evidenceLines = append(evidenceLines, value)
			}
		case strings.HasPrefix(lower, "sources:"):
			value := strings.TrimSpace(trimmed[len("sources:"):])
			note.SourceIDs = append(note.SourceIDs, splitSources(value)...)
		case strings.HasPrefix(lower, "uncertainty:"):
			note.Uncertainty = strings.TrimSpace(trimmed[len("uncertainty:"):])
		case strings.HasPrefix(lower, "confidence:"):
			note.Confidence = strings.TrimSpace(trimmed[len("confidence:"):])
		default:
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "• ") {
				evidenceLines = append(evidenceLines, strings.TrimSpace(trimmed[2:]))
			}
		}
	}

	refs := extractSourceRefs(note.Raw)
	note.SourceIDs = dedupe(append(note.SourceIDs, refs...))
	if note.Question == "" {
		note.Question = findQuestion(note.Raw)
	}
	if note.Evidence == "" && len(evidenceLines) > 0 {
		note.Evidence = strings.Join(evidenceLines, " ")
	}
	return note
}

// Validate checks a note against requirements.
func Validate(note Note, req Requirement) []Issue {
	issues := make([]Issue, 0, 4)

	if req.RequireQuestion && strings.TrimSpace(note.Question) == "" {
		issues = append(issues, Issue{Field: "question", Message: "missing discovery question"})
	}
	if req.RequireUncertainty && strings.TrimSpace(note.Uncertainty) == "" {
		issues = append(issues, Issue{Field: "uncertainty", Message: "missing uncertainty statement"})
	}

	minSources := req.MinSources
	if minSources <= 0 {
		minSources = 1
	}
	if req.RequireSources && len(note.SourceIDs) < minSources {
		issues = append(issues, Issue{Field: "source_ids", Message: fmt.Sprintf("need at least %d source citation(s)", minSources)})
	}

	if len(req.AllowedSourceIDs) > 0 && len(note.SourceIDs) > 0 {
		allowed := make(map[string]struct{}, len(req.AllowedSourceIDs))
		for _, id := range req.AllowedSourceIDs {
			id = strings.TrimSpace(id)
			if id != "" {
				allowed[id] = struct{}{}
			}
		}
		allowedCount := 0
		for _, id := range note.SourceIDs {
			if _, ok := allowed[id]; ok {
				allowedCount++
				continue
			}
			issues = append(issues, Issue{Field: "source_ids", Message: fmt.Sprintf("unknown source id %q", id)})
		}
		if req.RequireSources && allowedCount < minSources {
			issues = append(issues, Issue{Field: "source_ids", Message: "missing citations from allowed sources"})
		}
	}

	return issues
}

// IssueSummary renders concise issue text.
func IssueSummary(issues []Issue) string {
	if len(issues) == 0 {
		return ""
	}
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		field := strings.TrimSpace(issue.Field)
		msg := strings.TrimSpace(issue.Message)
		if field == "" {
			parts = append(parts, msg)
			continue
		}
		parts = append(parts, field+": "+msg)
	}
	return strings.Join(parts, "; ")
}

func splitSources(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		id := strings.TrimSpace(strings.Trim(field, "[]"))
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return dedupe(out)
}

func findQuestion(text string) string {
	for _, sentence := range splitSentences(text) {
		trimmed := strings.TrimSpace(sentence)
		if strings.Contains(trimmed, "?") {
			return trimmed
		}
	}
	return ""
}

func splitSentences(text string) []string {
	replacer := strings.NewReplacer("\n", " ", "\t", " ")
	clean := replacer.Replace(text)
	clean = strings.Join(strings.Fields(clean), " ")
	if clean == "" {
		return nil
	}
	parts := strings.FieldsFunc(clean, func(r rune) bool {
		return r == '.' || r == '?' || r == '!'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func extractSourceRefs(text string) []string {
	matches := sourceRefPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		out = append(out, strings.TrimSpace(match[1]))
	}
	return dedupe(out)
}

func dedupe(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
