package targets_test

import (
	"io/fs"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

func TestDirectoryItem(t *testing.T) {
	dirPath := "/tmp/testdir"
	dirMode := fs.FileMode(0700)

	dir := targets.NewDirectory(dirPath).
		WithMode(dirMode)

	if dir.Path() != dirPath {
		t.Errorf("Expected path %s, got %s", dirPath, dir.Path())
	}
	if dir.Type() != "directory" {
		t.Errorf("Expected type 'directory', got %s", dir.Type())
	}
	if dir.Mode() != dirMode {
		t.Errorf("Expected mode %v, got %v", dirMode, dir.Mode())
	}

	// Test default mode
	defaultModeDir := targets.NewDirectory("defaultdir")
	if defaultModeDir.Mode() != 0755 {
		t.Errorf("Expected default mode 0755, got %v", defaultModeDir.Mode())
	}
}
