package testutil

import (
	"runtime"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// RealFSTestHelper provides utilities for testing with real filesystem operations
// This helper is Unix-only (Linux/macOS) as SynthFS doesn't officially support Windows
type RealFSTestHelper struct {
	t       *testing.T
	tempDir string
	fs      synthfs.FileSystem
}

// NewRealFSTestHelper creates a new real filesystem test helper
// Tests are automatically skipped on Windows as SynthFS doesn't officially support it
func NewRealFSTestHelper(t *testing.T) *RealFSTestHelper {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}
	
	tempDir := t.TempDir()
	fs := filesystem.NewOSFileSystem(tempDir)
	
	return &RealFSTestHelper{
		t:       t,
		tempDir: tempDir,
		fs:      fs,
	}
}

// FileSystem returns the real filesystem instance
func (h *RealFSTestHelper) FileSystem() synthfs.FileSystem {
	return h.fs
}

// TempDir returns the temporary directory path
func (h *RealFSTestHelper) TempDir() string {
	return h.tempDir
}

// CreateSymlink creates a real symlink for testing
func (h *RealFSTestHelper) CreateSymlink(target, linkPath string) {
	if writeFS, ok := h.fs.(filesystem.WriteFS); ok {
		if err := writeFS.Symlink(target, linkPath); err != nil {
			h.t.Fatalf("Failed to create symlink %s -> %s: %v", linkPath, target, err)
		}
	} else {
		h.t.Fatal("Filesystem doesn't support symlinks")
	}
}

// ReadSymlink reads a real symlink target
func (h *RealFSTestHelper) ReadSymlink(linkPath string) string {
	if writeFS, ok := h.fs.(filesystem.WriteFS); ok {
		target, err := writeFS.Readlink(linkPath)
		if err != nil {
			h.t.Fatalf("Failed to read symlink %s: %v", linkPath, err)
		}
		return target
	}
	h.t.Fatal("Filesystem doesn't support symlinks")
	return ""
}

// AssertSymlinkTarget verifies a symlink points to the expected target
func (h *RealFSTestHelper) AssertSymlinkTarget(linkPath, expectedTarget string) {
	actual := h.ReadSymlink(linkPath)
	if actual != expectedTarget {
		h.t.Errorf("Symlink %s target mismatch: expected %q, got %q", linkPath, expectedTarget, actual)
	}
}

// AssertSymlinkExists verifies a symlink exists
func (h *RealFSTestHelper) AssertSymlinkExists(linkPath string) {
	// For real filesystem testing, we can just try to read the symlink
	// If it fails, it either doesn't exist or isn't a symlink
	if writeFS, ok := h.fs.(filesystem.WriteFS); ok {
		_, err := writeFS.Readlink(linkPath)
		if err != nil {
			h.t.Errorf("Expected symlink %s to exist and be readable, but got error: %v", linkPath, err)
		}
	} else {
		h.t.Fatal("Filesystem doesn't support symlinks")
	}
}