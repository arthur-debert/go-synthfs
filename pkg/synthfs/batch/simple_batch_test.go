package batch_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

// TestSimpleBatchBasicOperations tests basic functionality of SimpleBatch
func TestSimpleBatchBasicOperations(t *testing.T) {
	t.Run("SimpleBatch creates operations without auto-parent creation", func(t *testing.T) {
		fs := testutil.NewOperationsMockFS()
		registry := operations.NewFactory()
		
		// Create a SimpleBatch
		simpleBatch := batch.NewSimpleBatch(fs, registry)
		
		// Add a file operation that would need parent directory
		_, err := simpleBatch.CreateFile("subdir/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile should succeed without auto-parent creation: %v", err)
		}
		
		// Check that only one operation was created (no auto-parent)
		ops := simpleBatch.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
		
		// Verify it's a create_file operation
		if desc, ok := ops[0].(interface{ Describe() core.OperationDesc }); ok {
			opDesc := desc.Describe()
			if opDesc.Type != "create_file" {
				t.Errorf("Expected create_file operation, got %s", opDesc.Type)
			}
			if opDesc.Path != "subdir/file.txt" {
				t.Errorf("Expected path 'subdir/file.txt', got %s", opDesc.Path)
			}
		} else {
			t.Error("Operation should implement Describe()")
		}
	})
	
	t.Run("SimpleBatch operations declare prerequisites", func(t *testing.T) {
		fs := testutil.NewOperationsMockFS()
		registry := operations.NewFactory()
		
		simpleBatch := batch.NewSimpleBatch(fs, registry)
		
		// Create a file operation
		op, err := simpleBatch.CreateFile("subdir/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		
		// Check if operation declares prerequisites
		if prereqGetter, ok := op.(interface{ Prerequisites() []core.Prerequisite }); ok {
			prereqs := prereqGetter.Prerequisites()
			
			// Should have parent_dir and no_conflict prerequisites
			if len(prereqs) < 2 {
				t.Errorf("Expected at least 2 prerequisites, got %d", len(prereqs))
			}
			
			// Check for expected prerequisite types
			foundParentDir := false
			foundNoConflict := false
			
			for _, prereq := range prereqs {
				switch prereq.Type() {
				case "parent_dir":
					foundParentDir = true
				case "no_conflict":
					foundNoConflict = true
				}
			}
			
			if !foundParentDir {
				t.Error("Expected parent_dir prerequisite")
			}
			if !foundNoConflict {
				t.Error("Expected no_conflict prerequisite")
			}
		} else {
			t.Error("Operation should declare prerequisites")
		}
	})
}

// TestBatchOptionsFactory tests the factory method with different options
func TestBatchOptionsFactory(t *testing.T) {
	t.Run("NewBatchWithOptions creates SimpleBatch when UseSimpleBatch is true", func(t *testing.T) {
		fs := testutil.NewOperationsMockFS()
		registry := operations.NewFactory()
		
		// Create batch with SimpleBatch option
		opts := batch.BatchOptions{UseSimpleBatch: true}
		b := batch.NewBatchWithOptions(fs, registry, opts)
		
		// Add operation to verify it works
		_, err := b.CreateFile("test.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		
		// Verify operation was created
		ops := b.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("NewBatchWithOptions creates legacy Batch when UseSimpleBatch is false", func(t *testing.T) {
		fs := testutil.NewOperationsMockFS()
		registry := operations.NewFactory()
		
		// Create batch with legacy option
		opts := batch.BatchOptions{UseSimpleBatch: false}
		b := batch.NewBatchWithOptions(fs, registry, opts)
		
		// Add operation to verify it works
		_, err := b.CreateFile("test.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		
		// Verify operation was created
		ops := b.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
}

// TestSimpleBatchExecution tests that SimpleBatch can execute with prerequisite resolution
func TestSimpleBatchExecution(t *testing.T) {
	t.Run("SimpleBatch runs with prerequisite resolution", func(t *testing.T) {
		fs := testutil.NewOperationsMockFS()
		registry := operations.NewFactory()
		
		simpleBatch := batch.NewSimpleBatch(fs, registry)
		
		// Create a file that needs parent directory
		_, err := simpleBatch.CreateFile("subdir/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		
		// Run with prerequisite resolution - this should create parent dir automatically
		result, err := simpleBatch.RunWithPrerequisites()
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		
		// Check if result indicates success
		if resultChecker, ok := result.(interface{ IsSuccess() bool }); ok {
			if !resultChecker.IsSuccess() {
				t.Error("Execution should succeed with prerequisite resolution")
			}
		}
	})
}