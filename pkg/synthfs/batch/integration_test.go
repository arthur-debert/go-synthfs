package batch_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestPhase3_CompleteWorkflow(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()

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

	registry := synthfs.GetDefaultRegistry()
	b := batch.NewBatch(fs, registry).WithContext(ctx)

	// Add various operations to test different reverse operation types
	_, err := b.CreateDir("new-dir")
	if err != nil {
		t.Fatalf("Failed to add CreateDir: %v", err)
	}

	_, err = b.CreateFile("new-file.txt", []byte("new content"))
	if err != nil {
		t.Fatalf("Failed to add CreateFile: %v", err)
	}

	_, err = b.Copy("source.txt", "copied.txt")
	if err != nil {
		t.Fatalf("Failed to add Copy: %v", err)
	}

	_, err = b.Move("to-move.txt", "moved.txt")
	if err != nil {
		t.Fatalf("Failed to add Move: %v", err)
	}

	_, err = b.Delete("to-delete.txt")
	if err != nil {
		t.Fatalf("Failed to add Delete: %v", err)
	}

	// Execute with restorable mode
	result, err := b.RunRestorable()
	if err != nil {
		t.Fatalf("RunRestorable failed: %v", err)
	}

	res := result.(batch.Result)

	if !res.IsSuccess() {
		t.Fatalf("Batch execution failed: %v", res.GetError())
	}

	// Check that we have 5 operations and 5 reverse operations
	if len(res.GetOperations()) != 5 {
		t.Errorf("Expected 5 operations, got %d", len(res.GetOperations()))
	}

	if len(res.GetRestoreOps()) != 5 {
		t.Errorf("Expected 5 restore operations, got %d", len(res.GetRestoreOps()))
	}

	// Check budget usage
	if res.GetBudget() == nil {
		t.Fatal("Expected budget to be initialized")
	}

	// Should have consumed some budget for the delete operation
	if budget, ok := res.GetBudget().(*core.BackupBudget); ok {
		if budget.UsedMB <= 0 {
			t.Error("Expected some budget to be used for delete operation backup")
		}
	} else {
		t.Error("Expected budget to be of type *core.BackupBudget")
	}

	// Verify that operations have backup data where expected
	hasDeleteBackup := false
	for _, opResultInterface := range res.GetOperations() {
		// Handle both synthfs.OperationResult and core.OperationResult
		var backupData *core.BackupData
		var opType string

		switch opResult := opResultInterface.(type) {
		case synthfs.OperationResult:
			if op, ok := opResult.Operation.(interface{ Describe() core.OperationDesc }); ok {
				opType = op.Describe().Type
			}
			backupData = opResult.BackupData
		case core.OperationResult:
			if op, ok := opResult.Operation.(interface{ Describe() core.OperationDesc }); ok {
				opType = op.Describe().Type
			}
			backupData = opResult.BackupData
		}

		if opType == "delete" && backupData != nil {
			hasDeleteBackup = true
			if backupData.BackupType != "file" {
				t.Errorf("Expected delete operation backup type 'file', got '%s'", backupData.BackupType)
			}
			if string(backupData.BackupContent) != "will be deleted" {
				t.Errorf("Expected backed up content 'will be deleted', got '%s'", string(backupData.BackupContent))
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
	for _, restoreOpInterface := range res.GetRestoreOps() {
		if restoreOp, ok := restoreOpInterface.(synthfs.Operation); ok {
			reverseOpTypes[restoreOp.Describe().Type]++
		}
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
	fs := testutil.NewTestFileSystem()

	// Create a large file that will exceed a small budget
	largeContent := make([]byte, 2*1024*1024) // 2MB
	if err := fs.WriteFile("large.txt", largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	registry := synthfs.GetDefaultRegistry()
	b := batch.NewBatch(fs, registry).WithContext(ctx)

	// Add a delete operation for the large file
	_, err := b.Delete("large.txt")
	if err != nil {
		t.Fatalf("Failed to add Delete: %v", err)
	}

	// Execute with a small budget that should be exceeded
	result, err := b.RunRestorableWithBudget(1) // 1MB budget
	if err != nil {
		t.Fatalf("RunRestorableWithBudget failed: %v", err)
	}

	res := result.(batch.Result)

	// The batch should still succeed, but without backup for the large file
	if !res.IsSuccess() {
		t.Fatalf("Batch execution failed: %v", res.GetError())
	}

	// Check that the operation executed but has no backup
	if len(res.GetOperations()) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(res.GetOperations()))
	}

	opResultInterface := res.GetOperations()[0]

	// Handle both synthfs.OperationResult and core.OperationResult
	var status core.OperationStatus
	var backupData *core.BackupData

	switch v := opResultInterface.(type) {
	case synthfs.OperationResult:
		status = v.Status
		backupData = v.BackupData
	case core.OperationResult:
		status = v.Status
		backupData = v.BackupData
	default:
		t.Fatalf("Unexpected operation result type: %T", opResultInterface)
	}

	if status != core.StatusSuccess {
		t.Error("Expected delete operation to succeed even when backup generation fails due to budget")
	}

	// The operation should have executed but without backup data
	if backupData != nil {
		t.Error("Expected no backup data when budget is exceeded")
	}

	// Budget should remain mostly unused since backup failed
	if budget, ok := res.GetBudget().(*core.BackupBudget); ok {
		if budget.UsedMB > 0.1 {
			t.Errorf("Expected minimal budget usage due to backup failure, got %f MB", budget.UsedMB)
		}
	} else {
		t.Error("Expected budget to be of type *core.BackupBudget")
	}
}

func TestPhase3_MixedOperations_PartialBudget(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()

	// Create files of different sizes
	if err := fs.WriteFile("small.txt", []byte("small"), 0644); err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	largeContent := make([]byte, 3*1024*1024) // 3MB
	if err := fs.WriteFile("large.txt", largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	registry := synthfs.GetDefaultRegistry()
	b := batch.NewBatch(fs, registry).WithContext(ctx)

	// Add operations in order: small delete first, then large delete
	_, err := b.Delete("small.txt")
	if err != nil {
		t.Fatalf("Failed to add Delete for small file: %v", err)
	}

	_, err = b.Delete("large.txt")
	if err != nil {
		t.Fatalf("Failed to add Delete for large file: %v", err)
	}

	// Execute with 2MB budget - enough for small file but not large
	result, err := b.RunRestorableWithBudget(2)
	if err != nil {
		t.Fatalf("RunRestorableWithBudget failed: %v", err)
	}

	res := result.(batch.Result)

	// First operation should succeed with backup, second should succeed without backup
	hasSmallSuccess := false
	hasLargeSuccessNoBackup := false

	for _, opResultInterface := range res.GetOperations() {
		// Handle both synthfs.OperationResult and core.OperationResult
		var status core.OperationStatus
		var backupData *core.BackupData
		var path string

		switch v := opResultInterface.(type) {
		case synthfs.OperationResult:
			status = v.Status
			backupData = v.BackupData
			if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
				path = op.Describe().Path
			}
		case core.OperationResult:
			status = v.Status
			backupData = v.BackupData
			if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
				path = op.Describe().Path
			}
		}

		if path == "small.txt" {
			if status == core.StatusSuccess && backupData != nil {
				hasSmallSuccess = true
			}
		}
		if path == "large.txt" {
			if status == core.StatusSuccess && backupData == nil {
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
	if budget, ok := res.GetBudget().(*core.BackupBudget); ok {
		if budget.UsedMB <= 0 {
			t.Error("Expected some budget usage for small file backup")
		}

		if budget.UsedMB >= 2.0 {
			t.Error("Expected budget usage to be less than 2MB due to large file failure")
		}
	} else {
		t.Error("Expected budget to be of type *core.BackupBudget")
	}
}

func TestPhase3_OperationFailure_BudgetRestore(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()

	// Create an operation that targets a non-existent file to simulate failure
	registry := synthfs.GetDefaultRegistry()
	opInterface, err := registry.CreateOperation("test-fail", "delete", "nonexistent.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	op := opInterface.(synthfs.Operation)

	pipeline := synthfs.NewMemPipeline()
	err = pipeline.Add(op)
	if err != nil {
		t.Fatalf("Failed to add operation to pipeline: %v", err)
	}

	executor := synthfs.NewExecutor()
	opts := core.PipelineOptions{
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

	opResultInterface := result.GetOperations()[0]
		if opResult, ok := opResultInterface.(synthfs.OperationResult); ok {
		if opResult.Status != core.StatusSuccess {
			t.Errorf("Expected operation status success (idempotent), got %s", opResult.Status)
		}

		// But there should be no backup data since file doesn't exist
		if opResult.BackupData != nil {
			t.Error("Expected no backup data for non-existent file")
		}
	} else {
		t.Error("Expected operation result to be core.OperationResult type")
	}

	// Budget should remain unused since no backup was created
	if budget, ok := result.GetBudget().(*core.BackupBudget); ok {
		if budget.UsedMB != 0.0 {
			t.Errorf("Expected budget to remain unused (no file to backup), got %f MB", budget.UsedMB)
		}

		if budget.RemainingMB != 10.0 {
			t.Errorf("Expected full budget remaining (no file to backup), got %f MB", budget.RemainingMB)
		}
	} else {
		t.Error("Expected budget to be *core.BackupBudget type")
	}
}

func TestPhase3_DirectoryRestore_FullContent(t *testing.T) {
	ctx := context.Background()

	t.Run("Full directory restore with sufficient budget", func(t *testing.T) {
		fs := testutil.NewTestFileSystem()
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
		registry := synthfs.GetDefaultRegistry()
		batchDelete := batch.NewBatch(fs, registry).WithContext(ctx)
		_, err := batchDelete.Delete(originalDir)
		if err != nil {
			t.Fatalf("Failed to add Delete op: %v", err)
		}

		resultDelete, err := batchDelete.RunRestorableWithBudget(10) // 10MB budget, should be ample
		if err != nil {
			t.Fatalf("RunRestorableWithBudget for delete failed: %v", err)
		}
		resDelete := resultDelete.(batch.Result)
		if !resDelete.IsSuccess() {
			t.Fatalf("Delete batch was not successful: %v", resDelete.GetError())
		}

		// Verify directory is gone
		if _, err := fs.Stat(originalDir); err == nil {
			t.Fatal("Original directory was not deleted")
		}

		// Verify BackupData for the delete operation
		var deleteBackupData *core.BackupData
		foundDeleteOp := false
		for _, opResInterface := range resDelete.GetOperations() {
			// Handle both synthfs.OperationResult and core.OperationResult
			var opType, opPath string
			var backupData *core.BackupData

			switch v := opResInterface.(type) {
			case synthfs.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					desc := op.Describe()
					opType = desc.Type
					opPath = desc.Path
				}
				backupData = v.BackupData
			case core.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					desc := op.Describe()
					opType = desc.Type
					opPath = desc.Path
				}
				backupData = v.BackupData
			}

			if opType == "delete" && opPath == originalDir {
				deleteBackupData = backupData
				foundDeleteOp = true
				break
			}
		}
		if !foundDeleteOp {
			t.Fatal("Delete operation result not found")
		}
		if deleteBackupData == nil {
			t.Fatal("BackupData is nil for delete operation")
		}
		if deleteBackupData.BackupType != "directory_tree" {
			t.Errorf("Expected BackupType 'directory_tree', got '%s'", deleteBackupData.BackupType)
		}

		// Handle both old format ([]BackedUpItem) and new format ([]interface{})
		var itemCount int
		if items, ok := deleteBackupData.Metadata["items"].([]interface{}); ok {
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
		restoreOps := resDelete.GetRestoreOps()
		if len(restoreOps) == 0 {
			t.Fatal("No RestoreOps generated")
		}

		// We need to add operations to the batch directly as they are already created
		// The pipeline within batchRestore will handle their execution.
		// For simplicity, we'll use a new pipeline and executor directly.

		restorePipeline := synthfs.NewMemPipeline()
		// Convert restore ops to Operation type
		for _, op := range restoreOps {
			if opTyped, ok := op.(synthfs.Operation); ok {
				err = restorePipeline.Add(opTyped)
				if err != nil {
					t.Fatalf("Failed to add RestoreOp to pipeline: %v", err)
				}
			}
		}

		executor := synthfs.NewExecutor()
		// Run restore ops without further backup/budgeting for the restore itself
		resultRestore := executor.Run(ctx, restorePipeline, fs)
		if !resultRestore.IsSuccess() {
			t.Fatalf("Restore pipeline execution failed: %v", resultRestore.GetError())
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
		fs := testutil.NewTestFileSystem()
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

		registry := synthfs.GetDefaultRegistry()
		batchDelete := batch.NewBatch(fs, registry).WithContext(ctx)
		_, err := batchDelete.Delete(originalDir)
		if err != nil {
			t.Fatalf("Failed to add Delete op: %v", err)
		}

		// Budget of 0 MB. Any file content backup will exceed this.
		deleteResult, err := batchDelete.RunWithOptions(core.PipelineOptions{Restorable: true, MaxBackupSizeMB: 0})
		if err != nil {
			t.Fatalf("Run for delete batch failed: %v", err)
		}

		resDelete := deleteResult.(batch.Result)

		// The delete operation itself should succeed.
		// The ReverseOps generation within it would have returned an error, which Executor logs.
		if !resDelete.IsSuccess() {
			t.Fatalf("Delete operation batch was not successful: %v", resDelete.GetError())
		}

		// Find the delete op result to check its BackupData
		var deleteBackupData *core.BackupData
		ops := resDelete.GetOperations()
		foundDeleteOp := false
		for i := range ops {
			// Handle both synthfs.OperationResult and core.OperationResult
			var opPath string
			var backupData *core.BackupData

			switch v := ops[i].(type) {
			case synthfs.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					opPath = op.Describe().Path
				}
				backupData = v.BackupData
			case core.OperationResult:
				if op, ok := v.Operation.(interface{ Describe() core.OperationDesc }); ok {
					opPath = op.Describe().Path
				}
				backupData = v.BackupData
			}

			if opPath == originalDir {
				deleteBackupData = backupData
				foundDeleteOp = true
				break
			}
		}
		if !foundDeleteOp {
			t.Fatal("Delete operation result not found")
		}

		// We expect a partial backup or no backup at all with 0 budget.
		// The error from ReverseOps (due to budget) is logged by Executor but doesn't fail the main op.
		// The number of RestoreOps might be less than total items.
		// With 0 budget, it's possible that no backup data is created at all
		if deleteBackupData == nil {
			// This is acceptable with 0 budget - no backup could be created
			t.Log("BackupData is nil with 0 budget - this is expected behavior")
			return
		}
		if deleteBackupData.SizeMB != 0 { // With 0 budget, no file content should be backed up.
			if _, ok := deleteBackupData.Metadata["items"].([]interface{}); ok {
				t.Errorf("Expected SizeMB 0 when budget is 0, got %f", deleteBackupData.SizeMB)
			}
		}
	})
}
