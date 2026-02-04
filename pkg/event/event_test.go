package event

import "testing"

func TestNewID(t *testing.T) {
	id1 := NewID()
	id2 := NewID()

	if len(id1) != 32 {
		t.Fatalf("expected id length 32, got %d", len(id1))
	}
	if id1 == id2 {
		t.Fatal("expected unique IDs")
	}
}
