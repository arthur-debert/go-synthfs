package synthfs

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestSimpleOperation_GetItem(t *testing.T) {
	t.Run("GetItem returns nil when no item set", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_file", "test/path")

		item := op.GetItem()
		if item != nil {
			t.Errorf("Expected nil item, got %v", item)
		}
	})

	t.Run("GetItem returns set item", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_file", "test/file.txt")
		fileItem := NewFile("test/file.txt").WithContent([]byte("test content"))
		op.SetItem(fileItem)

		item := op.GetItem()
		if item != fileItem {
			t.Errorf("Expected fileItem, got %v", item)
		}
	})
}

func TestValidationError_Error(t *testing.T) {
	op := NewSimpleOperation("test-op", "create_file", "test/path")

	t.Run("Error without cause", func(t *testing.T) {
		err := &ValidationError{
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
		err := &ValidationError{
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

func TestValidationError_Unwrap(t *testing.T) {
	op := NewSimpleOperation("test-op", "create_file", "test/path")

	t.Run("Unwrap returns nil when no cause", func(t *testing.T) {
		err := &ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "test reason",
			Cause:         nil,
		}

		unwrapped := err.Unwrap()
		if unwrapped != nil {
			t.Errorf("Expected nil, got %v", unwrapped)
		}
	})

	t.Run("Unwrap returns cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := &ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "test reason",
			Cause:         cause,
		}

		unwrapped := err.Unwrap()
		if unwrapped != cause {
			t.Errorf("Expected %v, got %v", cause, unwrapped)
		}
	})
}

func TestDependencyError_Error(t *testing.T) {
	op := NewSimpleOperation(OperationID("test-op"), "create_file", "test/path")

	err := &DependencyError{
		Operation:    op,
		Dependencies: []OperationID{"dep1", "dep2"},
		Missing:      []OperationID{"dep1"},
	}

	expected := "dependency error for operation test-op: missing dependencies [dep1] (required: [dep1 dep2])"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestConflictError_Error(t *testing.T) {
	op := NewSimpleOperation(OperationID("test-op"), "create_file", "test/path")

	err := &ConflictError{
		Operation: op,
		Conflicts: []OperationID{"conflict1", "conflict2"},
	}

	expected := "conflict error for operation test-op: conflicts with operations [conflict1 conflict2]"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestReverseOperations_DeleteDirectory(t *testing.T) {
	ctx := context.Background()
	defaultBudgetMB := 10.0

	setupTestFSForDirDelete := func(t *testing.T, fs *TestFileSystem) {
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

	t.Run("delete empty directory", func(t *testing.T) {
		fs := NewTestFileSystem()
		if err := fs.MkdirAll("empty_dir", 0755); err != nil {
			t.Fatalf("test setup failed: %v", err)
		}

		op := NewSimpleOperation(OperationID("test-del-empty-dir"), "delete", "empty_dir")
		budget := &BackupBudget{TotalMB: defaultBudgetMB, RemainingMB: defaultBudgetMB}

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err != nil {
			t.Fatalf("ReverseOps for empty dir failed: %v", err)
		}

		if backupData.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", backupData.BackupType)
		}
		if backupData.SizeMB != 0 {
			t.Errorf("Expected SizeMB 0 for empty dir, got %f", backupData.SizeMB)
		}
		items, _ := backupData.Metadata["items"].([]BackedUpItem)
		if len(items) != 1 || items[0].RelativePath != "." || items[0].ItemType != "directory" {
			t.Errorf("Expected 1 BackedUpItem for the directory itself, got %v", items)
		}
		if len(reverseOps) != 1 || reverseOps[0].Describe().Type != "create_directory" || reverseOps[0].Describe().Path != "empty_dir" {
			t.Errorf("Expected 1 create_directory reverse op, got %v", reverseOps)
		}
	})

	t.Run("delete directory with content, sufficient budget", func(t *testing.T) {
		fs := NewTestFileSystem()
		setupTestFSForDirDelete(t, fs)

		op := NewSimpleOperation(OperationID("test-del-dir-full"), "delete", "dir1")
		budget := &BackupBudget{TotalMB: defaultBudgetMB, RemainingMB: defaultBudgetMB}

		reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err != nil {
			t.Fatalf("ReverseOps for dir with content failed: %v", err)
		}

		if backupData.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", backupData.BackupType)
		}
		expectedTotalSize := int64(8 + 18 + 8) // file1 + file2 + file3
		expectedTotalSizeMB := float64(expectedTotalSize) / (1024 * 1024)
		if backupData.SizeMB != expectedTotalSizeMB {
			t.Errorf("Expected SizeMB %f, got %f", expectedTotalSizeMB, backupData.SizeMB)
		}

		items, _ := backupData.Metadata["items"].([]BackedUpItem)
		if len(items) != 5 { // dir1, file1.txt, subdir, file2.txt, file3.txt
			t.Fatalf("Expected 5 BackedUpItems, got %d: %+v", len(items), items)
		}

		// Check item order and types (basic check)
		// Order of siblings is not guaranteed by ReadDir, so we check for presence and correctness.
		expectedItems := map[string]struct {
			ItemType       string
			Content_or_Nil interface{}
		}{
			".":                                  {ItemType: "directory"},
			"file1.txt":                          {ItemType: "file", Content_or_Nil: "content1"},
			"subdir":                             {ItemType: "directory"},
			filepath.Join("subdir", "file2.txt"): {ItemType: "file", Content_or_Nil: "content2 is longer"},
			"file3.txt":                          {ItemType: "file", Content_or_Nil: "content3"},
		}
		if len(items) != len(expectedItems) {
			t.Errorf("Number of backed up items mismatch. Expected %d, Got %d: %+v", len(expectedItems), len(items), items)
		}
		for _, item := range items {
			expected, found := expectedItems[item.RelativePath]
			if !found {
				t.Errorf("Unexpected item in backup: %+v", item)
				continue
			}
			if item.ItemType != expected.ItemType {
				t.Errorf("ItemType mismatch for %s. Expected %s, Got %s", item.RelativePath, expected.ItemType, item.ItemType)
			}
			if expected.ItemType == "file" && string(item.Content) != expected.Content_or_Nil.(string) {
				t.Errorf("Content mismatch for file %s. Expected '%s', Got '%s'", item.RelativePath, expected.Content_or_Nil.(string), string(item.Content))
			}
			delete(expectedItems, item.RelativePath) // Mark as found
		}
		if len(expectedItems) > 0 {
			t.Errorf("Not all expected items were found in backup. Missing: %v", expectedItems)
		}

		if len(reverseOps) != 5 {
			t.Fatalf("Expected 5 reverse operations, got %d", len(reverseOps))
		}
		// Check reverse op types and paths more flexibly
		expectedRevOps := map[string]string{ // path -> type
			"dir1":                                       "create_directory",
			filepath.Join("dir1", "file1.txt"):           "create_file",
			filepath.Join("dir1", "subdir"):              "create_directory",
			filepath.Join("dir1", "subdir", "file2.txt"): "create_file",
			filepath.Join("dir1", "file3.txt"):           "create_file",
		}
		for _, ro := range reverseOps {
			path := ro.Describe().Path
			expectedType, found := expectedRevOps[path]
			if !found {
				t.Errorf("Unexpected reverse operation path: %s", path)
				continue
			}
			if ro.Describe().Type != expectedType {
				t.Errorf("Reverse op type mismatch for %s. Expected %s, Got %s", path, expectedType, ro.Describe().Type)
			}
			delete(expectedRevOps, path)
		}
		if len(expectedRevOps) > 0 {
			t.Errorf("Not all expected reverse operations were generated. Missing for paths: %v", expectedRevOps)
		}

		// Verify content of a backed up file by finding it in items
		var foundFile2 bool
		for _, item := range items {
			if item.RelativePath == filepath.Join("subdir", "file2.txt") {
				foundFile2 = true
				if string(item.Content) != "content2 is longer" {
					t.Errorf("Expected content for file2.txt to be 'content2 is longer', got '%s'", string(item.Content))
				}
				break
			}
		}
		if !foundFile2 {
			t.Error("file2.txt not found in backed up items")
		}
	})

	t.Run("delete directory, budget insufficient for all files", func(t *testing.T) {
		fs := NewTestFileSystem()
		setupTestFSForDirDelete(t, fs) // total 34 bytes needed

		op := NewSimpleOperation(OperationID("test-del-dir-partial"), "delete", "dir1")
		// Budget enough for file1.txt (8b) and a bit more, but not file2.txt (18b)
		smallBudgetMB := float64(10) / (1024 * 1024) // Approx 10 bytes
		budget := &BackupBudget{TotalMB: smallBudgetMB, RemainingMB: smallBudgetMB}

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
		if backupData.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", backupData.BackupType)
		}

		items, _ := backupData.Metadata["items"].([]BackedUpItem)
		// Should have dir1, file1.txt. file2.txt should have caused the error.
		// So, 2 items successfully processed before error on the 3rd considered file (file2.txt).
		// The order of processing files within a dir level isn't strictly guaranteed by ReadDir,
		// but file1.txt (8b) is smaller than file2.txt (18b).
		// Let's check that at least file1.txt was backed up.
		var file1BackedUp bool
		for _, item := range items {
			if item.RelativePath == "file1.txt" && item.ItemType == "file" {
				file1BackedUp = true
				break
			}
		}
		if !file1BackedUp {
			t.Errorf("Expected file1.txt to be backed up before budget exhaustion. Items: %+v", items)
		}

		// Check that total backed up size is less than the full amount
		expectedFullSize := int64(8 + 18 + 8)
		if backupData.SizeMB >= float64(expectedFullSize)/(1024*1024) {
			t.Errorf("Expected partial backup size, got %f MB", backupData.SizeMB)
		}
		// Specifically, it should be the size of file1.txt
		if backupData.SizeMB != float64(8)/(1024*1024) {
			t.Errorf("Expected backup size to be for file1.txt (8 bytes), got %f MB. Items: %+v", backupData.SizeMB, items)
		}
	})

	t.Run("delete directory, budget insufficient for any file", func(t *testing.T) {
		fs := NewTestFileSystem()
		setupTestFSForDirDelete(t, fs) // Smallest file is 8 bytes

		op := NewSimpleOperation(OperationID("test-del-dir-none"), "delete", "dir1")
		tinyBudgetMB := float64(1) / (1024 * 1024) // 1 byte budget
		budget := &BackupBudget{TotalMB: tinyBudgetMB, RemainingMB: tinyBudgetMB}

		_, backupData, err := op.ReverseOps(ctx, fs, budget)
		if err == nil {
			t.Fatal("Expected ReverseOps to return an error due to insufficient budget for any file")
		}
		if !strings.Contains(err.Error(), "budget exceeded") {
			t.Fatalf("Expected budget exceeded error, got: %v", err)
		}
		if backupData.SizeMB != 0 {
			t.Errorf("Expected SizeMB 0 when no files could be backed up, got %f", backupData.SizeMB)
		}
		items, _ := backupData.Metadata["items"].([]BackedUpItem)
		// Directory structure should be preserved even when no files can be backed up
		// Expecting: "." (root), "subdir"
		dirCount := 0
		fileCount := 0
		for _, item := range items {
			switch item.ItemType {
			case "directory":
				dirCount++
			case "file":
				fileCount++
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

func TestSimpleOperation_Rollback(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	t.Run("Rollback create operation", func(t *testing.T) {
		// Setup: create a file first
		op := NewSimpleOperation("test-op", "create_file", "test/file.txt")
		fileItem := NewFile("test/file.txt").WithContent([]byte("test"))
		op.SetItem(fileItem)

		// Execute to create the file
		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file exists
		_, err = fs.Stat("test/file.txt")
		if err != nil {
			t.Fatalf("File should exist: %v", err)
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify file is removed
		_, err = fs.Stat("test/file.txt")
		if err == nil {
			t.Error("File should be removed after rollback")
		}
	})

	t.Run("Rollback copy operation", func(t *testing.T) {
		// Setup: create source file and copy operation
		err := fs.WriteFile("test/source.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := NewSimpleOperation("test-op", "copy", "test/source.txt")
		op.SetPaths("test/source.txt", "test/destination.txt")

		// Execute copy
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify destination exists
		_, err = fs.Stat("test/destination.txt")
		if err != nil {
			t.Fatalf("Destination should exist: %v", err)
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify destination is removed
		_, err = fs.Stat("test/destination.txt")
		if err == nil {
			t.Error("Destination should be removed after rollback")
		}

		// Verify source still exists
		_, err = fs.Stat("test/source.txt")
		if err != nil {
			t.Error("Source should still exist after copy rollback")
		}
	})

	t.Run("Rollback move operation", func(t *testing.T) {
		// Setup: create source file and move operation
		err := fs.WriteFile("test/movesource.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := NewSimpleOperation("test-op", "move", "test/movesource.txt")
		op.SetPaths("test/movesource.txt", "test/movedest.txt")

		// Execute move
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file moved
		_, err = fs.Stat("test/movedest.txt")
		if err != nil {
			t.Fatalf("Destination should exist: %v", err)
		}
		_, err = fs.Stat("test/movesource.txt")
		if err == nil {
			t.Error("Source should not exist after move")
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify file moved back
		_, err = fs.Stat("test/movesource.txt")
		if err != nil {
			t.Error("Source should exist after move rollback")
		}
		_, err = fs.Stat("test/movedest.txt")
		if err == nil {
			t.Error("Destination should not exist after move rollback")
		}
	})

	t.Run("Rollback delete operation returns error", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "delete", "test/file.txt")

		err := op.Rollback(ctx, fs)
		if err == nil {
			t.Error("Expected error for delete rollback")
		}

		expectedMsg := "rollback of delete operations not yet implemented"
		if err.Error() != expectedMsg {
			t.Errorf("Expected %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("Rollback unknown operation type", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "unknown", "test/path")

		err := op.Rollback(ctx, fs)
		if err == nil {
			t.Error("Expected error for unknown operation type")
		}

		expectedMsg := "unknown operation type for rollback: unknown"
		if err.Error() != expectedMsg {
			t.Errorf("Expected %q, got %q", expectedMsg, err.Error())
		}
	})
}

func TestSimpleOperation_ExecuteDelete_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("ExecuteDelete with filesystem that doesn't support Stat", func(t *testing.T) {
		// Use a basic filesystem that doesn't implement FullFileSystem
		fs := &basicFS{}
		op := NewSimpleOperation("test-op", "delete", "test/file.txt")

		// This should use the fallback path
		err := op.Execute(ctx, fs)
		// We expect an error since the file doesn't exist, but it should try Remove first
		if err == nil {
			t.Error("Expected error when deleting non-existent file")
		}
	})

	t.Run("ExecuteDelete removes directory", func(t *testing.T) {
		fs := NewTestFileSystem()

		// Create a directory
		err := fs.MkdirAll("test/dir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		op := NewSimpleOperation("test-op", "delete", "test/dir")
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify directory is removed
		_, err = fs.Stat("test/dir")
		if err == nil {
			t.Error("Directory should be removed")
		}
	})

	t.Run("ExecuteDelete removes file", func(t *testing.T) {
		fs := NewTestFileSystem()

		// Create a file
		err := fs.WriteFile("test/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		op := NewSimpleOperation("test-op", "delete", "test/file.txt")
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify file is removed
		_, err = fs.Stat("test/file.txt")
		if err == nil {
			t.Error("File should be removed")
		}
	})
}

func TestSimpleOperation_Validation_EdgeCases(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	t.Run("ValidateCreateFile with wrong item type", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_file", "test/file.txt")
		// Set wrong item type
		dirItem := NewDirectory("test/dir")
		op.SetItem(dirItem)

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for wrong item type")
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if !strings.Contains(validationErr.Reason, "expected FileItem") {
			t.Errorf("Expected wrong type error, got: %s", validationErr.Reason)
		}
	})

	t.Run("ValidateCreateDirectory with wrong item type", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_directory", "test/dir")
		fileItem := NewFile("test/file.txt")
		op.SetItem(fileItem)

		err := op.Validate(ctx, fs)
		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if !strings.Contains(validationErr.Reason, "expected DirectoryItem") {
			t.Errorf("Expected wrong type error, got: %s", validationErr.Reason)
		}
	})

	t.Run("ValidateCreateSymlink with empty target", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_symlink", "test/link")
		symlinkItem := NewSymlink("test/link", "") // Empty target
		op.SetItem(symlinkItem)

		err := op.Validate(ctx, fs)
		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if !strings.Contains(validationErr.Reason, "target cannot be empty") {
			t.Errorf("Expected empty target error, got: %s", validationErr.Reason)
		}
	})

	t.Run("ValidateCreateArchive with no sources", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.tar.gz")
		archiveItem := NewArchive("test/archive.tar.gz", ArchiveFormatTarGz, []string{})
		// Empty sources slice to test validation error
		op.SetItem(archiveItem)

		err := op.Validate(ctx, fs)
		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if !strings.Contains(validationErr.Reason, "at least one source") {
			t.Errorf("Expected no sources error, got: %s", validationErr.Reason)
		}
	})
}

// Helper type - basicFS is a minimal filesystem implementation that doesn't support Stat
type basicFS struct{}

func (fs *basicFS) Open(name string) (fs.File, error) {
	return nil, errors.New("not implemented")
}

func (fs *basicFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return errors.New("not implemented")
}

func (fs *basicFS) MkdirAll(path string, perm fs.FileMode) error {
	return errors.New("not implemented")
}

func (fs *basicFS) Remove(name string) error {
	return errors.New("file not found")
}

func (fs *basicFS) RemoveAll(path string) error {
	return errors.New("file not found")
}

func (fs *basicFS) Symlink(oldname, newname string) error {
	return errors.New("not implemented")
}

func (fs *basicFS) Readlink(name string) (string, error) {
	return "", errors.New("not implemented")
}

func (fs *basicFS) Rename(oldpath, newpath string) error {
	return errors.New("not implemented")
}

// Test addToZipArchive function coverage improvement
func TestSimpleOperation_AddToZipArchive_EdgeCases(t *testing.T) {
	t.Run("AddToZipArchive with filesystem that doesn't support Stat", func(t *testing.T) {
		// Use a basic filesystem that doesn't implement FullFileSystem
		fs := &basicFS{}
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		// Create a zip writer
		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)
		defer func() {
			if closeErr := zipWriter.Close(); closeErr != nil {
				t.Logf("Warning: failed to close zip writer: %v", closeErr)
			}
		}()

		err := op.addToZipArchive(zipWriter, "test/file.txt", fs)
		if err == nil {
			t.Error("Expected error when filesystem doesn't support Stat")
		}

		expectedMsg := "filesystem does not support Stat operation needed for archiving"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got: %s", expectedMsg, err.Error())
		}
	})

	t.Run("AddToZipArchive with Stat error", func(t *testing.T) {
		fs := NewTestFileSystem()
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)
		defer func() {
			if closeErr := zipWriter.Close(); closeErr != nil {
				t.Logf("Warning: failed to close zip writer: %v", closeErr)
			}
		}()

		// Try to add non-existent file
		err := op.addToZipArchive(zipWriter, "nonexistent/file.txt", fs)
		if err == nil {
			t.Error("Expected error when file doesn't exist")
		}

		expectedMsg := "failed to stat nonexistent/file.txt"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got: %s", expectedMsg, err.Error())
		}
	})

	t.Run("AddToZipArchive successfully adds directory", func(t *testing.T) {
		fs := NewTestFileSystem()
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		// Create a directory
		err := fs.MkdirAll("test/mydir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		// Add directory to zip
		err = op.addToZipArchive(zipWriter, "test/mydir", fs)
		if err != nil {
			t.Fatalf("Failed to add directory to zip: %v", err)
		}

		err = zipWriter.Close()
		if err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}

		// Verify directory entry was created
		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("Failed to read zip: %v", err)
		}

		found := false
		for _, file := range reader.File {
			if file.Name == "test/mydir/" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Directory entry not found in zip")
		}
	})

	t.Run("AddToZipArchive successfully adds file", func(t *testing.T) {
		fs := NewTestFileSystem()
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		// Create a file
		fileContent := []byte("test file content")
		err := fs.WriteFile("test/myfile.txt", fileContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		// Add file to zip
		err = op.addToZipArchive(zipWriter, "test/myfile.txt", fs)
		if err != nil {
			t.Fatalf("Failed to add file to zip: %v", err)
		}

		err = zipWriter.Close()
		if err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}

		// Verify file entry was created with correct content
		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("Failed to read zip: %v", err)
		}

		found := false
		for _, file := range reader.File {
			if file.Name == "test/myfile.txt" {
				found = true

				// Read file content from zip
				rc, err := file.Open()
				if err != nil {
					t.Fatalf("Failed to open file in zip: %v", err)
				}
				defer func() {
					if closeErr := rc.Close(); closeErr != nil {
						t.Logf("Warning: failed to close zip file reader: %v", closeErr)
					}
				}()

				content, err := io.ReadAll(rc)
				if err != nil {
					t.Fatalf("Failed to read file content: %v", err)
				}

				if !bytes.Equal(content, fileContent) {
					t.Errorf("File content mismatch. Expected %q, got %q", fileContent, content)
				}
				break
			}
		}
		if !found {
			t.Error("File entry not found in zip")
		}
	})

	t.Run("AddToZipArchive with file open error", func(t *testing.T) {
		fs := &errorFS{
			testFS:    NewTestFileSystem(),
			openError: true,
		}
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		// Create a file in the underlying test filesystem first
		err := fs.testFS.WriteFile("test/myfile.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)
		defer func() {
			if closeErr := zipWriter.Close(); closeErr != nil {
				t.Logf("Warning: failed to close zip writer: %v", closeErr)
			}
		}()

		// Try to add file - should fail on Open
		err = op.addToZipArchive(zipWriter, "test/myfile.txt", fs)
		if err == nil {
			t.Error("Expected error when file open fails")
		}

		expectedMsg := "failed to open file test/myfile.txt"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got: %s", expectedMsg, err.Error())
		}
	})
}

// Test validateCopy function coverage improvement
func TestSimpleOperation_ValidateCopy_EdgeCases(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	t.Run("ValidateCopy with empty source path", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "copy", "test/source.txt") // Non-empty description path
		op.SetPaths("", "test/destination.txt")                        // Empty source path

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty source path")
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		expectedMsg := "copy source path cannot be empty"
		if validationErr.Reason != expectedMsg {
			t.Errorf("Expected reason %q, got %q", expectedMsg, validationErr.Reason)
		}
	})

	t.Run("ValidateCopy with empty destination path", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "copy", "test/source.txt")
		op.SetPaths("test/source.txt", "") // Empty destination path

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty destination path")
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		expectedMsg := "copy destination path cannot be empty"
		if validationErr.Reason != expectedMsg {
			t.Errorf("Expected reason %q, got %q", expectedMsg, validationErr.Reason)
		}
	})

	t.Run("ValidateCopy with valid paths", func(t *testing.T) {
		// Phase I, Milestone 1: Create source file first since we now validate existence
		testFS := NewTestFileSystem()
		err := testFS.WriteFile("test/source.txt", []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test source file: %v", err)
		}

		op := NewSimpleOperation("test-op", "copy", "test/source.txt")
		op.SetPaths("test/source.txt", "test/destination.txt")

		err = op.Validate(ctx, testFS)
		if err != nil {
			t.Errorf("Expected no validation error for valid paths, got: %v", err)
		}
	})

	t.Run("ValidateCopy now checks source existence at validation time", func(t *testing.T) {
		// Phase I, Milestone 1: Updated test to reflect new validation behavior
		op := NewSimpleOperation("test-op", "copy", "nonexistent/source.txt")
		op.SetPaths("nonexistent/source.txt", "test/destination.txt")

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for non-existent source, but got none")
		}

		if !strings.Contains(err.Error(), "copy source does not exist") {
			t.Errorf("Expected source existence error, got: %v", err)
		}
	})

	t.Run("ValidateCopy with empty description path (general validation)", func(t *testing.T) {
		// This tests that general validation happens before validateCopy
		op := NewSimpleOperation("test-op", "copy", "") // Empty description path
		op.SetPaths("test/source.txt", "test/destination.txt")

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty description path")
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		expectedMsg := "path cannot be empty"
		if validationErr.Reason != expectedMsg {
			t.Errorf("Expected reason %q, got %q", expectedMsg, validationErr.Reason)
		}
	})
}

// Helper filesystem that can simulate errors
type errorFS struct {
	testFS    *TestFileSystem
	openError bool
	statError bool
}

func (fs *errorFS) Open(name string) (fs.File, error) {
	if fs.openError {
		return nil, errors.New("simulated open error")
	}
	return fs.testFS.Open(name)
}

func (fs *errorFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return fs.testFS.WriteFile(name, data, perm)
}

func (fs *errorFS) MkdirAll(path string, perm fs.FileMode) error {
	return fs.testFS.MkdirAll(path, perm)
}

func (fs *errorFS) Remove(name string) error {
	return fs.testFS.Remove(name)
}

func (fs *errorFS) RemoveAll(path string) error {
	return fs.testFS.RemoveAll(path)
}

func (fs *errorFS) Symlink(oldname, newname string) error {
	return fs.testFS.Symlink(oldname, newname)
}

func (fs *errorFS) Readlink(name string) (string, error) {
	return fs.testFS.Readlink(name)
}

func (fs *errorFS) Rename(oldpath, newpath string) error {
	return fs.testFS.Rename(oldpath, newpath)
}

func (fs *errorFS) Stat(name string) (fs.FileInfo, error) {
	if fs.statError {
		return nil, errors.New("simulated stat error")
	}
	return fs.testFS.Stat(name)
}

// Phase I, Milestone 1: Source Existence Validation Tests

func TestSourceExistenceValidation(t *testing.T) {
	ctx := context.Background()
	testFS := NewTestFileSystem()

	// Create some test files and directories for validation tests
	err := testFS.WriteFile("existing_file.txt", []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = testFS.MkdirAll("existing_dir", 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	err = testFS.WriteFile("source1.txt", []byte("source 1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source1.txt: %v", err)
	}

	err = testFS.WriteFile("source2.txt", []byte("source 2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source2.txt: %v", err)
	}

	// Create a test archive for unarchive tests
	archiveContent := createSimpleZip(t, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	})
	err = testFS.WriteFile("test_archive.zip", archiveContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create test archive: %v", err)
	}

	t.Run("Copy validation - source exists", func(t *testing.T) {
		op := NewSimpleOperation("copy-op", "copy", "existing_file.txt")
		op.SetPaths("existing_file.txt", "destination.txt")

		err := op.Validate(ctx, testFS)
		if err != nil {
			t.Errorf("Expected no validation error for existing source, got: %v", err)
		}
	})

	t.Run("Copy validation - source does not exist", func(t *testing.T) {
		op := NewSimpleOperation("copy-op", "copy", "nonexistent_file.txt")
		op.SetPaths("nonexistent_file.txt", "destination.txt")

		err := op.Validate(ctx, testFS)
		if err == nil {
			t.Error("Expected validation error for non-existent source, but got none")
		}

		if !strings.Contains(err.Error(), "copy source does not exist") {
			t.Errorf("Expected 'copy source does not exist' error, got: %v", err)
		}
	})

	t.Run("Move validation - source exists", func(t *testing.T) {
		op := NewSimpleOperation("move-op", "move", "existing_file.txt")
		op.SetPaths("existing_file.txt", "new_location.txt")

		err := op.Validate(ctx, testFS)
		if err != nil {
			t.Errorf("Expected no validation error for existing source, got: %v", err)
		}
	})
}

// Helper function to create a simple zip file for testing
func createSimpleZip(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for filename, content := range files {
		f, err := w.Create(filename)
		if err != nil {
			t.Fatalf("Failed to create zip entry %s: %v", filename, err)
		}

		_, err = f.Write([]byte(content))
		if err != nil {
			t.Fatalf("Failed to write zip entry %s: %v", filename, err)
		}
	}

	err := w.Close()
	if err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}
