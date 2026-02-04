package minutes

import "testing"

func TestMinutesPayloadIncludesStructuredFields(t *testing.T) {
	mins := New()
	mins.AddDecision(Decision{Statement: "Ship the beta", Rationale: "User demand"})
	mins.AddActionItem(ActionItem{Description: "Draft release notes", Owner: "PM"})
	mins.AddOpenQuestion(OpenQuestion{Question: "Do we need a migration?"})
	mins.AddRisk(Risk{Description: "Support load spike", Severity: "medium"})
	mins.AddAssumption(Assumption{Statement: "We can staff on-call"})
	mins.AddNote("Capture feedback from design partners")
	mins.SetSummary("Agreement reached on beta launch.")

	payload := mins.Payload()

	if payload["summary"] != "Agreement reached on beta launch." {
		t.Fatalf("expected summary in payload, got %#v", payload["summary"])
	}

	decisions, ok := payload["decisions"].([]Decision)
	if !ok || len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %#v", payload["decisions"])
	}

	actions, ok := payload["actions"].([]ActionItem)
	if !ok || len(actions) != 1 {
		t.Fatalf("expected 1 action, got %#v", payload["actions"])
	}

	questions, ok := payload["open_questions"].([]OpenQuestion)
	if !ok || len(questions) != 1 {
		t.Fatalf("expected 1 open question, got %#v", payload["open_questions"])
	}

	risks, ok := payload["risks"].([]Risk)
	if !ok || len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %#v", payload["risks"])
	}

	assumptions, ok := payload["assumptions"].([]Assumption)
	if !ok || len(assumptions) != 1 {
		t.Fatalf("expected 1 assumption, got %#v", payload["assumptions"])
	}

	notes, ok := payload["notes"].([]string)
	if !ok || len(notes) != 1 {
		t.Fatalf("expected 1 note, got %#v", payload["notes"])
	}
}

func TestMinutesSkipsEmptyEntries(t *testing.T) {
	mins := New()
	mins.AddDecision(Decision{})
	mins.AddActionItem(ActionItem{})
	mins.AddOpenQuestion(OpenQuestion{})
	mins.AddRisk(Risk{})
	mins.AddAssumption(Assumption{})
	mins.AddNote("   ")

	payload := mins.Payload()

	if _, ok := payload["decisions"]; ok {
		t.Fatal("expected no decisions in payload")
	}
	if _, ok := payload["actions"]; ok {
		t.Fatal("expected no actions in payload")
	}
	if _, ok := payload["open_questions"]; ok {
		t.Fatal("expected no open questions in payload")
	}
	if _, ok := payload["risks"]; ok {
		t.Fatal("expected no risks in payload")
	}
	if _, ok := payload["assumptions"]; ok {
		t.Fatal("expected no assumptions in payload")
	}
	if _, ok := payload["notes"]; ok {
		t.Fatal("expected no notes in payload")
	}
}
