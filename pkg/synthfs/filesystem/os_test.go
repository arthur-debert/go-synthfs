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

// TestOSFileSystem_PathValidation tests comprehensive path validation across all methods
func TestOSFileSystem_PathValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "synthfs-path-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	osfs := filesystem.NewOSFileSystem(tempDir)

	// Test various invalid path patterns
	invalidPaths := []string{
		"../outside",
		"../../escape",
		"../../../etc/passwd",
		"",
		"./relative",
		"path/../escape",
		"path/../../escape",
	}

	for _, invalidPath := range invalidPaths {
		t.Run("Invalid path: "+invalidPath, func(t *testing.T) {
			// Test Open
			_, err := osfs.Open(invalidPath)
			if err == nil {
				t.Errorf("Expected Open to fail for invalid path %q", invalidPath)
			}

			// Check error type
			var pathError *fs.PathError
			if errors.As(err, &pathError) {
				if pathError.Op != "open" {
					t.Errorf("Expected PathError op 'open', got %q", pathError.Op)
				}
				if !errors.Is(pathError.Err, fs.ErrInvalid) {
					t.Errorf("Expected PathError to wrap fs.ErrInvalid, got %v", pathError.Err)
				}
			} else {
				t.Errorf("Expected fs.PathError, got %T: %v", err, err)
			}

			// Test Stat
			_, err = osfs.Stat(invalidPath)
			if err == nil {
				t.Errorf("Expected Stat to fail for invalid path %q", invalidPath)
			}
			if errors.As(err, &pathError) {
				if pathError.Op != "stat" {
					t.Errorf("Expected PathError op 'stat', got %q", pathError.Op)
				}
			}

			// Test WriteFile
			err = osfs.WriteFile(invalidPath, []byte("test"), 0644)
			if err == nil {
				t.Errorf("Expected WriteFile to fail for invalid path %q", invalidPath)
			}
			if errors.As(err, &pathError) {
				if pathError.Op != "writefile" {
					t.Errorf("Expected PathError op 'writefile', got %q", pathError.Op)
				}
			}

			// Test MkdirAll
			err = osfs.MkdirAll(invalidPath, 0755)
			if err == nil {
				t.Errorf("Expected MkdirAll to fail for invalid path %q", invalidPath)
			}
			if errors.As(err, &pathError) {
				if pathError.Op != "mkdirall" {
					t.Errorf("Expected PathError op 'mkdirall', got %q", pathError.Op)
				}
			}

			// Test Remove
			err = osfs.Remove(invalidPath)
			if err == nil {
				t.Errorf("Expected Remove to fail for invalid path %q", invalidPath)
			}
			if errors.As(err, &pathError) {
				if pathError.Op != "remove" {
					t.Errorf("Expected PathError op 'remove', got %q", pathError.Op)
				}
			}

			// Test RemoveAll
			err = osfs.RemoveAll(invalidPath)
			if err == nil {
				t.Errorf("Expected RemoveAll to fail for invalid path %q", invalidPath)
			}
			if errors.As(err, &pathError) {
				if pathError.Op != "removeall" {
					t.Errorf("Expected PathError op 'removeall', got %q", pathError.Op)
				}
			}

			// Test Symlink (both parameters)
			err = osfs.Symlink(invalidPath, "valid.txt")
			if err == nil {
				t.Errorf("Expected Symlink to fail for invalid oldname %q", invalidPath)
			}
			if errors.As(err, &pathError) {
				if pathError.Op != "symlink" {
					t.Errorf("Expected PathError op 'symlink', got %q", pathError.Op)
				}
			}

			err = osfs.Symlink("valid.txt", invalidPath)
			if err == nil {
				t.Errorf("Expected Symlink to fail for invalid newname %q", invalidPath)
			}

			// Test Readlink
			_, err = osfs.Readlink(invalidPath)
			if err == nil {
				t.Errorf("Expected Readlink to fail for invalid path %q", invalidPath)
			}
			if errors.As(err, &pathError) {
				if pathError.Op != "readlink" {
					t.Errorf("Expected PathError op 'readlink', got %q", pathError.Op)
				}
			}

			// Test Rename (both parameters)
			err = osfs.Rename(invalidPath, "valid.txt")
			if err == nil {
				t.Errorf("Expected Rename to fail for invalid oldpath %q", invalidPath)
			}
			if errors.As(err, &pathError) {
				if pathError.Op != "rename" {
					t.Errorf("Expected PathError op 'rename', got %q", pathError.Op)
				}
			}

			err = osfs.Rename("valid.txt", invalidPath)
			if err == nil {
				t.Errorf("Expected Rename to fail for invalid newpath %q", invalidPath)
			}
		})
	}
}

// TestOSFileSystem_SymlinkOperations tests symlink creation and reading with edge cases
func TestOSFileSystem_SymlinkOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "synthfs-symlink-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	osfs := filesystem.NewOSFileSystem(tempDir)

	t.Run("Create and read basic symlink", func(t *testing.T) {
		// Create target file
		targetPath := "target.txt"
		err := osfs.WriteFile(targetPath, []byte("target content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create target file: %v", err)
		}

		// Create symlink
		linkPath := "link.txt"
		err = osfs.Symlink(targetPath, linkPath)
		if err != nil {
			t.Fatalf("Failed to create symlink: %v", err)
		}

		// Read symlink target
		target, err := osfs.Readlink(linkPath)
		if err != nil {
			t.Fatalf("Failed to read symlink: %v", err)
		}

		if target != targetPath {
			t.Errorf("Expected symlink target %q, got %q", targetPath, target)
		}
	})

	t.Run("Symlink to absolute path within root", func(t *testing.T) {
		// Create target file
		targetPath := "absolute-target.txt"
		err := osfs.WriteFile(targetPath, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create target file: %v", err)
		}

		// Create symlink with relative target path
		linkPath := "absolute-link.txt"
		err = osfs.Symlink(targetPath, linkPath)
		if err != nil {
			t.Fatalf("Failed to create symlink: %v", err)
		}

		// Read symlink
		target, err := osfs.Readlink(linkPath)
		if err != nil {
			t.Fatalf("Failed to read symlink: %v", err)
		}

		// Should return the target path as stored
		if target != targetPath {
			t.Errorf("Expected symlink target %q, got %q", targetPath, target)
		}
	})

	t.Run("Symlink to absolute path outside root is rejected", func(t *testing.T) {
		// OSFileSystem should reject attempts to create symlinks with absolute paths
		// outside its root directory for security reasons
		err := osfs.Symlink("/etc/passwd", "dangerous-link.txt")
		if err == nil {
			t.Fatal("Expected Symlink with absolute path outside root to fail")
		}
		
		// The error should indicate the path is invalid
		if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "outside") {
			t.Errorf("Expected error about invalid/outside path, got: %v", err)
		}
	})

	t.Run("Readlink on non-existent file", func(t *testing.T) {
		_, err := osfs.Readlink("non-existent-link.txt")
		if err == nil {
			t.Error("Expected Readlink to fail on non-existent file")
		}

		// Should get a PathError with os.ErrNotExist or similar
		var pathError *fs.PathError
		if !errors.As(err, &pathError) {
			t.Errorf("Expected fs.PathError, got %T: %v", err, err)
		}
	})

	t.Run("Readlink on regular file", func(t *testing.T) {
		// Create regular file
		regularFile := "regular.txt"
		err := osfs.WriteFile(regularFile, []byte("not a symlink"), 0644)
		if err != nil {
			t.Fatalf("Failed to create regular file: %v", err)
		}

		// Try to read it as symlink
		_, err = osfs.Readlink(regularFile)
		if err == nil {
			t.Error("Expected Readlink to fail on regular file")
		}

		// Error type depends on OS, but should be some kind of "not a symlink" error
		t.Logf("Readlink on regular file error (for info): %v", err)
	})
}

// TestOSFileSystem_RenameOperations tests file/directory renaming with edge cases
func TestOSFileSystem_RenameOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "synthfs-rename-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	osfs := filesystem.NewOSFileSystem(tempDir)

	t.Run("Rename file successfully", func(t *testing.T) {
		// Create source file
		oldPath := "old-file.txt"
		content := []byte("rename test")
		err := osfs.WriteFile(oldPath, content, 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		// Rename file
		newPath := "new-file.txt"
		err = osfs.Rename(oldPath, newPath)
		if err != nil {
			t.Fatalf("Failed to rename file: %v", err)
		}

		// Verify old path doesn't exist
		_, err = osfs.Stat(oldPath)
		if err == nil {
			t.Error("Expected old path to not exist after rename")
		}

		// Verify new path exists with correct content
		info, err := osfs.Stat(newPath)
		if err != nil {
			t.Fatalf("Failed to stat renamed file: %v", err)
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("Expected renamed file size %d, got %d", len(content), info.Size())
		}
	})

	t.Run("Rename directory successfully", func(t *testing.T) {
		// Create source directory with content
		oldDir := "old-dir"
		err := osfs.MkdirAll(oldDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}

		// Add file to directory
		filePath := filepath.Join(oldDir, "file.txt")
		err = osfs.WriteFile(filePath, []byte("dir content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file in directory: %v", err)
		}

		// Rename directory
		newDir := "new-dir"
		err = osfs.Rename(oldDir, newDir)
		if err != nil {
			t.Fatalf("Failed to rename directory: %v", err)
		}

		// Verify old directory doesn't exist
		_, err = osfs.Stat(oldDir)
		if err == nil {
			t.Error("Expected old directory to not exist after rename")
		}

		// Verify new directory exists and contains file
		info, err := osfs.Stat(newDir)
		if err != nil {
			t.Fatalf("Failed to stat renamed directory: %v", err)
		}

		if !info.IsDir() {
			t.Error("Expected renamed path to be a directory")
		}

		// Check file still exists in renamed directory
		newFilePath := filepath.Join(newDir, "file.txt")
		_, err = osfs.Stat(newFilePath)
		if err != nil {
			t.Errorf("Expected file to exist in renamed directory: %v", err)
		}
	})

	t.Run("Rename non-existent source", func(t *testing.T) {
		err := osfs.Rename("non-existent.txt", "target.txt")
		if err == nil {
			t.Error("Expected Rename to fail for non-existent source")
		}

		// Should get appropriate error
		if !errors.Is(err, fs.ErrNotExist) {
			t.Logf("Rename non-existent source error (for info): %v", err)
		}
	})

	t.Run("Rename to existing destination", func(t *testing.T) {
		// Create source and destination files
		sourcePath := "rename-source.txt"
		destPath := "rename-dest.txt"

		err := osfs.WriteFile(sourcePath, []byte("source"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		err = osfs.WriteFile(destPath, []byte("dest"), 0644)
		if err != nil {
			t.Fatalf("Failed to create destination file: %v", err)
		}

		// Try to rename source to existing destination
		err = osfs.Rename(sourcePath, destPath)
		// Behavior is OS-dependent: some allow overwrite, others don't
		if err != nil {
			t.Logf("Rename to existing destination failed (may be expected): %v", err)
		} else {
			t.Logf("Rename to existing destination succeeded (OS allows overwrite)")

			// Verify destination was overwritten
			file, err := osfs.Open(destPath)
			if err != nil {
				t.Fatalf("Failed to open destination after rename: %v", err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Logf("Warning: failed to close file: %v", err)
				}
			}()

			content := make([]byte, 6)
			n, err := file.Read(content)
			if err != nil {
				t.Fatalf("Failed to read destination after rename: %v", err)
			}

			if string(content[:n]) != "source" {
				t.Errorf("Expected destination to have source content, got %q", string(content[:n]))
			}
		}
	})
}

// TestOSFileSystem_EdgeCases tests various edge cases and error conditions
func TestOSFileSystem_EdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "synthfs-edge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to remove temp dir: %v", err)
		}
	}()

	osfs := filesystem.NewOSFileSystem(tempDir)

	t.Run("Open non-existent file", func(t *testing.T) {
		_, err := osfs.Open("non-existent.txt")
		if err == nil {
			t.Error("Expected Open to fail for non-existent file")
		}

		if !errors.Is(err, fs.ErrNotExist) {
			t.Logf("Open non-existent file error (for info): %v", err)
		}
	})

	t.Run("Stat non-existent file", func(t *testing.T) {
		_, err := osfs.Stat("non-existent.txt")
		if err == nil {
			t.Error("Expected Stat to fail for non-existent file")
		}

		if !errors.Is(err, fs.ErrNotExist) {
			t.Logf("Stat non-existent file error (for info): %v", err)
		}
	})

	t.Run("WriteFile to directory path", func(t *testing.T) {
		// Create directory
		dirPath := "test-directory"
		err := osfs.MkdirAll(dirPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Try to write file with same path as directory
		err = osfs.WriteFile(dirPath, []byte("content"), 0644)
		if err == nil {
			t.Error("Expected WriteFile to fail when writing to directory path")
		}

		// Error varies by OS but should indicate path is directory
		t.Logf("WriteFile to directory error (for info): %v", err)
	})

	t.Run("RemoveAll non-existent path", func(t *testing.T) {
		// RemoveAll should succeed even if path doesn't exist (like rm -rf)
		err := osfs.RemoveAll("completely-non-existent-path")
		if err != nil {
			t.Errorf("Expected RemoveAll to succeed for non-existent path, got: %v", err)
		}
	})

	t.Run("MkdirAll existing directory", func(t *testing.T) {
		// Create directory
		dirPath := "existing-directory"
		err := osfs.MkdirAll(dirPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// MkdirAll should succeed for existing directory
		err = osfs.MkdirAll(dirPath, 0755)
		if err != nil {
			t.Errorf("Expected MkdirAll to succeed for existing directory, got: %v", err)
		}
	})

	t.Run("WriteFile with zero-length content", func(t *testing.T) {
		path := "empty-file.txt"
		err := osfs.WriteFile(path, []byte{}, 0644)
		if err != nil {
			t.Fatalf("Failed to write empty file: %v", err)
		}

		// Verify file exists and is empty
		info, err := osfs.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat empty file: %v", err)
		}

		if info.Size() != 0 {
			t.Errorf("Expected empty file size 0, got %d", info.Size())
		}
	})

	t.Run("WriteFile with nil content", func(t *testing.T) {
		path := "nil-content-file.txt"
		err := osfs.WriteFile(path, nil, 0644)
		if err != nil {
			t.Fatalf("Failed to write file with nil content: %v", err)
		}

		// Verify file exists and is empty
		info, err := osfs.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat file with nil content: %v", err)
		}

		if info.Size() != 0 {
			t.Errorf("Expected file with nil content to have size 0, got %d", info.Size())
		}
	})
}

// TestNewOSFileSystem tests the constructor
func TestNewOSFileSystem(t *testing.T) {
	t.Run("NewOSFileSystem creates filesystem", func(t *testing.T) {
		root := "/tmp"
		osfs := filesystem.NewOSFileSystem(root)

		if osfs == nil {
			t.Fatal("Expected NewOSFileSystem to return non-nil filesystem")
		}

		// We can't directly access the root field since it's private,
		// but we can test that operations work relative to the root
		// by creating a temporary directory and testing basic operations

		tempDir, err := os.MkdirTemp("", "test-new-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				t.Logf("Warning: failed to remove temp dir: %v", err)
			}
		}()

		testFs := filesystem.NewOSFileSystem(tempDir)

		// Test basic operation works
		err = testFs.WriteFile("test.txt", []byte("test"), 0644)
		if err != nil {
			t.Errorf("Expected basic operation to work with new filesystem: %v", err)
		}

		// Verify file was created in correct location
		expectedPath := filepath.Join(tempDir, "test.txt")
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("Expected file to be created at %s: %v", expectedPath, err)
		}
	})
}
