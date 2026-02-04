package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeFS constrains filesystem access to allowed roots.
type SafeFS struct {
	roots       []string
	defaultRoot string
	cfg         Config
}

// NewSafeFS validates config and constructs a SafeFS.
func NewSafeFS(cfg Config) (*SafeFS, error) {
	cfg = normalizeConfig(cfg)
	roots := make([]string, 0, len(cfg.Roots))
	for _, root := range cfg.Roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return nil, fmt.Errorf("resolve root %q: %w", root, err)
		}
		roots = append(roots, filepath.Clean(abs))
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("at least one root required")
	}
	return &SafeFS{
		roots:       roots,
		defaultRoot: roots[0],
		cfg:         cfg,
	}, nil
}

// Resolve resolves a user path to an allowed absolute path.
func (fs *SafeFS) Resolve(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("path required")
	}
	cleaned := filepath.Clean(trimmed)
	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = cleaned
	} else {
		candidate = filepath.Join(fs.defaultRoot, cleaned)
	}

	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	abs = filepath.Clean(abs)

	checked := abs
	if resolved, ok := resolveExistingPath(abs); ok {
		checked = resolved
	}

	for _, root := range fs.roots {
		if withinRoot(root, checked) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path outside allowed roots")
}

func resolveExistingPath(path string) (string, bool) {
	_, err := os.Stat(path)
	if err == nil {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", false
		}
		return resolved, true
	}
	if !os.IsNotExist(err) {
		return "", false
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", false
	}
	_, err = os.Stat(parent)
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", false
	}
	return resolved, true
}

func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return true
	}
	prefix := ".." + string(os.PathSeparator)
	if rel == ".." || strings.HasPrefix(rel, prefix) {
		return false
	}
	return true
}
