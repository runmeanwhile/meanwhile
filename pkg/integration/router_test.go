package integration

import (
	"context"
	"errors"
	"testing"
)

type mockIntegration struct {
	id      string
	channel string
	err     error
	calls   int
}

func (m *mockIntegration) ID() string      { return m.id }
func (m *mockIntegration) Channel() string { return m.channel }
func (m *mockIntegration) Send(_ context.Context, _ Request) error {
	m.calls++
	return m.err
}

func TestRouterDispatchPrefersChannel(t *testing.T) {
	reg := NewRegistry()
	email := &mockIntegration{id: "email-1", channel: "email"}
	slack := &mockIntegration{id: "slack-1", channel: "slack"}
	if err := reg.Register(email); err != nil {
		t.Fatalf("register email: %v", err)
	}
	if err := reg.Register(slack); err != nil {
		t.Fatalf("register slack: %v", err)
	}

	router := NewRouter(reg)
	contacts := map[string]string{
		"email": "user@example.com",
		"slack": "U123",
	}
	result, err := router.Dispatch(context.Background(), Request{RequestID: "req-1"}, contacts, "email")
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if result.IntegrationID != "email-1" {
		t.Fatalf("expected email integration, got %s", result.IntegrationID)
	}
	if email.calls != 1 || slack.calls != 0 {
		t.Fatalf("expected email called once, slack none; email=%d slack=%d", email.calls, slack.calls)
	}
}

func TestRouterDispatchFallsBackOnError(t *testing.T) {
	reg := NewRegistry()
	email := &mockIntegration{id: "email-1", channel: "email", err: errors.New("fail")}
	slack := &mockIntegration{id: "slack-1", channel: "slack"}
	if err := reg.Register(email); err != nil {
		t.Fatalf("register email: %v", err)
	}
	if err := reg.Register(slack); err != nil {
		t.Fatalf("register slack: %v", err)
	}

	router := NewRouter(reg)
	contacts := map[string]string{
		"email": "user@example.com",
		"slack": "U123",
	}
	result, err := router.Dispatch(context.Background(), Request{RequestID: "req-2"}, contacts, "email")
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if result.IntegrationID != "slack-1" {
		t.Fatalf("expected slack fallback, got %s", result.IntegrationID)
	}
	if email.calls != 1 || slack.calls != 1 {
		t.Fatalf("expected both called once; email=%d slack=%d", email.calls, slack.calls)
	}
}

func TestRouterDispatchRequiresContacts(t *testing.T) {
	reg := NewRegistry()
	router := NewRouter(reg)
	if _, err := router.Dispatch(context.Background(), Request{}, nil, ""); err == nil {
		t.Fatalf("expected error for missing contacts")
	}
}
