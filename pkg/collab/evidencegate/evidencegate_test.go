package evidencegate

import "testing"

func TestValidateAndEligibleCards(t *testing.T) {
	cards := []Card{
		{
			Title:            "Confidence replay",
			Concept:          "Replay oldest commitment",
			CoreAssumption:   "Public replay increases action",
			CheapestTest:     "Pilot on one team",
			TargetSignal:     "Task close rate",
			SuccessThreshold: ">=15% lift",
			FailureThreshold: "<5% lift",
			TimeToLearn:      "2 weeks",
		},
		{
			Title:   "Weak card",
			Concept: "Unclear",
		},
	}
	eligible, rejected, issues := EligibleCards(cards, 6)
	if len(eligible) != 1 {
		t.Fatalf("expected 1 eligible card, got %d", len(eligible))
	}
	if len(rejected) != 1 {
		t.Fatalf("expected 1 rejected card, got %d", len(rejected))
	}
	if len(issues) == 0 {
		t.Fatal("expected validation issues")
	}
}
