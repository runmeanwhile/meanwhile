package engine

import (
	"testing"
	"time"

	"github.com/runmeanwhile/meanwhile/pkg/config"
	"github.com/runmeanwhile/meanwhile/pkg/contextpolicy"
)

func TestApplyConfigDurabilitySettings(t *testing.T) {
	enabled := false
	cfg := config.Config{
		Global: config.GlobalConfig{
			RunTimeoutSeconds: 12,
			ProviderRetry: config.ProviderRetryConfig{
				Enabled:         &enabled,
				MaxRetries:      9,
				InitialInterval: 2 * time.Second,
				MaxInterval:     5 * time.Second,
				Multiplier:      3,
			},
			Context: config.ContextConfig{
				AutoSummarize: config.AutoSummarizeConfig{
					Enabled:           &enabled,
					SummarizeAtTokens: 25,
					MinKeepMessages:   2,
				},
			},
		},
	}

	eng, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if eng.defaultRunTimeout != 12*time.Second {
		t.Fatalf("expected run timeout 12s, got %v", eng.defaultRunTimeout)
	}
	if eng.providerRetryEnabled {
		t.Fatalf("expected provider retry disabled")
	}
	if _, ok := eng.contextPolicy.(*contextpolicy.AutoSummarizePolicy); ok {
		t.Fatalf("expected auto-summarize disabled")
	}
}

func TestApplyConfigAutoSummarizeEnabled(t *testing.T) {
	cfg := config.Config{
		Global: config.GlobalConfig{
			Context: config.ContextConfig{
				AutoSummarize: config.AutoSummarizeConfig{
					SummarizeAtTokens: 10,
					MinKeepMessages:   1,
				},
			},
		},
	}

	eng, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if _, ok := eng.contextPolicy.(*contextpolicy.AutoSummarizePolicy); !ok {
		t.Fatalf("expected auto-summarize enabled")
	}
}
