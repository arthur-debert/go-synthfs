package operations_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestCreateFileOperation(t *testing.T) {
	ctx := context.Background()

	t.Run("create file with content", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateFileOperation(core.OperationID("test-create"), "test.txt")
		fileItem := &TestFileItem{
			path:    "test.txt",
			content: []byte("test content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file was created
		if content, ok := fs.Files()["test.txt"]; !ok {
			t.Error("Expected file to be created")
		} else if string(content) != "test content" {
			t.Errorf("Expected content 'test content', got '%s'", content)
		}
	})

	t.Run("create file requires item", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateFileOperation(core.OperationID("test-create"), "test.txt")
		// Don't set item

		err := op.Execute(ctx, fs)
		if err == nil {
			t.Error("Expected error when no item is set")
		}
	})

	t.Run("create file in subdirectory", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateFileOperation(core.OperationID("test-create"), "subdir/test.txt")
		fileItem := &TestFileItem{
			path:    "subdir/test.txt",
			content: []byte("test content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify directory was created
		if _, ok := fs.Dirs()["subdir"]; !ok {
			t.Error("Expected parent directory to be created")
		}

		// Verify file was created
		if _, ok := fs.Files()["subdir/test.txt"]; !ok {
			t.Error("Expected file to be created")
		}
	})

	t.Run("validate file creation", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateFileOperation(core.OperationID("test-create"), "test.txt")
		fileItem := &TestFileItem{
			path:    "test.txt",
			content: []byte("test content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		err := op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Validate failed: %v", err)
		}
	})

	t.Run("reverse ops for create file", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateFileOperation(core.OperationID("test-create"), "test.txt")
		fileItem := &TestFileItem{
			path:    "test.txt",
			content: []byte("test content"),
			mode:    0644,
		}
		op.SetItem(fileItem)

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, nil)
		if err != nil {
			t.Fatalf("ReverseOps failed: %v", err)
		}

		if backupData != nil {
			t.Error("Expected no backup data for create operation")
		}

		if len(reverseOps) != 1 {
			t.Fatalf("Expected 1 reverse op, got %d", len(reverseOps))
		}

		// The reverse of create is delete
		if reverseOp, ok := reverseOps[0].(*operations.DeleteOperation); ok {
			if reverseOp.Describe().Path != "test.txt" {
				t.Errorf("Expected reverse op to delete 'test.txt', got '%s'", reverseOp.Describe().Path)
			}
		} else {
			t.Error("Expected reverse op to be DeleteOperation")
		}
	})
}

func TestCreateDirectoryOperation(t *testing.T) {
	ctx := context.Background()

	t.Run("create directory", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateDirectoryOperation(core.OperationID("test-create-dir"), "testdir")
		dirItem := &TestDirItem{
			path: "testdir",
			mode: 0755,
		}
		op.SetItem(dirItem)

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify directory was created
		if _, ok := fs.Dirs()["testdir"]; !ok {
			t.Error("Expected directory to be created")
		}
	})

	t.Run("create nested directory", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateDirectoryOperation(core.OperationID("test-create-dir"), "parent/child/grandchild")
		dirItem := &TestDirItem{
			path: "parent/child/grandchild",
			mode: 0755,
		}
		op.SetItem(dirItem)

		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify all directories were created
		if _, ok := fs.Dirs()["parent/child/grandchild"]; !ok {
			t.Error("Expected nested directory to be created")
		}
	})

	t.Run("create directory requires item", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateDirectoryOperation(core.OperationID("test-create-dir"), "testdir")
		// Don't set item

		err := op.Execute(ctx, fs)
		if err == nil {
			t.Error("Expected error when no item is set")
		}
	})

	t.Run("reverse ops for create directory", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateDirectoryOperation(core.OperationID("test-create-dir"), "testdir")
		dirItem := &TestDirItem{
			path: "testdir",
			mode: 0755,
		}
		op.SetItem(dirItem)

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, nil)
		if err != nil {
			t.Fatalf("ReverseOps failed: %v", err)
		}

		if backupData != nil {
			t.Error("Expected no backup data for create operation")
		}

		if len(reverseOps) != 1 {
			t.Fatalf("Expected 1 reverse op, got %d", len(reverseOps))
		}

		// The reverse of create directory is delete
		if reverseOp, ok := reverseOps[0].(*operations.DeleteOperation); ok {
			if reverseOp.Describe().Path != "testdir" {
				t.Errorf("Expected reverse op to delete 'testdir', got '%s'", reverseOp.Describe().Path)
			}
		} else {
			t.Error("Expected reverse op to be DeleteOperation")
		}
	})
}
