package insightpack

import (
	"fmt"
	"sort"
	"strings"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

// Strategy controls how aggressively context gathering uses tools vs memory.
type Strategy string

const (
	// StrategyMemoryFirst prioritizes prior session/user memory and uses tools sparingly.
	StrategyMemoryFirst Strategy = "memory_first"
	// StrategyBalanced mixes memory with targeted document/web/tool calls.
	StrategyBalanced Strategy = "balanced"
	// StrategyResearchHeavy prioritizes active search and document/web retrieval.
	StrategyResearchHeavy Strategy = "research_heavy"
)

// SourceType captures the kind of context source.
type SourceType string

const (
	SourceMemory   SourceType = "memory"
	SourceInternal SourceType = "internal_docs"
	SourceExternal SourceType = "web"
	SourceProduct  SourceType = "product_data"
	SourceCustom   SourceType = "custom"
)

// Source describes one context source available to protocols.
type Source struct {
	ID          string
	Type        SourceType
	Description string
	ToolIDs     []string
	Priority    int
	Required    bool
}

// Budget controls the context-intake cost envelope.
type Budget struct {
	// MaxToolIterations is the cap passed to RunRequest.MaxToolIterations.
	MaxToolIterations int
	// MaxSources limits number of source chunks requested in one intake pass.
	MaxSources int
}

// Plan is a reusable context-intake plan that can be consumed by any protocol.
type Plan struct {
	Strategy        Strategy
	Sources         []Source
	Questions       []string
	Budget          Budget
	RequireCitation bool
}

// DefaultPlan returns a balanced baseline suitable for most protocols.
func DefaultPlan() Plan {
	return Plan{
		Strategy: StrategyBalanced,
		Budget: Budget{
			MaxToolIterations: 6,
			MaxSources:        8,
		},
		RequireCitation: true,
	}
}

// Validate validates the plan.
func (p Plan) Validate() error {
	if p.Strategy == "" {
		return fmt.Errorf("strategy required")
	}
	switch p.Strategy {
	case StrategyMemoryFirst, StrategyBalanced, StrategyResearchHeavy:
	default:
		return fmt.Errorf("unsupported strategy %q", p.Strategy)
	}
	if p.Budget.MaxToolIterations < 0 {
		return fmt.Errorf("max tool iterations must be >= 0")
	}
	if p.Budget.MaxSources < 0 {
		return fmt.Errorf("max sources must be >= 0")
	}
	for _, src := range p.Sources {
		if strings.TrimSpace(src.ID) == "" {
			return fmt.Errorf("source id required")
		}
	}
	return nil
}

// MaxToolIterations returns a safe max tool iteration budget.
func (p Plan) MaxToolIterations() int {
	if p.Budget.MaxToolIterations > 0 {
		return p.Budget.MaxToolIterations
	}
	switch p.Strategy {
	case StrategyMemoryFirst:
		return 2
	case StrategyResearchHeavy:
		return 10
	default:
		return 6
	}
}

// MaxSources returns a safe source count budget.
func (p Plan) MaxSources() int {
	if p.Budget.MaxSources > 0 {
		return p.Budget.MaxSources
	}
	switch p.Strategy {
	case StrategyMemoryFirst:
		return 4
	case StrategyResearchHeavy:
		return 12
	default:
		return 8
	}
}

// AllowedToolIDs returns a de-duplicated list of tool IDs across configured sources.
func (p Plan) AllowedToolIDs() []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, src := range p.Sources {
		for _, id := range src.ToolIDs {
			trimmed := strings.TrimSpace(id)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			out = append(out, trimmed)
		}
	}
	return out
}

// ToolPolicy builds an allowlist policy from the plan's declared tool IDs.
func (p Plan) ToolPolicy() tool.Policy {
	ids := p.AllowedToolIDs()
	if len(ids) == 0 {
		return tool.Policy{}
	}
	return tool.Policy{
		Mode:       tool.PolicyAllowlist,
		AllowIDs:   ids,
		Reason:     "insightpack context-intake plan",
		EnforcedBy: "collab.insightpack",
	}
}

// PromptContext renders plan details into a compact prompt block.
func (p Plan) PromptContext(scope string) string {
	var sb strings.Builder
	if strings.TrimSpace(scope) != "" {
		sb.WriteString("Scope: ")
		sb.WriteString(strings.TrimSpace(scope))
		sb.WriteString("\n")
	}
	sb.WriteString("Context strategy: ")
	sb.WriteString(string(p.Strategy))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Budgets: max_tool_iterations=%d, max_sources=%d\n", p.MaxToolIterations(), p.MaxSources()))
	if len(p.Questions) > 0 {
		sb.WriteString("Focus questions:\n")
		for _, q := range p.Questions {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(q)
			sb.WriteString("\n")
		}
	}
	if len(p.Sources) > 0 {
		sb.WriteString("Sources:\n")
		sources := append([]Source(nil), p.Sources...)
		sort.SliceStable(sources, func(i, j int) bool {
			if sources[i].Priority == sources[j].Priority {
				return sources[i].ID < sources[j].ID
			}
			return sources[i].Priority < sources[j].Priority
		})
		for _, src := range sources {
			desc := strings.TrimSpace(src.Description)
			if desc == "" {
				desc = string(src.Type)
			}
			sb.WriteString("- ")
			sb.WriteString(src.ID)
			sb.WriteString(": ")
			sb.WriteString(desc)
			if len(src.ToolIDs) > 0 {
				sb.WriteString(" (tools: ")
				sb.WriteString(strings.Join(src.ToolIDs, ", "))
				sb.WriteString(")")
			}
			if src.Required {
				sb.WriteString(" [required]")
			}
			sb.WriteString("\n")
		}
	}
	if p.RequireCitation {
		sb.WriteString("Evidence discipline: cite source ids in findings.\n")
	}
	return strings.TrimSpace(sb.String())
}

// Insight is a normalized insight extracted from research/context intake.
type Insight struct {
	Title       string `json:"title"`
	Evidence    string `json:"evidence,omitempty"`
	SourceID    string `json:"source_id,omitempty"`
	Risk        string `json:"risk,omitempty"`
	Opportunity string `json:"opportunity,omitempty"`
}

// ParseInsights parses simple bullet/text output into insights.
func ParseInsights(text string) []Insight {
	lines := strings.Split(text, "\n")
	insights := make([]Insight, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "• ")
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "|", 3)
		insight := Insight{Title: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			insight.Evidence = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			insight.SourceID = strings.TrimSpace(parts[2])
		}
		if insight.Title == "" {
			continue
		}
		insights = append(insights, insight)
	}
	return insights
}
