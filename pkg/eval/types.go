package eval

import "time"

// Scenario defines a single evaluation prompt/case.
type Scenario struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Prompt      string `json:"prompt"`
}

// TranscriptTurn is a single speaker utterance.
type TranscriptTurn struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// ProxyMetrics captures deterministic transcript quality signals.
type ProxyMetrics struct {
	TotalTurns          int     `json:"total_turns"`
	UniqueSpeakers      int     `json:"unique_speakers"`
	AvgWordsPerTurn     float64 `json:"avg_words_per_turn"`
	SpeakerBalanceRatio float64 `json:"speaker_balance_ratio"`
	DirectReferenceRate float64 `json:"direct_reference_rate"`
	QuestionRate        float64 `json:"question_rate"`
	RepetitionRate      float64 `json:"repetition_rate"`
}

// DimensionScores is the model-judged rubric.
type DimensionScores struct {
	FlowArc            float64 `json:"flow_arc"`
	PersonaSeparation  float64 `json:"persona_separation"`
	Responsiveness     float64 `json:"responsiveness"`
	Naturalness        float64 `json:"naturalness"`
	IdeaQuality        float64 `json:"idea_quality"`
	ConvergenceQuality float64 `json:"convergence_quality"`
	ReportQuality      float64 `json:"report_quality"`
}

// WeightedAverage returns weighted mean across all dimensions.
func (d DimensionScores) WeightedAverage(weights DimensionScores) float64 {
	type pair struct {
		score  float64
		weight float64
	}
	pairs := []pair{
		{d.FlowArc, weights.FlowArc},
		{d.PersonaSeparation, weights.PersonaSeparation},
		{d.Responsiveness, weights.Responsiveness},
		{d.Naturalness, weights.Naturalness},
		{d.IdeaQuality, weights.IdeaQuality},
		{d.ConvergenceQuality, weights.ConvergenceQuality},
		{d.ReportQuality, weights.ReportQuality},
	}
	var num float64
	var den float64
	for _, p := range pairs {
		if p.weight <= 0 {
			continue
		}
		num += p.score * p.weight
		den += p.weight
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// JudgeScore captures LLM-as-judge output.
type JudgeScore struct {
	Model      string          `json:"model"`
	Dimensions DimensionScores `json:"dimensions"`
	Overall    float64         `json:"overall"`
	Summary    string          `json:"summary"`
	StrengthA  string          `json:"strength_a"`
	StrengthB  string          `json:"strength_b"`
	RiskA      string          `json:"risk_a"`
	RiskB      string          `json:"risk_b"`
}

// RunRecord is one protocol execution under one model/variant/scenario.
type RunRecord struct {
	Protocol   string           `json:"protocol"`
	Model      string           `json:"model"`
	Variant    string           `json:"variant"`
	ScenarioID string           `json:"scenario_id"`
	Run        int              `json:"run"`
	DurationMs int64            `json:"duration_ms"`
	Turns      []TranscriptTurn `json:"turns,omitempty"`
	Shortlist  []string         `json:"shortlist,omitempty"`
	Final      string           `json:"final,omitempty"`
	Proxy      ProxyMetrics     `json:"proxy"`
	Judge      *JudgeScore      `json:"judge,omitempty"`
	Error      string           `json:"error,omitempty"`
}

// Summary aggregates run records per model+variant.
type Summary struct {
	Protocol       string          `json:"protocol"`
	Model          string          `json:"model"`
	Variant        string          `json:"variant"`
	Description    string          `json:"description,omitempty"`
	Runs           int             `json:"runs"`
	Successes      int             `json:"successes"`
	Failures       int             `json:"failures"`
	SuccessRate    float64         `json:"success_rate"`
	AvgDurationMs  int64           `json:"avg_duration_ms"`
	Proxy          ProxyMetrics    `json:"proxy"`
	JudgeOverall   float64         `json:"judge_overall"`
	JudgeDimension DimensionScores `json:"judge_dimensions"`
}

// Report is the persisted evaluation output.
type Report struct {
	GeneratedAt string      `json:"generated_at"`
	Protocol    string      `json:"protocol"`
	Models      []string    `json:"models"`
	RunsPerCase int         `json:"runs_per_case"`
	Scenarios   []Scenario  `json:"scenarios"`
	Summaries   []Summary   `json:"summaries"`
	Runs        []RunRecord `json:"runs"`
}

// AggregateResult is used by runner implementers.
type AggregateResult struct {
	Report   Report
	Duration time.Duration
}
