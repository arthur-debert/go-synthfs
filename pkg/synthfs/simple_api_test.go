package synthfs

import (
	"context"
	"errors"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestSimpleRunAPI(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("Run with multiple operations", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// Create operations
		op1 := sfs.CreateDir("testdir", 0755)
		op2 := sfs.CreateFile("testdir/file.txt", []byte("content"), 0644)
		op3 := sfs.Copy("testdir/file.txt", "testdir/file-copy.txt")

		// Run them
		result, err := Run(ctx, fs, op1, op2, op3)
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		// Check result
		if len(result.GetOperations()) != 3 {
			t.Errorf("Expected 3 operation results, got %d", len(result.GetOperations()))
		}

		// Verify filesystem state
		if _, err := fs.Stat("testdir"); err != nil {
			t.Error("Directory should exist")
		}
		if _, err := fs.Stat("testdir/file.txt"); err != nil {
			t.Error("File should exist")
		}
		if _, err := fs.Stat("testdir/file-copy.txt"); err != nil {
			t.Error("Copy should exist")
		}
	})

	t.Run("Run with no operations", func(t *testing.T) {
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		result, err := Run(ctx, fs)
		if err != nil {
			t.Fatalf("Run with no operations should succeed: %v", err)
		}

		if len(result.GetOperations()) != 0 {
			t.Error("Should have no operation results")
		}
	})

	t.Run("Run with failure", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// Create a conflict
		err := fs.WriteFile("conflict.txt", []byte("existing"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Create operations where the third will fail
		op1 := sfs.CreateDir("dir1", 0755)
		op2 := sfs.CreateFile("dir1/file.txt", []byte("content"), 0644)
		op3 := sfs.CreateDir("conflict.txt", 0755) // This will fail

		result, err := Run(ctx, fs, op1, op2, op3)
		if err == nil {
			t.Fatal("Expected error from conflicting operation")
		}

		// Check error type
		if pipelineErr, ok := err.(*PipelineError); ok {
			if pipelineErr.FailedIndex != 3 {
				t.Errorf("Expected failure at operation 3, got %d", pipelineErr.FailedIndex)
			}
			if len(pipelineErr.SuccessfulOps) != 2 {
				t.Errorf("Expected 2 successful operations, got %d", len(pipelineErr.SuccessfulOps))
			}
		} else {
			t.Errorf("Expected PipelineError, got %T", err)
		}

		// Result should still be returned with partial success
		if result == nil {
			t.Error("Result should be returned even on error")
		}

		// Verify partial success
		if _, err := fs.Stat("dir1"); err != nil {
			t.Error("First operation should have succeeded")
		}
		if _, err := fs.Stat("dir1/file.txt"); err != nil {
			t.Error("Second operation should have succeeded")
		}
	})

	t.Run("RunWithOptions with custom options", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// Create custom options
		options := DefaultPipelineOptions()
		options.DryRun = true // This is not implemented yet, but we can test that it doesn't crash

		// Create operations
		op1 := sfs.CreateDir("testdir", 0755)
		op2 := sfs.CreateFile("testdir/file.txt", []byte("content"), 0644)

		result, err := RunWithOptions(ctx, fs, options, op1, op2)
		if err != nil {
			t.Fatalf("RunWithOptions failed: %v", err)
		}

		if len(result.GetOperations()) != 2 {
			t.Error("Should have 2 operation results")
		}

		// Since DryRun is implemented, this test should be updated to check that the files are not created.
		if _, err := fs.Stat("testdir"); err == nil {
			t.Error("Directory should not exist")
		}
		if _, err := fs.Stat("testdir/file.txt"); err == nil {
			t.Error("File should not exist")
		}
	})
}

func TestSimpleRunAPIValidationFailure(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	ctx := context.Background()
	fs := filesystem.NewTestFileSystem()

	// Create operations where the second will fail validation
	op1 := sfs.CreateDir("dir1", 0755)
	op2 := sfs.CreateFile("", []byte("content"), 0644) // Invalid path

	result, err := Run(ctx, fs, op1, op2)
	if err == nil {
		t.Fatal("Expected validation error")
	}

	// Check error type
	if pipelineErr, ok := err.(*PipelineError); ok {
		if pipelineErr.FailedIndex != 2 {
			t.Errorf("Expected failure at operation 2, got %d", pipelineErr.FailedIndex)
		}
		if len(pipelineErr.SuccessfulOps) != 1 {
			t.Errorf("Expected 1 successful operation, got %d", len(pipelineErr.SuccessfulOps))
		}
	} else {
		t.Errorf("Expected PipelineError, got %T", err)
	}

	// Result should still be returned with partial success
	if result == nil {
		t.Error("Result should be returned even on error")
	}

	// Verify partial success
	if _, err := fs.Stat("dir1"); err != nil {
		t.Error("First operation should have succeeded")
	}
}

func TestSimpleRunWithRollback(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	ctx := context.Background()
	fs := filesystem.NewTestFileSystem()

	// Create a conflict
	err := fs.WriteFile("conflict.txt", []byte("existing"), 0644)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create operations where the third will fail
	op1 := sfs.CreateDir("dir1", 0755)
	op2 := sfs.CreateFile("dir1/file.txt", []byte("content"), 0644)
	op3 := sfs.CreateDir("conflict.txt", 0755) // This will fail

	options := DefaultPipelineOptions()
	options.RollbackOnError = true

	_, err = RunWithOptions(ctx, fs, options, op1, op2, op3)
	if err == nil {
		t.Fatal("Expected error from conflicting operation")
	}

	// Verify that the successful operations were rolled back
	if _, err := fs.Stat("dir1"); err == nil {
		t.Error("Directory 'dir1' should have been rolled back")
	}
	if _, err := fs.Stat("dir1/file.txt"); err == nil {
		t.Error("File 'dir1/file.txt' should have been rolled back")
	}
}

func TestSimpleRunWithDryRun(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	ctx := context.Background()
	fs := filesystem.NewTestFileSystem()

	// Create operations
	op1 := sfs.CreateDir("testdir", 0755)
	op2 := sfs.CreateFile("testdir/file.txt", []byte("content"), 0644)

	options := DefaultPipelineOptions()
	options.DryRun = true

	result, err := RunWithOptions(ctx, fs, options, op1, op2)
	if err != nil {
		t.Fatalf("RunWithOptions with DryRun failed: %v", err)
	}

	if len(result.GetOperations()) != 2 {
		t.Error("Should have 2 operation results")
	}

	// Verify that no changes were made to the real filesystem
	if _, err := fs.Stat("testdir"); err == nil {
		t.Error("Directory 'testdir' should not have been created")
	}
	if _, err := fs.Stat("testdir/file.txt"); err == nil {
		t.Error("File 'testdir/file.txt' should not have been created")
	}
}

func TestSimpleRunAPI_FirstOperationFailure(t *testing.T) {
	t.Run("failure on first operation", func(t *testing.T) {
		ResetSequenceCounter()
		fs := filesystem.NewTestFileSystem()
		sfs := New()

		// Create operations where the first one will fail
		ops := []Operation{
			sfs.CreateFile("/nonexistent/file.txt", []byte("fail"), 0644), // Will fail - parent doesn't exist
			sfs.CreateDir("/should-not-run", 0755),
			sfs.CreateFile("/also-should-not-run.txt", []byte("nope"), 0644),
		}

		result, err := Run(context.Background(), fs, ops...)
		
		// Should have error
		if err == nil {
			t.Fatal("Expected error when first operation fails")
		}

		// Check it's a PipelineError
		var pipelineErr *PipelineError
		if !errors.As(err, &pipelineErr) {
			t.Fatalf("Expected PipelineError, got %T", err)
		}

		// Verify pipeline error details
		if pipelineErr.FailedIndex != 1 {
			t.Errorf("Expected FailedIndex=1, got %d", pipelineErr.FailedIndex)
		}

		if pipelineErr.TotalOps != 3 {
			t.Errorf("Expected TotalOps=3, got %d", pipelineErr.TotalOps)
		}

		if len(pipelineErr.SuccessfulOps) != 0 {
			t.Errorf("Expected 0 successful operations, got %d", len(pipelineErr.SuccessfulOps))
		}

		// Check result
		if result.IsSuccess() {
			t.Error("Result should not be successful")
		}

		// Should have exactly 1 operation result (the failed one)
		if len(result.GetOperations()) != 1 {
			t.Errorf("Expected 1 operation result, got %d", len(result.GetOperations()))
		}

		// Verify the failed operation details
		if len(result.GetOperations()) > 0 {
			opResult := result.GetOperations()[0].(OperationResult)
			if opResult.Status != StatusFailure {
				t.Errorf("Expected first operation status to be StatusFailure, got %v", opResult.Status)
			}
			if opResult.Error == nil {
				t.Error("Expected first operation to have an error")
			}
		}

		// Verify no operations were executed
		entries, _ := fs.ReadDir("/")
		if len(entries) != 0 {
			t.Errorf("Expected filesystem to be empty, found %d entries", len(entries))
		}
	})

	t.Run("validation failure on first operation", func(t *testing.T) {
		ResetSequenceCounter()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)
		sfs := New()
		
		// Create a file that will cause validation to fail for copy
		if err := osFS.WriteFile("conflict.txt", []byte("existing"), 0644); err != nil {
			t.Fatalf("Failed to create conflict file: %v", err)
		}

		// First operation tries to copy non-existent file
		ops := []Operation{
			sfs.Copy("/nonexistent/source.txt", "conflict.txt"), // Will fail validation - source doesn't exist
			sfs.CreateFile("should-not-run.txt", []byte("nope"), 0644),
		}

		result, err := Run(context.Background(), fs, ops...)
		
		// Should have error
		if err == nil {
			t.Fatal("Expected error when first operation validation fails")
		}

		// Check it's a PipelineError
		var pipelineErr *PipelineError
		if !errors.As(err, &pipelineErr) {
			t.Fatalf("Expected PipelineError, got %T", err)
		}

		// Verify pipeline error details for validation failure
		if pipelineErr.FailedIndex != 1 {
			t.Errorf("Expected FailedIndex=1, got %d", pipelineErr.FailedIndex)
		}

		if len(pipelineErr.SuccessfulOps) != 0 {
			t.Errorf("Expected 0 successful operations, got %d", len(pipelineErr.SuccessfulOps))
		}

		// Verify the operation result shows validation failure
		if len(result.GetOperations()) > 0 {
			opResult := result.GetOperations()[0].(OperationResult)
			if opResult.Status != StatusValidation {
				t.Errorf("Expected first operation status to be StatusValidation, got %v", opResult.Status)
			}
		}

		// Verify second operation was not created
		shouldNotExist := "should-not-run.txt"
		if _, err := fs.Stat(shouldNotExist); err == nil {
			t.Error("Second operation should not have been executed")
		}
	})
}
