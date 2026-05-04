package ideationops

import "testing"

func TestForRoundDeterministic(t *testing.T) {
	a := ForRound(1, 3)
	b := ForRound(1, 3)
	if len(a) != len(b) {
		t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("operator mismatch at %d: %q vs %q", i, a[i].ID, b[i].ID)
		}
	}
}

func TestForRoundRotates(t *testing.T) {
	r0 := ForRound(0, 2)
	r1 := ForRound(1, 2)
	if len(r0) != 2 || len(r1) != 2 {
		t.Fatalf("unexpected lengths: %d %d", len(r0), len(r1))
	}
	if r0[0].ID == r1[0].ID {
		t.Fatalf("expected first operator to rotate across rounds")
	}
}
