package synthfs

import (
	"context"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// TestBatchPrerequisiteResolution verifies that prerequisite resolution works correctly
func TestBatchPrerequisiteResolution(t *testing.T) {
	t.Run("Batch creates files with prerequisite resolution", func(t *testing.T) {
		// Create a test filesystem
		fs := NewTestFileSystem()
		registry := GetDefaultRegistry()
		
		// Create a batch (prerequisite resolution is enabled by default)
		batch := NewBatch(fs, registry)
		
		// Create a file in a nested directory that doesn't exist
		_, err := batch.CreateFile("nested/dir/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add CreateFile to batch: %v", err)
		}
		
		// Execute the batch - should use prerequisite resolution
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}
		
		if !result.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", result.GetError())
		}
		
		// Verify the file was created
		if !FileExists(t, fs, "nested/dir/file.txt") {
			t.Error("File was not created in nested directory")
		}
		
		// Verify parent directories were created
		if !FileExists(t, fs, "nested") {
			t.Error("Parent directory 'nested' was not created")
		}
		
		if !FileExists(t, fs, "nested/dir") {
			t.Error("Parent directory 'nested/dir' was not created")
		}
	})
	
	t.Run("Complex operations with prerequisites", func(t *testing.T) {
		// Create a test filesystem
		fs := NewTestFileSystem()
		registry := GetDefaultRegistry()
		
		// Create a batch
		batch := NewBatch(fs, registry)
		
		// Create a file in a deep nested directory
		_, err := batch.CreateFile("deep/nested/path/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add CreateFile: %v", err)
		}
		
		// Execute the batch
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}
		
		if !result.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", result.GetError())
		}
		
		// Verify everything was created
		if !FileExists(t, fs, "deep/nested/path/file.txt") {
			t.Error("File was not created")
		}
		
		if !FileExists(t, fs, "deep/nested/path") {
			t.Error("Parent directories were not created")
		}
	})
}

// TestBatchPrerequisiteResolutionForAllOperations verifies that prerequisite resolution works for all operation types
func TestBatchPrerequisiteResolutionForAllOperations(t *testing.T) {
	t.Run("Prerequisites are resolved for complex operations", func(t *testing.T) {
		fs := NewTestFileSystem()
		registry := GetDefaultRegistry()
		
		// Create some source files
		CreateTestFile(t, fs, "source1.txt", []byte("content1"))
		CreateTestFile(t, fs, "source2.txt", []byte("content2"))
		
		batch := NewBatch(fs, registry)
		
		// Create operations that require prerequisites
		_, err := batch.Copy("source1.txt", "dest/dir/copy1.txt")
		if err != nil {
			t.Fatalf("Failed to add Copy operation: %v", err)
		}
		
		_, err = batch.Move("source2.txt", "dest/dir/move2.txt")
		if err != nil {
			t.Fatalf("Failed to add Move operation: %v", err)
		}
		
		_, err = batch.CreateSymlink("../../source1.txt", "dest/dir/link.txt")
		if err != nil {
			t.Fatalf("Failed to add CreateSymlink operation: %v", err)
		}
		
		// Execute with timeout to ensure it doesn't hang
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		result, err := batch.WithContext(ctx).Run()
		if err != nil {
			t.Fatalf("Batch execution failed with error: %v", err)
		}
		
		if !result.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", result.GetError())
		}
		
		// Verify all operations succeeded
		if !FileExists(t, fs, "dest/dir/copy1.txt") {
			t.Error("Copy operation did not create destination file")
		}
		
		if !FileExists(t, fs, "dest/dir/move2.txt") {
			t.Error("Move operation did not create destination file")
		}
		
		if !FileExists(t, fs, "dest/dir/link.txt") {
			t.Error("CreateSymlink operation did not create symlink")
		}
		
		// Verify parent directories were created
		if !FileExists(t, fs, "dest") {
			t.Error("Parent directory 'dest' was not created")
		}
		
		if !FileExists(t, fs, "dest/dir") {
			t.Error("Parent directory 'dest/dir' was not created")
		}
	})
}

// TestPrerequisiteDeclaration verifies that operations properly declare prerequisites
func TestPrerequisiteDeclaration(t *testing.T) {
	t.Run("Operations declare expected prerequisites", func(t *testing.T) {
		fs := NewTestFileSystem()
		registry := GetDefaultRegistry()
		batch := NewBatch(fs, registry)
		
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

// TestErrorHandling verifies error handling with prerequisite resolution
func TestErrorHandling(t *testing.T) {
	t.Run("Batch handles prerequisite validation errors", func(t *testing.T) {
		fs := NewTestFileSystem()
		registry := GetDefaultRegistry()
		batch := NewBatch(fs, registry)
		
		// Try to copy from non-existent source
		_, err := batch.Copy("nonexistent.txt", "dest/copy.txt")
		if err == nil {
			t.Fatalf("Expected an error when adding a Copy operation with a non-existent source")
		}
	})
}

// TestBatchWithBackup verifies that batch works with backup/restore functionality
func TestBatchWithBackup(t *testing.T) {
	t.Run("Batch can run with backup enabled", func(t *testing.T) {
		fs := NewTestFileSystem()
		registry := GetDefaultRegistry()
		
		// Create existing file
		CreateTestFile(t, fs, "existing.txt", []byte("original"))
		
		batch := NewBatch(fs, registry)
		
		// Overwrite existing file
		_, err := batch.CreateFile("existing.txt", []byte("new content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add file: %v", err)
		}
		
		// Run with backup enabled
		executor := NewExecutor()
		pipeline := NewMemPipeline()
		for _, op := range batch.Operations() {
			pipeline.Add(op.(Operation))
		}
		result := executor.RunWithOptions(context.Background(), pipeline, fs, core.PipelineOptions{
			Restorable: true,
		})
		
		if !result.Success {
			t.Fatalf("Batch execution was not successful: %v", result.Errors)
		}
		
		// Verify file was overwritten
		AssertFileContent(t, fs, "existing.txt", []byte("new content"))
		
		// Verify restore operations were created
		if len(result.RestoreOps) == 0 {
			t.Error("No restore operations were created")
		}
	})
}