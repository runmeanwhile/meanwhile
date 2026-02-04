package scheduler

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// InMemoryDriver stores scheduled jobs in memory.
type InMemoryDriver struct {
	mu   sync.Mutex
	jobs map[string]Job
}

// NewInMemoryDriver creates an in-memory scheduler driver.
func NewInMemoryDriver() *InMemoryDriver {
	return &InMemoryDriver{
		jobs: make(map[string]Job),
	}
}

// Schedule upserts a job.
func (d *InMemoryDriver) Schedule(_ context.Context, job Job) error {
	if err := validateJob(job); err != nil {
		return err
	}
	job.RunAt = job.RunAt.UTC()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.jobs == nil {
		d.jobs = make(map[string]Job)
	}
	d.jobs[job.ID] = job
	return nil
}

// Cancel removes a job.
func (d *InMemoryDriver) Cancel(_ context.Context, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("job id required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.jobs) == 0 {
		return ErrJobNotFound
	}
	if _, ok := d.jobs[jobID]; !ok {
		return ErrJobNotFound
	}
	delete(d.jobs, jobID)
	return nil
}

// ClaimDue returns and removes jobs that are due.
func (d *InMemoryDriver) ClaimDue(_ context.Context, now time.Time, limit int) ([]Job, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.jobs) == 0 {
		return nil, nil
	}
	due := make([]Job, 0)
	for _, job := range d.jobs {
		if !job.RunAt.After(now) {
			due = append(due, job)
		}
	}
	if len(due) == 0 {
		return nil, nil
	}
	sort.Slice(due, func(i, j int) bool {
		if !due[i].RunAt.Equal(due[j].RunAt) {
			return due[i].RunAt.Before(due[j].RunAt)
		}
		return due[i].ID < due[j].ID
	})
	if len(due) > limit {
		due = due[:limit]
	}
	for _, job := range due {
		delete(d.jobs, job.ID)
	}
	return due, nil
}

// Close releases resources.
func (d *InMemoryDriver) Close() error {
	return nil
}
