package protocol

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

// ToolFinding represents a finding from a tool call that should be shared with other agents.
type ToolFinding struct {
	ToolID  string `json:"tool_id"`
	Query   string `json:"query"`
	Summary string `json:"summary"`
	Agent   string `json:"agent"`
}

// SharedFindings tracks tool findings across all agents in a phase to prevent redundant calls.
// Use this when multiple agents take turns calling discovery/insight tools.
type SharedFindings struct {
	mu       sync.RWMutex
	findings []ToolFinding
}

// NewSharedFindings creates a new findings tracker.
func NewSharedFindings() *SharedFindings {
	return &SharedFindings{
		findings: make([]ToolFinding, 0),
	}
}

// Add records a new finding.
func (sf *SharedFindings) Add(finding ToolFinding) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.findings = append(sf.findings, finding)
}

// All returns all findings.
func (sf *SharedFindings) All() []ToolFinding {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	result := make([]ToolFinding, len(sf.findings))
	copy(result, sf.findings)
	return result
}

// Len returns the number of findings.
func (sf *SharedFindings) Len() int {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return len(sf.findings)
}

// FormatForPrompt formats findings as a prompt section for agents.
// Returns empty string if no findings exist.
func (sf *SharedFindings) FormatForPrompt() string {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	if len(sf.findings) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("PRIOR TOOL FINDINGS (already discovered - do NOT re-query):\n")
	for i, f := range sf.findings {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s found: %s\n", i+1, f.ToolID, f.Agent, f.Summary))
	}
	sb.WriteString("\nBuild on these findings. Only call tools if you have a DIFFERENT question.\n")
	return sb.String()
}

// ExtractFindingsFromResponse parses an agent's response for tool findings.
// Agents are prompted to use a specific format: [FINDING: tool_id] summary
// Also extracts implicit findings from common patterns.
func ExtractFindingsFromResponse(agentName string, resp agent.Message) []ToolFinding {
	text := resp.Text()
	if text == "" {
		return nil
	}

	findings := make([]ToolFinding, 0)

	// Look for [FINDING: tool_id] patterns
	findingPattern := regexp.MustCompile(`\[FINDING:\s*(\w+)\]\s*(.+?)(?:\n|$)`)
	matches := findingPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			findings = append(findings, ToolFinding{
				ToolID:  match[1],
				Summary: strings.TrimSpace(match[2]),
				Agent:   agentName,
			})
		}
	}

	// Fallback: Look for implicit findings from common patterns
	if len(findings) == 0 {
		insightPatterns := []string{
			`(?i)(?:I found|I discovered|I learned|The data shows|Research shows|According to|The recall shows)[\s:]+(.{20,100}?)(?:\.|$)`,
		}
		for _, pattern := range insightPatterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllStringSubmatch(text, 2)
			for _, match := range matches {
				if len(match) >= 2 {
					findings = append(findings, ToolFinding{
						ToolID:  "implicit",
						Summary: strings.TrimSpace(match[1]),
						Agent:   agentName,
					})
				}
			}
		}
	}

	return findings
}
