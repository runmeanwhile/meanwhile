package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestWorkerDispatchesJobs(t *testing.T) {
	driver := NewInMemoryDriver()
	runAt := time.Now().UTC().Add(-time.Second)
	if err := driver.Schedule(context.Background(), Job{ID: "job-1", Type: "test", RunAt: runAt}); err != nil {
		t.Fatalf("schedule job: %v", err)
	}

	handled := make(chan Job, 1)
	worker, err := NewWorker(driver, func(_ context.Context, job Job) error {
		handled <- job
		return nil
	}, WithInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	select {
	case job := <-handled:
		if job.ID != "job-1" {
			t.Fatalf("unexpected job id %s", job.ID)
		}
		cancel()
	case <-ctx.Done():
		t.Fatalf("timeout waiting for job")
	}

	<-done
}
