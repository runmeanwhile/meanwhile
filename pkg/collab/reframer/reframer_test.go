package reframer

import "testing"

func TestParseAndSelectDiverse(t *testing.T) {
	text := `- operational | How might we reduce meeting setup friction? | teams lose time
- behavioral | How might we make commitments feel urgent? | week 4 slippage
- operational | How might we auto-highlight stale owners? | visible pressure`
	frames := Parse(text)
	if len(frames) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(frames))
	}
	diverse := SelectDiverse(frames, 2)
	if len(diverse) != 2 {
		t.Fatalf("expected 2 diverse frames, got %d", len(diverse))
	}
	if diverse[0].Lens == diverse[1].Lens {
		t.Fatalf("expected unique lenses, got duplicated %q", diverse[0].Lens)
	}
}

func TestBuildPrompt(t *testing.T) {
	system, user := BuildPrompt("Improve planning quality", []string{"Users stop updating tasks"}, 6)
	if system == "" || user == "" {
		t.Fatal("expected non-empty prompts")
	}
}

func TestParseSanitizesMarkdownFields(t *testing.T) {
	text := `- **Operational** | **How might we reduce rerun meetings?** | *Focus attention*`
	frames := Parse(text)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if frames[0].Lens != "Operational" {
		t.Fatalf("unexpected lens: %q", frames[0].Lens)
	}
	if frames[0].HMW != "How might we reduce rerun meetings?" {
		t.Fatalf("unexpected hmw: %q", frames[0].HMW)
	}
	if frames[0].Rationale != "Focus attention" {
		t.Fatalf("unexpected rationale: %q", frames[0].Rationale)
	}
}
