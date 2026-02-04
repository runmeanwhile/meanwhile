package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/agent"
)

// HumanRequestStatus represents the lifecycle status of a human request.
type HumanRequestStatus string

const (
	// HumanRequestStatusPending indicates the request is awaiting dispatch or response.
	HumanRequestStatusPending HumanRequestStatus = "pending"
	// HumanRequestStatusSent indicates the request was dispatched to an integration.
	HumanRequestStatusSent HumanRequestStatus = "sent"
	// HumanRequestStatusFailed indicates dispatch failed.
	HumanRequestStatusFailed HumanRequestStatus = "failed"
	// HumanRequestStatusTimedOut indicates the request timed out.
	HumanRequestStatusTimedOut HumanRequestStatus = "timed_out"
	// HumanRequestStatusAnswered indicates a human response was received.
	HumanRequestStatusAnswered HumanRequestStatus = "answered"
)

// HumanRequestDispatch captures delivery details for a dispatched request.
type HumanRequestDispatch struct {
	IntegrationID string    `json:"integration_id"`
	Channel       string    `json:"channel"`
	Contact       string    `json:"contact"`
	DispatchedAt  time.Time `json:"dispatched_at"`
}

// HumanRequestFailure captures a dispatch failure.
type HumanRequestFailure struct {
	Error    string    `json:"error"`
	FailedAt time.Time `json:"failed_at"`
}

// HumanRequestRecord captures the lifecycle state of a human request.
type HumanRequestRecord struct {
	Request         HumanRequest          `json:"request"`
	Status          HumanRequestStatus    `json:"status"`
	StatusUpdatedAt time.Time             `json:"status_updated_at"`
	Dispatch        *HumanRequestDispatch `json:"dispatch,omitempty"`
	Failure         *HumanRequestFailure  `json:"failure,omitempty"`
	Response        *HumanResponse        `json:"response,omitempty"`
	TimedOutAt      time.Time             `json:"timed_out_at,omitempty"`
}

// HumanRequestFilter filters request listings.
type HumanRequestFilter struct {
	Statuses        []HumanRequestStatus
	SessionID       string
	ParticipantID   string
	RequestedAfter  time.Time
	RequestedBefore time.Time
	Limit           int
}

// HumanRequestStore persists human request lifecycle records.
type HumanRequestStore interface {
	Upsert(ctx context.Context, record HumanRequestRecord) error
	Get(ctx context.Context, requestID string) (HumanRequestRecord, error)
	List(ctx context.Context, filter HumanRequestFilter) ([]HumanRequestRecord, error)
	Delete(ctx context.Context, requestID string) error
}

// InMemoryHumanRequestStore stores requests in memory.
type InMemoryHumanRequestStore struct {
	mu       sync.RWMutex
	requests map[string]HumanRequestRecord
}

// NewInMemoryHumanRequestStore creates an in-memory request store.
func NewInMemoryHumanRequestStore() *InMemoryHumanRequestStore {
	return &InMemoryHumanRequestStore{
		requests: make(map[string]HumanRequestRecord),
	}
}

// Upsert creates or updates a request record.
func (s *InMemoryHumanRequestStore) Upsert(_ context.Context, record HumanRequestRecord) error {
	if record.Request.RequestID == "" {
		return fmt.Errorf("request id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[record.Request.RequestID] = cloneHumanRequestRecord(record)
	return nil
}

// Get retrieves a request record by ID.
func (s *InMemoryHumanRequestStore) Get(_ context.Context, requestID string) (HumanRequestRecord, error) {
	if requestID == "" {
		return HumanRequestRecord{}, fmt.Errorf("request id required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.requests[requestID]
	if !ok {
		return HumanRequestRecord{}, ErrHumanRequestNotFound
	}
	return cloneHumanRequestRecord(record), nil
}

// List returns request records matching the filter.
func (s *InMemoryHumanRequestStore) List(_ context.Context, filter HumanRequestFilter) ([]HumanRequestRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]HumanRequestRecord, 0, len(s.requests))
	for _, record := range s.requests {
		if !matchHumanRequestFilter(record, filter) {
			continue
		}
		records = append(records, cloneHumanRequestRecord(record))
	}

	sort.Slice(records, func(i, j int) bool {
		if !records[i].Request.RequestedAt.Equal(records[j].Request.RequestedAt) {
			return records[i].Request.RequestedAt.After(records[j].Request.RequestedAt)
		}
		return records[i].Request.RequestID < records[j].Request.RequestID
	})

	limit := filter.Limit
	if limit <= 0 || limit > len(records) {
		limit = len(records)
	}
	return records[:limit], nil
}

// Delete removes a request record.
func (s *InMemoryHumanRequestStore) Delete(_ context.Context, requestID string) error {
	if requestID == "" {
		return fmt.Errorf("request id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[requestID]; !ok {
		return ErrHumanRequestNotFound
	}
	delete(s.requests, requestID)
	return nil
}

// ParseHumanRequestStatus parses a status string into a known status value.
func ParseHumanRequestStatus(value string) (HumanRequestStatus, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch HumanRequestStatus(trimmed) {
	case HumanRequestStatusPending,
		HumanRequestStatusSent,
		HumanRequestStatusFailed,
		HumanRequestStatusTimedOut,
		HumanRequestStatusAnswered:
		return HumanRequestStatus(trimmed), nil
	case "":
		return "", fmt.Errorf("status required")
	default:
		return "", fmt.Errorf("unknown status: %s", value)
	}
}

func matchHumanRequestFilter(record HumanRequestRecord, filter HumanRequestFilter) bool {
	if len(filter.Statuses) > 0 {
		match := false
		for _, status := range filter.Statuses {
			if record.Status == status {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if filter.SessionID != "" && record.Request.SessionID != filter.SessionID {
		return false
	}
	if filter.ParticipantID != "" && record.Request.ParticipantID != filter.ParticipantID {
		return false
	}
	if !filter.RequestedAfter.IsZero() && !record.Request.RequestedAt.After(filter.RequestedAfter) {
		return false
	}
	if !filter.RequestedBefore.IsZero() && !record.Request.RequestedAt.Before(filter.RequestedBefore) {
		return false
	}
	return true
}

func cloneHumanRequestRecord(record HumanRequestRecord) HumanRequestRecord {
	record.Request = cloneHumanRequest(record.Request)
	if record.Dispatch != nil {
		dispatch := *record.Dispatch
		record.Dispatch = &dispatch
	}
	if record.Failure != nil {
		failure := *record.Failure
		record.Failure = &failure
	}
	if record.Response != nil {
		response := *record.Response
		response.Response = cloneAgentMessage(response.Response)
		record.Response = &response
	}
	return record
}

func cloneHumanRequest(req HumanRequest) HumanRequest {
	req.SuggestedResponses = append([]string(nil), req.SuggestedResponses...)
	return req
}

func cloneAgentMessage(msg agent.Message) agent.Message {
	msg.Metadata = cloneMetadata(msg.Metadata)
	if len(msg.Parts) > 0 {
		parts := make([]agent.ContentPart, len(msg.Parts))
		for i, part := range msg.Parts {
			parts[i] = cloneContentPart(part)
		}
		msg.Parts = parts
	}
	return msg
}

func cloneContentPart(part agent.ContentPart) agent.ContentPart {
	if len(part.Data) > 0 {
		data := make([]byte, len(part.Data))
		copy(data, part.Data)
		part.Data = data
	}
	if part.Metadata != nil {
		part.Metadata = cloneMetadata(part.Metadata)
	}
	return part
}
