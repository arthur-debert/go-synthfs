package synthfs

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPhase3_CompleteWorkflow(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Prepare some initial files
	if err := fs.WriteFile("source.txt", []byte("source content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}
	if err := fs.WriteFile("to-delete.txt", []byte("will be deleted"), 0644); err != nil {
		t.Fatalf("Failed to create file to delete: %v", err)
	}
	if err := fs.WriteFile("to-move.txt", []byte("will be moved"), 0644); err != nil {
		t.Fatalf("Failed to create file to move: %v", err)
	}

	registry := GetDefaultRegistry()
	batch := NewBatch(fs, registry).WithContext(ctx)

	// Add various operations to test different reverse operation types
	_, err := batch.CreateDir("new-dir")
	if err != nil {
		t.Fatalf("Failed to add CreateDir: %v", err)
	}

	_, err = batch.CreateFile("new-file.txt", []byte("new content"))
	if err != nil {
		t.Fatalf("Failed to add CreateFile: %v", err)
	}

	_, err = batch.Copy("source.txt", "copied.txt")
	if err != nil {
		t.Fatalf("Failed to add Copy: %v", err)
	}

	_, err = batch.Move("to-move.txt", "moved.txt")
	if err != nil {
		t.Fatalf("Failed to add Move: %v", err)
	}

	_, err = batch.Delete("to-delete.txt")
	if err != nil {
		t.Fatalf("Failed to add Delete: %v", err)
	}

	// Execute with restorable mode
	result, err := batch.RunRestorable()
	if err != nil {
		t.Fatalf("RunRestorable failed: %v", err)
	}

	if !result.IsSuccess() {
		t.Fatalf("Batch execution failed: %v", result.GetError())
	}

	// Check that we have 5 operations and 5 reverse operations
	if len(result.GetOperations()) != 5 {
		t.Errorf("Expected 5 operations, got %d", len(result.GetOperations()))
	}

	if len(result.GetRestoreOps()) != 5 {
		t.Errorf("Expected 5 restore operations, got %d", len(result.GetRestoreOps()))
	}

	// Check budget usage
	if result.GetBudget() == nil {
		t.Fatal("Expected budget to be initialized")
	}

	// Should have consumed some budget for the delete operation
	if result.GetBudget().UsedMB <= 0 {
		t.Error("Expected some budget to be used for delete operation backup")
	}

	// Verify that operations have backup data where expected
	hasDeleteBackup := false
	for _, opResult := range result.GetOperations() {
		if opResult.Operation.Describe().Type == "delete" && opResult.BackupData != nil {
			hasDeleteBackup = true
			if opResult.BackupData.BackupType != "file" {
				t.Errorf("Expected delete operation backup type 'file', got '%s'", opResult.BackupData.BackupType)
			}
			if string(opResult.BackupData.BackupContent) != "will be deleted" {
				t.Errorf("Expected backed up content 'will be deleted', got '%s'", string(opResult.BackupData.BackupContent))
			}
		}
	}

	if !hasDeleteBackup {
		t.Error("Expected delete operation to have backup data")
	}

	// Verify filesystem state after operations
	if _, err := fs.Stat("new-dir"); err != nil {
		t.Error("Expected new-dir to exist after operations")
	}

	if _, err := fs.Stat("new-file.txt"); err != nil {
		t.Error("Expected new-file.txt to exist after operations")
	}

	if _, err := fs.Stat("copied.txt"); err != nil {
		t.Error("Expected copied.txt to exist after operations")
	}

	if _, err := fs.Stat("moved.txt"); err != nil {
		t.Error("Expected moved.txt to exist after operations")
	}

	if _, err := fs.Stat("to-move.txt"); err == nil {
		t.Error("Expected to-move.txt to be gone after move operation")
	}

	if _, err := fs.Stat("to-delete.txt"); err == nil {
		t.Error("Expected to-delete.txt to be gone after delete operation")
	}

	// Check that reverse operations are correctly typed
	reverseOpTypes := make(map[string]int)
	for _, restoreOp := range result.GetRestoreOps() {
		reverseOpTypes[restoreOp.Describe().Type]++
	}

	expectedTypes := map[string]int{
		"delete":      3, // For CreateDir, CreateFile, Copy
		"move":        1, // For Move (reverse move)
		"create_file": 1, // For Delete (recreate file)
	}

	for expectedType, expectedCount := range expectedTypes {
		if reverseOpTypes[expectedType] != expectedCount {
			t.Errorf("Expected %d reverse operations of type '%s', got %d",
				expectedCount, expectedType, reverseOpTypes[expectedType])
		}
	}
}

func TestPhase3_BudgetExceeded(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Create a large file that will exceed a small budget
	largeContent := make([]byte, 2*1024*1024) // 2MB
	if err := fs.WriteFile("large.txt", largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	registry := GetDefaultRegistry()
	batch := NewBatch(fs, registry).WithContext(ctx)

	// Add a delete operation for the large file
	_, err := batch.Delete("large.txt")
	if err != nil {
		t.Fatalf("Failed to add Delete: %v", err)
	}

	// Execute with a small budget that should be exceeded
	result, err := batch.RunRestorableWithBudget(1) // 1MB budget
	if err != nil {
		t.Fatalf("RunRestorableWithBudget failed: %v", err)
	}

	// The batch should still succeed, but without backup for the large file
	if !result.IsSuccess() {
		t.Fatalf("Batch execution failed: %v", result.GetError())
	}

	// Check that the operation executed but has no backup
	if len(result.GetOperations()) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(result.GetOperations()))
	}

	opResult := result.GetOperations()[0]
	if opResult.Status != StatusSuccess {
		t.Error("Expected delete operation to succeed even when backup generation fails due to budget")
	}

	// The operation should have executed but without backup data
	if opResult.BackupData != nil {
		t.Error("Expected no backup data when budget is exceeded")
	}

	// Budget should remain mostly unused since backup failed
	if result.GetBudget().UsedMB > 0.1 {
		t.Errorf("Expected minimal budget usage due to backup failure, got %f MB", result.GetBudget().UsedMB)
	}
}

func TestPhase3_MixedOperations_PartialBudget(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Create files of different sizes
	if err := fs.WriteFile("small.txt", []byte("small"), 0644); err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	largeContent := make([]byte, 3*1024*1024) // 3MB
	if err := fs.WriteFile("large.txt", largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	registry := GetDefaultRegistry()
	batch := NewBatch(fs, registry).WithContext(ctx)

	// Add operations in order: small delete first, then large delete
	_, err := batch.Delete("small.txt")
	if err != nil {
		t.Fatalf("Failed to add Delete for small file: %v", err)
	}

	_, err = batch.Delete("large.txt")
	if err != nil {
		t.Fatalf("Failed to add Delete for large file: %v", err)
	}

	// Execute with 2MB budget - enough for small file but not large
	result, err := batch.RunRestorableWithBudget(2)
	if err != nil {
		t.Fatalf("RunRestorableWithBudget failed: %v", err)
	}

	// First operation should succeed with backup, second should succeed without backup
	hasSmallSuccess := false
	hasLargeSuccessNoBackup := false

	for _, opResult := range result.GetOperations() {
		if opResult.Operation.Describe().Path == "small.txt" {
			if opResult.Status == StatusSuccess && opResult.BackupData != nil {
				hasSmallSuccess = true
			}
		}
		if opResult.Operation.Describe().Path == "large.txt" {
			if opResult.Status == StatusSuccess && opResult.BackupData == nil {
				hasLargeSuccessNoBackup = true
			}
		}
	}

	if !hasSmallSuccess {
		t.Error("Expected small file delete to succeed with backup")
	}

	if !hasLargeSuccessNoBackup {
		t.Error("Expected large file delete to succeed without backup due to budget constraint")
	}

	// Check budget usage - should have consumed some for small file
	if result.GetBudget().UsedMB <= 0 {
		t.Error("Expected some budget usage for small file backup")
	}

	if result.GetBudget().UsedMB >= 2.0 {
		t.Error("Expected budget usage to be less than 2MB due to large file failure")
	}
}

// Commented out as TestPhase3_DirectoryRestore_FullContent provides more comprehensive checks.
/*
func TestPhase3_DirectoryDelete(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Create a directory
	if err := fs.MkdirAll("test-dir", 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	registry := GetDefaultRegistry()
	batch := NewBatch(fs, registry).WithContext(ctx)

	// Add delete operation for directory
	_, err := batch.Delete("test-dir")
	if err != nil {
		t.Fatalf("Failed to add Delete for directory: %v", err)
	}

	// Execute with restorable mode
	result, err := batch.RunRestorable()
	if err != nil {
		t.Fatalf("RunRestorable failed: %v", err)
	}

	if !result.IsSuccess() {
		t.Fatalf("Batch execution failed: %v", result.GetError())
	}

	// Check operation result
	if len(result.GetOperations()) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(result.GetOperations()))
	}

	opResult := result.GetOperations()[0]
	if opResult.Status != StatusSuccess {
		t.Errorf("Expected operation to succeed, got status %s", opResult.Status)
	}

	if opResult.BackupData == nil {
		t.Fatal("Expected backup data for directory delete")
	}

	if opResult.BackupData.BackupType != "directory" {
		t.Errorf("Expected backup type 'directory', got '%s'", opResult.BackupData.BackupType)
	}

	// Directory backup should have minimal size
	if opResult.BackupData.SizeMB != 0.01 {
		t.Errorf("Expected directory backup size 0.01 MB, got %f", opResult.BackupData.SizeMB)
	}

	// Should have a reverse operation to recreate the directory
	if len(result.GetRestoreOps()) != 1 {
		t.Errorf("Expected 1 restore operation, got %d", len(result.GetRestoreOps()))
	}

	restoreOp := result.GetRestoreOps()[0]
	if restoreOp.Describe().Type != "create_directory" {
		t.Errorf("Expected restore operation type 'create_directory', got '%s'", restoreOp.Describe().Type)
	}
}
*/

func TestPhase3_OperationFailure_BudgetRestore(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Create an operation that targets a non-existent file to simulate failure
	registry := GetDefaultRegistry()
	opInterface, err := registry.CreateOperation("test-fail", "delete", "nonexistent.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	op := opInterface.(Operation)

	pipeline := NewMemPipeline()
	err = pipeline.Add(op)
	if err != nil {
		t.Fatalf("Failed to add operation to pipeline: %v", err)
	}

	executor := NewExecutor()
	opts := PipelineOptions{
		Restorable:      true,
		MaxBackupSizeMB: 10,
	}

	result := executor.RunWithOptions(ctx, pipeline, fs, opts)

	// With the operations package, delete is idempotent - it succeeds even if file doesn't exist
	// But it should have no backup data since the file doesn't exist
	if !result.IsSuccess() {
		t.Errorf("Expected operation to succeed (idempotent), but got errors: %v", result.GetError())
	}

	if len(result.GetOperations()) != 1 {
		t.Errorf("Expected 1 operation result, got %d", len(result.GetOperations()))
	}

	opResult := result.GetOperations()[0]
	if opResult.Status != StatusSuccess {
		t.Errorf("Expected operation status success (idempotent), got %s", opResult.Status)
	}
	
	// But there should be no backup data since file doesn't exist
	if opResult.BackupData != nil {
		t.Error("Expected no backup data for non-existent file")
	}

	// Budget should remain unused since no backup was created
	if result.GetBudget().UsedMB != 0.0 {
		t.Errorf("Expected budget to remain unused (no file to backup), got %f MB", result.GetBudget().UsedMB)
	}

	if result.GetBudget().RemainingMB != 10.0 {
		t.Errorf("Expected full budget remaining (no file to backup), got %f MB", result.GetBudget().RemainingMB)
	}
}

// This test is now superseded by TestPhase3_DirectoryRestore_FullContent below,
// as the metadata details are part of the full content restoration test.
// Keeping it commented out for reference or if simpler metadata-only check is needed later.
// func TestPhase3_DirectoryDelete_BackupMetadata(t *testing.T) { ... }

func TestPhase3_DirectoryRestore_FullContent(t *testing.T) {
	ctx := context.Background()

	t.Run("Full directory restore with sufficient budget", func(t *testing.T) {
		fs := NewTestFileSystem()
		originalDir := "my_restorable_dir"
		originalFiles := map[string]string{
			filepath.Join(originalDir, "file1.txt"):                "content of file1",
			filepath.Join(originalDir, "sub", "file2.txt"):         "content of file2 in sub",
			filepath.Join(originalDir, "sub", "deep", "file3.txt"): "content of file3 in sub/deep",
		}

		// Setup initial directory structure
		for path, content := range originalFiles {
			dir := filepath.Dir(path)
			if err := fs.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Setup: Failed to create dir %s: %v", dir, err)
			}
			if err := fs.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("Setup: Failed to write file %s: %v", path, err)
			}
		}
		// Create an empty subdirectory as well
		emptySubDir := filepath.Join(originalDir, "empty_sub")
		if err := fs.MkdirAll(emptySubDir, 0755); err != nil {
			t.Fatalf("Setup: Failed to create empty_sub dir %s: %v", emptySubDir, err)
		}

		// 1. Delete the directory in a restorable batch
		registry := GetDefaultRegistry()
		batchDelete := NewBatch(fs, registry).WithContext(ctx)
		_, err := batchDelete.Delete(originalDir)
		if err != nil {
			t.Fatalf("Failed to add Delete op: %v", err)
		}

		resultDelete, err := batchDelete.RunRestorableWithBudget(10) // 10MB budget, should be ample
		if err != nil {
			t.Fatalf("RunRestorableWithBudget for delete failed: %v", err)
		}
		if !resultDelete.IsSuccess() {
			t.Fatalf("Delete batch was not successful: %v", resultDelete.GetError())
		}

		// Verify directory is gone
		if _, err := fs.Stat(originalDir); err == nil {
			t.Fatal("Original directory was not deleted")
		}

		// Verify BackupData for the delete operation
		var deleteOpResult OperationResult
		foundDeleteOp := false
		for _, opRes := range resultDelete.GetOperations() {
			if opRes.Operation.Describe().Type == "delete" && opRes.Operation.Describe().Path == originalDir {
				deleteOpResult = opRes
				foundDeleteOp = true
				break
			}
		}
		if !foundDeleteOp {
			t.Fatal("Delete operation result not found")
		}
		if deleteOpResult.BackupData == nil {
			t.Fatal("BackupData is nil for delete operation")
		}
		if deleteOpResult.BackupData.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", deleteOpResult.BackupData.BackupType)
		}

		// Handle both old format ([]BackedUpItem) and new format ([]interface{})
		var itemCount int
		if backedUpItems, ok := deleteOpResult.BackupData.Metadata["items"].([]BackedUpItem); ok {
			// Old format from SimpleOperation
			itemCount = len(backedUpItems)
		} else if items, ok := deleteOpResult.BackupData.Metadata["items"].([]interface{}); ok {
			// New format from operations package
			itemCount = len(items)
		} else {
			t.Fatal("Metadata items not found or not in expected format")
		}
		
		// Expected items: originalDir (.), file1, sub, sub/file2, sub/deep, sub/deep/file3, empty_sub (7 items)
		if itemCount != 7 {
			t.Errorf("Expected 7 backed up items, got %d", itemCount)
		}

		// 2. Execute the RestoreOps
		if len(resultDelete.RestoreOps) == 0 {
			t.Fatal("No RestoreOps generated")
		}

		// We need to add operations to the batch directly as they are already created
		// The pipeline within batchRestore will handle their execution.
		// For simplicity, we'll use a new pipeline and executor directly.

		restorePipeline := NewMemPipeline()
		err = restorePipeline.Add(resultDelete.RestoreOps...)
		if err != nil {
			t.Fatalf("Failed to add RestoreOps to pipeline: %v", err)
		}

		executor := NewExecutor()
		// Run restore ops without further backup/budgeting for the restore itself
		resultRestore := executor.Run(ctx, restorePipeline, fs)
		if !resultRestore.Success {
			t.Fatalf("Restore pipeline execution failed: %v", resultRestore.Errors)
		}

		// 3. Verify restored structure and content
		for path, expectedContent := range originalFiles {
			content, err := fs.ReadFile(path)
			if err != nil {
				t.Errorf("Failed to read restored file %s: %v", path, err)
				continue
			}
			if string(content) != expectedContent {
				t.Errorf("Content mismatch for %s: expected '%s', got '%s'", path, expectedContent, string(content))
			}
		}
		if _, err := fs.Stat(emptySubDir); err != nil {
			t.Errorf("Empty subdirectory %s not restored: %v", emptySubDir, err)
		}
		if _, err := fs.Stat(originalDir); err != nil {
			t.Errorf("Original directory %s not restored: %v", originalDir, err)
		}
	})

	t.Run("Partial directory restore due to budget exhaustion", func(t *testing.T) {
		fs := NewTestFileSystem()
		originalDir := "my_partial_dir"
		// file1: 8 bytes, file2: 18 bytes
		originalFiles := map[string]string{
			filepath.Join(originalDir, "file1.txt"):        "content1",
			filepath.Join(originalDir, "sub", "file2.txt"): "content2 is longer",
		}
		// Total content size: 8 + 18 = 26 bytes

		// Setup initial directory structure
		for path, content := range originalFiles {
			dir := filepath.Dir(path)
			if err := fs.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Setup: Failed to create dir %s: %v", dir, err)
			}
			if err := fs.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatalf("Setup: Failed to write file %s: %v", path, err)
			}
		}

		batchDelete := NewBatch().WithFileSystem(fs).WithContext(ctx)
		_, err := batchDelete.Delete(originalDir)
		if err != nil {
			t.Fatalf("Failed to add Delete op: %v", err)
		}

		// Budget of 0 MB. Any file content backup will exceed this.
		deleteResult, err := batchDelete.RunWithOptions(PipelineOptions{Restorable: true, MaxBackupSizeMB: 0})
		if err != nil {
			t.Fatalf("Run for delete batch failed: %v", err)
		}

		// The delete operation itself should succeed.
		// The ReverseOps generation within it would have returned an error, which Executor logs.
		if !deleteResult.Success {
			t.Fatalf("Delete operation batch was not successful: %v", deleteResult.Errors)
		}

		// Find the delete op result to check its BackupData
		var deleteOpRes *OperationResult
		for i := range deleteResult.Operations {
			if deleteResult.Operations[i].Operation.Describe().Path == originalDir {
				deleteOpRes = &deleteResult.Operations[i]
				break
			}
		}
		if deleteOpRes == nil {
			t.Fatal("Delete operation result not found")
		}

		// We expect a partial backup.
		// The error from ReverseOps (due to budget) is logged by Executor but doesn't fail the main op.
		// The number of RestoreOps might be less than total items.
		if deleteOpRes.BackupData == nil {
			t.Fatal("BackupData is nil")
		}
		if deleteOpRes.BackupData.SizeMB != 0 { // With 0 budget, no file content should be backed up.
			var items interface{}
			if backedUpItems, ok := deleteOpRes.BackupData.Metadata["items"].([]BackedUpItem); ok {
				items = backedUpItems
			} else {
				items = deleteOpRes.BackupData.Metadata["items"]
			}
			t.Errorf("Expected SizeMB 0 when budget is 0, got %f. Items: %+v", deleteOpRes.BackupData.SizeMB, items)
		}

		// Check RestoreOps: should only contain directory creations
		expectedRestoreOpsCount := 2 // my_partial_dir, my_partial_dir/sub
		if len(deleteResult.RestoreOps) != expectedRestoreOpsCount {
			t.Errorf("Expected %d RestoreOps (only directories), got %d", expectedRestoreOpsCount, len(deleteResult.RestoreOps))
			for idx, rop := range deleteResult.RestoreOps {
				t.Logf("RestoreOp[%d]: %s %s", idx, rop.Describe().Type, rop.Describe().Path)
			}
		}
		for _, ro := range deleteResult.RestoreOps {
			if ro.Describe().Type != "create_directory" {
				t.Errorf("Expected only 'create_directory' RestoreOps, got %s for %s", ro.Describe().Type, ro.Describe().Path)
			}
		}

		// Execute the RestoreOps
		restorePipeline := NewMemPipeline()
		if len(deleteResult.RestoreOps) > 0 {
			err = restorePipeline.Add(deleteResult.RestoreOps...)
			if err != nil {
				t.Fatalf("Failed to add RestoreOps to pipeline: %v", err)
			}
		}

		executor := NewExecutor()
		restoreRunResult := executor.Run(ctx, restorePipeline, fs)
		if !restoreRunResult.Success {
			t.Fatalf("Restore pipeline execution failed: %v", restoreRunResult.Errors)
		}

		// Verify what was restored: only directory structure
		if _, err := fs.Stat(originalDir); err != nil { // Check root dir
			t.Errorf("Original directory %s not restored: %v", originalDir, err)
		}
		if _, err := fs.Stat(filepath.Join(originalDir, "sub")); err != nil { // Check sub dir
			t.Errorf("Subdirectory %s/sub not restored: %v", originalDir, err)
		}

		// Verify files were NOT restored
		_, errFile1 := fs.Stat(filepath.Join(originalDir, "file1.txt"))
		if errFile1 == nil {
			t.Error("Expected file1.txt NOT to be restored due to 0 budget, but it was found.")
		}
		_, errFile2 := fs.Stat(filepath.Join(originalDir, "sub", "file2.txt"))
		if errFile2 == nil {
			t.Error("Expected file2.txt NOT to be restored due to 0 budget, but it was found.")
		}
	})
}

// Commented out as TestPhase3_DirectoryRestore_FullContent provides more comprehensive checks
// including metadata.
// func TestPhase3_DirectoryDelete_BackupMetadata(t *testing.T) {
// 	ctx := context.Background()
// 	fs := NewTestFileSystem()

// 	// Create a directory with a file inside to ensure it's not empty
// 	if err := fs.MkdirAll("my-dir", 0755); err != nil {
// 		t.Fatalf("Failed to create directory: %v", err)
// 	}
// 	if err := fs.WriteFile("my-dir/dummy.txt", []byte("content"), 0644); err != nil {
// 		t.Fatalf("Failed to create file in directory: %v", err)
// 	}

// 	registry := GetDefaultRegistry()
// 	batch := NewBatch(fs, registry).WithContext(ctx)

// 	// Add delete operation for the directory
// 	_, err := batch.Delete("my-dir")
// 	if err != nil {
// 		t.Fatalf("Failed to add Delete for directory: %v", err)
// 	}

// 	// Execute with restorable mode
// 	result, err := batch.RunRestorable()
// 	if err != nil {
// 		t.Fatalf("RunRestorable failed: %v", err)
// 	}

// 	if !result.IsSuccess() {
// 		t.Fatalf("Batch execution failed: %v", result.GetError())
// 	}

// 	// Find the delete operation result
// 	var deleteOpResult *OperationResult
// 	for i := range result.GetOperations() {
// 		if result.GetOperations()[i].Operation.Describe().Type == "delete" && result.GetOperations()[i].Operation.Describe().Path == "my-dir" {
// 			deleteOpResult = &result.GetOperations()[i]
// 			break
// 		}
// 	}

// 	if deleteOpResult == nil {
// 		t.Fatal("Could not find the delete operation result for 'my-dir'")
// 	}

// 	if deleteOpResult.BackupData == nil {
// 		t.Fatal("Expected BackupData for directory delete operation")
// 	}

// 	// This check would change with full directory backup; previously "directory"
// 	// if deleteOpResult.BackupData.BackupType != "directory_tree" {
// 	// 	t.Errorf("Expected BackupData type 'directory_tree', got '%s'", deleteOpResult.BackupData.BackupType)
// 	// }

// 	meta := deleteOpResult.BackupData.Metadata
// 	if meta == nil {
// 		t.Fatal("Expected metadata in BackupData")
// 	}
// 	// These metadata keys were specific to the simplified directory backup
// 	// contentsRestored, ok := meta["contents_restored"].(bool)
// 	// if !ok || contentsRestored {
// 	// 	t.Errorf("Expected metadata 'contents_restored' to be false, got %v (ok: %v)", meta["contents_restored"], ok)
// 	// }
// 	// note, ok := meta["note"].(string)
// 	// expectedNote := "Directory structure restored empty; contents are not backed up."
// 	// if !ok || note != expectedNote {
// 	// 	t.Errorf("Expected metadata 'note' to be '%s', got '%s' (ok: %v)", expectedNote, note, ok)
// 	// }

// 	// Verify the directory is actually gone
// 	if _, err := fs.Stat("my-dir"); err == nil {
// 		t.Error("Expected 'my-dir' to be deleted from filesystem")
// 	}

// 	// Optional: Verify the reverse operation would create an empty directory
// 	// This also changes with full directory content backup
// 	// if len(result.GetRestoreOps()) == 0 {
// 	// 	t.Fatal("Expected restore operations to be generated")
// 	// }
// 	// foundRestoreDirOp := false
// 	// for _, ro := range result.GetRestoreOps() {
// 	// 	if ro.Describe().Type == "create_directory" && ro.Describe().Path == "my-dir" {
// 	// 		foundRestoreDirOp = true
// 	// 		break
// 	// 	}
// 	// }
// 	// if !foundRestoreDirOp {
// 	// 	t.Error("Expected a 'create_directory' restore operation for 'my-dir'")
// 	// }
// }
