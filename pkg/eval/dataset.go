package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Dataset is a collection of scenarios.
type Dataset struct {
	Name      string     `json:"name"`
	Protocol  string     `json:"protocol,omitempty"`
	Scenarios []Scenario `json:"scenarios"`
}

// LoadDatasetJSON reads dataset JSON from disk.
func LoadDatasetJSON(path string) (Dataset, error) {
	if strings.TrimSpace(path) == "" {
		return Dataset{}, fmt.Errorf("dataset path required")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Dataset{}, fmt.Errorf("read dataset: %w", err)
	}
	var ds Dataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return Dataset{}, fmt.Errorf("parse dataset: %w", err)
	}
	if len(ds.Scenarios) == 0 {
		return Dataset{}, fmt.Errorf("dataset has no scenarios")
	}
	for i, s := range ds.Scenarios {
		if strings.TrimSpace(s.ID) == "" {
			return Dataset{}, fmt.Errorf("scenario %d missing id", i)
		}
		if strings.TrimSpace(s.Prompt) == "" {
			return Dataset{}, fmt.Errorf("scenario %q missing prompt", s.ID)
		}
	}
	return ds, nil
}
