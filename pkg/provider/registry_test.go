package provider

import (
	"context"
	"testing"
)

type dummyProvider struct{ id string }

func (d dummyProvider) ID() string { return d.id }
func (d dummyProvider) Stream(_ context.Context, _ Request) (Stream, error) {
	return nil, nil
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.Register(dummyProvider{id: "p1"})
	if _, ok := reg.Get("p1"); !ok {
		t.Fatal("expected provider")
	}
}
