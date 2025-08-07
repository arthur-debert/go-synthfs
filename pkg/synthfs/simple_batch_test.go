package synthfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Define a custom type for context keys to avoid collisions
type testContextKey string

func TestSimpleBatchAPI(t *testing.T) {
	t.Run("Basic batch operations", func(t *testing.T) {
		ResetSequenceCounter()
		fs := filesystem.NewTestFileSystem()

		// First batch: create directory and files
		batch1 := NewSimpleBatch(fs)
		batch1.
			CreateDir("dir1", 0755).
			WriteFile("dir1/file1.txt", []byte("content1"), 0644).
			WriteFile("dir1/file2.txt", []byte("content2"), 0644)

		if len(batch1.Operations()) != 3 {
			t.Errorf("Expected 3 operations in first batch, got %d", len(batch1.Operations()))
		}

		err := batch1.Execute()
		if err != nil {
			t.Fatalf("First batch execution failed: %v", err)
		}

		// Second batch: copy operation (needs files to exist)
		batch2 := NewSimpleBatch(fs)
		batch2.Copy("dir1/file1.txt", "dir1/file1-copy.txt")

		if len(batch2.Operations()) != 1 {
			t.Errorf("Expected 1 operation in second batch, got %d", len(batch2.Operations()))
		}

		err = batch2.Execute()
		if err != nil {
			t.Fatalf("Second batch execution failed: %v", err)
		}

		// Verify all operations succeeded
		if _, err := fs.Stat("dir1"); err != nil {
			t.Error("Directory dir1 should exist")
		}
		if _, err := fs.Stat("dir1/file1.txt"); err != nil {
			t.Error("File dir1/file1.txt should exist")
		}
		if _, err := fs.Stat("dir1/file2.txt"); err != nil {
			t.Error("File dir1/file2.txt should exist")
		}
		if _, err := fs.Stat("dir1/file1-copy.txt"); err != nil {
			t.Error("File dir1/file1-copy.txt should exist")
		}
	})

	t.Run("Batch with context", func(t *testing.T) {
		ResetSequenceCounter()
		fs := filesystem.NewTestFileSystem()
		ctx := context.WithValue(context.Background(), testContextKey("test"), "value")

		batch := NewSimpleBatch(fs).WithContext(ctx)
		batch.CreateDir("testdir", 0755)

		if batch.ctx != ctx {
			t.Error("Context should be set")
		}

		err := batch.Execute()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}
	})

	t.Run("Empty batch", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		batch := NewSimpleBatch(fs)

		err := batch.Execute()
		if err != nil {
			t.Error("Empty batch should execute without error")
		}
	})

	t.Run("Batch with validation failure", func(t *testing.T) {
		ResetSequenceCounter()
		fs := filesystem.NewTestFileSystem()

		// Create a file that will cause a conflict
		err := fs.WriteFile("existing.txt", []byte("existing"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		batch := NewSimpleBatch(fs)
		batch.
			CreateDir("dir1", 0755).
			WriteFile("dir1/file1.txt", []byte("content"), 0644).
			CreateDir("existing.txt", 0755) // This should fail validation

		err = batch.Execute()
		if err == nil {
			t.Fatal("Expected batch to fail on conflicting directory creation")
		}

		// With upfront validation, the error happens before any operations execute
		// Verify NO operations were executed
		if _, err := fs.Stat("dir1"); err == nil {
			t.Error("Directory dir1 should NOT have been created (validation failed)")
		}
		if _, err := fs.Stat("dir1/file1.txt"); err == nil {
			t.Error("File dir1/file1.txt should NOT have been created (validation failed)")
		}
	})

	t.Run("Batch clear", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		batch := NewSimpleBatch(fs)

		batch.CreateDir("dir1", 0755).CreateDir("dir2", 0755)
		if len(batch.Operations()) != 2 {
			t.Error("Should have 2 operations")
		}

		batch.Clear()
		if len(batch.Operations()) != 0 {
			t.Error("Should have 0 operations after clear")
		}

		// Can continue using batch after clear
		batch.CreateDir("dir3", 0755)
		if len(batch.Operations()) != 1 {
			t.Error("Should have 1 operation after adding to cleared batch")
		}
	})

	t.Run("All operation types", func(t *testing.T) {
		ResetSequenceCounter()
		fs := filesystem.NewTestFileSystem()

		// Create some initial content
		err := fs.WriteFile("source.txt", []byte("source"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// First batch: create directory, file, and copy
		batch1 := NewSimpleBatch(fs)
		batch1.
			CreateDir("testdir", 0755).
			WriteFile("testdir/file.txt", []byte("content"), 0644).
			Copy("source.txt", "source-copy.txt")

		if len(batch1.Operations()) != 3 {
			t.Errorf("Expected 3 operations in first batch, got %d", len(batch1.Operations()))
		}

		err = batch1.Execute()
		if err != nil {
			t.Fatalf("First batch execution failed: %v", err)
		}

		// Second batch: move, symlink, and delete operations
		batch2 := NewSimpleBatch(fs)
		batch2.
			Move("source-copy.txt", "source-moved.txt").
			CreateSymlink("source.txt", "source-link.txt").
			Delete("source.txt")

		if len(batch2.Operations()) != 3 {
			t.Errorf("Expected 3 operations in second batch, got %d", len(batch2.Operations()))
		}

		err = batch2.Execute()
		if err != nil {
			t.Fatalf("Second batch execution failed: %v", err)
		}

		// Verify results
		if _, err := fs.Stat("testdir"); err != nil {
			t.Error("Directory should exist")
		}
		if _, err := fs.Stat("testdir/file.txt"); err != nil {
			t.Error("File should exist")
		}
		if _, err := fs.Stat("source-moved.txt"); err != nil {
			t.Error("Moved file should exist")
		}
		if _, err := fs.Stat("source-link.txt"); err != nil {
			t.Error("Symlink should exist")
		}
		if _, err := fs.Stat("source.txt"); err == nil {
			t.Error("Original source file should be deleted")
		}
		if _, err := fs.Stat("source-copy.txt"); err == nil {
			t.Error("Copy should have been moved")
		}
	})
}

func TestSimpleBatchWithRollback(t *testing.T) {
	// Testing error handling alignment with expected error types
	
	t.Run("Successful rollback", func(t *testing.T) {
		ResetSequenceCounter()
		fs := filesystem.NewTestFileSystem()

		// Create a conflict
		err := fs.WriteFile("conflict.txt", []byte("existing"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		batch := NewSimpleBatch(fs)
		batch.
			CreateDir("dir1", 0755).
			WriteFile("dir1/file1.txt", []byte("content"), 0644).
			CreateDir("conflict.txt", 0755) // This will fail

		err = batch.ExecuteWithRollback()
		if err == nil {
			t.Fatal("Expected error from conflicting operation")
		}

		// Verify rollback - created items should be gone
		if _, err := fs.Stat("dir1"); err == nil {
			t.Error("Directory dir1 should have been rolled back")
		}
		if _, err := fs.Stat("dir1/file1.txt"); err == nil {
			t.Error("File should have been rolled back")
		}

		// Original conflict file should still exist
		if _, err := fs.Stat("conflict.txt"); err != nil {
			t.Error("Original conflict file should still exist")
		}
	})

	t.Run("Rollback with errors", func(t *testing.T) {
		ResetSequenceCounter()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)

		// Create initial files
		batch := NewSimpleBatch(fs)
		batch.
			CreateDir("protected", 0755).
			WriteFile("protected/file1.txt", []byte("content1"), 0644).
			WriteFile("protected/file2.txt", []byte("content2"), 0644)

		err := batch.Execute()
		if err != nil {
			t.Fatalf("Initial setup failed: %v", err)
		}

		// Make the directory read-only to cause rollback failures
		protectedPath := filepath.Join(tempDir, "protected")
		if err := os.Chmod(protectedPath, 0555); err != nil {
			t.Fatalf("Failed to change permissions: %v", err)
		}

		// Create a new batch that will fail and trigger rollback
		batch2 := NewSimpleBatch(fs)
		batch2.
			WriteFile("protected/file3.txt", []byte("content3"), 0644). // This will succeed
			WriteFile("protected/file4.txt", []byte("content4"), 0644). // This will succeed
			CreateDir("protected/subdir", 0755) // This will fail due to read-only parent

		// Execute with rollback - should fail and rollback should also fail
		err = batch2.ExecuteWithRollback()
		if err == nil {
			t.Fatal("Expected error from batch execution")
		}

		// Check if it's a RollbackError
		var rollbackErr *RollbackError
		if !errors.As(err, &rollbackErr) {
			// Not a rollback error - the main operation failed but rollback succeeded
			// This is OK, but let's verify the files were rolled back
			if _, statErr := fs.Stat("protected/file3.txt"); statErr == nil {
				t.Error("File should have been rolled back")
			}
			if _, statErr := fs.Stat("protected/file4.txt"); statErr == nil {
				t.Error("File should have been rolled back")
			}
		} else {
			// We got a rollback error - verify the details
			if rollbackErr.OriginalErr == nil {
				t.Error("RollbackError should contain original error")
			}
			if len(rollbackErr.RollbackErrs) == 0 {
				t.Error("RollbackError should contain rollback errors")
			}

			// Files that couldn't be rolled back should still exist
			if _, statErr := fs.Stat("protected/file3.txt"); statErr != nil {
				t.Error("File that failed rollback should still exist")
			}
		}

		// Restore permissions for cleanup
		_ = os.Chmod(protectedPath, 0755)
	})

	t.Run("Rollback failure with custom operation", func(t *testing.T) {
		ResetSequenceCounter()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)
		sfs := New()

		// Track what operations were executed and rolled back
		var executed []string
		var rolledBack []string

		// Create custom operation with a rollback that will fail
		op1 := NewCustomOperation("op1", func(ctx context.Context, fs filesystem.FileSystem) error {
			executed = append(executed, "op1")
			return osFS.WriteFile("file1.txt", []byte("content1"), 0644)
		}).WithRollback(func(ctx context.Context, fs filesystem.FileSystem) error {
			rolledBack = append(rolledBack, "op1")
			// Try to delete but simulate failure
			return errors.New("rollback failed: file locked")
		})
		op1Adapter := op1

		op2 := sfs.CreateFileWithID("op2", "file2.txt", []byte("content2"), 0644)

		// This operation will fail
		op3 := sfs.CustomOperationWithID("op3", func(ctx context.Context, fs filesystem.FileSystem) error {
			executed = append(executed, "op3")
			return errors.New("operation failed")
		})

		// Run with rollback
		options := DefaultPipelineOptions()
		options.RollbackOnError = true
		result, err := RunWithOptions(context.Background(), fs, options, op1Adapter, op2, op3)

		// Should have error
		if err == nil {
			t.Fatal("Expected error from failed operation")
		}

		// Check execution order (op2 is not a custom op anymore, so only op1 and op3)
		if len(executed) != 2 || executed[0] != "op1" || executed[1] != "op3" {
			t.Errorf("Expected execution order [op1, op3], got %v", executed)
		}

		// Check rollback was attempted
		if len(rolledBack) == 0 {
			t.Error("Expected rollback to be attempted")
		}

		// Should be a pipeline error wrapping a rollback error
		var pipelineErr *PipelineError
		if !errors.As(err, &pipelineErr) {
			t.Fatalf("Expected PipelineError, got %T", err)
		}

		// The inner error should be a rollback error
		var rollbackErr *RollbackError
		if !errors.As(pipelineErr.Err, &rollbackErr) {
			t.Fatalf("Expected RollbackError in pipeline error, got %T", pipelineErr.Err)
		}

		// Verify rollback error details
		if rollbackErr.OriginalErr == nil || !strings.Contains(rollbackErr.OriginalErr.Error(), "operation failed") {
			t.Errorf("Expected original error 'operation failed', got %v", rollbackErr.OriginalErr)
		}

		if len(rollbackErr.RollbackErrs) == 0 {
			t.Error("Expected rollback errors to be recorded")
		}

		// Files should still exist since rollback failed
		if _, err := fs.Stat("file1.txt"); err != nil {
			t.Error("File1 should exist since rollback failed")
		}
		// File2 should NOT exist because it was successfully rolled back
		if _, err := fs.Stat("file2.txt"); err == nil {
			t.Error("File2 should not exist - it should have been rolled back")
		}

		// Verify result details
		if result.Success {
			t.Error("Result should not be successful")
		}
		if len(result.Operations) != 3 {
			t.Errorf("Expected 3 operation results, got %d", len(result.Operations))
		}
	})
}
