package operations_test

import (
	"context"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestOperationRollback(t *testing.T) {
	ctx := context.Background()

	t.Run("Rollback create file operation", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Setup: create a file first
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test/file.txt")
		fileItem := &TestFileItem{
			path:    "test/file.txt",
			content: []byte("test"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		// Execute to create the file
		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file exists
		if _, err := fs.Stat("test/file.txt"); err != nil {
			t.Fatalf("File should exist: %v", err)
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify file is removed
		if _, err := fs.Stat("test/file.txt"); err == nil {
			t.Error("File should be removed after rollback")
		}
	})

	t.Run("Rollback create directory operation", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Setup: create a directory first
		op := operations.NewCreateDirectoryOperation(core.OperationID("test-op"), "test/dir")
		dirItem := &TestDirItem{
			path: "test/dir",
			mode: 0755,
		}
		op.SetItem(dirItem)

		// Execute to create the directory
		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify directory exists
		if _, err := fs.Stat("test/dir"); err != nil {
			t.Fatalf("Directory should exist: %v", err)
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify directory is removed
		if _, err := fs.Stat("test/dir"); err == nil {
			t.Error("Directory should be removed after rollback")
		}
	})

	t.Run("Rollback copy operation", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Setup: create source file and copy operation
		err := fs.WriteFile("test/source.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewCopyOperation(core.OperationID("test-op"), "test/source.txt")
		op.SetPaths("test/source.txt", "test/destination.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "test/destination.txt")

		// Execute copy
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify destination exists
		if _, err := fs.Stat("test/destination.txt"); err != nil {
			t.Fatalf("Destination should exist: %v", err)
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify destination is removed
		if _, err := fs.Stat("test/destination.txt"); err == nil {
			t.Error("Destination should be removed after rollback")
		}

		// Verify source still exists
		if _, err := fs.Stat("test/source.txt"); err != nil {
			t.Error("Source should still exist after copy rollback")
		}
	})

	t.Run("Rollback move operation", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Setup: create source file and move operation
		err := fs.WriteFile("test/movesource.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewMoveOperation(core.OperationID("test-op"), "test/movesource.txt")
		op.SetPaths("test/movesource.txt", "test/movedest.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "test/movedest.txt")

		// Execute move
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file moved
		if _, err := fs.Stat("test/movedest.txt"); err != nil {
			t.Fatalf("Destination should exist: %v", err)
		}
		if _, err := fs.Stat("test/movesource.txt"); err == nil {
			t.Error("Source should not exist after move")
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify file moved back
		if _, err := fs.Stat("test/movesource.txt"); err != nil {
			t.Error("Source should exist after move rollback")
		}
		if _, err := fs.Stat("test/movedest.txt"); err == nil {
			t.Error("Destination should not exist after move rollback")
		}
	})

	t.Run("Rollback delete operation returns error", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewDeleteOperation(core.OperationID("test-op"), "test/file.txt")

		err := op.Rollback(ctx, fs)
		if err == nil {
			t.Error("Expected error for delete rollback")
		}

		expectedMsg := "rollback not implemented for delete operations"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error containing %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("Rollback create symlink operation", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Setup: create a symlink first
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "test/link")
		// Set target in description
		op.SetDescriptionDetail("target", "../target")

		// Note: MockFilesystem doesn't support symlinks, so Execute will fail
		// But we can still test that Rollback attempts to remove it

		// Create a fake symlink (as a file for testing)
		if err := fs.WriteFile("test/link", []byte("symlink"), 0644); err != nil {
			t.Fatalf("Failed to create fake symlink: %v", err)
		}

		// Rollback
		err := op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify symlink is removed
		if _, err := fs.Stat("test/link"); err == nil {
			t.Error("Symlink should be removed after rollback")
		}
	})
}
