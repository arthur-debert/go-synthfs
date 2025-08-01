package synthfs

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PathMode defines how paths are handled
type PathMode int

const (
	// PathModeAuto automatically detects and handles both absolute and relative paths
	PathModeAuto PathMode = iota
	// PathModeAbsolute forces all paths to be treated as absolute
	PathModeAbsolute
	// PathModeRelative forces all paths to be treated as relative to the base
	PathModeRelative
)

// PathHandler manages path resolution and normalization
type PathHandler struct {
	base string
	mode PathMode
}

// NewPathHandler creates a new path handler with the given base and mode
func NewPathHandler(base string, mode PathMode) *PathHandler {
	// Ensure base is clean and absolute
	if base == "" {
		base = "/"
	}
	base = filepath.Clean(base)
	if !filepath.IsAbs(base) {
		// Try to make it absolute
		if abs, err := filepath.Abs(base); err == nil {
			base = abs
		}
	}

	return &PathHandler{
		base: base,
		mode: mode,
	}
}

// ResolvePath resolves a path according to the handler's mode
func (ph *PathHandler) ResolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	// Clean the path first
	path = filepath.Clean(path)

	switch ph.mode {
	case PathModeAuto:
		return ph.resolveAuto(path)
	case PathModeAbsolute:
		return ph.resolveAbsolute(path)
	case PathModeRelative:
		return ph.resolveRelative(path)
	default:
		return "", fmt.Errorf("invalid path mode: %v", ph.mode)
	}
}

// resolveAuto automatically detects path type and resolves appropriately
func (ph *PathHandler) resolveAuto(path string) (string, error) {
	if filepath.IsAbs(path) {
		// Absolute path - use as is but validate it doesn't escape root
		if ph.base != "/" && !strings.HasPrefix(path, ph.base) {
			return "", fmt.Errorf("path %q escapes filesystem root in Auto mode (absolute path outside root %q)", path, ph.base)
		}
		return path, nil
	}

	// Relative path - resolve against base
	resolved := filepath.Join(ph.base, path)

	// Ensure resolved path doesn't escape the base
	if !strings.HasPrefix(resolved, ph.base) {
		return "", fmt.Errorf("path %q escapes filesystem root in Auto mode (resolved as %q)", path, resolved)
	}

	return resolved, nil
}

// resolveAbsolute treats all paths as absolute
func (ph *PathHandler) resolveAbsolute(path string) (string, error) {
	if !filepath.IsAbs(path) {
		// Convert to absolute by prepending /
		path = "/" + strings.TrimPrefix(path, "./")
	}

	// Validate it doesn't escape root
	if ph.base != "/" && !strings.HasPrefix(path, ph.base) {
		return "", fmt.Errorf("path %q escapes filesystem root in Absolute mode (outside root %q)", path, ph.base)
	}

	return path, nil
}

// resolveRelative treats all paths as relative to base
func (ph *PathHandler) resolveRelative(path string) (string, error) {
	// Strip any leading / to make it relative
	originalPath := path
	path = strings.TrimPrefix(path, "/")

	// Resolve against base
	resolved := filepath.Join(ph.base, path)

	// Ensure resolved path doesn't escape the base
	if !strings.HasPrefix(resolved, ph.base) {
		return "", fmt.Errorf("path %q escapes filesystem root in Relative mode (resolved as %q)", originalPath, resolved)
	}

	return resolved, nil
}

// MakeRelative converts an absolute path to relative from the base
func (ph *PathHandler) MakeRelative(path string) (string, error) {
	path = filepath.Clean(path)

	if !filepath.IsAbs(path) {
		return path, nil // Already relative
	}

	rel, err := filepath.Rel(ph.base, path)
	if err != nil {
		return "", err
	}

	// Ensure it doesn't escape
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q is outside filesystem root %q", path, ph.base)
	}

	return rel, nil
}

// ValidatePath checks if a path is valid without modifying it
func (ph *PathHandler) ValidatePath(path string) error {
	_, err := ph.ResolvePath(path)
	return err
}

// NormalizePath cleans and normalizes a path
func NormalizePath(path string) string {
	// Clean the path
	path = filepath.Clean(path)

	// Remove double slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}
