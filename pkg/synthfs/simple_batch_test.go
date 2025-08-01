package synthfs

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Define a custom type for context keys to avoid collisions
type testContextKey string

func TestSimpleBatchAPI(t *testing.T) {
	// Use sequence generator for predictable IDs
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	SetIDGenerator(SequenceIDGenerator)
	
	t.Run("Basic batch operations", func(t *testing.T) {
		ResetSequenceCounter()
		fs := filesystem.NewTestFileSystem()
		
		batch := NewSimpleBatch(fs)
		batch.
			CreateDir("dir1", 0755).
			WriteFile("dir1/file1.txt", []byte("content1"), 0644).
			WriteFile("dir1/file2.txt", []byte("content2"), 0644).
			Copy("dir1/file1.txt", "dir1/file1-copy.txt")
		
		if len(batch.Operations()) != 4 {
			t.Errorf("Expected 4 operations, got %d", len(batch.Operations()))
		}
		
		err := batch.Execute()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
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
	
	t.Run("Batch with failure", func(t *testing.T) {
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
			CreateDir("existing.txt", 0755) // This should fail
		
		err = batch.Execute()
		if err == nil {
			t.Fatal("Expected batch to fail on conflicting directory creation")
		}
		
		// Check error type
		if pipelineErr, ok := err.(*PipelineError); ok {
			if pipelineErr.FailedIndex != 3 {
				t.Errorf("Expected failure at operation 3, got %d", pipelineErr.FailedIndex)
			}
			if pipelineErr.TotalOps != 3 {
				t.Errorf("Expected 3 total operations, got %d", pipelineErr.TotalOps)
			}
			if len(pipelineErr.SuccessfulOps) != 2 {
				t.Errorf("Expected 2 successful operations, got %d", len(pipelineErr.SuccessfulOps))
			}
		} else {
			t.Errorf("Expected PipelineError, got %T", err)
		}
		
		// Verify partial success
		if _, err := fs.Stat("dir1"); err != nil {
			t.Error("Directory dir1 should have been created before failure")
		}
		if _, err := fs.Stat("dir1/file1.txt"); err != nil {
			t.Error("File dir1/file1.txt should have been created before failure")
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
		
		batch := NewSimpleBatch(fs)
		batch.
			CreateDir("testdir", 0755).
			WriteFile("testdir/file.txt", []byte("content"), 0644).
			Copy("source.txt", "source-copy.txt").
			Move("source-copy.txt", "source-moved.txt").
			CreateSymlink("source.txt", "source-link.txt").
			Delete("source.txt")
		
		if len(batch.Operations()) != 6 {
			t.Errorf("Expected 6 operations, got %d", len(batch.Operations()))
		}
		
		err = batch.Execute()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
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
	// Use sequence generator for predictable IDs
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	SetIDGenerator(SequenceIDGenerator)
	
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
		fs := filesystem.NewTestFileSystem()
		
		// This is a simplified test - in reality, rollback errors would occur
		// when files are locked, permissions changed, etc.
		batch := NewSimpleBatch(fs)
		batch.CreateDir("dir1", 0755)
		
		// Execute successfully first
		err := batch.Execute()
		if err != nil {
			t.Fatalf("Initial execution failed: %v", err)
		}
		
		// Now if we try ExecuteWithRollback on an empty batch, it should succeed
		batch.Clear()
		err = batch.ExecuteWithRollback()
		if err != nil {
			t.Error("Empty batch rollback should succeed")
		}
	})
}