package evidencegate

import "strings"

// Card is an experiment-ready concept card.
type Card struct {
	Title            string `json:"title"`
	Concept          string `json:"concept"`
	CoreAssumption   string `json:"core_assumption"`
	CheapestTest     string `json:"cheapest_test"`
	TargetSignal     string `json:"target_signal"`
	SuccessThreshold string `json:"success_threshold"`
	FailureThreshold string `json:"failure_threshold"`
	TimeToLearn      string `json:"time_to_learn"`
	RiskLevel        string `json:"risk_level,omitempty"`
	EvidenceRefs     string `json:"evidence_refs,omitempty"`
	Confidence       string `json:"confidence,omitempty"`
	Unknowns         string `json:"unknowns,omitempty"`
}

// ValidationIssue captures missing fields for a card.
type ValidationIssue struct {
	CardTitle string `json:"card_title"`
	Field     string `json:"field"`
	Issue     string `json:"issue"`
}

// ValidateCard validates a card and returns issues.
func ValidateCard(card Card) []ValidationIssue {
	issues := make([]ValidationIssue, 0, 8)
	title := strings.TrimSpace(card.Title)
	if title == "" {
		title = strings.TrimSpace(card.Concept)
	}
	check := func(field, value string) {
		if strings.TrimSpace(value) == "" {
			issues = append(issues, ValidationIssue{
				CardTitle: title,
				Field:     field,
				Issue:     "missing",
			})
		}
	}
	check("title", card.Title)
	check("concept", card.Concept)
	check("core_assumption", card.CoreAssumption)
	check("cheapest_test", card.CheapestTest)
	check("target_signal", card.TargetSignal)
	check("success_threshold", card.SuccessThreshold)
	check("failure_threshold", card.FailureThreshold)
	check("time_to_learn", card.TimeToLearn)
	return issues
}

// ScoreCard computes a simple readiness score in [0,8].
func ScoreCard(card Card) int {
	issues := ValidateCard(card)
	score := 8 - len(issues)
	if score < 0 {
		score = 0
	}
	return score
}

// EligibleCards returns cards that pass minimum required score.
func EligibleCards(cards []Card, minScore int) (eligible []Card, rejected []Card, issues []ValidationIssue) {
	if minScore <= 0 {
		minScore = 6
	}
	eligible = make([]Card, 0, len(cards))
	rejected = make([]Card, 0)
	issues = make([]ValidationIssue, 0)
	for _, card := range cards {
		cardIssues := ValidateCard(card)
		issues = append(issues, cardIssues...)
		if 8-len(cardIssues) >= minScore {
			eligible = append(eligible, card)
			continue
		}
		rejected = append(rejected, card)
	}
	return eligible, rejected, issues
}
