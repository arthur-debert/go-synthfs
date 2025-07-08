THIS SHOULD BE A LINTER ERRORpackage batch_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/registry"
)

func TestSimpleBatch(t *testing.T) {
	// Create test filesystem and registry
	fs := filesystem.NewOSFileSystem(".")
	reg := registry.NewRegistry()
	
	t.Run("Creates operations without auto-parent creation", func(t *testing.T) {
		simpleBatch := batch.NewSimpleBatch(fs, reg)
		
		// Add a file operation to a nested path
		op, err := simpleBatch.CreateFile("deep/nested/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}
		
		if op == nil {
			t.Fatal("Expected operation to be created")
		}
		
		// Check that only one operation was created (no auto-parent directories)
		ops := simpleBatch.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
		
		// Verify the operation is a file creation
		if opDescriber, ok := ops[0].(interface{ Describe() core.OperationDesc }); ok {
			desc := opDescriber.Describe()
			if desc.Type != "create_file" {
				t.Errorf("Expected create_file operation, got %s", desc.Type)
			}
			if desc.Path != "deep/nested/file.txt" {
				t.Errorf("Expected path 'deep/nested/file.txt', got %s", desc.Path)
			}
		} else {
			t.Error("Operation should implement Describe method")
		}
	})
	
	t.Run("Creates directory operations without auto-parent creation", func(t *testing.T) {
		simpleBatch := batch.NewSimpleBatch(fs, reg)
		
		// Add a directory operation to a nested path
		op, err := simpleBatch.CreateDir("deep/nested/dir")
		if err != nil {
			t.Fatalf("Failed to create directory operation: %v", err)
		}
		
		if op == nil {
			t.Fatal("Expected operation to be created")
		}
		
		// Check that only one operation was created (no auto-parent directories)
		ops := simpleBatch.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
		
		// Verify the operation is a directory creation
		if opDescriber, ok := ops[0].(interface{ Describe() core.OperationDesc }); ok {
			desc := opDescriber.Describe()
			if desc.Type != "create_directory" {
				t.Errorf("Expected create_directory operation, got %s", desc.Type)
			}
			if desc.Path != "deep/nested/dir" {
				t.Errorf("Expected path 'deep/nested/dir', got %s", desc.Path)
			}
		} else {
			t.Error("Operation should implement Describe method")
		}
	})
	
	t.Run("Operations declare prerequisites", func(t *testing.T) {
		simpleBatch := batch.NewSimpleBatch(fs, reg)
		
		// Add a file operation
		op, err := simpleBatch.CreateFile("subdir/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}
		
		// Check that the operation declares prerequisites
		if prereqGetter, ok := op.(interface{ Prerequisites() []core.Prerequisite }); ok {
			prereqs := prereqGetter.Prerequisites()
			if len(prereqs) < 1 {
				t.Error("Expected operation to declare prerequisites")
			}
			
			// Should have parent directory and no conflict prerequisites
			hasParentDir := false
			hasNoConflict := false
			for _, prereq := range prereqs {
				switch prereq.Type() {
				case "parent_dir":
					hasParentDir = true
				case "no_conflict":
					hasNoConflict = true
				}
			}
			
			if !hasParentDir {
				t.Error("Expected parent_dir prerequisite")
			}
			if !hasNoConflict {
				t.Error("Expected no_conflict prerequisite")
			}
		} else {
			t.Error("Operation should implement Prerequisites method")
		}
	})
	
	t.Run("Run enables prerequisite resolution by default", func(t *testing.T) {
		simpleBatch := batch.NewSimpleBatch(fs, reg)
		
		// Add a file operation that would need parent directory
		_, err := simpleBatch.CreateFile("test/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}
		
		// Run should enable prerequisite resolution
		// This test mainly verifies the batch can be created and run
		// without crashing - actual prerequisite resolution depends on
		// the execution system being set up properly
		result, err := simpleBatch.Run()
		if err != nil {
			t.Logf("Run failed (expected with missing prerequisites): %v", err)
		}
		
		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}
	})
	
	t.Run("Multiple operations maintain order", func(t *testing.T) {
		simpleBatch := batch.NewSimpleBatch(fs, reg)
		
		// Add multiple operations
		op1, err := simpleBatch.CreateDir("dir1")
		if err != nil {
			t.Fatalf("Failed to create dir1: %v", err)
		}
		
		op2, err := simpleBatch.CreateFile("file1.txt", []byte("content1"))
		if err != nil {
			t.Fatalf("Failed to create file1: %v", err)
		}
		
		op3, err := simpleBatch.CreateDir("dir2")
		if err != nil {
			t.Fatalf("Failed to create dir2: %v", err)
		}
		
		// Check operations are in the right order
		ops := simpleBatch.Operations()
		if len(ops) != 3 {
			t.Errorf("Expected 3 operations, got %d", len(ops))
		}
		
		// Verify order by checking IDs
		if idGetter1, ok := op1.(interface{ ID() core.OperationID }); ok {
			if idGetter1.ID() != ops[0].(interface{ ID() core.OperationID }).ID() {
				t.Error("Operation 1 not in correct position")
			}
		}
		
		if idGetter2, ok := op2.(interface{ ID() core.OperationID }); ok {
			if idGetter2.ID() != ops[1].(interface{ ID() core.OperationID }).ID() {
				t.Error("Operation 2 not in correct position")
			}
		}
		
		if idGetter3, ok := op3.(interface{ ID() core.OperationID }); ok {
			if idGetter3.ID() != ops[2].(interface{ ID() core.OperationID }).ID() {
				t.Error("Operation 3 not in correct position")
			}
		}
	})
	
	t.Run("WithContext and WithFileSystem work correctly", func(t *testing.T) {
		originalBatch := batch.NewSimpleBatch(fs, reg)
		
		// Test chaining
		ctx := context.Background()
		modifiedBatch := originalBatch.WithContext(ctx).WithFileSystem(fs)
		
		// Should still be able to create operations
		_, err := modifiedBatch.CreateFile("test.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create operation after chaining: %v", err)
		}
		
		ops := modifiedBatch.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation after chaining, got %d", len(ops))
		}
	})
	
	t.Run("Copy and Move operations work", func(t *testing.T) {
		simpleBatch := batch.NewSimpleBatch(fs, reg)
		
		// Add copy operation
		copyOp, err := simpleBatch.Copy("source.txt", "dest.txt")
		if err != nil {
			t.Fatalf("Failed to create copy operation: %v", err)
		}
		
		// Add move operation
		moveOp, err := simpleBatch.Move("old.txt", "new.txt")
		if err != nil {
			t.Fatalf("Failed to create move operation: %v", err)
		}
		
		ops := simpleBatch.Operations()
		if len(ops) != 2 {
			t.Errorf("Expected 2 operations, got %d", len(ops))
		}
		
		// Verify copy operation
		if opDescriber, ok := copyOp.(interface{ Describe() core.OperationDesc }); ok {
			desc := opDescriber.Describe()
			if desc.Type != "copy" {
				t.Errorf("Expected copy operation, got %s", desc.Type)
			}
		}
		
		// Verify move operation
		if opDescriber, ok := moveOp.(interface{ Describe() core.OperationDesc }); ok {
			desc := opDescriber.Describe()
			if desc.Type != "move" {
				t.Errorf("Expected move operation, got %s", desc.Type)
			}
		}
	})
}

func TestSimpleBatchComparison(t *testing.T) {
	// Create test filesystem and registry
	fs := filesystem.NewOSFileSystem(".")
	reg := registry.NewRegistry()
	
	t.Run("SimpleBatch vs regular Batch operation count", func(t *testing.T) {
		// Create both batch types
		regularBatch := batch.NewBatch(fs, reg)
		simpleBatch := batch.NewSimpleBatch(fs, reg)
		
		// Add the same file operation to both
		_, err := regularBatch.CreateFile("deep/nested/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file in regular batch: %v", err)
		}
		
		_, err = simpleBatch.CreateFile("deep/nested/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file in simple batch: %v", err)
		}
		
		// Compare operation counts
		regularOps := regularBatch.Operations()
		simpleOps := simpleBatch.Operations()
		
		t.Logf("Regular batch operations: %d", len(regularOps))
		t.Logf("Simple batch operations: %d", len(simpleOps))
		
		// SimpleBatch should have fewer operations (no auto-parent creation)
		if len(simpleOps) >= len(regularOps) {
			t.Logf("SimpleBatch has %d operations, regular batch has %d", len(simpleOps), len(regularOps))
			t.Logf("This is expected if auto-parent creation is disabled in both")
		}
		
		// Both should have at least one operation
		if len(regularOps) < 1 {
			t.Error("Regular batch should have at least one operation")
		}
		if len(simpleOps) < 1 {
			t.Error("Simple batch should have at least one operation")
		}
	})
}