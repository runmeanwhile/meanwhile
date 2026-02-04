package scheduler

import (
	"context"
	"fmt"
	"time"
)

// Handler processes a claimed job.
type Handler func(ctx context.Context, job Job) error

// ErrorHandler handles job processing errors.
type ErrorHandler func(ctx context.Context, job Job, err error)

// Worker polls a scheduler driver and dispatches due jobs.
type Worker struct {
	driver    Driver
	handler   Handler
	interval  time.Duration
	batchSize int
	clock     func() time.Time
	onError   ErrorHandler
}

// WorkerOption configures a Worker.
type WorkerOption func(*Worker)

// WithInterval sets the poll interval.
func WithInterval(interval time.Duration) WorkerOption {
	return func(w *Worker) {
		if interval > 0 {
			w.interval = interval
		}
	}
}

// WithBatchSize sets the maximum number of jobs claimed per poll.
func WithBatchSize(size int) WorkerOption {
	return func(w *Worker) {
		if size > 0 {
			w.batchSize = size
		}
	}
}

// WithClock overrides the clock used to determine due jobs.
func WithClock(clock func() time.Time) WorkerOption {
	return func(w *Worker) {
		if clock != nil {
			w.clock = clock
		}
	}
}

// WithErrorHandler handles errors without stopping the worker.
func WithErrorHandler(handler ErrorHandler) WorkerOption {
	return func(w *Worker) {
		w.onError = handler
	}
}

// NewWorker builds a worker for the provided driver and handler.
func NewWorker(driver Driver, handler Handler, opts ...WorkerOption) (*Worker, error) {
	if driver == nil {
		return nil, fmt.Errorf("driver required")
	}
	if handler == nil {
		return nil, fmt.Errorf("handler required")
	}
	w := &Worker{
		driver:    driver,
		handler:   handler,
		interval:  time.Second,
		batchSize: 50,
		clock:     func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		if opt != nil {
			opt(w)
		}
	}
	return w, nil
}

// Run starts polling until the context is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := w.runOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.runOnce(ctx); err != nil {
				return err
			}
		}
	}
}

// Close closes the underlying driver.
func (w *Worker) Close() error {
	if w == nil || w.driver == nil {
		return nil
	}
	return w.driver.Close()
}

func (w *Worker) runOnce(ctx context.Context) error {
	jobs, err := w.driver.ClaimDue(ctx, w.clock(), w.batchSize)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := w.handler(ctx, job); err != nil {
			if w.onError != nil {
				w.onError(ctx, job, err)
				continue
			}
			return err
		}
	}
	return nil
}
