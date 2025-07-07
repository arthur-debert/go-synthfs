package operations_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// MockFilesystem implements a minimal filesystem for testing
type MockFilesystem struct {
	files map[string][]byte
	dirs  map[string]bool
}

func NewMockFilesystem() *MockFilesystem {
	return &MockFilesystem{
		files: make(map[string][]byte),
		dirs:  make(map[string]bool),
	}
}

func (m *MockFilesystem) WriteFile(name string, data []byte, perm interface{}) error {
	m.files[name] = data
	return nil
}

func (m *MockFilesystem) MkdirAll(path string, perm interface{}) error {
	m.dirs[path] = true
	return nil
}

func (m *MockFilesystem) Remove(name string) error {
	delete(m.files, name)
	delete(m.dirs, name)
	return nil
}

func (m *MockFilesystem) Stat(name string) (interface{}, error) {
	if _, ok := m.files[name]; ok {
		return &mockFileInfo{name: name, size: int64(len(m.files[name]))}, nil
	}
	if _, ok := m.dirs[name]; ok {
		return &mockFileInfo{name: name, isDir: true}, nil
	}
	return nil, fs.ErrNotExist
}

func (m *MockFilesystem) Open(name string) (interface{}, error) {
	if content, ok := m.files[name]; ok {
		return &mockFile{Reader: bytes.NewReader(content)}, nil
	}
	return nil, fs.ErrNotExist
}

type mockFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode  { return 0644 }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

type mockFile struct {
	io.Reader
}

func (m *mockFile) Close() error { return nil }

// TestFileItem is a mock item for testing
type TestFileItem struct {
	path    string
	content []byte
	mode    fs.FileMode
}

func (i *TestFileItem) Path() string      { return i.path }
func (i *TestFileItem) Type() string      { return "file" }
func (i *TestFileItem) Content() []byte   { return i.content }
func (i *TestFileItem) Mode() fs.FileMode { return i.mode }

// TestDirItem is a mock directory item for testing
type TestDirItem struct {
	path string
	mode fs.FileMode
}

func (i *TestDirItem) Path() string      { return i.path }
func (i *TestDirItem) Type() string      { return "directory" }
func (i *TestDirItem) Mode() fs.FileMode { return i.mode }

func TestOperations_GetItem(t *testing.T) {
	t.Run("GetItem returns nil when no item set", func(t *testing.T) {
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test/path")

		item := op.GetItem()
		if item != nil {
			t.Errorf("Expected nil item, got %v", item)
		}
	})

	t.Run("GetItem returns set item", func(t *testing.T) {
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test/file.txt")
		fileItem := &TestFileItem{
			path:    "test/file.txt",
			content: []byte("test content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		item := op.GetItem()
		if item != fileItem {
			t.Errorf("Expected fileItem, got %v", item)
		}
	})
}

func TestValidationError_Error(t *testing.T) {
	op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test/path")

	t.Run("Error without cause", func(t *testing.T) {
		err := &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "test reason",
			Cause:         nil,
		}

		expected := "validation error for operation test-op (test/path): test reason"
		if err.Error() != expected {
			t.Errorf("Expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("Error with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "test reason",
			Cause:         cause,
		}

		expected := "validation error for operation test-op (test/path): test reason: underlying error"
		if err.Error() != expected {
			t.Errorf("Expected %q, got %q", expected, err.Error())
		}
	})
}