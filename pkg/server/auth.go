package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SignatureVerifier verifies request signatures.
type SignatureVerifier interface {
	Verify(ctx context.Context, payload []byte, signature string) error
}

// HMACVerifier verifies HMAC-SHA256 signatures.
type HMACVerifier struct {
	secret []byte
}

// NewHMACVerifier creates an HMAC verifier.
func NewHMACVerifier(secret string) (*HMACVerifier, error) {
	if secret == "" {
		return nil, fmt.Errorf("hmac secret required")
	}
	return &HMACVerifier{secret: []byte(secret)}, nil
}

// Verify validates the signature for a payload.
func (h *HMACVerifier) Verify(_ context.Context, payload []byte, signature string) error {
	if h == nil {
		return fmt.Errorf("hmac verifier required")
	}
	if signature == "" {
		return fmt.Errorf("signature required")
	}
	decoded, err := hex.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	mac := hmac.New(sha256.New, h.secret)
	if _, err := mac.Write(payload); err != nil {
		return fmt.Errorf("signature write: %w", err)
	}
	expected := mac.Sum(nil)
	if !hmac.Equal(decoded, expected) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
