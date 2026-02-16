package portfolio

import (
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/collab/evidencegate"
)

func TestBuildPrefersDiversity(t *testing.T) {
	cards := []evidencegate.Card{
		{
			Title:            "Safe tweak",
			Concept:          "Improve existing agenda handoff",
			CoreAssumption:   "Small change helps",
			CheapestTest:     "pilot",
			TargetSignal:     "completion",
			SuccessThreshold: "10%",
			FailureThreshold: "2%",
			TimeToLearn:      "1 week",
			RiskLevel:        "low",
		},
		{
			Title:            "Adjacent bet",
			Concept:          "Use async prep ritual before meeting",
			CoreAssumption:   "Prep improves focus",
			CheapestTest:     "pilot",
			TargetSignal:     "engagement",
			SuccessThreshold: "15%",
			FailureThreshold: "5%",
			TimeToLearn:      "2 weeks",
			RiskLevel:        "medium",
		},
		{
			Title:            "Bold bet",
			Concept:          "Replace workflow with AI co-pilot summary",
			CoreAssumption:   "Automation shifts behavior",
			CheapestTest:     "smoke test",
			TargetSignal:     "retention",
			SuccessThreshold: "20%",
			FailureThreshold: "5%",
			TimeToLearn:      "2 weeks",
			RiskLevel:        "high",
		},
	}

	bets := Build(cards, 3)
	if len(bets) != 3 {
		t.Fatalf("expected 3 bets, got %d", len(bets))
	}

	seen := map[BetType]bool{}
	for _, bet := range bets {
		seen[bet.Type] = true
	}
	if !seen[BetSafe] || !seen[BetAdjacent] || !seen[BetBold] {
		t.Fatalf("expected safe/adjacent/bold coverage, got %+v", seen)
	}
}
