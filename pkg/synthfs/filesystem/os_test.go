package filesystem_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestOSFileSystem(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "synthfs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	osfs := filesystem.NewOSFileSystem(tempDir)

	t.Run("WriteFile and Open", func(t *testing.T) {
		content := []byte("Hello, World!")
		path := "test.txt"

		// Write file
		err := osfs.WriteFile(path, content, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Open and read file
		file, err := osfs.Open(path)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				t.Logf("Warning: failed to close file: %v", closeErr)
			}
		}()

		info, err := file.Stat()
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.IsDir() {
			t.Errorf("Expected file, got directory")
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("Expected size %d, got %d", len(content), info.Size())
		}
	})

	t.Run("MkdirAll and Stat", func(t *testing.T) {
		dirPath := "nested/deep/directory"

		// Create directory
		err := osfs.MkdirAll(dirPath, 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		// Stat directory
		info, err := osfs.Stat(dirPath)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if !info.IsDir() {
			t.Errorf("Expected directory, got file")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		// Create a file to remove
		path := "to-remove.txt"
		err := osfs.WriteFile(path, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Remove it
		err = osfs.Remove(path)
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		// Verify it's gone
		_, err = osfs.Stat(path)
		if err == nil {
			t.Errorf("Expected file to be removed")
		}
	})

	t.Run("RemoveAll", func(t *testing.T) {
		// Create a directory tree
		dirPath := "remove-tree"
		err := osfs.MkdirAll(filepath.Join(dirPath, "subdir"), 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		err = osfs.WriteFile(filepath.Join(dirPath, "file.txt"), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Remove tree
		err = osfs.RemoveAll(dirPath)
		if err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}

		// Verify it's gone
		_, err = osfs.Stat(dirPath)
		if err == nil {
			t.Errorf("Expected directory tree to be removed")
		}
	})

	t.Run("Invalid paths", func(t *testing.T) {
		invalidPath := "../../../etc/passwd"

		// WriteFile with invalid path
		err := osfs.WriteFile(invalidPath, []byte("test"), 0644)
		if err == nil {
			t.Errorf("Expected WriteFile to fail with invalid path")
		}

		// Open with invalid path
		_, err = osfs.Open(invalidPath)
		if err == nil {
			t.Errorf("Expected Open to fail with invalid path")
		}

		// Stat with invalid path
		_, err = osfs.Stat(invalidPath)
		if err == nil {
			t.Errorf("Expected Stat to fail with invalid path")
		}
	})

	t.Run("ErrorConditions", func(t *testing.T) {
		// Setup: file 'existing_file.txt', directory 'existing_dir'
		existingFilePath := "existing_file.txt"
		existingDirPath := "existing_dir"
		nestedFilePath := filepath.Join(existingDirPath, "nested_file.txt")

		if err := osfs.WriteFile(existingFilePath, []byte("i am a file"), 0644); err != nil {
			t.Fatalf("Setup: WriteFile failed for existing_file.txt: %v", err)
		}
		if err := osfs.MkdirAll(existingDirPath, 0755); err != nil {
			t.Fatalf("Setup: MkdirAll failed for existing_dir: %v", err)
		}
		if err := osfs.WriteFile(nestedFilePath, []byte("i am nested"), 0644); err != nil {
			t.Fatalf("Setup: WriteFile failed for nested_file.txt: %v", err)
		}

		// Test WriteFile to a path where a directory already exists
		err := osfs.WriteFile(existingDirPath, []byte("overwrite dir?"), 0644)
		if err == nil {
			t.Errorf("Expected WriteFile to fail when path is an existing directory, but it succeeded")
		}
		// Note: The exact error varies by OS (e.g., "is a directory" on Linux, "Access is denied" on Windows when trying to write to a dir).
		// A generic check for non-nil error is usually sufficient for this kind of test.

		// Test MkdirAll when a file exists at the target path
		err = osfs.MkdirAll(existingFilePath, 0755)
		if err == nil {
			t.Errorf("Expected MkdirAll to fail when path is an existing file, but it succeeded")
		} else {
			// Check if error indicates file exists or similar (os.ErrExist is often wrapped)
			// For now, a non-nil error is the main check. More specific check:
			if !errors.Is(err, fs.ErrExist) && !strings.Contains(err.Error(), "file exists") && !strings.Contains(err.Error(), "not a directory") {
				t.Logf("MkdirAll on existing file produced unexpected error: %v", err)
			}
		}

		// Test Remove on a non-empty directory
		err = osfs.Remove(existingDirPath)
		if err == nil {
			t.Errorf("Expected Remove to fail on non-empty directory %s, but it succeeded", existingDirPath)
		} else {
			// This error can also be OS-dependent, e.g. "directory not empty"
			t.Logf("Remove on non-empty dir error (for info): %v", err)
		}

		// Test Remove on a non-existent file
		nonExistentPath := "non_existent_file.txt"
		err = osfs.Remove(nonExistentPath)
		if err == nil {
			t.Errorf("Expected Remove to fail on non-existent file %s, but it succeeded", nonExistentPath)
		} else if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Expected Remove on non-existent file to return fs.ErrNotExist, got %v", err)
		}
	})
}


