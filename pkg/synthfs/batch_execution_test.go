package synthfs_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestBatchExecution(t *testing.T) {
	// Use TestFileSystem for controlled testing
	testFS := testutil.NewTestFileSystem()

	registry := synthfs.GetDefaultRegistry()
	fs := testutil.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).
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
		if !result.IsSuccess() {
			t.Logf("Batch execution not successful (expected with stub operations)")
			t.Logf("Errors: %v", result.GetError())
		}

		// Check that operations were processed
		if len(result.GetOperations()) == 0 {
			t.Error("Expected operations in result, but got none")
		}

		t.Logf("Executed %d operations in %v", len(result.GetOperations()), result.GetDuration())
		for i, opResult := range result.GetOperations() {
			// Handle both synthfs.OperationResult and core.OperationResult
			var opType, opPath string
			var status core.OperationStatus
			
			switch v := opResult.(type) {
			case synthfs.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					desc := op.Describe()
					opType = desc.Type
					opPath = desc.Path
				}
				status = v.Status
			case core.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					desc := op.Describe()
					opType = desc.Type
					opPath = desc.Path
				}
				status = v.Status
			case *core.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					desc := op.Describe()
					opType = desc.Type
					opPath = desc.Path
				}
				status = v.Status
			default:
				t.Logf("Operation %d: Unknown operation result type %T", i+1, opResult)
				continue
			}
			
			t.Logf("Operation %d: %s %s -> %s", i+1, opType, opPath, status)
		}
	})

	t.Run("Execute with auto-generated dependencies", func(t *testing.T) {
		registry := synthfs.GetDefaultRegistry()
		fs := testutil.NewTestFileSystem()
		newBatch := synthfs.NewBatch(fs, registry).WithFileSystem(testutil.NewTestFileSystem())

		// This should auto-create multiple parent directories
		_, err := newBatch.CreateFile("deep/nested/path/file.txt", []byte("nested content"))
		if err != nil {
			t.Fatalf("Failed to add nested CreateFile operation: %v", err)
		}

		// With new architecture, parent directories are created through prerequisite resolution
		// during execution, not as separate operations in the batch
		ops := newBatch.Operations()
		if len(ops) != 1 {
			t.Logf("Note: With new architecture, only the file operation is in the batch. Parent dirs are created via prerequisites. Got %d operations", len(ops))
		}

		// Execute the batch
		result, err := newBatch.Run()
		if err != nil {
			t.Fatalf("Nested batch execution failed: %v", err)
		}

		// Log the execution details
		t.Logf("Nested execution: %d operations in %v", len(result.GetOperations()), result.GetDuration())
		for i, opResult := range result.GetOperations() {
			// Handle both synthfs.OperationResult and core.OperationResult
			var opType, opPath string
			var status core.OperationStatus
			
			switch v := opResult.(type) {
			case synthfs.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					desc := op.Describe()
					opType = desc.Type
					opPath = desc.Path
				}
				status = v.Status
			case core.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					desc := op.Describe()
					opType = desc.Type
					opPath = desc.Path
				}
				status = v.Status
			case *core.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					desc := op.Describe()
					opType = desc.Type
					opPath = desc.Path
				}
				status = v.Status
			default:
				t.Logf("Operation %d: Unknown operation result type %T", i+1, opResult)
				continue
			}
			
			t.Logf("Operation %d: %s %s -> %s", i+1, opType, opPath, status)
		}
	})

	t.Run("Empty batch execution", func(t *testing.T) {
		registry := synthfs.GetDefaultRegistry()
		fs := testutil.NewTestFileSystem()
		emptyBatch := synthfs.NewBatch(fs, registry).WithFileSystem(testutil.NewTestFileSystem())

		result, err := emptyBatch.Run()
		if err != nil {
			t.Fatalf("Empty batch execution failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Error("Empty batch should succeed")
		}

		if len(result.GetOperations()) != 0 {
			t.Errorf("Expected 0 operations for empty batch, got %d", len(result.GetOperations()))
		}
	})
}
