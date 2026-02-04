package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(path, []byte(`{"global":{"providers":{"openai":{"type":"openai"}}}}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load json: %v", err)
	}
	if cfg.Global.Providers["openai"].Type != "openai" {
		t.Fatalf("expected provider type openai")
	}
}

func TestLoadFileYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("global:\n  providers:\n    openai:\n      type: openai\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load yaml: %v", err)
	}
	if cfg.Global.Providers["openai"].Type != "openai" {
		t.Fatalf("expected provider type openai")
	}
}

func TestLoadFileUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := LoadFile(path); err == nil {
		t.Fatal("expected error for unknown format")
	}
}
