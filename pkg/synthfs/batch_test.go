package synthfs_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestBatchBasicUsage(t *testing.T) {
	// Use a test filesystem for controlled testing
	testFS := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch().WithFileSystem(testFS).WithContext(context.Background())

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

		desc := op.Describe()
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

		desc := op.Describe()
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

		desc := op.Describe()
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
	testFS := synthfs.NewTestFileSystem()

	// Pre-populate with some files
	if err := testFS.WriteFile("existing.txt", []byte("existing content"), 0644); err != nil {
		t.Fatalf("Failed to setup test file: %v", err)
	}
	if err := testFS.MkdirAll("existing-dir", 0755); err != nil {
		t.Fatalf("Failed to setup test directory: %v", err)
	}

	batch := synthfs.NewBatch().WithFileSystem(testFS)

	t.Run("CreateFile with parent directory auto-creation", func(t *testing.T) {
		// This should auto-create the "auto-dir" directory
		_, err := batch.CreateFile("auto-dir/nested-file.txt", []byte("nested content"))
		if err != nil {
			t.Fatalf("CreateFile with nested path failed: %v", err)
		}

		ops := batch.Operations()
		// Should have at least 2 operations: CreateDir for parent + CreateFile
		if len(ops) < 2 {
			t.Errorf("Expected at least 2 operations (auto-dir creation + file creation), got %d", len(ops))
		}

		// Check that auto-dir creation operation was added
		foundAutoDirOp := false
		for _, op := range ops {
			desc := op.Describe()
			if desc.Type == "create_directory" && desc.Path == "auto-dir" {
				foundAutoDirOp = true
				break
			}
		}
		if !foundAutoDirOp {
			t.Error("Expected auto-generated CreateDir operation for 'auto-dir', but not found")
		}
	})
}

func TestBatchIDGeneration(t *testing.T) {
	batch := synthfs.NewBatch()

	// Create multiple operations and check ID uniqueness
	op1, _ := batch.CreateDir("dir1")
	op2, _ := batch.CreateDir("dir2")
	op3, _ := batch.CreateFile("file1.txt", []byte("content"))

	id1 := op1.ID()
	id2 := op2.ID()
	id3 := op3.ID()

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
