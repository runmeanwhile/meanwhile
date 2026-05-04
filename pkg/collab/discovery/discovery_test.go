package discovery

import "testing"

func TestParseNoteExtractsFields(t *testing.T) {
	text := `Question: Why do action items stall after week 3?
Evidence: replayed dashboard takes over the first 15 minutes [memory.kpi_drop]
Sources: usage.weekly_kpi, memory.kpi_drop
Uncertainty: We still do not know whether role confusion or meeting format is the bigger driver.
Confidence: medium`

	note := ParseNote(text)
	if note.Question == "" {
		t.Fatal("expected question")
	}
	if len(note.SourceIDs) != 2 {
		t.Fatalf("expected 2 source IDs, got %d", len(note.SourceIDs))
	}
	if note.Uncertainty == "" {
		t.Fatal("expected uncertainty")
	}
}

func TestValidateStrictRequirement(t *testing.T) {
	note := ParseNote("We should improve meetings.")
	req := DefaultRequirement()
	issues := Validate(note, req)
	if len(issues) < 2 {
		t.Fatalf("expected validation issues, got %d", len(issues))
	}
}

func TestValidateAllowedSources(t *testing.T) {
	note := ParseNote(`Question: What changed?
Evidence: There is drop-off [usage.weekly_kpi]
Sources: usage.weekly_kpi, fake.source
Uncertainty: Need cohort split.`)
	req := DefaultRequirement()
	req.AllowedSourceIDs = []string{"usage.weekly_kpi", "memory.kpi_drop"}

	issues := Validate(note, req)
	foundUnknown := false
	for _, issue := range issues {
		if issue.Field == "source_ids" && issue.Message != "" {
			foundUnknown = true
			break
		}
	}
	if !foundUnknown {
		t.Fatal("expected unknown source validation issue")
	}
}
