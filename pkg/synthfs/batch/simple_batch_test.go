package batch_test

import (
	"context"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

// TestBatchPrerequisiteResolution verifies that prerequisite resolution works correctly
func TestBatchPrerequisiteResolution(t *testing.T) {
	t.Run("Batch creates files with prerequisite resolution", func(t *testing.T) {
		// Create a test filesystem
		fs := testutil.NewTestFileSystem()
		registry := synthfs.GetDefaultRegistry()

		// Create a batch (prerequisite resolution is enabled by default)
		b := batch.NewBatch(fs, registry)

		// Create a file in a nested directory that doesn't exist
		_, err := b.CreateFile("nested/dir/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add CreateFile to batch: %v", err)
		}

		// Execute the batch - should use prerequisite resolution
		result, err := b.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}

		res := result.(batch.Result)
		if !res.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", res.GetError())
		}

		// Verify the file was created
		if !testutil.FileExists(t, fs, "nested/dir/file.txt") {
			t.Error("File was not created in nested directory")
		}

		// Verify parent directories were created
		if !testutil.FileExists(t, fs, "nested") {
			t.Error("Parent directory 'nested' was not created")
		}

		if !testutil.FileExists(t, fs, "nested/dir") {
			t.Error("Parent directory 'nested/dir' was not created")
		}
	})

	t.Run("Complex operations with prerequisites", func(t *testing.T) {
		// Create a test filesystem
		fs := testutil.NewTestFileSystem()
		registry := synthfs.GetDefaultRegistry()

		// Create a batch
		b := batch.NewBatch(fs, registry)

		// Create a file in a deep nested directory
		_, err := b.CreateFile("deep/nested/path/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to add CreateFile: %v", err)
		}

		// Execute the batch
		result, err := b.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}

		res := result.(batch.Result)
		if !res.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", res.GetError())
		}

		// Verify everything was created
		if !testutil.FileExists(t, fs, "deep/nested/path/file.txt") {
			t.Error("File was not created")
		}

		if !testutil.FileExists(t, fs, "deep/nested/path") {
			t.Error("Parent directories were not created")
		}
	})
}

// TestBatchPrerequisiteResolutionForAllOperations verifies that prerequisite resolution works for all operation types
func TestBatchPrerequisiteResolutionForAllOperations(t *testing.T) {
	t.Run("Prerequisites are resolved for complex operations", func(t *testing.T) {
		fs := testutil.NewTestFileSystem()
		registry := synthfs.GetDefaultRegistry()

		// Create some source files
		testutil.CreateTestFile(t, fs, "source1.txt", []byte("content1"))
		testutil.CreateTestFile(t, fs, "source2.txt", []byte("content2"))

		b := batch.NewBatch(fs, registry)

		// Create operations that require prerequisites
		_, err := b.Copy("source1.txt", "dest/dir/copy1.txt")
		if err != nil {
			t.Fatalf("Failed to add Copy operation: %v", err)
		}

		_, err = b.Move("source2.txt", "dest/dir/move2.txt")
		if err != nil {
			t.Fatalf("Failed to add Move operation: %v", err)
		}

		_, err = b.CreateSymlink("../../source1.txt", "dest/dir/link.txt")
		if err != nil {
			t.Fatalf("Failed to add CreateSymlink operation: %v", err)
		}

		// Execute with timeout to ensure it doesn't hang
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := b.WithContext(ctx).Run()
		if err != nil {
			t.Fatalf("Batch execution failed with error: %v", err)
		}

		res := result.(batch.Result)
		if !res.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", res.GetError())
		}

		// Verify all operations succeeded
		if !testutil.FileExists(t, fs, "dest/dir/copy1.txt") {
			t.Error("Copy operation did not create destination file")
		}

		if !testutil.FileExists(t, fs, "dest/dir/move2.txt") {
			t.Error("Move operation did not create destination file")
		}

		if !testutil.FileExists(t, fs, "dest/dir/link.txt") {
			t.Error("CreateSymlink operation did not create symlink")
		}

		// Verify parent directories were created
		if !testutil.FileExists(t, fs, "dest") {
			t.Error("Parent directory 'dest' was not created")
		}

		if !testutil.FileExists(t, fs, "dest/dir") {
			t.Error("Parent directory 'dest/dir' was not created")
		}
	})
}
