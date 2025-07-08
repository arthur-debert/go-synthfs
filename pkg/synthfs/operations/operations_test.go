package operations_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

// Type aliases for the consolidated mock filesystem
type MockFilesystem = testutil.OperationsMockFS

func NewMockFilesystem() *MockFilesystem {
	return testutil.NewOperationsMockFS()
}

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
func (i *TestFileItem) IsDir() bool       { return false }

// TestDirItem is a mock directory item for testing
type TestDirItem struct {
	path string
	mode fs.FileMode
}

func (i *TestDirItem) Path() string      { return i.path }
func (i *TestDirItem) Type() string      { return "directory" }
func (i *TestDirItem) Mode() fs.FileMode { return i.mode }
func (i *TestDirItem) IsDir() bool       { return true }

// TestSymlinkItem is a mock symlink item for testing
type TestSymlinkItem struct {
	path   string
	target string
	mode   fs.FileMode
}

func (i *TestSymlinkItem) Path() string      { return i.path }
func (i *TestSymlinkItem) Type() string      { return "symlink" }
func (i *TestSymlinkItem) Mode() fs.FileMode { return i.mode }
func (i *TestSymlinkItem) IsDir() bool       { return false }
func (i *TestSymlinkItem) Target() string    { return i.target }

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