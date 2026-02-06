package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/eval"
	"github.com/runmeanwhile/meanwhile/pkg/eval/brainstorm"
	"github.com/runmeanwhile/meanwhile/pkg/provider/openai"
)

func main() {
	var (
		protocolID       = flag.String("protocol", "brainstorming", "Protocol to evaluate")
		modelsCSV        = flag.String("models", "gpt-5.2-chat-latest", "Comma-separated models to evaluate")
		runsPerCase      = flag.Int("runs", 2, "Runs per model x variant x scenario")
		rounds           = flag.Int("rounds", 5, "Brainstorm interaction rounds")
		datasetPath      = flag.String("dataset", "", "Path to dataset JSON file")
		judgeModel       = flag.String("judge-model", "gpt-5.2-chat-latest", "Judge model")
		disableJudge     = flag.Bool("disable-judge", false, "Disable model-based judging")
		runTimeout       = flag.Duration("run-timeout", 2*time.Minute, "Per-run timeout")
		outputPath       = flag.String("out", "", "Output report path")
		showTurns        = flag.Bool("show-turns", false, "Print transcript turns while running")
		baselinePath     = flag.String("baseline", "", "Path to baseline report JSON for regression checks")
		maxOverallDrop   = flag.Float64("max-overall-drop", 0.25, "Maximum allowed weighted overall score drop")
		maxCriticalDrop  = flag.Float64("max-critical-drop", 0.40, "Maximum allowed drop for critical dimensions")
		requireAllKeys   = flag.Bool("require-all-keys", true, "Fail regression if baseline/current summary keys do not match")
		failOnRegression = flag.Bool("fail-on-regression", true, "Exit non-zero if regression gate fails")
	)
	flag.Parse()

	if strings.TrimSpace(*protocolID) != "brainstorming" {
		log.Fatalf("unsupported protocol %q (currently only brainstorming is implemented)", *protocolID)
	}
	if *runsPerCase <= 0 {
		log.Fatal("runs must be > 0")
	}
	if *rounds <= 0 {
		log.Fatal("rounds must be > 0")
	}

	models := parseCSV(*modelsCSV)
	if len(models) == 0 {
		log.Fatal("at least one model required")
	}

	scenarios, err := loadScenarios(*datasetPath)
	if err != nil {
		log.Fatal(err)
	}

	provider, err := openai.FromEnv()
	if err != nil {
		log.Fatal(err)
	}

	var judge eval.Judge
	if !*disableJudge {
		j, err := brainstorm.NewOpenAIJudge(provider, *judgeModel)
		if err != nil {
			log.Fatalf("init judge: %v", err)
		}
		judge = j
	}

	runner := brainstorm.NewRunner(provider)
	ctx := context.Background()
	result, err := runner.Run(ctx, brainstorm.Config{
		Models:      models,
		RunsPerCase: *runsPerCase,
		Scenarios:   scenarios,
		Variants:    brainstorm.DefaultVariants(*rounds),
		ShowTurns:   *showTurns,
		RunTimeout:  *runTimeout,
		Judge:       judge,
	})
	if err != nil {
		log.Fatal(err)
	}

	printSummary(result.Report)

	reportPath := *outputPath
	if strings.TrimSpace(reportPath) == "" {
		reportPath = defaultOutputPath(*protocolID)
	}
	if err := writeJSON(reportPath, result.Report); err != nil {
		log.Fatalf("write report: %v", err)
	}
	fmt.Printf("\nSaved report: %s\n", reportPath)

	if strings.TrimSpace(*baselinePath) != "" {
		baseline, err := readReport(*baselinePath)
		if err != nil {
			log.Fatalf("load baseline: %v", err)
		}
		reg := eval.CompareReports(result.Report, baseline, eval.RegressionConfig{
			Weights:         eval.DefaultDimensionWeights(),
			MaxOverallDrop:  *maxOverallDrop,
			MaxCriticalDrop: *maxCriticalDrop,
			CriticalDims:    eval.CriticalDimensionNames(),
			RequireAllKeys:  *requireAllKeys,
		})
		printRegression(reg)
		regPath := filepath.Join(filepath.Dir(reportPath), "regression.json")
		if err := writeJSON(regPath, reg); err != nil {
			log.Fatalf("write regression report: %v", err)
		}
		fmt.Printf("Saved regression report: %s\n", regPath)
		if *failOnRegression && !reg.Passed {
			os.Exit(2)
		}
	}
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadScenarios(datasetPath string) ([]eval.Scenario, error) {
	if strings.TrimSpace(datasetPath) == "" {
		return brainstorm.DefaultScenarios(), nil
	}
	ds, err := eval.LoadDatasetJSON(datasetPath)
	if err != nil {
		return nil, err
	}
	return ds.Scenarios, nil
}

func defaultOutputPath(protocolID string) string {
	ts := time.Now().UTC().Format("20060102-150405")
	return filepath.Join("artifacts", "evals", protocolID, ts, "report.json")
}

func writeJSON(path string, v any) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readReport(path string) (eval.Report, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return eval.Report{}, err
	}
	var report eval.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return eval.Report{}, err
	}
	return report, nil
}

func printSummary(report eval.Report) {
	fmt.Println("\nEval summary:")
	fmt.Println("| Model | Variant | Runs | Success | Proxy(balance/ref/repeat) | Judge(overall) |")
	fmt.Println("| --- | --- | ---: | ---: | --- | ---: |")
	for _, s := range report.Summaries {
		fmt.Printf("| %s | %s | %d | %.2f | %.2f / %.2f / %.2f | %.2f |\n",
			s.Model,
			s.Variant,
			s.Runs,
			s.SuccessRate,
			s.Proxy.SpeakerBalanceRatio,
			s.Proxy.DirectReferenceRate,
			s.Proxy.RepetitionRate,
			s.JudgeOverall,
		)
	}
}

func printRegression(result eval.RegressionResult) {
	fmt.Println("\nRegression check:")
	if result.Passed {
		fmt.Println("- PASSED")
	} else {
		fmt.Println("- FAILED")
	}
	for _, d := range result.Deltas {
		fmt.Printf("- %s overall_drop=%.3f", d.Key, d.OverallDrop)
		if d.Failure != "" {
			fmt.Printf(" failure=%s", d.Failure)
		}
		fmt.Println()
	}
}
