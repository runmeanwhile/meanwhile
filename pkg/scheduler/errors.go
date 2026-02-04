package scheduler

import "errors"

var (
	// ErrJobNotFound indicates a job could not be found for cancellation.
	ErrJobNotFound = errors.New("job not found")
)
