package synthfs_test

import (
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestBatchCopyValidation(t *testing.T) {
	t.Run("Copy non-existent source file", func(t *testing.T) {
		// Create empty test filesystem
		testFS := filesystem.NewTestFileSystem()
		fs := testutil.NewTestFileSystem()
		batch := synthfs.NewBatch(fs).WithFileSystem(testFS)

		// Try to copy non-existent file
		_, err := batch.Copy("nonexistent.txt", "dest.txt")

		if err == nil {
			t.Error("Expected error when copying non-existent file, got nil")
		} else {
			// Verify error message contains expected text
			if !strings.Contains(err.Error(), "copy source does not exist") {
				t.Errorf("Expected error about source not existing, got: %v", err)
			}
		}
	})

	t.Run("Copy existing file", func(t *testing.T) {
		// Create test filesystem with a file
		testFS := filesystem.NewTestFileSystem()
		if err := testFS.WriteFile("source.txt", []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		fs := testutil.NewTestFileSystem()
		batch := synthfs.NewBatch(fs).WithFileSystem(testFS)

		// Copy should succeed during validation
		op, err := batch.Copy("source.txt", "dest.txt")

		if err != nil {
			t.Errorf("Expected no error when copying existing file, got: %v", err)
		}
		if op == nil {
			t.Error("Expected operation to be returned")
		}
	})
}

func TestBatchMoveValidation(t *testing.T) {
	t.Run("Move non-existent source file", func(t *testing.T) {
		// Create empty test filesystem
		testFS := filesystem.NewTestFileSystem()
		fs := testutil.NewTestFileSystem()
		batch := synthfs.NewBatch(fs).WithFileSystem(testFS)

		// Try to move non-existent file
		_, err := batch.Move("nonexistent.txt", "dest.txt")

		if err == nil {
			t.Error("Expected error when moving non-existent file, got nil")
		} else {
			// Verify error message contains expected text
			if !strings.Contains(err.Error(), "copy source does not exist") {
				t.Errorf("Expected error about source not existing, got: %v", err)
			}
		}
	})

	t.Run("Move existing file", func(t *testing.T) {
		// Create test filesystem with a file
		testFS := filesystem.NewTestFileSystem()
		if err := testFS.WriteFile("source.txt", []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		fs := testutil.NewTestFileSystem()
		batch := synthfs.NewBatch(fs).WithFileSystem(testFS)

		// Move should succeed during validation
		op, err := batch.Move("source.txt", "dest.txt")

		if err != nil {
			t.Errorf("Expected no error when moving existing file, got: %v", err)
		}
		if op == nil {
			t.Error("Expected operation to be returned")
		}
	})
}
