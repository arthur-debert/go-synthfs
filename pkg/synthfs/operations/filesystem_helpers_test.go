package operations_test

import (
	"context"
	"io/fs"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// TestFilesystemHelperMethods tests the filesystem interface detection through CreateFileOperation
func TestFilesystemHelperMethods(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateFile with filesystem using interface{} perm parameter", func(t *testing.T) {
		fs := &FilesystemWithInterfacePerm{
			files: make(map[string][]byte),
			dirs:  make(map[string]bool),
		}

		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "subdir/test.txt")
		op.SetItem(&TestFileItem{
			path:    "subdir/test.txt",
			content: []byte("content"),
			mode:    0644,
		})

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Errorf("Expected no error for interface{} filesystem, got: %v", err)
		}

		// Verify file was written
		if content, exists := fs.files["subdir/test.txt"]; !exists {
			t.Error("File was not written")
		} else if string(content) != "content" {
			t.Errorf("Expected 'content', got: %s", string(content))
		}

		// Verify directory was created
		if !fs.dirs["subdir"] {
			t.Error("Parent directory was not created")
		}
	})

	t.Run("CreateFile with filesystem using fs.FileMode perm parameter", func(t *testing.T) {
		fs := &FilesystemWithFileModePerm{
			files: make(map[string][]byte),
			dirs:  make(map[string]bool),
		}

		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "subdir/test.txt")
		op.SetItem(&TestFileItem{
			path:    "subdir/test.txt",
			content: []byte("content"),
			mode:    0644,
		})

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Errorf("Expected no error for fs.FileMode filesystem, got: %v", err)
		}

		// Verify file was written with correct mode
		if content, exists := fs.files["subdir/test.txt"]; !exists {
			t.Error("File was not written")
		} else if string(content) != "content" {
			t.Errorf("Expected 'content', got: %s", string(content))
		}

		// Verify mode conversion worked
		if fs.fileModeUsed != 0644 {
			t.Errorf("Expected file mode 0644, got: %v", fs.fileModeUsed)
		}

		// Verify directory was created with correct mode
		if !fs.dirs["subdir"] {
			t.Error("Parent directory was not created")
		}

		if fs.dirModeUsed != 0755 {
			t.Errorf("Expected directory mode 0755, got: %v", fs.dirModeUsed)
		}
	})

	t.Run("CreateFile fails with filesystem lacking WriteFile method", func(t *testing.T) {
		fs := &FilesystemWithoutWriteFile{}

		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test.txt")
		op.SetItem(&TestFileItem{
			path:    "test.txt",
			content: []byte("content"),
			mode:    0644,
		})

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for filesystem without WriteFile")
		}

		if err.Error() != "filesystem does not support WriteFile" {
			t.Errorf("Expected 'filesystem does not support WriteFile' error, got: %s", err.Error())
		}
	})

	t.Run("CreateFile fails with filesystem lacking MkdirAll method", func(t *testing.T) {
		fs := &FilesystemWithoutMkdirAll{}

		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "subdir/test.txt")
		op.SetItem(&TestFileItem{
			path:    "subdir/test.txt",
			content: []byte("content"),
			mode:    0644,
		})

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected error for filesystem without MkdirAll")
		}

		if err.Error() != "filesystem does not support MkdirAll" {
			t.Errorf("Expected 'filesystem does not support MkdirAll' error, got: %s", err.Error())
		}
	})

	t.Run("CreateFile validation fails with filesystem lacking WriteFile", func(t *testing.T) {
		fs := &FilesystemWithoutWriteFile{}

		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test.txt")
		op.SetItem(&TestFileItem{
			path:    "test.txt",
			content: []byte("content"),
			mode:    0644,
		})

		err := op.Validate(ctx, fs)

		if err == nil {
			t.Error("Expected validation error for filesystem without WriteFile")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if valErr.Reason != "filesystem does not support WriteFile" {
			t.Errorf("Expected 'filesystem does not support WriteFile' reason, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateFile in root directory (no parent dir needed)", func(t *testing.T) {
		fs := &FilesystemWithInterfacePerm{
			files: make(map[string][]byte),
			dirs:  make(map[string]bool),
		}

		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test.txt")
		op.SetItem(&TestFileItem{
			path:    "test.txt",
			content: []byte("content"),
			mode:    0644,
		})

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Errorf("Expected no error for root directory file, got: %v", err)
		}

		// Verify file was written
		if content, exists := fs.files["test.txt"]; !exists {
			t.Error("File was not written")
		} else if string(content) != "content" {
			t.Errorf("Expected 'content', got: %s", string(content))
		}

		// Should not have created any directories
		if len(fs.dirs) > 0 {
			t.Errorf("Expected no directories to be created for root file, got: %v", fs.dirs)
		}
	})
}

// Helper filesystem types for testing interface detection

// FilesystemWithInterfacePerm implements WriteFile/MkdirAll with interface{} perm
type FilesystemWithInterfacePerm struct {
	files map[string][]byte
	dirs  map[string]bool
}

func (fs *FilesystemWithInterfacePerm) WriteFile(name string, data []byte, perm interface{}) error {
	fs.files[name] = data
	return nil
}

func (fs *FilesystemWithInterfacePerm) MkdirAll(path string, perm interface{}) error {
	fs.dirs[path] = true
	return nil
}

// FilesystemWithFileModePerm implements WriteFile/MkdirAll with fs.FileMode perm
type FilesystemWithFileModePerm struct {
	files        map[string][]byte
	dirs         map[string]bool
	fileModeUsed fs.FileMode
	dirModeUsed  fs.FileMode
}

func (fs *FilesystemWithFileModePerm) WriteFile(name string, data []byte, perm fs.FileMode) error {
	fs.files[name] = data
	fs.fileModeUsed = perm
	return nil
}

func (fs *FilesystemWithFileModePerm) MkdirAll(path string, perm fs.FileMode) error {
	fs.dirs[path] = true
	fs.dirModeUsed = perm
	return nil
}

// FilesystemWithoutWriteFile has no WriteFile method
type FilesystemWithoutWriteFile struct{}

func (fs *FilesystemWithoutWriteFile) MkdirAll(path string, perm fs.FileMode) error {
	return nil
}

// FilesystemWithoutMkdirAll has no MkdirAll method
type FilesystemWithoutMkdirAll struct{}

func (fs *FilesystemWithoutMkdirAll) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return nil
}
