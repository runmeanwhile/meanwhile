package pulse

import (
	"testing"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
)

func TestPulseCheckInitialPositions(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	positions := p.Positions()
	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(positions))
	}

	for _, pos := range positions {
		if pos.Position != PositionPending {
			t.Fatalf("expected pending position, got %s", pos.Position)
		}
	}
}

func TestRecordPosition(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}}
	p := New(agents)

	p.RecordPosition("Alice", PositionAgree, "looks good", nil)

	positions := p.Positions()
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}

	if positions[0].Position != PositionAgree {
		t.Fatalf("expected agree, got %s", positions[0].Position)
	}
	if positions[0].Reasoning != "looks good" {
		t.Fatalf("expected reasoning, got %s", positions[0].Reasoning)
	}
}

func TestAllSignaled(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	if p.AllSignaled() {
		t.Fatal("expected false, all agents still pending")
	}

	p.RecordPosition("Alice", PositionAgree, "good", nil)
	if p.AllSignaled() {
		t.Fatal("expected false, Bob still pending")
	}

	p.RecordPosition("Bob", PositionAgree, "good", nil)
	if !p.AllSignaled() {
		t.Fatal("expected true, all agents signaled")
	}
}

func TestHasBlockers(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	if p.HasBlockers() {
		t.Fatal("expected no blockers initially")
	}

	p.RecordPosition("Alice", PositionBlock, "security concern", nil)
	if !p.HasBlockers() {
		t.Fatal("expected blocker after Alice blocks")
	}
}

func TestStateFullAgreement(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	p.RecordPosition("Alice", PositionAgree, "good", nil)
	p.RecordPosition("Bob", PositionAgree, "good", nil)

	state := p.State(2, 10)
	if state != StateFullAgreement {
		t.Fatalf("expected full agreement, got %s", state)
	}
}

func TestStateBlocked(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	p.RecordPosition("Alice", PositionAgree, "good", nil)
	p.RecordPosition("Bob", PositionBlock, "no way", nil)

	state := p.State(2, 10)
	if state != StateBlocked {
		t.Fatalf("expected blocked, got %s", state)
	}
}

func TestStateConditional(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	p.RecordPosition("Alice", PositionConditional, "ok if X", []string{"X must happen"})
	p.RecordPosition("Bob", PositionAgree, "good", nil)

	state := p.State(2, 10)
	if state != StateConditional {
		t.Fatalf("expected conditional, got %s", state)
	}
}

func TestStateUnresolved(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	p.RecordPosition("Bob", PositionAgree, "good", nil)

	state := p.State(10, 10)
	if state != StateUnresolved {
		t.Fatalf("expected unresolved, got %s", state)
	}
}

func TestConditions(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	p.RecordPosition("Alice", PositionConditional, "ok", []string{"cond1", "cond2"})
	p.RecordPosition("Bob", PositionConditional, "ok", []string{"cond3"})

	conditions := p.Conditions()
	if len(conditions) != 3 {
		t.Fatalf("expected 3 conditions, got %d", len(conditions))
	}
}

func TestBlockingIssues(t *testing.T) {
	agents := []agent.Agent{{Name: "Alice"}, {Name: "Bob"}}
	p := New(agents)

	p.RecordPosition("Alice", PositionBlock, "security risk", nil)
	p.RecordPosition("Bob", PositionBlock, "too expensive", nil)

	issues := p.BlockingIssues()
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
}
