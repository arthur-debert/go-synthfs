package targets_test

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

func TestSymlinkItem(t *testing.T) {
	linkPath := "/tmp/testlink"
	targetPath := "/tmp/originalfile"

	link := targets.NewSymlink(linkPath, targetPath)

	if link.Path() != linkPath {
		t.Errorf("Expected path %s, got %s", linkPath, link.Path())
	}
	if link.Type() != "symlink" {
		t.Errorf("Expected type 'symlink', got %s", link.Type())
	}
	if link.Target() != targetPath {
		t.Errorf("Expected target %s, got %s", targetPath, link.Target())
	}
}
