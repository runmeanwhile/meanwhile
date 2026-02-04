package scheduler

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisDriverScheduleClaimCancel(t *testing.T) {
	redisServer := miniredis.RunT(t)
	driver, err := NewRedisDriver(redis.UniversalOptions{Addrs: []string{redisServer.Addr()}})
	if err != nil {
		t.Fatalf("new redis driver: %v", err)
	}

	runAt := time.Now().UTC().Add(-time.Second)
	job := Job{ID: "job-1", Type: "test", RunAt: runAt}
	if err := driver.Schedule(context.Background(), job); err != nil {
		t.Fatalf("schedule job: %v", err)
	}

	jobs, err := driver.ClaimDue(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("claim due: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("expected job-1, got %+v", jobs)
	}

	if err := driver.Cancel(context.Background(), job.ID); err != ErrJobNotFound {
		t.Fatalf("expected ErrJobNotFound after cancel, got %v", err)
	}
}
