package event

import (
	"crypto/rand"
	"encoding/hex"
)

const idBytes = 16

// NewID returns a secure random event ID.
func NewID() string {
	buf := make([]byte, idBytes)
	if _, err := rand.Read(buf); err != nil {
		// Fall back to zero buffer; rand.Read should not fail on supported platforms.
		return hex.EncodeToString(buf)
	}
	return hex.EncodeToString(buf)
}
