package openai

import (
	"strings"
	"testing"
)

func TestSSEDecoder(t *testing.T) {
	raw := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\n" +
		"data: {\"type\":\"response.output_text.done\",\"text\":\"hi\"}\n\n"

	dec := newSSEDecoder(strings.NewReader(raw))
	first, err := dec.Next()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if string(first) == "" {
		t.Fatal("expected first event")
	}

	second, err := dec.Next()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(second) == "" {
		t.Fatal("expected second event")
	}
}
