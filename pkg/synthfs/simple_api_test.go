package synthfs

import (
	"context"
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
