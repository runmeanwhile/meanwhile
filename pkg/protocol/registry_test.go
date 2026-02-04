package protocol

import "testing"

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.Register("test", func(_ Config) Protocol { return nil })

	if _, ok := reg.Get("test"); !ok {
		t.Fatal("expected factory")
	}
	if _, ok := reg.Get("missing"); ok {
		t.Fatal("unexpected factory")
	}
}
