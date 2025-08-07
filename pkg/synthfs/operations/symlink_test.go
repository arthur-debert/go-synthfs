package operations_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestSymlinkOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("create symlink validation with no item", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "test/link")
		// Don't set item - should fail validation

		err := op.Validate(ctx, nil, fs)
		if err == nil {
			t.Error("Expected validation error for missing item")
			return
		}

		if !strings.Contains(err.Error(), "no item provided") {
			t.Errorf("Expected 'no item provided' error, got: %s", err.Error())
		}
	})

	t.Run("create symlink validation with target", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "test/link")
		// Set symlink item with target
		symlinkItem := &TestSymlinkItem{
			path:   "test/link",
			target: "../target",
			mode:   0755,
		}
		op.SetItem(symlinkItem)

		// Also set target in description
		op.SetDescriptionDetail("target", "../target")

		err := op.Validate(ctx, nil, fs)
		if err != nil {
			t.Errorf("Expected no validation error, got: %v", err)
		}
	})

	t.Run("create symlink execution not supported", func(t *testing.T) {
		// Use a filesystem that doesn't support symlinks
		fs := &noSymlinkFS{NewMockFilesystem()}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "test/link")
		// Set symlink item with target
		symlinkItem := &TestSymlinkItem{
			path:   "test/link",
			target: "../target",
			mode:   0755,
		}
		op.SetItem(symlinkItem)

		// Also set target in description
		op.SetDescriptionDetail("target", "../target")

		// MockFilesystem doesn't support symlinks
		err := op.Execute(ctx, nil, fs)
		if err == nil {
			t.Error("Expected error for unsupported symlink operation")
			return
		}

		if !strings.Contains(err.Error(), "does not support Symlink") {
			t.Errorf("Expected 'does not support Symlink' error, got: %s", err.Error())
		}
	})

	t.Run("reverse ops for create symlink", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "test/link")
		// Set symlink item with target
		symlinkItem := &TestSymlinkItem{
			path:   "test/link",
			target: "../target",
			mode:   0755,
		}
		op.SetItem(symlinkItem)

		// Also set target in description
		op.SetDescriptionDetail("target", "../target")

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, nil)
		if err != nil {
			t.Fatalf("ReverseOps failed: %v", err)
		}

		if backupData != nil {
			t.Error("Expected no backup data for create symlink operation")
		}

		if len(reverseOps) != 1 {
			t.Fatalf("Expected 1 reverse op, got %d", len(reverseOps))
		}

		// The reverse of create symlink is delete
		if reverseOp, ok := reverseOps[0].(*operations.DeleteOperation); ok {
			if reverseOp.Describe().Path != "test/link" {
				t.Errorf("Expected reverse op to delete 'test/link', got '%s'", reverseOp.Describe().Path)
			}
		} else {
			t.Error("Expected reverse op to be DeleteOperation")
		}
	})
}

// noSymlinkFS wraps a filesystem and always returns an error for Symlink operations
type noSymlinkFS struct {
	*MockFilesystem
}

func (fs *noSymlinkFS) Symlink(oldname, newname string) error {
	return fmt.Errorf("filesystem does not support Symlink")
}
