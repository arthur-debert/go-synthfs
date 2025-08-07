package operations_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

var helpersErrNotExist = fs.ErrNotExist

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

		err := op.Execute(ctx, nil, fs)
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

		err := op.Execute(ctx, nil, fs)
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

		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for filesystem without WriteFile")
		}

		if !strings.Contains(err.Error(), "filesystem does not support WriteFile") {
			t.Errorf("Expected error containing 'filesystem does not support WriteFile', got: %s", err.Error())
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

		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for filesystem without MkdirAll")
		}

		if !strings.Contains(err.Error(), "filesystem does not support MkdirAll") {
			t.Errorf("Expected error containing 'filesystem does not support MkdirAll', got: %s", err.Error())
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

		err := op.Validate(ctx, nil, fs)

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

		err := op.Execute(ctx, nil, fs)
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

// FilesystemWithInterfacePerm implements WriteFile/MkdirAll with fs.FileMode perm (updated from interface{})
type FilesystemWithInterfacePerm struct {
	files map[string][]byte
	dirs  map[string]bool
}

func (fs *FilesystemWithInterfacePerm) WriteFile(name string, data []byte, perm fs.FileMode) error {
	fs.files[name] = data
	return nil
}

func (fs *FilesystemWithInterfacePerm) MkdirAll(path string, perm fs.FileMode) error {
	fs.dirs[path] = true
	return nil
}

func (fs *FilesystemWithInterfacePerm) Open(name string) (fs.File, error) {
	if content, ok := fs.files[name]; ok {
		return &mockFile{Reader: bytes.NewReader(content), content: content, name: name}, nil
	}
	return nil, helpersErrNotExist
}

func (fs *FilesystemWithInterfacePerm) Stat(name string) (fs.FileInfo, error) {
	if _, ok := fs.files[name]; ok {
		return &mockFileInfo{name: name, size: int64(len(fs.files[name]))}, nil
	}
	if _, ok := fs.dirs[name]; ok {
		return &mockFileInfo{name: name, isDir: true}, nil
	}
	return nil, helpersErrNotExist
}

func (fs *FilesystemWithInterfacePerm) Remove(name string) error {
	delete(fs.files, name)
	delete(fs.dirs, name)
	return nil
}

func (fs *FilesystemWithInterfacePerm) RemoveAll(path string) error {
	delete(fs.files, path)
	delete(fs.dirs, path)
	return nil
}

func (fs *FilesystemWithInterfacePerm) Symlink(oldname, newname string) error {
	return errors.New("symlink not supported")
}

func (fs *FilesystemWithInterfacePerm) Readlink(name string) (string, error) {
	return "", errors.New("readlink not supported")
}

func (fs *FilesystemWithInterfacePerm) Rename(oldpath, newpath string) error {
	if content, ok := fs.files[oldpath]; ok {
		fs.files[newpath] = content
		delete(fs.files, oldpath)
		return nil
	}
	return helpersErrNotExist
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

func (fs *FilesystemWithFileModePerm) Open(name string) (fs.File, error) {
	if content, ok := fs.files[name]; ok {
		return &mockFile{Reader: bytes.NewReader(content), content: content, name: name}, nil
	}
	return nil, helpersErrNotExist
}

func (fs *FilesystemWithFileModePerm) Stat(name string) (fs.FileInfo, error) {
	if _, ok := fs.files[name]; ok {
		return &mockFileInfo{name: name, size: int64(len(fs.files[name]))}, nil
	}
	if _, ok := fs.dirs[name]; ok {
		return &mockFileInfo{name: name, isDir: true}, nil
	}
	return nil, helpersErrNotExist
}

func (fs *FilesystemWithFileModePerm) Remove(name string) error {
	delete(fs.files, name)
	delete(fs.dirs, name)
	return nil
}

func (fs *FilesystemWithFileModePerm) RemoveAll(path string) error {
	delete(fs.files, path)
	delete(fs.dirs, path)
	return nil
}

func (fs *FilesystemWithFileModePerm) Symlink(oldname, newname string) error {
	return errors.New("symlink not supported")
}

func (fs *FilesystemWithFileModePerm) Readlink(name string) (string, error) {
	return "", errors.New("readlink not supported")
}

func (fs *FilesystemWithFileModePerm) Rename(oldpath, newpath string) error {
	if content, ok := fs.files[oldpath]; ok {
		fs.files[newpath] = content
		delete(fs.files, oldpath)
		return nil
	}
	return helpersErrNotExist
}

// FilesystemWithoutWriteFile has no WriteFile method
type FilesystemWithoutWriteFile struct{}

func (fs *FilesystemWithoutWriteFile) MkdirAll(path string, perm fs.FileMode) error {
	return nil
}

func (fs *FilesystemWithoutWriteFile) Open(name string) (fs.File, error) {
	return nil, helpersErrNotExist
}

func (fs *FilesystemWithoutWriteFile) Stat(name string) (fs.FileInfo, error) {
	return nil, helpersErrNotExist
}

func (fs *FilesystemWithoutWriteFile) Remove(name string) error {
	return nil
}

func (fs *FilesystemWithoutWriteFile) RemoveAll(path string) error {
	return nil
}

func (fs *FilesystemWithoutWriteFile) Symlink(oldname, newname string) error {
	return errors.New("symlink not supported")
}

func (fs *FilesystemWithoutWriteFile) Readlink(name string) (string, error) {
	return "", errors.New("readlink not supported")
}

func (fs *FilesystemWithoutWriteFile) Rename(oldpath, newpath string) error {
	return helpersErrNotExist
}

func (fs *FilesystemWithoutWriteFile) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return errors.New("filesystem does not support WriteFile")
}

// FilesystemWithoutMkdirAll has no MkdirAll method
type FilesystemWithoutMkdirAll struct{}

func (fs *FilesystemWithoutMkdirAll) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return nil
}

func (fs *FilesystemWithoutMkdirAll) Open(name string) (fs.File, error) {
	return nil, helpersErrNotExist
}

func (fs *FilesystemWithoutMkdirAll) Stat(name string) (fs.FileInfo, error) {
	return nil, helpersErrNotExist
}

func (fs *FilesystemWithoutMkdirAll) Remove(name string) error {
	return nil
}

func (fs *FilesystemWithoutMkdirAll) RemoveAll(path string) error {
	return nil
}

func (fs *FilesystemWithoutMkdirAll) Symlink(oldname, newname string) error {
	return errors.New("symlink not supported")
}

func (fs *FilesystemWithoutMkdirAll) Readlink(name string) (string, error) {
	return "", errors.New("readlink not supported")
}

func (fs *FilesystemWithoutMkdirAll) Rename(oldpath, newpath string) error {
	return helpersErrNotExist
}

func (fs *FilesystemWithoutMkdirAll) MkdirAll(path string, perm fs.FileMode) error {
	return errors.New("filesystem does not support MkdirAll")
}

// Helper structs for mock filesystem implementations

type mockFile struct {
	*bytes.Reader
	content []byte
	name    string
}

func (m *mockFile) Close() error                { return nil }
func (m *mockFile) Stat() (fs.FileInfo, error) { 
	return &mockFileInfo{name: m.name, size: int64(len(m.content))}, nil 
}

type mockFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode  { 
	if m.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }
