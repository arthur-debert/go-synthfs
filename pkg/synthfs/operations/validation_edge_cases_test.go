package operations_test

import (
	"context"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// TestCreateFileValidationEdgeCases targets the missing 21.4% coverage in create.go Validate
func TestCreateFileValidationEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateFile validation with item type mismatch - directory type", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test.txt")

		// Set directory item when expecting file
		dirItem := &TestItemWithType{
			path:     "test.txt",
			itemType: "directory", // Wrong type
			content:  []byte("content"),
			mode:     0644,
		}
		op.SetItem(dirItem)

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for directory item on file operation")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "expected file item but got directory") {
			t.Errorf("Expected 'expected file item but got directory' error, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateFile validation with item IsDir() returning true", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test.txt")

		// Set item that claims to be a directory via IsDir()
		dirItem := &TestItemWithIsDir{
			path:    "test.txt",
			content: []byte("content"),
			mode:    0644,
			isDir:   true, // This will trigger the IsDir() check
		}
		op.SetItem(dirItem)

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for item with IsDir() returning true")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "item IsDir() returns true") {
			t.Errorf("Expected 'item IsDir() returns true' error, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateFile validation passes with valid file item", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test.txt")

		// Set proper file item
		fileItem := &TestFileItem{
			path:    "test.txt",
			content: []byte("content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		err := op.Validate(ctx, nil, fs)

		if err != nil {
			t.Errorf("Expected no validation error for valid file item, got: %v", err)
		}
	})
}

// TestCreateDirectoryValidationEdgeCases targets the missing coverage in directory.go Validate
func TestCreateDirectoryValidationEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateDirectory validation with wrong item type", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateDirectoryOperation(core.OperationID("test-op"), "testdir")

		// Set file item when expecting directory
		fileItem := &TestItemWithType{
			path:     "testdir",
			itemType: "file", // Wrong type
			content:  []byte("content"),
			mode:     0755,
		}
		op.SetItem(fileItem)

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for file item on directory operation")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "expected directory item but got file") {
			t.Errorf("Expected 'expected directory item but got file' error, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateDirectory validation with item IsDir() returning false", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateDirectoryOperation(core.OperationID("test-op"), "testdir")

		// Set item that claims not to be a directory via IsDir()
		fileItem := &TestItemWithIsDir{
			path:    "testdir",
			content: []byte{},
			mode:    0755,
			isDir:   false, // This will trigger the IsDir() check
		}
		op.SetItem(fileItem)

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for item with IsDir() returning false")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "expected directory item but got") {
			t.Errorf("Expected 'expected directory item but got' error, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateDirectory validation with path existing as file", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create existing file at the path
		if err := fs.WriteFile("testdir", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}

		op := operations.NewCreateDirectoryOperation(core.OperationID("test-op"), "testdir")

		// Set proper directory item
		dirItem := &TestDirItem{
			path: "testdir",
			mode: 0755,
		}
		op.SetItem(dirItem)

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for path existing as file")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "path exists but is not a directory") {
			t.Errorf("Expected 'path exists but is not a directory' error, got: %s", valErr.Reason)
		}
	})
}

// TestSymlinkValidationEdgeCases targets the missing 25% coverage in symlink.go Validate
func TestSymlinkValidationEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateSymlink validation with empty target", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set symlink item but with empty target in description
		symlinkItem := &TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
			mode:   0755,
		}
		op.SetItem(symlinkItem)
		op.SetDescriptionDetail("target", "") // Empty target

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for empty target")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "symlink target cannot be empty") {
			t.Errorf("Expected 'symlink target cannot be empty' error, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateSymlink validation with symlink already exists", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create existing file at the symlink path
		if err := fs.WriteFile("link.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}

		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set symlink item with valid target
		symlinkItem := &TestSymlinkItem{
			path:   "link.txt",
			target: "target.txt",
			mode:   0755,
		}
		op.SetItem(symlinkItem)
		op.SetDescriptionDetail("target", "target.txt")

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for existing symlink")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "symlink already exists") {
			t.Errorf("Expected 'symlink already exists' error, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateSymlink validation passes with dangling symlink target", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "link.txt")

		// Set symlink item pointing to non-existent target (should be allowed)
		symlinkItem := &TestSymlinkItem{
			path:   "link.txt",
			target: "nonexistent-target.txt",
			mode:   0755,
		}
		op.SetItem(symlinkItem)
		op.SetDescriptionDetail("target", "nonexistent-target.txt")

		err := op.Validate(ctx, nil, fs)

		if err != nil {
			t.Errorf("Expected no validation error for dangling symlink (should be allowed), got: %v", err)
		}
	})
}

// TestCopyMoveValidationEdgeCases targets the missing coverage in copy_move.go Validate
func TestCopyMoveValidationEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("Copy validation with empty source path", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCopyOperation(core.OperationID("test-op"), "")
		op.SetPaths("", "dst.txt") // Empty source

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for empty source path")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "path cannot be empty") {
			t.Errorf("Expected 'path cannot be empty' error, got: %s", valErr.Reason)
		}
	})

	t.Run("Copy validation with empty destination path", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file
		if err := fs.WriteFile("src.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewCopyOperation(core.OperationID("test-op"), "src.txt")
		op.SetPaths("src.txt", "") // Empty destination

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for empty destination path")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "destination path cannot be empty") {
			t.Errorf("Expected 'destination path cannot be empty' error, got: %s", valErr.Reason)
		}
	})

	t.Run("Move validation inherits copy validation", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewMoveOperation(core.OperationID("test-op"), "")
		op.SetPaths("", "dst.txt") // Empty source - should trigger copy validation

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for empty source path in move")
		}

		// Move validation uses copy validation, so same error expected
		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "path cannot be empty") {
			t.Errorf("Expected 'path cannot be empty' error, got: %s", valErr.Reason)
		}
	})
}

// Helper types for testing validation edge cases

// TestItemWithType implements Type() method for testing type validation
type TestItemWithType struct {
	path     string
	itemType string
	content  []byte
	mode     int
}

func (t *TestItemWithType) Path() string    { return t.path }
func (t *TestItemWithType) Type() string    { return t.itemType }
func (t *TestItemWithType) Content() []byte { return t.content }
func (t *TestItemWithType) Mode() int       { return t.mode }

// TestItemWithIsDir implements IsDir() method for testing directory validation
type TestItemWithIsDir struct {
	path    string
	content []byte
	mode    int
	isDir   bool
}

func (t *TestItemWithIsDir) Path() string    { return t.path }
func (t *TestItemWithIsDir) Type() string    { return "test" }
func (t *TestItemWithIsDir) Content() []byte { return t.content }
func (t *TestItemWithIsDir) Mode() int       { return t.mode }
func (t *TestItemWithIsDir) IsDir() bool     { return t.isDir }
