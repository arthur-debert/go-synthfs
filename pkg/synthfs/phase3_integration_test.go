package synthfs

import (
	"context"
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

	batch := NewBatch().WithFileSystem(fs).WithContext(ctx)

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

	if !result.Success {
		t.Fatalf("Batch execution failed: %v", result.Errors)
	}

	// Check that we have 5 operations and 5 reverse operations
	if len(result.Operations) != 5 {
		t.Errorf("Expected 5 operations, got %d", len(result.Operations))
	}

	if len(result.RestoreOps) != 5 {
		t.Errorf("Expected 5 restore operations, got %d", len(result.RestoreOps))
	}

	// Check budget usage
	if result.Budget == nil {
		t.Fatal("Expected budget to be initialized")
	}

	// Should have consumed some budget for the delete operation
	if result.Budget.UsedMB <= 0 {
		t.Error("Expected some budget to be used for delete operation backup")
	}

	// Verify that operations have backup data where expected
	hasDeleteBackup := false
	for _, opResult := range result.Operations {
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
	for _, restoreOp := range result.RestoreOps {
		reverseOpTypes[restoreOp.Describe().Type]++
	}

	expectedTypes := map[string]int{
		"delete":           3, // For CreateDir, CreateFile, Copy
		"move":             1, // For Move (reverse move)
		"create_file":      1, // For Delete (recreate file)
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

	batch := NewBatch().WithFileSystem(fs).WithContext(ctx)

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
	if !result.Success {
		t.Fatalf("Batch execution failed: %v", result.Errors)
	}

	// Check that the operation executed but has no backup
	if len(result.Operations) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(result.Operations))
	}

	opResult := result.Operations[0]
	if opResult.Status != StatusSuccess {
		t.Error("Expected delete operation to succeed even when backup generation fails due to budget")
	}

	// The operation should have executed but without backup data
	if opResult.BackupData != nil {
		t.Error("Expected no backup data when budget is exceeded")
	}

	// Budget should remain mostly unused since backup failed
	if result.Budget.UsedMB > 0.1 {
		t.Errorf("Expected minimal budget usage due to backup failure, got %f MB", result.Budget.UsedMB)
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

	batch := NewBatch().WithFileSystem(fs).WithContext(ctx)

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

	for _, opResult := range result.Operations {
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
	if result.Budget.UsedMB <= 0 {
		t.Error("Expected some budget usage for small file backup")
	}

	if result.Budget.UsedMB >= 2.0 {
		t.Error("Expected budget usage to be less than 2MB due to large file failure")
	}
}

func TestPhase3_DirectoryDelete(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Create a directory
	if err := fs.MkdirAll("test-dir", 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	batch := NewBatch().WithFileSystem(fs).WithContext(ctx)

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

	if !result.Success {
		t.Fatalf("Batch execution failed: %v", result.Errors)
	}

	// Check operation result
	if len(result.Operations) != 1 {
		t.Errorf("Expected 1 operation, got %d", len(result.Operations))
	}

	opResult := result.Operations[0]
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
	if len(result.RestoreOps) != 1 {
		t.Errorf("Expected 1 restore operation, got %d", len(result.RestoreOps))
	}

	restoreOp := result.RestoreOps[0]
	if restoreOp.Describe().Type != "create_directory" {
		t.Errorf("Expected restore operation type 'create_directory', got '%s'", restoreOp.Describe().Type)
	}
}

func TestPhase3_OperationFailure_BudgetRestore(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Create an operation that targets a non-existent file to simulate failure
	op := NewSimpleOperation("test-fail", "delete", "nonexistent.txt")

	pipeline := NewMemPipeline()
	err := pipeline.Add(op)
	if err != nil {
		t.Fatalf("Failed to add operation to pipeline: %v", err)
	}

	executor := NewExecutor()
	opts := PipelineOptions{
		Restorable:      true,
		MaxBackupSizeMB: 10,
	}

	result := executor.RunWithOptions(ctx, pipeline, fs, opts)

	// The operation should fail
	if result.Success {
		t.Error("Expected operation to fail for non-existent file")
	}

	if len(result.Operations) != 1 {
		t.Errorf("Expected 1 operation result, got %d", len(result.Operations))
	}

	opResult := result.Operations[0]
	if opResult.Status != StatusFailure {
		t.Errorf("Expected operation status failure, got %s", opResult.Status)
	}

	// Budget should remain unused since the operation failed
	if result.Budget.UsedMB != 0.0 {
		t.Errorf("Expected budget to remain unused on operation failure, got %f MB", result.Budget.UsedMB)
	}

	if result.Budget.RemainingMB != 10.0 {
		t.Errorf("Expected full budget remaining on operation failure, got %f MB", result.Budget.RemainingMB)
	}
}