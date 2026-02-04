package openai

import (
	"fmt"
	"os"
	"time"
)

// FromEnv builds a client using environment variables.
// Required: OPENAI_API_KEY
// Optional: OPENAI_BASE_URL, OPENAI_TIMEOUT (e.g. "30s", "2m")
func FromEnv() (*Client, error) {
	cfg := Config{APIKey: os.Getenv("OPENAI_API_KEY")}
	if cfg.APIKey == "" {
		return nil, ErrMissingAPIKey
	}

	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if timeout := os.Getenv("OPENAI_TIMEOUT"); timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("parse OPENAI_TIMEOUT: %w", err)
		}
		cfg.Timeout = d
	}

	return NewClient(cfg)
}
