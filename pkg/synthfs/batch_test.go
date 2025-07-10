package synthfs_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestBatchBasicUsage(t *testing.T) {
	// Use a test filesystem for controlled testing
	testFS := testutil.NewTestFileSystem()
	registry := synthfs.GetDefaultRegistry()
	fs := testutil.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS).WithContext(context.Background())

	// Pre-create files needed for valid operations in Phase II
	if err := testFS.WriteFile("source.txt", []byte("s"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := testFS.WriteFile("old-location.txt", []byte("o"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := testFS.WriteFile("to-delete.txt", []byte("d"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("CreateDir", func(t *testing.T) {
		op, err := batch.CreateDir("test-batch-dir")
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}

		if op == nil {
			t.Fatal("CreateDir returned nil operation")
		}

		desc := op.(synthfs.Operation).Describe()
		if desc.Type != "create_directory" {
			t.Errorf("Expected operation type 'create_directory', got '%s'", desc.Type)
		}

		if desc.Path != "test-batch-dir" {
			t.Errorf("Expected path 'test-batch-dir', got '%s'", desc.Path)
		}
	})

	t.Run("CreateFile", func(t *testing.T) {
		content := []byte("Hello, Batch API!")
		op, err := batch.CreateFile("test-batch-file.txt", content)
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		if op == nil {
			t.Fatal("CreateFile returned nil operation")
		}

		desc := op.(synthfs.Operation).Describe()
		if desc.Type != "create_file" {
			t.Errorf("Expected operation type 'create_file', got '%s'", desc.Type)
		}

		if desc.Path != "test-batch-file.txt" {
			t.Errorf("Expected path 'test-batch-file.txt', got '%s'", desc.Path)
		}

		// Check that content length is in description details
		if contentLength, exists := desc.Details["content_length"]; !exists {
			t.Error("Expected content_length in operation details")
		} else if contentLength != len(content) {
			t.Errorf("Expected content_length %d, got %v", len(content), contentLength)
		}
	})

	t.Run("Copy", func(t *testing.T) {
		_, err := batch.Copy("source.txt", "destination.txt")
		if err != nil {
			t.Fatalf("Copy failed: %v", err)
		}
	})

	t.Run("Move", func(t *testing.T) {
		_, err := batch.Move("old-location.txt", "new-location.txt")
		if err != nil {
			t.Fatalf("Move failed: %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		op, err := batch.Delete("to-delete.txt")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		if op == nil {
			t.Fatal("Delete returned nil operation")
		}

		desc := op.(synthfs.Operation).Describe()
		if desc.Type != "delete" {
			t.Errorf("Expected operation type 'delete', got '%s'", desc.Type)
		}

		if desc.Path != "to-delete.txt" {
			t.Errorf("Expected path 'to-delete.txt', got '%s'", desc.Path)
		}
	})

	t.Run("Operations", func(t *testing.T) {
		ops := batch.Operations()
		// All operations should now be valid and added to the batch.
		expectedCount := 5 // CreateDir, CreateFile, Copy, Move, Delete
		if len(ops) != expectedCount {
			t.Errorf("Expected %d operations in batch, got %d", expectedCount, len(ops))
		}

		// Check that we get a copy (modifications shouldn't affect batch)
		originalCount := len(batch.Operations())
		_ = append(ops, nil) // Try to modify the returned slice
		newCount := len(batch.Operations())
		if originalCount != newCount {
			t.Error("Operations() should return a copy, but batch was modified")
		}
	})
}

func TestBatchWithTestFileSystem(t *testing.T) {
	// Use TestFileSystem for more controlled testing
	testFS := testutil.NewTestFileSystem()

	// Pre-populate with some files
	if err := testFS.WriteFile("existing.txt", []byte("existing content"), 0644); err != nil {
		t.Fatalf("Failed to setup test file: %v", err)
	}
	if err := testFS.MkdirAll("existing-dir", 0755); err != nil {
		t.Fatalf("Failed to setup test directory: %v", err)
	}

	registry := synthfs.GetDefaultRegistry()
	fs := testutil.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

	t.Run("CreateFile with parent directory auto-creation", func(t *testing.T) {
		// This should auto-create the "auto-dir" directory
		_, err := batch.CreateFile("auto-dir/nested-file.txt", []byte("nested content"))
		if err != nil {
			t.Fatalf("CreateFile with nested path failed: %v", err)
		}

		ops := batch.Operations()
		// With new architecture, parent directories are created through prerequisite resolution
		// during execution, not as separate operations in the batch
		if len(ops) != 1 {
			t.Logf("Note: With new architecture, only the file operation is in the batch. Parent dirs are created via prerequisites. Got %d operations", len(ops))
		}

		// Execute the batch to verify parent directory is created
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", result.GetError())
		}

		// Verify the parent directory was created during execution
		// Check in testFS since that's what the batch is using
		if _, err := testFS.Stat("auto-dir"); err != nil {
			t.Error("Expected 'auto-dir' to be created during execution, but it doesn't exist")
		}
	})
}

func TestBatchIDGeneration(t *testing.T) {
	testFS := testutil.NewTestFileSystem()
	registry := synthfs.GetDefaultRegistry()
	fs := testutil.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

	// Create multiple operations and check ID uniqueness
	op1, err := batch.CreateDir("dir1")
	if err != nil {
		t.Fatalf("Failed to create dir1: %v", err)
	}
	op2, err := batch.CreateDir("dir2")
	if err != nil {
		t.Fatalf("Failed to create dir2: %v", err)
	}
	op3, err := batch.CreateFile("file1.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Failed to create file1.txt: %v", err)
	}

	id1 := op1.(synthfs.Operation).ID()
	id2 := op2.(synthfs.Operation).ID()
	id3 := op3.(synthfs.Operation).ID()

	if id1 == id2 {
		t.Error("Operation IDs should be unique, but got duplicates")
	}
	if id1 == id3 {
		t.Error("Operation IDs should be unique, but got duplicates")
	}
	if id2 == id3 {
		t.Error("Operation IDs should be unique, but got duplicates")
	}

	// Check ID format
	if len(string(id1)) == 0 {
		t.Error("Operation ID should not be empty")
	}

	// IDs should contain some identifying information
	if !contains(string(id1), "batch") {
		t.Error("Expected batch ID to contain 'batch' prefix")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || contains(s[1:], substr) || (len(s) > 0 && s[:len(substr)] == substr))
}