package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	// ErrUnknownFormat indicates unsupported config format.
	ErrUnknownFormat = errors.New("unknown config format")
)

// LoadFile loads configuration from a JSON or YAML file.
func LoadFile(path string) (Config, error) {
	clean := filepath.Clean(path)
	data, err := os.ReadFile(clean)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	switch ext := filepath.Ext(clean); ext {
	case ".json":
		return loadJSON(data)
	case ".yaml", ".yml":
		return loadYAML(data)
	default:
		return Config{}, ErrUnknownFormat
	}
}

func loadJSON(data []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode json: %w", err)
	}
	return cfg, nil
}

func loadYAML(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode yaml: %w", err)
	}
	return cfg, nil
}
