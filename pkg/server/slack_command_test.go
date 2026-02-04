package server

import "testing"

func TestParseSlackResponseText(t *testing.T) {
	requestID, response, err := parseSlackResponseText("respond req-123 scoped down version")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if requestID != "req-123" {
		t.Fatalf("expected request id, got %q", requestID)
	}
	if response != "scoped down version" {
		t.Fatalf("unexpected response %q", response)
	}
}

func TestParseSlackResponseTextRequiresResponse(t *testing.T) {
	if _, _, err := parseSlackResponseText("respond req-123"); err == nil {
		t.Fatalf("expected error for missing response")
	}
}
