package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Job defines a scheduled unit of work.
type Job struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	RunAt   time.Time       `json:"run_at"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Driver persists and claims scheduled jobs.
// Implementations must remove jobs they return from ClaimDue.
type Driver interface {
	Schedule(ctx context.Context, job Job) error
	Cancel(ctx context.Context, jobID string) error
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]Job, error)
	Close() error
}

func validateJob(job Job) error {
	if job.ID == "" {
		return fmt.Errorf("job id required")
	}
	if job.RunAt.IsZero() {
		return fmt.Errorf("job run_at required")
	}
	return nil
}
