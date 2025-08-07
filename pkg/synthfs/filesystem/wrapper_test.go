package filesystem_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

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

	wrapper := filesystem.NewReadOnlyWrapper(testFS)

	t.Run("Open", func(t *testing.T) {
		file, err := wrapper.Open("file.txt")
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

func TestReadOnlyWrapper_WithNonFileSystem(t *testing.T) {
	// Create a simple fs.FS that doesn't implement fs.FileSystem
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{
			Data: []byte("test content"),
			Mode: 0644,
		},
	}

	// Wrap it to hide the FileSystem interface
	var plainFS fs.FS = testFS
	wrapper := filesystem.NewReadOnlyWrapper(plainFS)

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
