package eval

import "testing"

func TestComputeProxyMetrics(t *testing.T) {
	turns := []TranscriptTurn{
		{Speaker: "Moderator", Text: "What changed this week?"},
		{Speaker: "Marketing", Text: "I agree with Engineering on urgency."},
		{Speaker: "Engineering", Text: "Moderator is right. Keep it simple."},
		{Speaker: "Marketing", Text: "I agree with Engineering on urgency."},
	}

	m := ComputeProxyMetrics(turns)
	if m.TotalTurns != 4 {
		t.Fatalf("expected 4 turns, got %d", m.TotalTurns)
	}
	if m.UniqueSpeakers != 3 {
		t.Fatalf("expected 3 speakers, got %d", m.UniqueSpeakers)
	}
	if m.DirectReferenceRate <= 0 {
		t.Fatalf("expected direct reference rate > 0, got %.3f", m.DirectReferenceRate)
	}
	if m.RepetitionRate <= 0 {
		t.Fatalf("expected repetition rate > 0, got %.3f", m.RepetitionRate)
	}
}

func TestDimensionWeightedAverage(t *testing.T) {
	s := DimensionScores{
		FlowArc:            5,
		PersonaSeparation:  4,
		Responsiveness:     3,
		Naturalness:        2,
		IdeaQuality:        1,
		ConvergenceQuality: 5,
		ReportQuality:      4,
	}
	w := DefaultDimensionWeights()
	avg := s.WeightedAverage(w)
	if avg <= 0 {
		t.Fatalf("expected positive weighted average, got %.3f", avg)
	}
}

func TestAggregateDimensionScores(t *testing.T) {
	items := []DimensionScores{{FlowArc: 4, PersonaSeparation: 3}, {FlowArc: 2, PersonaSeparation: 5}}
	agg := AggregateDimensionScores(items)
	if agg.FlowArc != 3 {
		t.Fatalf("expected flow arc avg 3, got %.2f", agg.FlowArc)
	}
	if agg.PersonaSeparation != 4 {
		t.Fatalf("expected persona avg 4, got %.2f", agg.PersonaSeparation)
	}
}
