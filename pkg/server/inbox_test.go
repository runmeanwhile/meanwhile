package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/engine"
)

func TestHumanRequestInboxHandler(t *testing.T) {
	store := engine.NewInMemoryHumanRequestStore()
	eng, err := engine.New(engine.WithHumanRequestStore(store))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	now := time.Now().UTC()
	record := engine.HumanRequestRecord{
		Request: engine.HumanRequest{
			RequestID:   "req-1",
			SessionID:   "sess-1",
			Question:    "Need input",
			RequestedAt: now,
		},
		Status:          engine.HumanRequestStatusPending,
		StatusUpdatedAt: now,
	}
	if err := store.Upsert(context.Background(), record); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	handler := &HumanRequestInboxHandler{Engine: eng, MaxLimit: 10}
	req := httptest.NewRequest(http.MethodGet, "/inbox?status=pending&limit=5", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Count    int                         `json:"count"`
		Requests []engine.HumanRequestRecord `json:"requests"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count != 1 || len(payload.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", payload.Count)
	}
	if payload.Requests[0].Request.RequestID != "req-1" {
		t.Fatalf("unexpected request id %s", payload.Requests[0].Request.RequestID)
	}
}
