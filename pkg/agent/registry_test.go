package agent

import "testing"

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.Register(Profile{ID: "p1"})

	if _, ok := reg.Get("p1"); !ok {
		t.Fatal("expected profile")
	}
}
