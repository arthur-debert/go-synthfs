package operations_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// TestSymlinkExecuteErrorPaths tests the untested 52.4% of symlink Execute method
func TestSymlinkExecuteErrorPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("Symlink execute with no item", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Don't set item
		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for missing item")
		}

		if !strings.Contains(err.Error(), "create_symlink operation requires an item") {
			t.Errorf("Expected 'create_symlink operation requires an item' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with no target in details", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set item but no target in details
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for missing target")
		}

		if !strings.Contains(err.Error(), "create_symlink operation requires a target") {
			t.Errorf("Expected 'create_symlink operation requires a target' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with empty target in details", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set empty target
		op.SetDescriptionDetail("target", "")
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for empty target")
		}

		if !strings.Contains(err.Error(), "create_symlink operation requires a target") {
			t.Errorf("Expected 'create_symlink operation requires a target' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with non-ItemInterface item", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set target
		op.SetDescriptionDetail("target", "target.txt")

		// Set item that doesn't implement ItemInterface
		op.SetItem("not-an-item-interface")

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for non-ItemInterface item")
		}

		if !strings.Contains(err.Error(), "item does not implement ItemInterface") {
			t.Errorf("Expected 'item does not implement ItemInterface' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with filesystem lacking Symlink method", func(t *testing.T) {
		fs := &FilesystemWithoutSymlink{
			files: make(map[string][]byte),
			dirs:  make(map[string]bool),
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for filesystem without Symlink")
		}

		if !strings.Contains(err.Error(), "filesystem does not support Symlink") {
			t.Errorf("Expected 'filesystem does not support Symlink' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with parent directory creation", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "dir/subdir/link.txt")
		op.SetDescriptionDetail("target", "../../target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "dir/subdir/link.txt",
			target: "../../target.txt",
		})

		err := op.Execute(ctx, fs)

		if err != nil {
			t.Errorf("Expected no error for valid symlink with parent dirs, got: %v", err)
		}

		// Verify parent directory was created
		if !fs.dirs["dir/subdir"] {
			t.Error("Parent directory was not created")
		}

		// Verify symlink was created
		if target, ok := fs.symlinks["dir/subdir/link.txt"]; !ok {
			t.Error("Symlink was not created")
		} else if target != "../../target.txt" {
			t.Errorf("Expected symlink target '../../target.txt', got: %s", target)
		}
	})

	t.Run("Symlink execute in root directory (no parent dir needed)", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, fs)

		if err != nil {
			t.Errorf("Expected no error for root directory symlink, got: %v", err)
		}

		// Verify symlink was created
		if target, ok := fs.symlinks["link.txt"]; !ok {
			t.Error("Symlink was not created")
		} else if target != "target.txt" {
			t.Errorf("Expected symlink target 'target.txt', got: %s", target)
		}

		// Should not have created any directories
		if len(fs.dirs) > 0 {
			t.Errorf("Expected no directories to be created for root symlink, got: %v", fs.dirs)
		}
	})

	t.Run("Symlink execute with filesystem lacking MkdirAll (should still work)", func(t *testing.T) {
		fs := &FilesystemWithSymlinkButNoMkdirAll{
			symlinks: make(map[string]string),
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "dir/link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "dir/link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, fs)

		if err != nil {
			t.Errorf("Expected no error even without MkdirAll, got: %v", err)
		}

		// Verify symlink was created
		if target, ok := fs.symlinks["dir/link.txt"]; !ok {
			t.Error("Symlink was not created")
		} else if target != "target.txt" {
			t.Errorf("Expected symlink target 'target.txt', got: %s", target)
		}
	})

	t.Run("Symlink execute with symlink creation failure", func(t *testing.T) {
		fs := &FilesystemWithFailingSymlink{
			dirs: make(map[string]bool),
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for failing symlink creation")
		}

		if !strings.Contains(err.Error(), "failed to create symlink") {
			t.Errorf("Expected 'failed to create symlink' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with parent directory creation failure", func(t *testing.T) {
		fs := &FilesystemWithFailingMkdirAll{
			symlinks: make(map[string]string),
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "dir/link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "dir/link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for failing parent directory creation")
		}

		if !strings.Contains(err.Error(), "failed to create parent directory") {
			t.Errorf("Expected 'failed to create parent directory' error, got: %s", err.Error())
		}
	})
}

// Helper filesystem types for testing symlink edge cases

// MockFilesystemWithSymlink for testing symlink functionality
type MockFilesystemWithSymlink struct {
	files    map[string][]byte
	dirs     map[string]bool
	symlinks map[string]string
}

func NewMockFilesystemWithSymlink() *MockFilesystemWithSymlink {
	return &MockFilesystemWithSymlink{
		files:    make(map[string][]byte),
		dirs:     make(map[string]bool),
		symlinks: make(map[string]string),
	}
}

func (fs *MockFilesystemWithSymlink) Symlink(oldname, newname string) error {
	fs.symlinks[newname] = oldname
	return nil
}

func (fs *MockFilesystemWithSymlink) MkdirAll(path string, perm interface{}) error {
	fs.dirs[path] = true
	return nil
}

// FilesystemWithoutSymlink has no Symlink method
type FilesystemWithoutSymlink struct {
	files map[string][]byte
	dirs  map[string]bool
}

func (fs *FilesystemWithoutSymlink) MkdirAll(path string, perm interface{}) error {
	fs.dirs[path] = true
	return nil
}

// FilesystemWithSymlinkButNoMkdirAll has Symlink but no MkdirAll
type FilesystemWithSymlinkButNoMkdirAll struct {
	symlinks map[string]string
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) Symlink(oldname, newname string) error {
	fs.symlinks[newname] = oldname
	return nil
}

// FilesystemWithFailingSymlink always fails Symlink calls
type FilesystemWithFailingSymlink struct {
	dirs map[string]bool
}

func (fs *FilesystemWithFailingSymlink) Symlink(oldname, newname string) error {
	return fmt.Errorf("symlink creation failed")
}

func (fs *FilesystemWithFailingSymlink) MkdirAll(path string, perm interface{}) error {
	fs.dirs[path] = true
	return nil
}

// FilesystemWithFailingMkdirAll always fails MkdirAll calls
type FilesystemWithFailingMkdirAll struct {
	symlinks map[string]string
}

func (fs *FilesystemWithFailingMkdirAll) Symlink(oldname, newname string) error {
	fs.symlinks[newname] = oldname
	return nil
}

func (fs *FilesystemWithFailingMkdirAll) MkdirAll(path string, perm interface{}) error {
	return fmt.Errorf("mkdir failed")
}
