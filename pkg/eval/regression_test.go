package eval

import "testing"

func TestCompareReportsPass(t *testing.T) {
	base := Report{Summaries: []Summary{{Protocol: "brainstorming", Model: "m1", Variant: "v1", Failures: 0, JudgeDimension: DimensionScores{FlowArc: 4, PersonaSeparation: 4, Naturalness: 4, ConvergenceQuality: 4}}}}
	curr := Report{Summaries: []Summary{{Protocol: "brainstorming", Model: "m1", Variant: "v1", Failures: 0, JudgeDimension: DimensionScores{FlowArc: 3.9, PersonaSeparation: 3.8, Naturalness: 3.9, ConvergenceQuality: 3.9}}}}
	res := CompareReports(curr, base, RegressionConfig{
		Weights:         DefaultDimensionWeights(),
		MaxOverallDrop:  0.5,
		MaxCriticalDrop: 0.5,
		CriticalDims:    CriticalDimensionNames(),
		RequireAllKeys:  true,
	})
	if !res.Passed {
		t.Fatalf("expected pass, got %+v", res)
	}
}

func TestCompareReportsFailOnCriticalDrop(t *testing.T) {
	base := Report{Summaries: []Summary{{Protocol: "brainstorming", Model: "m1", Variant: "v1", Failures: 0, JudgeDimension: DimensionScores{FlowArc: 4, PersonaSeparation: 4, Naturalness: 4, ConvergenceQuality: 4}}}}
	curr := Report{Summaries: []Summary{{Protocol: "brainstorming", Model: "m1", Variant: "v1", Failures: 0, JudgeDimension: DimensionScores{FlowArc: 2.9, PersonaSeparation: 4, Naturalness: 4, ConvergenceQuality: 4}}}}
	res := CompareReports(curr, base, RegressionConfig{
		Weights:         DefaultDimensionWeights(),
		MaxOverallDrop:  2,
		MaxCriticalDrop: 0.5,
		CriticalDims:    []string{"flow_arc"},
		RequireAllKeys:  true,
	})
	if res.Passed {
		t.Fatalf("expected fail, got %+v", res)
	}
}
