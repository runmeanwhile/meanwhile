package provider

import (
	"context"

	"github.com/runmeanwhile/meanwhile/pkg/modelruntime"
)

// ErrResilientReplayMismatch indicates the provider stream diverged after a retry.
var ErrResilientReplayMismatch = modelruntime.ErrResilientReplayMismatch

// ResilientConfig configures retry behavior for provider streams.
type ResilientConfig = modelruntime.ResilientConfig

// DefaultResilientConfig returns default retry settings.
func DefaultResilientConfig() ResilientConfig {
	return modelruntime.DefaultResilientConfig()
}

// ResilientStream wraps a provider stream with retry logic.
type ResilientStream = modelruntime.ResilientStream

// NewResilientStream creates a resilient stream that retries on transient errors.
func NewResilientStream(ctx context.Context, create func(context.Context) (Stream, error), cfg ResilientConfig) *ResilientStream {
	return modelruntime.NewResilientStream(ctx, create, cfg)
}
