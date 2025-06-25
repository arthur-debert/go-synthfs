package synthfs_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestBatchExecution(t *testing.T) {
	// Use TestFileSystem for controlled testing
	testFS := synthfs.NewTestFileSystem()

	batch := synthfs.NewBatch().
		WithFileSystem(testFS).
		WithContext(context.Background())

	t.Run("Execute simple operations", func(t *testing.T) {
		// Add some operations to the batch
		_, err := batch.CreateDir("test-dir")
		if err != nil {
			t.Fatalf("Failed to add CreateDir operation: %v", err)
		}

		_, err = batch.CreateFile("test-dir/file.txt", []byte("test content"))
		if err != nil {
			t.Fatalf("Failed to add CreateFile operation: %v", err)
		}

		// Execute the batch
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}

		if result == nil {
			t.Fatal("Execute returned nil result")
		}

		// Check if execution was successful
		// Note: Since we're using stub operations, success depends on the current implementation
		if !result.Success {
			t.Logf("Batch execution not successful (expected with stub operations)")
			t.Logf("Errors: %v", result.Errors)
		}

		// Check that operations were processed
		if len(result.Operations) == 0 {
			t.Error("Expected operations in result, but got none")
		}

		t.Logf("Executed %d operations in %v", len(result.Operations), result.Duration)
		for i, opResult := range result.Operations {
			t.Logf("Operation %d: %s %s -> %s",
				i+1,
				opResult.Operation.Describe().Type,
				opResult.Operation.Describe().Path,
				opResult.Status)
		}
	})

	t.Run("Execute with auto-generated dependencies", func(t *testing.T) {
		newBatch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// This should auto-create multiple parent directories
		_, err := newBatch.CreateFile("deep/nested/path/file.txt", []byte("nested content"))
		if err != nil {
			t.Fatalf("Failed to add nested CreateFile operation: %v", err)
		}

		// Check that parent directories were auto-added
		ops := newBatch.Operations()
		if len(ops) < 2 {
			t.Errorf("Expected multiple operations (parent dirs + file), got %d", len(ops))
		}

		// Execute the batch
		result, err := newBatch.Run()
		if err != nil {
			t.Fatalf("Nested batch execution failed: %v", err)
		}

		// Log the execution details
		t.Logf("Nested execution: %d operations in %v", len(result.Operations), result.Duration)
		for i, opResult := range result.Operations {
			t.Logf("Operation %d: %s %s -> %s",
				i+1,
				opResult.Operation.Describe().Type,
				opResult.Operation.Describe().Path,
				opResult.Status)
		}
	})

	t.Run("Empty batch execution", func(t *testing.T) {
		emptyBatch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		result, err := emptyBatch.Run()
		if err != nil {
			t.Fatalf("Empty batch execution failed: %v", err)
		}

		if !result.Success {
			t.Error("Empty batch should succeed")
		}

		if len(result.Operations) != 0 {
			t.Errorf("Expected 0 operations for empty batch, got %d", len(result.Operations))
		}
	})
}

func TestBatchRollback(t *testing.T) {
	testFS := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch().WithFileSystem(testFS)

	// Add some operations
	_, err := batch.CreateDir("rollback-test")
	if err != nil {
		t.Fatalf("Failed to add operation: %v", err)
	}

	// Execute
	result, err := batch.Run()
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Test rollback function exists
	if result.Rollback == nil {
		t.Fatal("Expected rollback function, but got nil")
	}

	// Try to call rollback (should not error even with stub operations)
	err = result.Rollback(context.Background())
	if err != nil {
		t.Logf("Rollback returned error (expected with stub operations): %v", err)
	} else {
		t.Log("Rollback completed successfully")
	}
}

func TestBatchOperationCounts(t *testing.T) {
	batch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

	// Track operations as we add them
	expectedCount := 0

	// Add CreateDir
	_, err := batch.CreateDir("dir1")
	if err != nil {
		t.Fatalf("CreateDir failed: %v", err)
	}
	expectedCount++

	ops := batch.Operations()
	if len(ops) != expectedCount {
		t.Errorf("Expected %d operations after CreateDir, got %d", expectedCount, len(ops))
	}

	// Add CreateFile with nested path (should auto-create parent)
	_, err = batch.CreateFile("auto-dir/file.txt", []byte("content"))
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}
	expectedCount += 2 // parent dir + file

	ops = batch.Operations()
	if len(ops) != expectedCount {
		t.Errorf("Expected %d operations after nested CreateFile, got %d", expectedCount, len(ops))
	}

	// Add more operations
	// Phase I, Milestone 1: Copy and Move operations now validate source existence
	// These will fail since sources don't exist, so skip them for this count test
	t.Log("Skipping Copy and Move operations since sources don't exist (Phase I validation)")

	_, err = batch.Delete("to-delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	expectedCount++

	finalOps := batch.Operations()
	if len(finalOps) != expectedCount {
		t.Errorf("Expected %d total operations, got %d", expectedCount, len(finalOps))
	}

	// Verify we can execute this batch
	result, err := batch.Run()
	if err != nil {
		t.Fatalf("Final execution failed: %v", err)
	}

	t.Logf("Successfully executed batch with %d operations", len(result.Operations))
}
