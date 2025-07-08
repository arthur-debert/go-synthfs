package synthfs

import (
	"context"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// TestSimpleBatchBasicFunctionality verifies that SimpleBatch works correctly
func TestSimpleBatchBasicFunctionality(t *testing.T) {
	t.Run("NewBatchWithSimpleBatch creates files with prerequisites", func(t *testing.T) {
		// Create a test filesystem
		fs := NewTestFileSystem()
		
		// Create a SimpleBatch
		batch := NewBatchWithSimpleBatch().WithFileSystem(fs)
		
		// Create a file in a nested directory that doesn't exist
		_, err := batch.CreateFile("nested/dir/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add CreateFile to SimpleBatch: %v", err)
		}
		
		// Execute the batch - should use prerequisite resolution
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("SimpleBatch execution failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("SimpleBatch execution was not successful: %v", result.Errors)
		}
		
		// Verify the file was created
		if !fs.FileExists("nested/dir/file.txt") {
			t.Error("File was not created in nested directory")
		}
		
		// Verify parent directories were created
		if !fs.FileExists("nested") {
			t.Error("Parent directory 'nested' was not created")
		}
		
		if !fs.FileExists("nested/dir") {
			t.Error("Parent directory 'nested/dir' was not created")
		}
	})
	
	t.Run("WithSimpleBatch toggles behavior", func(t *testing.T) {
		// Create a test filesystem
		fs := NewTestFileSystem()
		
		// Create a regular batch and enable SimpleBatch mode
		batch := NewBatch().WithFileSystem(fs).WithSimpleBatch(true)
		
		// Create a file in a nested directory
		_, err := batch.CreateFile("deep/nested/path/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add CreateFile: %v", err)
		}
		
		// Execute the batch
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Batch execution was not successful: %v", result.Errors)
		}
		
		// Verify everything was created
		if !fs.FileExists("deep/nested/path/file.txt") {
			t.Error("File was not created")
		}
		
		if !fs.FileExists("deep/nested/path") {
			t.Error("Parent directories were not created")
		}
	})
}

// TestSimpleBatchPrerequisiteResolution verifies that prerequisite resolution works
func TestSimpleBatchPrerequisiteResolution(t *testing.T) {
	t.Run("Prerequisites are resolved for complex operations", func(t *testing.T) {
		fs := NewTestFileSystem()
		
		// Create some source files
		fs.WriteFile("source1.txt", []byte("content1"), 0644)
		fs.WriteFile("source2.txt", []byte("content2"), 0644)
		
		batch := NewBatchWithSimpleBatch().WithFileSystem(fs)
		
		// Create operations that require prerequisites
		_, err := batch.Copy("source1.txt", "dest/dir/copy1.txt")
		if err != nil {
			t.Fatalf("Failed to add Copy operation: %v", err)
		}
		
		_, err = batch.Move("source2.txt", "dest/dir/move2.txt")
		if err != nil {
			t.Fatalf("Failed to add Move operation: %v", err)
		}
		
		_, err = batch.CreateSymlink("../source1.txt", "dest/dir/link.txt")
		if err != nil {
			t.Fatalf("Failed to add CreateSymlink operation: %v", err)
		}
		
		// Execute with timeout to ensure it doesn't hang
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		result, err := batch.WithContext(ctx).Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Batch execution was not successful: %v", result.Errors)
		}
		
		// Verify all operations succeeded
		if !fs.FileExists("dest/dir/copy1.txt") {
			t.Error("Copy operation did not create destination file")
		}
		
		if !fs.FileExists("dest/dir/move2.txt") {
			t.Error("Move operation did not create destination file")
		}
		
		if !fs.FileExists("dest/dir/link.txt") {
			t.Error("CreateSymlink operation did not create symlink")
		}
		
		// Verify parent directories were created
		if !fs.FileExists("dest") {
			t.Error("Parent directory 'dest' was not created")
		}
		
		if !fs.FileExists("dest/dir") {
			t.Error("Parent directory 'dest/dir' was not created")
		}
	})
}

// TestSimpleBatchVsLegacyBehavior compares the two modes
func TestSimpleBatchVsLegacyBehavior(t *testing.T) {
	t.Run("Both modes create same end result", func(t *testing.T) {
		// Test legacy mode
		legacyFS := NewTestFileSystem()
		legacyBatch := NewBatch().WithFileSystem(legacyFS)
		
		_, err := legacyBatch.CreateFile("nested/deep/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add file to legacy batch: %v", err)
		}
		
		legacyResult, err := legacyBatch.Run()
		if err != nil {
			t.Fatalf("Legacy batch failed: %v", err)
		}
		
		// Test SimpleBatch mode
		simpleFS := NewTestFileSystem()
		simpleBatch := NewBatchWithSimpleBatch().WithFileSystem(simpleFS)
		
		_, err = simpleBatch.CreateFile("nested/deep/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add file to simple batch: %v", err)
		}
		
		simpleResult, err := simpleBatch.Run()
		if err != nil {
			t.Fatalf("Simple batch failed: %v", err)
		}
		
		// Both should succeed
		if !legacyResult.Success || !simpleResult.Success {
			t.Errorf("Legacy success: %v, Simple success: %v", legacyResult.Success, simpleResult.Success)
		}
		
		// Both should create the same files
		if legacyFS.FileExists("nested/deep/file.txt") != simpleFS.FileExists("nested/deep/file.txt") {
			t.Error("Different file creation results between modes")
		}
		
		if legacyFS.FileExists("nested") != simpleFS.FileExists("nested") {
			t.Error("Different directory creation results between modes")
		}
	})
}

// TestPrerequisiteDeclaration verifies that operations properly declare prerequisites
func TestPrerequisiteDeclaration(t *testing.T) {
	t.Run("Operations declare expected prerequisites", func(t *testing.T) {
		fs := NewTestFileSystem()
		batch := NewBatchWithSimpleBatch().WithFileSystem(fs)
		
		// Create a file operation
		fileOp, err := batch.CreateFile("subdir/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}
		
		// Check that the operation declares prerequisites
		if prerequisiteGetter, ok := fileOp.(interface{ Prerequisites() []core.Prerequisite }); ok {
			prereqs := prerequisiteGetter.Prerequisites()
			
			if len(prereqs) != 2 {
				t.Errorf("Expected 2 prerequisites for file creation, got %d", len(prereqs))
			}
			
			// Check for parent directory prerequisite
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
				t.Error("File operation should declare parent_dir prerequisite")
			}
			
			if !hasNoConflict {
				t.Error("File operation should declare no_conflict prerequisite")
			}
		} else {
			t.Error("Operation should implement Prerequisites method")
		}
	})
}

// TestErrorHandling verifies error handling in SimpleBatch mode
func TestErrorHandling(t *testing.T) {
	t.Run("SimpleBatch handles prerequisite validation errors", func(t *testing.T) {
		fs := NewTestFileSystem()
		batch := NewBatchWithSimpleBatch().WithFileSystem(fs)
		
		// Try to copy from non-existent source
		_, err := batch.Copy("nonexistent.txt", "dest/copy.txt")
		if err != nil {
			t.Fatalf("Failed to add Copy operation: %v", err)
		}
		
		// Execute - should fail due to missing source
		result, err := batch.Run()
		if err != nil {
			// This is acceptable - the batch failed with an error
			t.Logf("Batch failed with error as expected: %v", err)
		} else if result.Success {
			t.Error("Batch should have failed due to missing source file")
		}
	})
}

// TestMigrationPath verifies that existing code can migrate gradually
func TestMigrationPath(t *testing.T) {
	t.Run("Existing code can opt into SimpleBatch behavior", func(t *testing.T) {
		fs := NewTestFileSystem()
		
		// Start with legacy batch
		batch := NewBatch().WithFileSystem(fs)
		
		// Migrate to SimpleBatch behavior
		batch = batch.WithSimpleBatch(true)
		
		// Should now use prerequisite resolution
		_, err := batch.CreateFile("migrated/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add file after migration: %v", err)
		}
		
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Migrated batch failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Migrated batch was not successful: %v", result.Errors)
		}
		
		// Verify file was created
		if !fs.FileExists("migrated/file.txt") {
			t.Error("File was not created after migration")
		}
	})
}