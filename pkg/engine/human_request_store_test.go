package engine

import (
	"context"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

func TestInMemoryHumanRequestStoreCRUD(t *testing.T) {
	store := NewInMemoryHumanRequestStore()
	now := time.Now().UTC()

	record := HumanRequestRecord{
		Request: HumanRequest{
			RequestID:   "req-1",
			SessionID:   "sess-1",
			Question:    "Need input",
			RequestedAt: now,
		},
		Status:          HumanRequestStatusPending,
		StatusUpdatedAt: now,
		Response: &HumanResponse{
			RequestID:  "req-1",
			SessionID:  "sess-1",
			ReceivedAt: now,
			Response: agent.Message{
				Role: agent.RoleUser,
				Parts: []agent.ContentPart{{
					Type: agent.ContentPartText,
					Text: "ok",
				}},
			},
		},
	}

	if err := store.Upsert(context.Background(), record); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := store.Get(context.Background(), "req-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Request.RequestID != "req-1" {
		t.Fatalf("expected request id req-1, got %s", got.Request.RequestID)
	}
	if got.Response == nil || got.Response.Response.Text() != "ok" {
		t.Fatalf("expected response text ok")
	}

	got.Request.Question = "mutated"
	reloaded, err := store.Get(context.Background(), "req-1")
	if err != nil {
		t.Fatalf("get reload: %v", err)
	}
	if reloaded.Request.Question != "Need input" {
		t.Fatalf("expected stored question to be immutable, got %s", reloaded.Request.Question)
	}

	list, err := store.List(context.Background(), HumanRequestFilter{
		Statuses:  []HumanRequestStatus{HumanRequestStatusPending},
		SessionID: "sess-1",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 record, got %d", len(list))
	}
}

func TestParseHumanRequestStatus(t *testing.T) {
	status, err := ParseHumanRequestStatus("pending")
	if err != nil {
		t.Fatalf("parse pending: %v", err)
	}
	if status != HumanRequestStatusPending {
		t.Fatalf("expected pending status")
	}
	if _, err := ParseHumanRequestStatus("unknown"); err == nil {
		t.Fatalf("expected error for unknown status")
	}
}
