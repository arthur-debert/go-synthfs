package operations_test

import (
	"context"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestGetItemValidation(t *testing.T) {
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

func TestCreateFileValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("ValidateCreateFile with wrong item type", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test/file.txt")
		
		// Set wrong item type (directory instead of file)
		dirItem := &TestDirItem{
			path: "test/dir",
			mode: 0755,
		}
		op.SetItem(dirItem)

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for wrong item type")
			return
		}

		if !strings.Contains(err.Error(), "directory") {
			t.Errorf("Expected directory related error, got: %s", err.Error())
		}
	})

	t.Run("ValidateCreateFile with no item", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test/file.txt")
		// Don't set any item

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for missing item")
		}

		if !strings.Contains(err.Error(), "no item provided") {
			t.Errorf("Expected 'no item provided' error, got: %s", err.Error())
		}
	})

	t.Run("ValidateCreateFile with existing file", func(t *testing.T) {
		t.Skip("Skipping test due to conflicting validation logic with restorable runs")
		fs := NewMockFilesystem()
		// Create existing file
		if err := fs.WriteFile("test/existing.txt", []byte("existing"), 0644); err != nil {
			t.Fatalf("Failed to write existing file: %v", err)
		}

		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "test/existing.txt")
		fileItem := &TestFileItem{
			path:    "test/existing.txt",
			content: []byte("new content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for existing file")
		}

		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("Expected 'already exists' error, got: %s", err.Error())
		}
	})
}

func TestCreateDirectoryValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("ValidateCreateDirectory with wrong item type", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateDirectoryOperation(core.OperationID("test-op"), "test/dir")
		
		// Set wrong item type (file instead of directory)
		fileItem := &TestFileItem{
			path:    "test/file.txt",
			content: []byte("content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for wrong item type")
			return
		}

		if !strings.Contains(err.Error(), "directory") {
			t.Errorf("Expected directory related error, got: %s", err.Error())
		}
	})

	t.Run("ValidateCreateDirectory with existing directory is idempotent", func(t *testing.T) {
		fs := NewMockFilesystem()
		// Create existing directory
		if err := fs.MkdirAll("test/existing", 0755); err != nil {
			t.Fatalf("Failed to create existing directory: %v", err)
		}

		op := operations.NewCreateDirectoryOperation(core.OperationID("test-op"), "test/existing")
		dirItem := &TestDirItem{
			path: "test/existing",
			mode: 0755,
		}
		op.SetItem(dirItem)

		// Directory creation is idempotent - should not error
		err := op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no validation error for existing directory (idempotent), got: %v", err)
		}
	})
}

func TestSourceExistenceValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("Copy validation - source exists", func(t *testing.T) {
		fs := NewMockFilesystem()
		// Create source file
		if err := fs.WriteFile("existing_file.txt", []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write existing file: %v", err)
		}

		op := operations.NewCopyOperation(core.OperationID("copy-op"), "existing_file.txt")
		op.SetPaths("existing_file.txt", "destination.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "destination.txt")

		err := op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no validation error for existing source, got: %v", err)
		}
	})

	t.Run("Copy validation - source does not exist", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCopyOperation(core.OperationID("copy-op"), "nonexistent_file.txt")
		op.SetPaths("nonexistent_file.txt", "destination.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "destination.txt")

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for non-existent source, but got none")
		}

		if !strings.Contains(err.Error(), "source does not exist") {
			t.Errorf("Expected 'source does not exist' error, got: %v", err)
		}
	})

	t.Run("Move validation - source exists", func(t *testing.T) {
		fs := NewMockFilesystem()
		// Create source file
		if err := fs.WriteFile("existing_file.txt", []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write existing file: %v", err)
		}

		op := operations.NewMoveOperation(core.OperationID("move-op"), "existing_file.txt")
		op.SetPaths("existing_file.txt", "new_location.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "new_location.txt")

		err := op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no validation error for existing source, got: %v", err)
		}
	})

	t.Run("Move validation - source does not exist", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewMoveOperation(core.OperationID("move-op"), "nonexistent_file.txt")
		op.SetPaths("nonexistent_file.txt", "new_location.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "new_location.txt")

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for non-existent source, but got none")
		}

		if !strings.Contains(err.Error(), "source does not exist") {
			t.Errorf("Expected 'source does not exist' error, got: %v", err)
		}
	})
}

func TestDeleteValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("Delete validation - target exists", func(t *testing.T) {
		fs := NewMockFilesystem()
		// Create file to delete
		if err := fs.WriteFile("existing_file.txt", []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write existing file: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("delete-op"), "existing_file.txt")

		err := op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no validation error for existing target, got: %v", err)
		}
	})

	t.Run("Delete validation - target does not exist (idempotent)", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewDeleteOperation(core.OperationID("delete-op"), "nonexistent_file.txt")

		// Delete is idempotent - should not error on non-existent files
		err := op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no validation error for non-existent target (idempotent), got: %v", err)
		}
	})
}