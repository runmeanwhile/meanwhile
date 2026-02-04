// Package tool provides typed tools for agent actions.
package tool

import "strings"

// PolicyMode defines how allow/deny lists are interpreted.
type PolicyMode string

const (
	// PolicyAllowAll allows all tools unless explicitly denied.
	PolicyAllowAll PolicyMode = "allow_all"
	// PolicyAllowlist allows only explicitly allowed tools (and not denied).
	PolicyAllowlist PolicyMode = "allowlist"
)

// Policy constrains which tools can be invoked.
type Policy struct {
	Mode       PolicyMode
	AllowIDs   []string
	DenyIDs    []string
	AllowTags  []string
	DenyTags   []string
	Reason     string
	EnforcedBy string
}

// Empty reports whether the policy imposes any restrictions.
func (p Policy) Empty() bool {
	return p.Mode == "" && len(p.AllowIDs) == 0 && len(p.DenyIDs) == 0 && len(p.AllowTags) == 0 && len(p.DenyTags) == 0
}

// Allows reports whether a tool ID + tags are permitted by policy.
func (p Policy) Allows(toolID string, tags []string) bool {
	if toolID == "" {
		return false
	}
	if contains(p.DenyIDs, toolID) {
		return false
	}
	if len(tags) > 0 && intersects(tags, p.DenyTags) {
		return false
	}
	if p.Mode == PolicyAllowlist || len(p.AllowIDs) > 0 || len(p.AllowTags) > 0 {
		if contains(p.AllowIDs, toolID) {
			return true
		}
		if len(tags) > 0 && intersects(tags, p.AllowTags) {
			return true
		}
		return false
	}
	return true
}

// MergePolicy combines two policies, applying the override as an additional restriction.
func MergePolicy(base Policy, override Policy) Policy {
	merged := Policy{
		Mode:       base.Mode,
		AllowIDs:   append([]string(nil), base.AllowIDs...),
		DenyIDs:    append([]string(nil), base.DenyIDs...),
		AllowTags:  append([]string(nil), base.AllowTags...),
		DenyTags:   append([]string(nil), base.DenyTags...),
		Reason:     base.Reason,
		EnforcedBy: base.EnforcedBy,
	}
	if override.Mode != "" {
		merged.Mode = override.Mode
	}
	if override.Reason != "" {
		merged.Reason = override.Reason
	}
	if override.EnforcedBy != "" {
		merged.EnforcedBy = override.EnforcedBy
	}

	if len(override.AllowIDs) > 0 {
		if len(merged.AllowIDs) == 0 {
			merged.AllowIDs = append([]string(nil), override.AllowIDs...)
		} else {
			merged.AllowIDs = intersect(merged.AllowIDs, override.AllowIDs)
		}
	}
	if len(override.AllowTags) > 0 {
		if len(merged.AllowTags) == 0 {
			merged.AllowTags = append([]string(nil), override.AllowTags...)
		} else {
			merged.AllowTags = intersect(merged.AllowTags, override.AllowTags)
		}
	}

	merged.DenyIDs = appendUnique(merged.DenyIDs, override.DenyIDs...)
	merged.DenyTags = appendUnique(merged.DenyTags, override.DenyTags...)

	return normalizePolicy(merged)
}

func normalizePolicy(p Policy) Policy {
	p.AllowIDs = uniqueStrings(p.AllowIDs)
	p.DenyIDs = uniqueStrings(p.DenyIDs)
	p.AllowTags = normalizeTags(p.AllowTags)
	p.DenyTags = normalizeTags(p.DenyTags)
	if p.Mode == "" {
		if len(p.AllowIDs) > 0 || len(p.AllowTags) > 0 {
			p.Mode = PolicyAllowlist
		}
	}
	return p
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return uniqueStrings(out)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func appendUnique(values []string, extras ...string) []string {
	if len(extras) == 0 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	for _, extra := range extras {
		if extra == "" {
			continue
		}
		if _, ok := seen[extra]; ok {
			continue
		}
		seen[extra] = struct{}{}
		values = append(values, extra)
	}
	return values
}

func intersects(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	for _, value := range right {
		if _, ok := seen[value]; ok {
			return true
		}
	}
	return false
}

func intersect(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	out := make([]string, 0)
	for _, value := range right {
		if _, ok := seen[value]; ok {
			out = append(out, value)
		}
	}
	return uniqueStrings(out)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
