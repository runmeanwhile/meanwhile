package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryDriverScheduleClaimCancel(t *testing.T) {
	driver := NewInMemoryDriver()
	now := time.Now().UTC()

	jobDue := Job{ID: "job-1", Type: "test", RunAt: now.Add(-time.Second)}
	jobLater := Job{ID: "job-2", Type: "test", RunAt: now.Add(time.Minute)}

	if err := driver.Schedule(context.Background(), jobDue); err != nil {
		t.Fatalf("schedule due job: %v", err)
	}
	if err := driver.Schedule(context.Background(), jobLater); err != nil {
		t.Fatalf("schedule later job: %v", err)
	}

	jobs, err := driver.ClaimDue(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("claim due: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != jobDue.ID {
		t.Fatalf("expected only due job, got %+v", jobs)
	}

	jobs, err = driver.ClaimDue(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("claim due again: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %+v", jobs)
	}

	if err := driver.Cancel(context.Background(), jobLater.ID); err != nil {
		t.Fatalf("cancel job: %v", err)
	}
	if err := driver.Cancel(context.Background(), jobLater.ID); err != ErrJobNotFound {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}
