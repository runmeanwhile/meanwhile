package openai

import (
	"fmt"
	"time"

	"github.com/darkostanimirovic/meanwhile/pkg/config"
	"github.com/darkostanimirovic/meanwhile/pkg/provider"
)

func init() {
	provider.RegisterFactory("openai", func(cfg config.ProviderConfig) (provider.Provider, error) {
		clientCfg := Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
		}
		if cfg.Params != nil {
			if v, ok := cfg.Params["timeout"]; ok {
				if d, err := parseTimeout(v); err == nil {
					clientCfg.Timeout = d
				} else {
					return nil, err
				}
			}
		}
		return NewClient(clientCfg)
	})
}

func parseTimeout(value any) (time.Duration, error) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return 0, nil
		}
		return time.ParseDuration(v)
	case float64:
		if v <= 0 {
			return 0, nil
		}
		return time.Duration(v * float64(time.Second)), nil
	case int:
		if v <= 0 {
			return 0, nil
		}
		return time.Duration(v) * time.Second, nil
	case int64:
		if v <= 0 {
			return 0, nil
		}
		return time.Duration(v) * time.Second, nil
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid timeout type %T", value)
	}
}
