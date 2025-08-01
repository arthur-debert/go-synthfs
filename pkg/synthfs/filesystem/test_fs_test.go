package filesystem_test

import (
	"io/fs"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestTestFileSystemExtended(t *testing.T) {
	testFS := filesystem.NewTestFileSystem()

	t.Run("Symlink and Readlink", func(t *testing.T) {
		// Create a target file
		targetPath := "target.txt"
		targetContent := []byte("target content")
		err := testFS.WriteFile(targetPath, targetContent, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Create a symlink
		linkPath := "link.txt"
		err = testFS.Symlink(targetPath, linkPath)
		if err != nil {
			t.Fatalf("Symlink failed: %v", err)
		}

		// Read the symlink target
		target, err := testFS.Readlink(linkPath)
		if err != nil {
			t.Fatalf("Readlink failed: %v", err)
		}

		if target != targetPath {
			t.Errorf("Expected symlink target %q, got %q", targetPath, target)
		}

		// Verify symlink shows up in Stat
		info, err := testFS.Stat(linkPath)
		if err != nil {
			t.Fatalf("Stat symlink failed: %v", err)
		}

		if info.Mode()&fs.ModeSymlink == 0 {
			t.Error("Expected symlink to have ModeSymlink set")
		}
	})

	t.Run("Rename", func(t *testing.T) {
		// Create a file to rename
		oldPath := "old-name.txt"
		content := []byte("rename test content")
		err := testFS.WriteFile(oldPath, content, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Rename the file
		newPath := "new-name.txt"
		err = testFS.Rename(oldPath, newPath)
		if err != nil {
			t.Fatalf("Rename failed: %v", err)
		}

		// Old path should not exist
		_, err = testFS.Stat(oldPath)
		if err == nil {
			t.Error("Expected old path to not exist after rename")
		}

		// New path should exist with correct content
		info, err := testFS.Stat(newPath)
		if err != nil {
			t.Fatalf("Stat renamed file failed: %v", err)
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("Expected renamed file size %d, got %d", len(content), info.Size())
		}

		// Verify content is correct
		file, exists := testFS.MapFS[newPath]
		if !exists {
			t.Fatal("Renamed file not found in MapFS")
		}

		if string(file.Data) != string(content) {
			t.Errorf("Content mismatch after rename. Expected %q, got %q", string(content), string(file.Data))
		}
	})

	t.Run("Error cases", func(t *testing.T) {
		// Symlink with non-existent target (dangling symlinks are allowed)
		err := testFS.Symlink("non-existent.txt", "broken-link.txt")
		if err != nil {
			t.Errorf("Symlink to non-existent target should be allowed (dangling symlink): %v", err)
		}

		// Readlink on non-symlink
		normalFile := "normal.txt"
		err = testFS.WriteFile(normalFile, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		_, err = testFS.Readlink(normalFile)
		if err == nil {
			t.Error("Expected Readlink on normal file to fail")
		}

		// Rename non-existent file
		err = testFS.Rename("does-not-exist.txt", "target.txt")
		if err == nil {
			t.Error("Expected Rename of non-existent file to fail")
		}
	})
}
