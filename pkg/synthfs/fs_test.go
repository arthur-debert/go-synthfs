package synthfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestOSFileSystem(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "synthfs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	osfs := synthfs.NewOSFileSystem(tempDir)

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
		defer file.Close()

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
}

func TestReadOnlyWrapper(t *testing.T) {
	// Create a test filesystem
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{
			Data: []byte("test content"),
			Mode: 0644,
		},
		"dir": &fstest.MapFile{
			Mode: fs.ModeDir | 0755,
		},
		"dir/nested.txt": &fstest.MapFile{
			Data: []byte("nested content"),
			Mode: 0644,
		},
	}

	wrapper := synthfs.NewReadOnlyWrapper(testFS)

	t.Run("Open", func(t *testing.T) {
		file, err := wrapper.Open("file.txt")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.IsDir() {
			t.Errorf("Expected file, got directory")
		}
	})

	t.Run("Stat", func(t *testing.T) {
		info, err := wrapper.Stat("file.txt")
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.IsDir() {
			t.Errorf("Expected file, got directory")
		}

		if info.Size() != 12 { // len("test content")
			t.Errorf("Expected size 12, got %d", info.Size())
		}
	})

	t.Run("Stat directory", func(t *testing.T) {
		info, err := wrapper.Stat("dir")
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if !info.IsDir() {
			t.Errorf("Expected directory, got file")
		}
	})

	t.Run("Stat non-existent", func(t *testing.T) {
		_, err := wrapper.Stat("nonexistent.txt")
		if err == nil {
			t.Errorf("Expected Stat to fail for non-existent file")
		}
	})
}

func TestReadOnlyWrapper_WithNonStatFS(t *testing.T) {
	// Create a simple fs.FS that doesn't implement fs.StatFS
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{
			Data: []byte("test content"),
			Mode: 0644,
		},
	}

	// Wrap it to hide the StatFS interface
	var plainFS fs.FS = testFS
	wrapper := synthfs.NewReadOnlyWrapper(plainFS)

	t.Run("Stat fallback to Open", func(t *testing.T) {
		info, err := wrapper.Stat("file.txt")
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.IsDir() {
			t.Errorf("Expected file, got directory")
		}

		if info.Size() != 12 { // len("test content")
			t.Errorf("Expected size 12, got %d", info.Size())
		}
	})
}
