package operations_test

import (
	"context"
	"io/fs"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestDeleteOperation_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("ExecuteDelete removes directory", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create a directory
		err := fs.MkdirAll("test/dir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-op"), "test/dir")
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify directory is removed
		if _, err := fs.Stat("test/dir"); err == nil {
			t.Error("Directory should be removed")
		}
	})

	t.Run("ExecuteDelete removes file", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create a file
		err := fs.WriteFile("test/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-op"), "test/file.txt")
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify file is removed
		if _, err := fs.Stat("test/file.txt"); err == nil {
			t.Error("File should be removed")
		}
	})

	t.Run("ExecuteDelete on non-existent file is idempotent", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewDeleteOperation(core.OperationID("test-op"), "nonexistent.txt")
		err := op.Execute(ctx, fs)
		// Should not error - delete is idempotent
		if err != nil {
			t.Errorf("Delete should be idempotent, got error: %v", err)
		}
	})

	t.Run("ExecuteDelete removes non-empty directory", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create a directory with content
		err := fs.MkdirAll("test/dir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = fs.WriteFile("test/dir/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file in directory: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-op"), "test/dir")
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify directory is removed
		if _, err := fs.Stat("test/dir"); err == nil {
			t.Error("Directory should be removed")
		}
		// Verify file is also removed
		if _, err := fs.Stat("test/dir/file.txt"); err == nil {
			t.Error("File in directory should be removed")
		}
	})
}

func TestCreateOperations_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateFile with parent directory creation", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "nested/deep/file.txt")
		fileItem := &TestFileItem{
			path:    "nested/deep/file.txt",
			content: []byte("content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify parent directories were created
		if _, ok := fs.Dirs()["nested"]; !ok {
			t.Error("Parent directory 'nested' should be created")
		}
		if _, ok := fs.Dirs()["nested/deep"]; !ok {
			t.Error("Parent directory 'nested/deep' should be created")
		}

		// Verify file was created
		if _, ok := fs.Files()["nested/deep/file.txt"]; !ok {
			t.Error("File should be created")
		}
	})

	t.Run("CreateDirectory with nested path", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateDirectoryOperation(core.OperationID("test-op"), "a/b/c/d")
		dirItem := &TestDirItem{
			path: "a/b/c/d",
			mode: 0755,
		}
		op.SetItem(dirItem)

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify all directories were created
		expectedDirs := []string{"a", "a/b", "a/b/c", "a/b/c/d"}
		for _, dir := range expectedDirs {
			if _, ok := fs.Dirs()[dir]; !ok {
				t.Errorf("Directory '%s' should be created", dir)
			}
		}
	})

	t.Run("CreateSymlink validation with empty target", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "test/link")
		// Don't set target in description - should fail validation

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for missing target")
		}
	})

	t.Run("CreateArchive validation with no sources", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test/archive.tar.gz")
		// Don't set sources in description - should fail validation

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for missing sources")
		}
	})
}

func TestCopyMoveOperations_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("Copy file to directory", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file
		if err := fs.WriteFile("source.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write source.txt: %v", err)
		}

		// Create target directory
		if err := fs.MkdirAll("targetdir", 0755); err != nil {
			t.Fatalf("Failed to create targetdir: %v", err)
		}

		op := operations.NewCopyOperation(core.OperationID("test-op"), "source.txt")
		op.SetPaths("source.txt", "targetdir/source.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "targetdir/source.txt")

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file was copied
		if _, ok := fs.Files()["targetdir/source.txt"]; !ok {
			t.Error("File should be copied to target directory")
		}

		// Verify source still exists
		if _, ok := fs.Files()["source.txt"]; !ok {
			t.Error("Source file should still exist after copy")
		}
	})

	t.Run("Move file across directories", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file in one directory
		if err := fs.MkdirAll("dir1", 0755); err != nil {
			t.Fatalf("Failed to create dir1: %v", err)
		}
		if err := fs.WriteFile("dir1/file.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		// Create target directory
		if err := fs.MkdirAll("dir2", 0755); err != nil {
			t.Fatalf("Failed to create dir2: %v", err)
		}

		op := operations.NewMoveOperation(core.OperationID("test-op"), "dir1/file.txt")
		op.SetPaths("dir1/file.txt", "dir2/file.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "dir2/file.txt")

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file was moved
		if _, ok := fs.Files()["dir2/file.txt"]; !ok {
			t.Error("File should be moved to target directory")
		}

		// Verify source no longer exists
		if _, ok := fs.Files()["dir1/file.txt"]; ok {
			t.Error("Source file should not exist after move")
		}
	})

	t.Run("Copy directory validation", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source directory
		if err := fs.MkdirAll("sourcedir", 0755); err != nil {
			t.Fatalf("Failed to create sourcedir: %v", err)
		}

		op := operations.NewCopyOperation(core.OperationID("test-op"), "sourcedir")
		op.SetPaths("sourcedir", "destdir")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "destdir")

		// Copy of directories is not yet implemented
		err := op.Execute(ctx, fs)
		if err == nil {
			t.Error("Expected error for directory copy (not implemented)")
		}
	})
}

// BasicFS is a minimal filesystem implementation for testing edge cases
type BasicFS struct{}

func (bfs *BasicFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (bfs *BasicFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return fs.ErrNotExist
}

func (bfs *BasicFS) MkdirAll(path string, perm fs.FileMode) error {
	return fs.ErrNotExist
}

func (bfs *BasicFS) Remove(name string) error {
	return fs.ErrNotExist
}

func (bfs *BasicFS) RemoveAll(path string) error {
	return fs.ErrNotExist
}

func (bfs *BasicFS) Symlink(oldname, newname string) error {
	return fs.ErrNotExist
}

func (bfs *BasicFS) Readlink(name string) (string, error) {
	return "", fs.ErrNotExist
}

func (bfs *BasicFS) Rename(oldpath, newpath string) error {
	return fs.ErrNotExist
}
