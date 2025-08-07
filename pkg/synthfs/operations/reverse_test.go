package operations_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestReverseOperations_DeleteDirectory(t *testing.T) {
	ctx := context.Background()
	defaultBudgetMB := 10.0

	setupTestFSForDirDelete := func(t *testing.T, fs *ExtendedMockFilesystem) {
		t.Helper()
		must := func(err error) {
			t.Helper()
			if err != nil {
				t.Fatalf("test setup failed: %v", err)
			}
		}
		must(fs.MkdirAll("dir1", 0755))
		must(fs.WriteFile("dir1/file1.txt", []byte("content1"), 0644)) // 8 bytes
		must(fs.MkdirAll("dir1/subdir", 0755))
		must(fs.WriteFile("dir1/subdir/file2.txt", []byte("content2 is longer"), 0644)) // 18 bytes
		must(fs.WriteFile("dir1/file3.txt", []byte("content3"), 0644))                  // 8 bytes
	}

	t.Run("delete directory with content, sufficient budget", func(t *testing.T) {
		fs := NewExtendedMockFilesystem()
		setupTestFSForDirDelete(t, fs)

		op := operations.NewDeleteOperation(core.OperationID("test-del-dir-full"), "dir1")
		budget := &core.BackupBudget{TotalMB: defaultBudgetMB, RemainingMB: defaultBudgetMB}

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err != nil {
			t.Fatalf("ReverseOps for dir with content failed: %v", err)
		}

		bd, ok := backupData.(*core.BackupData)
		if !ok || bd == nil {
			t.Fatal("Expected BackupData to be *core.BackupData")
		}

		if bd.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", bd.BackupType)
		}

		expectedTotalSize := int64(8 + 18 + 8) // file1 + file2 + file3
		expectedTotalSizeMB := float64(expectedTotalSize) / (1024 * 1024)
		if bd.SizeMB != expectedTotalSizeMB {
			t.Errorf("Expected SizeMB %f, got %f", expectedTotalSizeMB, bd.SizeMB)
		}

		items, ok := bd.Metadata["items"].([]interface{})
		if !ok {
			t.Fatal("Expected items to be []interface{}")
		}
		if len(items) != 5 { // dir1, file1.txt, subdir, file2.txt, file3.txt
			t.Fatalf("Expected 5 items, got %d: %+v", len(items), items)
		}

		// Check item structure and content
		expectedItems := map[string]struct {
			ItemType string
			Content  string
		}{
			".":                                  {ItemType: "directory"},
			"file1.txt":                          {ItemType: "file", Content: "content1"},
			"subdir":                             {ItemType: "directory"},
			filepath.Join("subdir", "file2.txt"): {ItemType: "file", Content: "content2 is longer"},
			"file3.txt":                          {ItemType: "file", Content: "content3"},
		}

		foundItems := make(map[string]bool)
		for _, item := range items {
			if itemMap, ok := item.(map[string]interface{}); ok {
				relPath := itemMap["RelativePath"].(string)
				itemType := itemMap["ItemType"].(string)

				expected, found := expectedItems[relPath]
				if !found {
					t.Errorf("Unexpected item in backup: %+v", itemMap)
					continue
				}

				foundItems[relPath] = true

				if itemType != expected.ItemType {
					t.Errorf("ItemType mismatch for %s. Expected %s, Got %s", relPath, expected.ItemType, itemType)
				}

				if expected.ItemType == "file" {
					content := itemMap["Content"].([]byte)
					if string(content) != expected.Content {
						t.Errorf("Content mismatch for file %s. Expected '%s', Got '%s'", relPath, expected.Content, string(content))
					}
				}
			}
		}

		for path := range expectedItems {
			if !foundItems[path] {
				t.Errorf("Expected item not found in backup: %s", path)
			}
		}

		if len(reverseOps) != 5 {
			t.Fatalf("Expected 5 reverse operations, got %d", len(reverseOps))
		}

		// Check reverse op types and paths
		expectedRevOps := map[string]string{ // path -> type
			"dir1":                                       "create_directory",
			filepath.Join("dir1", "file1.txt"):           "create_file",
			filepath.Join("dir1", "subdir"):              "create_directory",
			filepath.Join("dir1", "subdir", "file2.txt"): "create_file",
			filepath.Join("dir1", "file3.txt"):           "create_file",
		}

		foundRevOps := make(map[string]bool)
		for _, ro := range reverseOps {
			// ro is already operations.Operation interface
			path := ro.Describe().Path
			expectedType, found := expectedRevOps[path]
			if !found {
				t.Errorf("Unexpected reverse operation path: %s", path)
				continue
			}

			foundRevOps[path] = true

			if ro.Describe().Type != expectedType {
				t.Errorf("Reverse op type mismatch for %s. Expected %s, Got %s", path, expectedType, ro.Describe().Type)
			}
		}

		for path := range expectedRevOps {
			if !foundRevOps[path] {
				t.Errorf("Expected reverse operation not found for path: %s", path)
			}
		}
	})

	t.Run("delete directory, budget insufficient for all files", func(t *testing.T) {
		fs := NewExtendedMockFilesystem()
		setupTestFSForDirDelete(t, fs) // total 34 bytes needed

		op := operations.NewDeleteOperation(core.OperationID("test-del-dir-partial"), "dir1")
		// Budget enough for file1.txt (8b) and a bit more, but not file2.txt (18b)
		smallBudgetMB := float64(10) / (1024 * 1024) // Approx 10 bytes
		budget := &core.BackupBudget{TotalMB: smallBudgetMB, RemainingMB: smallBudgetMB}

		_, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err == nil {
			t.Fatal("Expected ReverseOps to return an error due to insufficient budget")
		}
		if !strings.Contains(err.Error(), "budget exceeded") {
			t.Fatalf("Expected budget exceeded error, got: %v", err)
		}

		// BackupData should still exist and contain partially backed up items
		if backupData == nil {
			t.Fatal("Expected partial BackupData even on budget error")
		}

		bd, ok := backupData.(*core.BackupData)
		if !ok || bd == nil {
			t.Fatal("Expected BackupData to be *core.BackupData")
		}

		if bd.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", bd.BackupType)
		}

		items, ok := bd.Metadata["items"].([]interface{})
		if !ok {
			t.Fatal("Expected items to be []interface{}")
		}

		// Should have backed up at least one file before hitting budget limit
		var fileBackedUp bool
		for _, item := range items {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["ItemType"] == "file" {
					fileBackedUp = true
					break
				}
			}
		}

		if !fileBackedUp {
			t.Errorf("Expected at least one file to be backed up before budget exhaustion. Items: %+v", items)
		}

		// Check that total backed up size is less than the full amount
		expectedFullSize := int64(8 + 18 + 8)
		if bd.SizeMB >= float64(expectedFullSize)/(1024*1024) {
			t.Errorf("Expected partial backup size, got %f MB", bd.SizeMB)
		}
	})

	t.Run("delete directory, budget insufficient for any file", func(t *testing.T) {
		fs := NewExtendedMockFilesystem()
		setupTestFSForDirDelete(t, fs) // Smallest file is 8 bytes

		op := operations.NewDeleteOperation(core.OperationID("test-del-dir-none"), "dir1")
		tinyBudgetMB := float64(1) / (1024 * 1024) // 1 byte budget
		budget := &core.BackupBudget{TotalMB: tinyBudgetMB, RemainingMB: tinyBudgetMB}

		_, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err == nil {
			t.Fatal("Expected ReverseOps to return an error due to insufficient budget for any file")
		}
		if !strings.Contains(err.Error(), "budget exceeded") {
			t.Fatalf("Expected budget exceeded error, got: %v", err)
		}

		bd, ok := backupData.(*core.BackupData)
		if !ok || bd == nil {
			t.Fatal("Expected BackupData to be *core.BackupData")
		}

		if bd.SizeMB != 0 {
			t.Errorf("Expected SizeMB 0 when no files could be backed up, got %f", bd.SizeMB)
		}

		items, ok := bd.Metadata["items"].([]interface{})
		if !ok {
			t.Fatal("Expected items to be []interface{}")
		}

		// Directory structure should be preserved even when no files can be backed up
		dirCount := 0
		fileCount := 0
		for _, item := range items {
			if itemMap, ok := item.(map[string]interface{}); ok {
				switch itemMap["ItemType"] {
				case "directory":
					dirCount++
				case "file":
					fileCount++
				}
			}
		}

		if dirCount != 2 {
			t.Errorf("Expected 2 directories in backup (root and subdir), got %d. Items: %+v", dirCount, items)
		}
		if fileCount != 0 {
			t.Errorf("Expected 0 files in backup due to budget constraints, got %d. Items: %+v", fileCount, items)
		}
	})
}

func TestReverseOperations_Files(t *testing.T) {
	ctx := context.Background()
	defaultBudgetMB := 10.0

	t.Run("create file reverse operation", func(t *testing.T) {
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
			t.Error("Expected no backup data for create file operation")
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

	t.Run("delete file reverse operation", func(t *testing.T) {
		fs := NewMockFilesystem()
		content := []byte("test file content")
		if err := fs.WriteFile("test.txt", content, 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
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

		// The reverse of delete is create
		if reverseOp, ok := reverseOps[0].(*operations.CreateFileOperation); ok {
			if reverseOp.Describe().Path != "test.txt" {
				t.Errorf("Expected reverse op to create 'test.txt', got '%s'", reverseOp.Describe().Path)
			}
		} else {
			t.Error("Expected reverse op to be CreateFileOperation")
		}
	})

	t.Run("copy file reverse operation", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCopyOperation(core.OperationID("test-copy"), "source.txt")
		op.SetPaths("source.txt", "dest.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "dest.txt")

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, nil)
		if err != nil {
			t.Fatalf("ReverseOps failed: %v", err)
		}

		if backupData != nil {
			t.Error("Expected no backup data for copy operation")
		}

		if len(reverseOps) != 1 {
			t.Fatalf("Expected 1 reverse op, got %d", len(reverseOps))
		}

		// The reverse of copy is delete the destination
		if reverseOp, ok := reverseOps[0].(*operations.DeleteOperation); ok {
			if reverseOp.Describe().Path != "dest.txt" {
				t.Errorf("Expected reverse op to delete 'dest.txt', got '%s'", reverseOp.Describe().Path)
			}
		} else {
			t.Error("Expected reverse op to be DeleteOperation")
		}
	})

	t.Run("move file reverse operation", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewMoveOperation(core.OperationID("test-move"), "source.txt")
		op.SetPaths("source.txt", "dest.txt")
		// Also set destination in description for consistency
		op.SetDescriptionDetail("destination", "dest.txt")

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, nil)
		if err != nil {
			t.Fatalf("ReverseOps failed: %v", err)
		}

		if backupData != nil {
			t.Error("Expected no backup data for move operation")
		}

		if len(reverseOps) != 1 {
			t.Fatalf("Expected 1 reverse op, got %d", len(reverseOps))
		}

		// The reverse of move is move back
		if reverseOp, ok := reverseOps[0].(*operations.MoveOperation); ok {
			if reverseOp.Describe().Path != "dest.txt" {
				t.Errorf("Expected reverse op source to be 'dest.txt', got '%s'", reverseOp.Describe().Path)
			}
			// Check destination in details
			if dest, ok := reverseOp.Describe().Details["destination"].(string); ok && dest != "source.txt" {
				t.Errorf("Expected reverse op destination to be 'source.txt', got '%s'", dest)
			}
		} else {
			t.Error("Expected reverse op to be MoveOperation")
		}
	})
}
