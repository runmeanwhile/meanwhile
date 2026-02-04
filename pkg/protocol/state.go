package protocol

import (
	"encoding/json"
	"fmt"
)

// EncodeState converts a structured state into a serializable map.
func EncodeState(state any) (map[string]any, error) {
	if state == nil {
		return nil, nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("encode state: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode state map: %w", err)
	}
	return out, nil
}

// DecodeState restores structured state from a serialized map.
func DecodeState(state map[string]any, out any) error {
	if out == nil {
		return fmt.Errorf("state target required")
	}
	if len(state) == 0 {
		return nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode state map: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode state: %w", err)
	}
	return nil
}
