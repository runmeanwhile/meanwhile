package eval

import (
	"fmt"
	"sort"
)

// RegressionConfig controls pass/fail thresholds.
type RegressionConfig struct {
	Weights         DimensionScores
	MaxOverallDrop  float64
	MaxCriticalDrop float64
	CriticalDims    []string
	RequireAllKeys  bool
}

// RegressionDelta captures summary-to-summary change.
type RegressionDelta struct {
	Key             string             `json:"key"`
	BaselineOverall float64            `json:"baseline_overall"`
	CurrentOverall  float64            `json:"current_overall"`
	OverallDrop     float64            `json:"overall_drop"`
	DimensionDrops  map[string]float64 `json:"dimension_drops,omitempty"`
	Failure         string             `json:"failure,omitempty"`
}

// RegressionResult aggregates gate output.
type RegressionResult struct {
	Passed bool              `json:"passed"`
	Deltas []RegressionDelta `json:"deltas"`
}

// CompareReports checks current report against a baseline report.
func CompareReports(current, baseline Report, cfg RegressionConfig) RegressionResult {
	if cfg.Weights == (DimensionScores{}) {
		cfg.Weights = DefaultDimensionWeights()
	}
	if len(cfg.CriticalDims) == 0 {
		cfg.CriticalDims = CriticalDimensionNames()
	}

	baseIndex := make(map[string]Summary, len(baseline.Summaries))
	for _, s := range baseline.Summaries {
		baseIndex[summaryKey(s)] = s
	}

	keys := make([]string, 0, len(current.Summaries))
	currIndex := make(map[string]Summary, len(current.Summaries))
	for _, s := range current.Summaries {
		key := summaryKey(s)
		keys = append(keys, key)
		currIndex[key] = s
	}
	sort.Strings(keys)

	result := RegressionResult{Passed: true, Deltas: make([]RegressionDelta, 0, len(keys))}

	for _, key := range keys {
		curr := currIndex[key]
		base, ok := baseIndex[key]
		delta := RegressionDelta{Key: key, DimensionDrops: map[string]float64{}}
		if !ok {
			if cfg.RequireAllKeys {
				delta.Failure = "missing baseline summary"
				result.Passed = false
			}
			result.Deltas = append(result.Deltas, delta)
			continue
		}

		baseOverall := base.JudgeDimension.WeightedAverage(cfg.Weights)
		currOverall := curr.JudgeDimension.WeightedAverage(cfg.Weights)
		delta.BaselineOverall = baseOverall
		delta.CurrentOverall = currOverall
		delta.OverallDrop = baseOverall - currOverall
		if delta.OverallDrop > cfg.MaxOverallDrop {
			delta.Failure = fmt.Sprintf("overall drop %.3f exceeds max %.3f", delta.OverallDrop, cfg.MaxOverallDrop)
			result.Passed = false
		}

		for _, name := range cfg.CriticalDims {
			drop := DimensionValue(base.JudgeDimension, name) - DimensionValue(curr.JudgeDimension, name)
			delta.DimensionDrops[name] = drop
			if drop > cfg.MaxCriticalDrop {
				msg := fmt.Sprintf("critical %s drop %.3f exceeds max %.3f", name, drop, cfg.MaxCriticalDrop)
				if delta.Failure == "" {
					delta.Failure = msg
				} else {
					delta.Failure += "; " + msg
				}
				result.Passed = false
			}
		}

		if curr.Failures > base.Failures {
			msg := fmt.Sprintf("failure count increased (%d -> %d)", base.Failures, curr.Failures)
			if delta.Failure == "" {
				delta.Failure = msg
			} else {
				delta.Failure += "; " + msg
			}
			result.Passed = false
		}

		result.Deltas = append(result.Deltas, delta)
	}

	if cfg.RequireAllKeys {
		for key := range baseIndex {
			if _, ok := currIndex[key]; !ok {
				result.Passed = false
				result.Deltas = append(result.Deltas, RegressionDelta{Key: key, Failure: "missing current summary"})
			}
		}
	}

	sort.Slice(result.Deltas, func(i, j int) bool {
		return result.Deltas[i].Key < result.Deltas[j].Key
	})

	return result
}

func summaryKey(s Summary) string {
	return s.Protocol + "|" + s.Model + "|" + s.Variant
}
