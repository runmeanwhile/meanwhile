// Package logger provides event logging with workplace-themed formatting.
//
// The logger package offers clean, human-readable output for Meanwhile sessions,
// emphasizing the "clean workplace log" brand promise. The Worklog formatter
// presents agent activities as a terse narrative suitable for 1999-2001 office scenarios.
package logger

import (
	"github.com/darkostanimirovic/meanwhile/pkg/event"
)

// Logger formats and outputs events from Meanwhile sessions.
type Logger interface {
	// Log processes a single event and writes formatted output.
	Log(event.Event) error
}
