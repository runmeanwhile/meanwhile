package eval

import (
	"strings"
	"unicode"
)

// ComputeProxyMetrics computes deterministic transcript metrics.
func ComputeProxyMetrics(turns []TranscriptTurn) ProxyMetrics {
	if len(turns) == 0 {
		return ProxyMetrics{}
	}

	speakerCounts := make(map[string]int)
	seenNormalized := make(map[string]int)
	nameSet := make(map[string]struct{})

	totalWords := 0
	directRefs := 0
	questions := 0
	duplicates := 0

	for _, turn := range turns {
		speakerCounts[turn.Speaker]++
		nameSet[strings.ToLower(strings.TrimSpace(turn.Speaker))] = struct{}{}
		words := strings.Fields(turn.Text)
		totalWords += len(words)
		if strings.Contains(turn.Text, "?") {
			questions++
		}
		normalized := normalizeTurn(turn.Text)
		if normalized != "" {
			seenNormalized[normalized]++
			if seenNormalized[normalized] > 1 {
				duplicates++
			}
		}
	}

	for _, turn := range turns {
		if referencesOtherSpeaker(turn, nameSet) {
			directRefs++
		}
	}

	minTurns := len(turns)
	maxTurns := 0
	for _, c := range speakerCounts {
		if c < minTurns {
			minTurns = c
		}
		if c > maxTurns {
			maxTurns = c
		}
	}

	balance := 1.0
	if maxTurns > 0 {
		balance = float64(minTurns) / float64(maxTurns)
	}

	return ProxyMetrics{
		TotalTurns:          len(turns),
		UniqueSpeakers:      len(speakerCounts),
		AvgWordsPerTurn:     float64(totalWords) / float64(len(turns)),
		SpeakerBalanceRatio: balance,
		DirectReferenceRate: float64(directRefs) / float64(len(turns)),
		QuestionRate:        float64(questions) / float64(len(turns)),
		RepetitionRate:      float64(duplicates) / float64(len(turns)),
	}
}

// AggregateProxyMetrics averages multiple ProxyMetrics values.
func AggregateProxyMetrics(items []ProxyMetrics) ProxyMetrics {
	if len(items) == 0 {
		return ProxyMetrics{}
	}
	var out ProxyMetrics
	for _, m := range items {
		out.TotalTurns += m.TotalTurns
		out.UniqueSpeakers += m.UniqueSpeakers
		out.AvgWordsPerTurn += m.AvgWordsPerTurn
		out.SpeakerBalanceRatio += m.SpeakerBalanceRatio
		out.DirectReferenceRate += m.DirectReferenceRate
		out.QuestionRate += m.QuestionRate
		out.RepetitionRate += m.RepetitionRate
	}
	n := float64(len(items))
	out.TotalTurns = int(float64(out.TotalTurns) / n)
	out.UniqueSpeakers = int(float64(out.UniqueSpeakers) / n)
	out.AvgWordsPerTurn /= n
	out.SpeakerBalanceRatio /= n
	out.DirectReferenceRate /= n
	out.QuestionRate /= n
	out.RepetitionRate /= n
	return out
}

// AggregateDimensionScores averages rubric scores.
func AggregateDimensionScores(items []DimensionScores) DimensionScores {
	if len(items) == 0 {
		return DimensionScores{}
	}
	var out DimensionScores
	for _, m := range items {
		out.FlowArc += m.FlowArc
		out.PersonaSeparation += m.PersonaSeparation
		out.Responsiveness += m.Responsiveness
		out.Naturalness += m.Naturalness
		out.IdeaQuality += m.IdeaQuality
		out.ConvergenceQuality += m.ConvergenceQuality
		out.ReportQuality += m.ReportQuality
	}
	n := float64(len(items))
	out.FlowArc /= n
	out.PersonaSeparation /= n
	out.Responsiveness /= n
	out.Naturalness /= n
	out.IdeaQuality /= n
	out.ConvergenceQuality /= n
	out.ReportQuality /= n
	return out
}

func referencesOtherSpeaker(turn TranscriptTurn, allNames map[string]struct{}) bool {
	text := strings.ToLower(turn.Text)
	self := strings.ToLower(strings.TrimSpace(turn.Speaker))
	for name := range allNames {
		if name == "" || name == self {
			continue
		}
		if strings.Contains(text, name) {
			return true
		}
	}
	return false
}

func normalizeTurn(text string) string {
	text = strings.ToLower(text)
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	var b strings.Builder
	lastSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if lastSpace {
				continue
			}
			b.WriteRune(' ')
			lastSpace = true
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
