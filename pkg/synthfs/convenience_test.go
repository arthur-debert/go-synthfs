package synthfs

import (
	"context"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestDirectExecutionMethods(t *testing.T) {
	ctx := context.Background()

	t.Run("WriteFile", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()

		err := WriteFile(ctx, fs, "test/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Verify file was created
		info, err := fs.Stat("test/file.txt")
		if err != nil {
			t.Fatalf("File not created: %v", err)
		}
		if info.Size() != 7 {
			t.Errorf("Expected file size 7, got: %d", info.Size())
		}
	})

	t.Run("MkdirAll", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()

		err := MkdirAll(ctx, fs, "test/nested/dir", 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		// Verify directory was created
		info, err := fs.Stat("test/nested/dir")
		if err != nil {
			t.Fatalf("Directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("Expected directory, got file")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()

		// Create a file first
		err := WriteFile(ctx, fs, "test/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Remove it
		err = Remove(ctx, fs, "test/file.txt")
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		// Verify file was removed
		_, err = fs.Stat("test/file.txt")
		if err == nil {
			t.Error("File should have been removed")
		}
	})

	t.Run("WriteFile with validation error", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()

		// Try to write with empty path
		err := WriteFile(ctx, fs, "", []byte("content"), 0644)
		if err == nil {
			t.Error("Expected validation error for empty path")
		}
		if !strings.Contains(err.Error(), "path cannot be empty") {
			t.Errorf("Expected 'path cannot be empty' error, got: %v", err)
		}
	})
}
