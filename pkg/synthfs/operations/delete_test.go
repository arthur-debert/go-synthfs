package operations_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestDeleteOperation_ReverseOps(t *testing.T) {
	ctx := context.Background()
	defaultBudgetMB := 10.0

	t.Run("delete empty directory", func(t *testing.T) {
		fs := NewExtendedMockFilesystem()
		if err := fs.MkdirAll("empty_dir", 0755); err != nil {
			t.Fatalf("test setup failed: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-del-empty-dir"), "empty_dir")
		budget := &core.BackupBudget{TotalMB: defaultBudgetMB, RemainingMB: defaultBudgetMB}

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err != nil {
			t.Fatalf("ReverseOps for empty dir failed: %v", err)
		}

		bd, ok := backupData.(*core.BackupData)
		if !ok || bd == nil {
			t.Fatal("Expected BackupData to be *core.BackupData")
		}

		if bd.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", bd.BackupType)
		}
		if bd.SizeMB != 0 {
			t.Errorf("Expected SizeMB 0 for empty dir, got %f", bd.SizeMB)
		}

		// Check items - operations package creates []interface{} not []BackedUpItem
		items, ok := bd.Metadata["items"].([]interface{})
		if !ok {
			t.Fatal("Expected items to be []interface{}")
		}
		if len(items) != 1 {
			t.Errorf("Expected 1 item for the directory itself, got %d", len(items))
		}

		// Check the directory item
		if len(items) > 0 {
			if item, ok := items[0].(map[string]interface{}); ok {
				if relPath, _ := item["RelativePath"].(string); relPath != "." {
					t.Errorf("Expected RelativePath '.', got '%s'", relPath)
				}
				if itemType, _ := item["ItemType"].(string); itemType != "directory" {
					t.Errorf("Expected ItemType 'directory', got '%s'", itemType)
				}
			}
		}

		if len(reverseOps) != 1 {
			t.Errorf("Expected 1 reverse op, got %d", len(reverseOps))
		}
	})

	t.Run("delete directory with content, sufficient budget", func(t *testing.T) {
		fs := NewExtendedMockFilesystem()
		
		// Setup directory structure
		if err := fs.MkdirAll("dir1", 0755); err != nil {
			t.Fatalf("Failed to create dir1: %v", err)
		}
		if err := fs.WriteFile("dir1/file1.txt", []byte("content1"), 0644); err != nil {
			t.Fatalf("Failed to write file1.txt: %v", err)
		}
		if err := fs.MkdirAll("dir1/subdir", 0755); err != nil {
			t.Fatalf("Failed to create subdir: %v", err)
		}
		if err := fs.WriteFile("dir1/subdir/file2.txt", []byte("content2"), 0644); err != nil {
			t.Fatalf("Failed to write file2.txt: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-del-dir"), "dir1")
		budget := &core.BackupBudget{TotalMB: defaultBudgetMB, RemainingMB: defaultBudgetMB}

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err != nil {
			t.Fatalf("ReverseOps failed: %v", err)
		}

		// Should have one reverse op for each item in the directory
		if len(reverseOps) == 0 {
			t.Fatal("Expected reverse operations for directory deletion")
		}

		if backupData == nil {
			t.Fatal("Expected backup data for directory deletion")
		}

		// Check that backup contains the directory structure
		if bd, ok := backupData.(*core.BackupData); ok {
			items, ok := bd.Metadata["items"].([]interface{})
			if !ok || len(items) == 0 {
				t.Error("Expected backup metadata to contain items")
			}
		} else {
			t.Error("Expected backupData to be *core.BackupData")
		}
	})

	t.Run("delete file", func(t *testing.T) {
		fs := NewMockFilesystem()
		content := []byte("test file content")
		if err := fs.WriteFile("test.txt", content, 0644); err != nil {
			t.Fatalf("test setup failed: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-del-file"), "test.txt")
		budget := &core.BackupBudget{TotalMB: defaultBudgetMB, RemainingMB: defaultBudgetMB}

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err != nil {
			t.Fatalf("ReverseOps for file failed: %v", err)
		}

		bd, ok := backupData.(*core.BackupData)
		if !ok || bd == nil {
			t.Fatal("Expected BackupData to be *core.BackupData")
		}

		if bd.BackupType != "file" {
			t.Errorf("Expected BackupType 'file', got '%s'", bd.BackupType)
		}

		if string(bd.BackupContent) != string(content) {
			t.Errorf("Expected backup content %q, got %q", content, bd.BackupContent)
		}

		if len(reverseOps) != 1 {
			t.Errorf("Expected 1 reverse op, got %d", len(reverseOps))
		}
	})

	t.Run("delete non-existent path", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewDeleteOperation(core.OperationID("test-del-nonexist"), "nonexistent")
		budget := &core.BackupBudget{TotalMB: defaultBudgetMB, RemainingMB: defaultBudgetMB}

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err == nil {
			t.Error("Expected error for non-existent path")
		}
		if reverseOps != nil {
			t.Error("Expected nil reverseOps for non-existent path")
		}
		if backupData != nil {
			t.Error("Expected nil backupData for non-existent path")
		}
	})
}

func TestDeleteOperation_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("delete existing file", func(t *testing.T) {
		fs := NewMockFilesystem()
		if err := fs.WriteFile("test.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-del"), "test.txt")
		err := op.Execute(ctx, fs)
		if err != nil {
			t.Errorf("Execute failed: %v", err)
		}

		// Verify file is deleted
		if _, err := fs.Stat("test.txt"); err == nil {
			t.Error("Expected file to be deleted")
		}
	})

	t.Run("delete non-existent file is idempotent", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewDeleteOperation(core.OperationID("test-del"), "nonexistent.txt")
		err := op.Execute(ctx, fs)
		if err != nil {
			t.Errorf("Execute should not fail for non-existent file: %v", err)
		}
	})

	t.Run("delete directory", func(t *testing.T) {
		fs := NewMockFilesystem()
		if err := fs.MkdirAll("testdir", 0755); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-del"), "testdir")
		err := op.Execute(ctx, fs)
		if err != nil {
			t.Errorf("Execute failed: %v", err)
		}

		// Verify directory is deleted
		if _, err := fs.Stat("testdir"); err == nil {
			t.Error("Expected directory to be deleted")
		}
	})
}

// Type aliases for the consolidated mock filesystem with ReadDir support
type ExtendedMockFilesystem = testutil.OperationsMockFSWithReadDir

func NewExtendedMockFilesystem() *ExtendedMockFilesystem {
	return testutil.NewOperationsMockFSWithReadDir()
}

func TestDeleteOperation_ReverseOps_WithReadDir(t *testing.T) {
	ctx := context.Background()
	defaultBudgetMB := 10.0

	t.Run("delete directory with content", func(t *testing.T) {
		fs := NewExtendedMockFilesystem()
		
		// Setup directory structure
		if err := fs.MkdirAll("dir1", 0755); err != nil {
			t.Fatalf("Failed to create dir1: %v", err)
		}
		if err := fs.WriteFile("dir1/file1.txt", []byte("content1"), 0644); err != nil {
			t.Fatalf("Failed to write file1.txt: %v", err)
		}
		if err := fs.MkdirAll("dir1/subdir", 0755); err != nil {
			t.Fatalf("Failed to create subdir: %v", err)
		}
		if err := fs.WriteFile("dir1/subdir/file2.txt", []byte("content2 is longer"), 0644); err != nil {
			t.Fatalf("Failed to write file2.txt: %v", err)
		}
		if err := fs.WriteFile("dir1/file3.txt", []byte("content3"), 0644); err != nil {
			t.Fatalf("Failed to write file3.txt: %v", err)
		}

		op := operations.NewDeleteOperation(core.OperationID("test-del-dir"), "dir1")
		budget := &core.BackupBudget{TotalMB: defaultBudgetMB, RemainingMB: defaultBudgetMB}

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err != nil {
			t.Fatalf("ReverseOps failed: %v", err)
		}

		bd, ok := backupData.(*core.BackupData)
		if !ok || bd == nil {
			t.Fatal("Expected BackupData to be *core.BackupData")
		}

		if bd.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", bd.BackupType)
		}

		// Check that all items were backed up
		items, ok := bd.Metadata["items"].([]interface{})
		if !ok {
			t.Fatal("Expected items to be []interface{}")
		}
		
		// Should have: dir1 (.), file1.txt, subdir, subdir/file2.txt, file3.txt
		if len(items) != 5 {
			t.Errorf("Expected 5 items, got %d", len(items))
		}

		// Check that we have the right number of reverse operations
		// Should create directories first, then files
		if len(reverseOps) < 5 {
			t.Errorf("Expected at least 5 reverse ops, got %d", len(reverseOps))
		}

		// Verify backup size
		expectedSize := float64(8+18+8) / (1024 * 1024) // Total file content size
		if bd.SizeMB != expectedSize {
			t.Errorf("Expected SizeMB %f, got %f", expectedSize, bd.SizeMB)
		}
	})
}