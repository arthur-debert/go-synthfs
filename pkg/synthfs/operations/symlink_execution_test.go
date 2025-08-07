package operations_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

var symlinkErrNotExist = fs.ErrNotExist
var symlinkModeSymlink = fs.ModeSymlink

// TestSymlinkExecuteErrorPaths tests the untested 52.4% of symlink Execute method
func TestSymlinkExecuteErrorPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("Symlink execute with no item", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Don't set item
		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for missing item")
		}

		if !strings.Contains(err.Error(), "create_symlink operation requires an item") {
			t.Errorf("Expected 'create_symlink operation requires an item' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with no target in details", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set item but no target in details
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for missing target")
		}

		if !strings.Contains(err.Error(), "create_symlink operation requires a target") {
			t.Errorf("Expected 'create_symlink operation requires a target' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with empty target in details", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set empty target
		op.SetDescriptionDetail("target", "")
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for empty target")
		}

		if !strings.Contains(err.Error(), "create_symlink operation requires a target") {
			t.Errorf("Expected 'create_symlink operation requires a target' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with non-ItemInterface item", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set target
		op.SetDescriptionDetail("target", "target.txt")

		// Set item that doesn't implement ItemInterface
		op.SetItem("not-an-item-interface")

		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for non-ItemInterface item")
		}

		if !strings.Contains(err.Error(), "item does not implement ItemInterface") {
			t.Errorf("Expected 'item does not implement ItemInterface' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with filesystem lacking Symlink method", func(t *testing.T) {
		fs := &FilesystemWithoutSymlink{
			files: make(map[string][]byte),
			dirs:  make(map[string]bool),
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for filesystem without Symlink")
		}

		if !strings.Contains(err.Error(), "filesystem does not support Symlink") {
			t.Errorf("Expected 'filesystem does not support Symlink' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with parent directory creation", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "dir/subdir/link.txt")
		op.SetDescriptionDetail("target", "../../target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "dir/subdir/link.txt",
			target: "../../target.txt",
		})

		err := op.Execute(ctx, nil, fs)

		if err != nil {
			t.Errorf("Expected no error for valid symlink with parent dirs, got: %v", err)
		}

		// Verify parent directory was created
		if !fs.dirs["dir/subdir"] {
			t.Error("Parent directory was not created")
		}

		// Verify symlink was created
		if target, ok := fs.symlinks["dir/subdir/link.txt"]; !ok {
			t.Error("Symlink was not created")
		} else if target != "../../target.txt" {
			t.Errorf("Expected symlink target '../../target.txt', got: %s", target)
		}
	})

	t.Run("Symlink execute in root directory (no parent dir needed)", func(t *testing.T) {
		fs := NewMockFilesystemWithSymlink()

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, nil, fs)

		if err != nil {
			t.Errorf("Expected no error for root directory symlink, got: %v", err)
		}

		// Verify symlink was created
		if target, ok := fs.symlinks["link.txt"]; !ok {
			t.Error("Symlink was not created")
		} else if target != "target.txt" {
			t.Errorf("Expected symlink target 'target.txt', got: %s", target)
		}

		// Should not have created any directories
		if len(fs.dirs) > 0 {
			t.Errorf("Expected no directories to be created for root symlink, got: %v", fs.dirs)
		}
	})

	t.Run("Symlink execute with filesystem lacking MkdirAll (should still work)", func(t *testing.T) {
		fs := &FilesystemWithSymlinkButNoMkdirAll{
			symlinks: make(map[string]string),
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "dir/link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "dir/link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, nil, fs)

		if err != nil {
			t.Errorf("Expected no error even without MkdirAll, got: %v", err)
		}

		// Verify symlink was created
		if target, ok := fs.symlinks["dir/link.txt"]; !ok {
			t.Error("Symlink was not created")
		} else if target != "target.txt" {
			t.Errorf("Expected symlink target 'target.txt', got: %s", target)
		}
	})

	t.Run("Symlink execute with symlink creation failure", func(t *testing.T) {
		fs := &FilesystemWithFailingSymlink{
			dirs: make(map[string]bool),
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for failing symlink creation")
		}

		if !strings.Contains(err.Error(), "failed to create symlink") {
			t.Errorf("Expected 'failed to create symlink' error, got: %s", err.Error())
		}
	})

	t.Run("Symlink execute with parent directory creation failure", func(t *testing.T) {
		fs := &FilesystemWithFailingMkdirAll{
			symlinks: make(map[string]string),
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "dir/link.txt")
		op.SetDescriptionDetail("target", "target.txt")
		op.SetItem(&TestSymlinkItem{
			path:   "dir/link.txt",
			target: "target.txt",
		})

		err := op.Execute(ctx, nil, fs)

		if err == nil {
			t.Error("Expected error for failing parent directory creation")
		}

		if !strings.Contains(err.Error(), "failed to create parent directory") {
			t.Errorf("Expected 'failed to create parent directory' error, got: %s", err.Error())
		}
	})
}

// Helper filesystem types for testing symlink edge cases

// MockFilesystemWithSymlink for testing symlink functionality
type MockFilesystemWithSymlink struct {
	files    map[string][]byte
	dirs     map[string]bool
	symlinks map[string]string
}

func NewMockFilesystemWithSymlink() *MockFilesystemWithSymlink {
	return &MockFilesystemWithSymlink{
		files:    make(map[string][]byte),
		dirs:     make(map[string]bool),
		symlinks: make(map[string]string),
	}
}

func (fs *MockFilesystemWithSymlink) Symlink(oldname, newname string) error {
	fs.symlinks[newname] = oldname
	return nil
}

func (fs *MockFilesystemWithSymlink) MkdirAll(path string, perm fs.FileMode) error {
	fs.dirs[path] = true
	return nil
}

func (fs *MockFilesystemWithSymlink) Open(name string) (fs.File, error) {
	if content, ok := fs.files[name]; ok {
		return &symlinkMockFile{Reader: bytes.NewReader(content), content: content, name: name}, nil
	}
	return nil, symlinkErrNotExist
}

func (fs *MockFilesystemWithSymlink) Stat(name string) (fs.FileInfo, error) {
	if _, ok := fs.files[name]; ok {
		return &symlinkMockFileInfo{name: name, size: int64(len(fs.files[name]))}, nil
	}
	if _, ok := fs.dirs[name]; ok {
		return &symlinkMockFileInfo{name: name, isDir: true}, nil
	}
	if _, ok := fs.symlinks[name]; ok {
		return &symlinkMockFileInfo{name: name, mode: symlinkModeSymlink}, nil
	}
	return nil, symlinkErrNotExist
}

func (fs *MockFilesystemWithSymlink) WriteFile(name string, data []byte, perm fs.FileMode) error {
	fs.files[name] = data
	return nil
}

func (fs *MockFilesystemWithSymlink) Remove(name string) error {
	delete(fs.files, name)
	delete(fs.dirs, name)
	delete(fs.symlinks, name)
	return nil
}

func (fs *MockFilesystemWithSymlink) RemoveAll(path string) error {
	delete(fs.files, path)
	delete(fs.dirs, path)
	delete(fs.symlinks, path)
	return nil
}

func (fs *MockFilesystemWithSymlink) Readlink(name string) (string, error) {
	if target, ok := fs.symlinks[name]; ok {
		return target, nil
	}
	return "", errors.New("not a symlink")
}

func (fs *MockFilesystemWithSymlink) Rename(oldpath, newpath string) error {
	if content, ok := fs.files[oldpath]; ok {
		fs.files[newpath] = content
		delete(fs.files, oldpath)
		return nil
	}
	if target, ok := fs.symlinks[oldpath]; ok {
		fs.symlinks[newpath] = target
		delete(fs.symlinks, oldpath)
		return nil
	}
	return symlinkErrNotExist
}

// FilesystemWithoutSymlink has no Symlink method
type FilesystemWithoutSymlink struct {
	files map[string][]byte
	dirs  map[string]bool
}

func (fs *FilesystemWithoutSymlink) MkdirAll(path string, perm fs.FileMode) error {
	fs.dirs[path] = true
	return nil
}

func (fs *FilesystemWithoutSymlink) Open(name string) (fs.File, error) {
	if content, ok := fs.files[name]; ok {
		return &symlinkMockFile{Reader: bytes.NewReader(content), content: content, name: name}, nil
	}
	return nil, symlinkErrNotExist
}

func (fs *FilesystemWithoutSymlink) Stat(name string) (fs.FileInfo, error) {
	if _, ok := fs.files[name]; ok {
		return &symlinkMockFileInfo{name: name, size: int64(len(fs.files[name]))}, nil
	}
	if _, ok := fs.dirs[name]; ok {
		return &symlinkMockFileInfo{name: name, isDir: true}, nil
	}
	return nil, symlinkErrNotExist
}

func (fs *FilesystemWithoutSymlink) WriteFile(name string, data []byte, perm fs.FileMode) error {
	fs.files[name] = data
	return nil
}

func (fs *FilesystemWithoutSymlink) Remove(name string) error {
	delete(fs.files, name)
	delete(fs.dirs, name)
	return nil
}

func (fs *FilesystemWithoutSymlink) RemoveAll(path string) error {
	delete(fs.files, path)
	delete(fs.dirs, path)
	return nil
}

func (fs *FilesystemWithoutSymlink) Readlink(name string) (string, error) {
	return "", errors.New("readlink not supported")
}

func (fs *FilesystemWithoutSymlink) Rename(oldpath, newpath string) error {
	if content, ok := fs.files[oldpath]; ok {
		fs.files[newpath] = content
		delete(fs.files, oldpath)
		return nil
	}
	return symlinkErrNotExist
}

func (fs *FilesystemWithoutSymlink) Symlink(oldname, newname string) error {
	return errors.New("filesystem does not support Symlink")
}

// FilesystemWithSymlinkButNoMkdirAll has Symlink but no MkdirAll
type FilesystemWithSymlinkButNoMkdirAll struct {
	symlinks map[string]string
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) Symlink(oldname, newname string) error {
	fs.symlinks[newname] = oldname
	return nil
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) Open(name string) (fs.File, error) {
	return nil, symlinkErrNotExist
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) Stat(name string) (fs.FileInfo, error) {
	if _, ok := fs.symlinks[name]; ok {
		return &symlinkMockFileInfo{name: name, mode: symlinkModeSymlink}, nil
	}
	return nil, symlinkErrNotExist
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return errors.New("writefile not supported")
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) Remove(name string) error {
	delete(fs.symlinks, name)
	return nil
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) RemoveAll(path string) error {
	delete(fs.symlinks, path)
	return nil
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) Readlink(name string) (string, error) {
	if target, ok := fs.symlinks[name]; ok {
		return target, nil
	}
	return "", errors.New("not a symlink")
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) Rename(oldpath, newpath string) error {
	if target, ok := fs.symlinks[oldpath]; ok {
		fs.symlinks[newpath] = target
		delete(fs.symlinks, oldpath)
		return nil
	}
	return symlinkErrNotExist
}

func (fs *FilesystemWithSymlinkButNoMkdirAll) MkdirAll(path string, perm fs.FileMode) error {
	return errors.New("filesystem does not support MkdirAll")
}

// FilesystemWithFailingSymlink always fails Symlink calls
type FilesystemWithFailingSymlink struct {
	dirs map[string]bool
}

func (fs *FilesystemWithFailingSymlink) Symlink(oldname, newname string) error {
	return fmt.Errorf("symlink creation failed")
}

func (fs *FilesystemWithFailingSymlink) MkdirAll(path string, perm fs.FileMode) error {
	fs.dirs[path] = true
	return nil
}

func (fs *FilesystemWithFailingSymlink) Open(name string) (fs.File, error) {
	return nil, symlinkErrNotExist
}

func (fs *FilesystemWithFailingSymlink) Stat(name string) (fs.FileInfo, error) {
	if _, ok := fs.dirs[name]; ok {
		return &symlinkMockFileInfo{name: name, isDir: true}, nil
	}
	return nil, symlinkErrNotExist
}

func (fs *FilesystemWithFailingSymlink) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return errors.New("writefile not supported")
}

func (fs *FilesystemWithFailingSymlink) Remove(name string) error {
	delete(fs.dirs, name)
	return nil
}

func (fs *FilesystemWithFailingSymlink) RemoveAll(path string) error {
	delete(fs.dirs, path)
	return nil
}

func (fs *FilesystemWithFailingSymlink) Readlink(name string) (string, error) {
	return "", errors.New("readlink not supported")
}

func (fs *FilesystemWithFailingSymlink) Rename(oldpath, newpath string) error {
	return symlinkErrNotExist
}

// FilesystemWithFailingMkdirAll always fails MkdirAll calls
type FilesystemWithFailingMkdirAll struct {
	symlinks map[string]string
}

func (fs *FilesystemWithFailingMkdirAll) Symlink(oldname, newname string) error {
	fs.symlinks[newname] = oldname
	return nil
}

func (fs *FilesystemWithFailingMkdirAll) MkdirAll(path string, perm fs.FileMode) error {
	return fmt.Errorf("mkdir failed")
}

func (fs *FilesystemWithFailingMkdirAll) Open(name string) (fs.File, error) {
	return nil, symlinkErrNotExist
}

func (fs *FilesystemWithFailingMkdirAll) Stat(name string) (fs.FileInfo, error) {
	if _, ok := fs.symlinks[name]; ok {
		return &symlinkMockFileInfo{name: name, mode: symlinkModeSymlink}, nil
	}
	return nil, symlinkErrNotExist
}

func (fs *FilesystemWithFailingMkdirAll) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return errors.New("writefile not supported")
}

func (fs *FilesystemWithFailingMkdirAll) Remove(name string) error {
	delete(fs.symlinks, name)
	return nil
}

func (fs *FilesystemWithFailingMkdirAll) RemoveAll(path string) error {
	delete(fs.symlinks, path)
	return nil
}

func (fs *FilesystemWithFailingMkdirAll) Readlink(name string) (string, error) {
	if target, ok := fs.symlinks[name]; ok {
		return target, nil
	}
	return "", errors.New("not a symlink")
}

func (fs *FilesystemWithFailingMkdirAll) Rename(oldpath, newpath string) error {
	if target, ok := fs.symlinks[oldpath]; ok {
		fs.symlinks[newpath] = target
		delete(fs.symlinks, oldpath)
		return nil
	}
	return symlinkErrNotExist
}

// Helper structs for symlink test mock filesystem implementations

type symlinkMockFile struct {
	*bytes.Reader
	content []byte
	name    string
}

func (m *symlinkMockFile) Close() error                { return nil }
func (m *symlinkMockFile) Stat() (fs.FileInfo, error) { 
	return &symlinkMockFileInfo{name: m.name, size: int64(len(m.content))}, nil 
}

type symlinkMockFileInfo struct {
	name  string
	size  int64
	isDir bool
	mode  fs.FileMode
}

func (m *symlinkMockFileInfo) Name() string       { return m.name }
func (m *symlinkMockFileInfo) Size() int64        { return m.size }
func (m *symlinkMockFileInfo) Mode() fs.FileMode  { 
	if m.mode != 0 {
		return m.mode
	}
	if m.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (m *symlinkMockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *symlinkMockFileInfo) IsDir() bool        { return m.isDir }
func (m *symlinkMockFileInfo) Sys() interface{}   { return nil }
