package synthfs

import (
	"context"
	"testing"
	"time"
)

func TestBackupBudget(t *testing.T) {
	t.Run("Budget initialization", func(t *testing.T) {
		budget := &BackupBudget{
			TotalMB:     10.0,
			RemainingMB: 10.0,
			UsedMB:      0.0,
		}

		if budget.TotalMB != 10.0 {
			t.Errorf("Expected TotalMB 10.0, got %f", budget.TotalMB)
		}
		if budget.RemainingMB != 10.0 {
			t.Errorf("Expected RemainingMB 10.0, got %f", budget.RemainingMB)
		}
		if budget.UsedMB != 0.0 {
			t.Errorf("Expected UsedMB 0.0, got %f", budget.UsedMB)
		}
	})

	t.Run("ConsumeBackup success", func(t *testing.T) {
		budget := &BackupBudget{
			TotalMB:     10.0,
			RemainingMB: 10.0,
			UsedMB:      0.0,
		}

		err := budget.ConsumeBackup(3.5)
		if err != nil {
			t.Fatalf("ConsumeBackup failed: %v", err)
		}

		if budget.RemainingMB != 6.5 {
			t.Errorf("Expected RemainingMB 6.5, got %f", budget.RemainingMB)
		}
		if budget.UsedMB != 3.5 {
			t.Errorf("Expected UsedMB 3.5, got %f", budget.UsedMB)
		}
	})

	t.Run("ConsumeBackup exceeds budget", func(t *testing.T) {
		budget := &BackupBudget{
			TotalMB:     10.0,
			RemainingMB: 5.0,
			UsedMB:      5.0,
		}

		err := budget.ConsumeBackup(6.0)
		if err == nil {
			t.Fatal("Expected ConsumeBackup to fail when exceeding budget")
		}

		// Budget should remain unchanged
		if budget.RemainingMB != 5.0 {
			t.Errorf("Expected RemainingMB unchanged at 5.0, got %f", budget.RemainingMB)
		}
		if budget.UsedMB != 5.0 {
			t.Errorf("Expected UsedMB unchanged at 5.0, got %f", budget.UsedMB)
		}
	})

	t.Run("RestoreBackup", func(t *testing.T) {
		budget := &BackupBudget{
			TotalMB:     10.0,
			RemainingMB: 3.0,
			UsedMB:      7.0,
		}

		budget.RestoreBackup(2.0)

		if budget.RemainingMB != 5.0 {
			t.Errorf("Expected RemainingMB 5.0, got %f", budget.RemainingMB)
		}
		if budget.UsedMB != 5.0 {
			t.Errorf("Expected UsedMB 5.0, got %f", budget.UsedMB)
		}
	})

	t.Run("RestoreBackup doesn't exceed total", func(t *testing.T) {
		budget := &BackupBudget{
			TotalMB:     10.0,
			RemainingMB: 8.0,
			UsedMB:      2.0,
		}

		budget.RestoreBackup(5.0)

		if budget.RemainingMB != 10.0 {
			t.Errorf("Expected RemainingMB capped at 10.0, got %f", budget.RemainingMB)
		}
		if budget.UsedMB != 0.0 {
			t.Errorf("Expected UsedMB to be 0.0, got %f", budget.UsedMB)
		}
	})
}

func TestPipelineOptions(t *testing.T) {
	t.Run("DefaultPipelineOptions", func(t *testing.T) {
		opts := DefaultPipelineOptions()

		if opts.Restorable != false {
			t.Errorf("Expected Restorable false, got %v", opts.Restorable)
		}
		if opts.MaxBackupSizeMB != 10 {
			t.Errorf("Expected MaxBackupSizeMB 10, got %d", opts.MaxBackupSizeMB)
		}
	})

	t.Run("Custom PipelineOptions", func(t *testing.T) {
		opts := PipelineOptions{
			Restorable:      true,
			MaxBackupSizeMB: 25,
		}

		if opts.Restorable != true {
			t.Errorf("Expected Restorable true, got %v", opts.Restorable)
		}
		if opts.MaxBackupSizeMB != 25 {
			t.Errorf("Expected MaxBackupSizeMB 25, got %d", opts.MaxBackupSizeMB)
		}
	})
}

func TestBackupData(t *testing.T) {
	t.Run("BackupData creation", func(t *testing.T) {
		backup := &BackupData{
			OperationID:   "test-op-1",
			BackupType:    "file",
			OriginalPath:  "test.txt",
			BackupContent: []byte("test content"),
			BackupTime:    time.Now(),
			SizeMB:        0.001,
			Metadata:      map[string]interface{}{"test": "value"},
		}

		if backup.OperationID != "test-op-1" {
			t.Errorf("Expected OperationID 'test-op-1', got '%s'", backup.OperationID)
		}
		if backup.BackupType != "file" {
			t.Errorf("Expected BackupType 'file', got '%s'", backup.BackupType)
		}
		if backup.OriginalPath != "test.txt" {
			t.Errorf("Expected OriginalPath 'test.txt', got '%s'", backup.OriginalPath)
		}
		if string(backup.BackupContent) != "test content" {
			t.Errorf("Expected BackupContent 'test content', got '%s'", string(backup.BackupContent))
		}
	})
}

func TestReverseOperations_CreateFile(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	op := NewSimpleOperation("test-create-file", "create_file", "test.txt")
	budget := &BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
	if err != nil {
		t.Fatalf("ReverseOps failed: %v", err)
	}

	if len(reverseOps) != 1 {
		t.Fatalf("Expected 1 reverse operation, got %d", len(reverseOps))
	}

	if backupData == nil {
		t.Fatal("Expected backup data to be created")
	}

	reverseOp := reverseOps[0]
	if reverseOp.Describe().Type != "delete" {
		t.Errorf("Expected reverse operation type 'delete', got '%s'", reverseOp.Describe().Type)
	}
	if reverseOp.Describe().Path != "test.txt" {
		t.Errorf("Expected reverse operation path 'test.txt', got '%s'", reverseOp.Describe().Path)
	}

	if backupData.BackupType != "none" {
		t.Errorf("Expected backup type 'none', got '%s'", backupData.BackupType)
	}
	if backupData.SizeMB != 0 {
		t.Errorf("Expected backup size 0, got %f", backupData.SizeMB)
	}
}

func TestReverseOperations_Delete(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Create a test file first
	content := []byte("test file content")
	err := fs.WriteFile("test.txt", content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	op := NewSimpleOperation("test-delete", "delete", "test.txt")
	budget := &BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
	if err != nil {
		t.Fatalf("ReverseOps failed: %v", err)
	}

	if len(reverseOps) != 1 {
		t.Fatalf("Expected 1 reverse operation, got %d", len(reverseOps))
	}

	if backupData == nil {
		t.Fatal("Expected backup data to be created")
	}

	reverseOp := reverseOps[0]
	if reverseOp.Describe().Type != "create_file" {
		t.Errorf("Expected reverse operation type 'create_file', got '%s'", reverseOp.Describe().Type)
	}
	if reverseOp.Describe().Path != "test.txt" {
		t.Errorf("Expected reverse operation path 'test.txt', got '%s'", reverseOp.Describe().Path)
	}

	if backupData.BackupType != "file" {
		t.Errorf("Expected backup type 'file', got '%s'", backupData.BackupType)
	}
	if string(backupData.BackupContent) != string(content) {
		t.Errorf("Expected backup content '%s', got '%s'", string(content), string(backupData.BackupContent))
	}

	// Check budget consumption
	expectedSizeMB := float64(len(content)) / (1024 * 1024)
	if backupData.SizeMB != expectedSizeMB {
		t.Errorf("Expected backup size %f MB, got %f MB", expectedSizeMB, backupData.SizeMB)
	}

	if budget.UsedMB != expectedSizeMB {
		t.Errorf("Expected budget used %f MB, got %f MB", expectedSizeMB, budget.UsedMB)
	}
}

func TestReverseOperations_DeleteBudgetExceeded(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	// Create a large test file that exceeds budget
	largeContent := make([]byte, 11*1024*1024) // 11MB
	err := fs.WriteFile("large.txt", largeContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	op := NewSimpleOperation("test-delete", "delete", "large.txt")
	budget := &BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	_, _, err = op.ReverseOps(ctx, fs, budget)
	if err == nil {
		t.Fatal("Expected ReverseOps to fail due to budget exceeded")
	}

	// Budget should remain unchanged
	if budget.UsedMB != 0.0 {
		t.Errorf("Expected budget used to remain 0, got %f", budget.UsedMB)
	}
	if budget.RemainingMB != 10.0 {
		t.Errorf("Expected remaining budget to remain 10.0, got %f", budget.RemainingMB)
	}
}

func TestReverseOperations_Move(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	op := NewSimpleOperation("test-move", "move", "src.txt")
	op.SetPaths("src.txt", "dst.txt")
	budget := &BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
	if err != nil {
		t.Fatalf("ReverseOps failed: %v", err)
	}

	if len(reverseOps) != 1 {
		t.Fatalf("Expected 1 reverse operation, got %d", len(reverseOps))
	}

	reverseOp := reverseOps[0]
	if reverseOp.Describe().Type != "move" {
		t.Errorf("Expected reverse operation type 'move', got '%s'", reverseOp.Describe().Type)
	}

	// Check that paths are reversed
	simpleReverseOp := reverseOp.(*SimpleOperation)
	if simpleReverseOp.GetSrcPath() != "dst.txt" {
		t.Errorf("Expected reverse src path 'dst.txt', got '%s'", simpleReverseOp.GetSrcPath())
	}
	if simpleReverseOp.GetDstPath() != "src.txt" {
		t.Errorf("Expected reverse dst path 'src.txt', got '%s'", simpleReverseOp.GetDstPath())
	}

	if backupData.BackupType != "none" {
		t.Errorf("Expected backup type 'none' for move operation, got '%s'", backupData.BackupType)
	}
}

func TestReverseOperations_Copy(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	op := NewSimpleOperation("test-copy", "copy", "src.txt")
	op.SetPaths("src.txt", "dst.txt")
	budget := &BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	reverseOps, backupData, err := op.ReverseOps(ctx, fs, budget)
	if err != nil {
		t.Fatalf("ReverseOps failed: %v", err)
	}

	if len(reverseOps) != 1 {
		t.Fatalf("Expected 1 reverse operation, got %d", len(reverseOps))
	}

	reverseOp := reverseOps[0]
	if reverseOp.Describe().Type != "delete" {
		t.Errorf("Expected reverse operation type 'delete', got '%s'", reverseOp.Describe().Type)
	}
	if reverseOp.Describe().Path != "dst.txt" {
		t.Errorf("Expected reverse operation path 'dst.txt', got '%s'", reverseOp.Describe().Path)
	}

	if backupData.BackupType != "none" {
		t.Errorf("Expected backup type 'none' for copy operation, got '%s'", backupData.BackupType)
	}
}

func TestExecutor_RunWithOptions_Basic(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()
	executor := NewExecutor()
	pipeline := NewMemPipeline()

	// Create a simple operation
	op := NewSimpleOperation("test", "create_file", "test.txt")
	fileItem := NewFile("test.txt").WithContent([]byte("content"))
	op.SetItem(fileItem)

	err := pipeline.Add(op)
	if err != nil {
		t.Fatalf("Failed to add operation to pipeline: %v", err)
	}

	// Test with default options
	opts := DefaultPipelineOptions()
	result := executor.RunWithOptions(ctx, pipeline, fs, opts)

	if !result.Success {
		t.Errorf("Expected success, got errors: %v", result.Errors)
	}

	if len(result.Operations) != 1 {
		t.Errorf("Expected 1 operation result, got %d", len(result.Operations))
	}

	if result.Budget != nil {
		t.Error("Expected budget to be nil when restorable=false")
	}

	if len(result.RestoreOps) != 0 {
		t.Errorf("Expected 0 restore operations when restorable=false, got %d", len(result.RestoreOps))
	}
}

func TestExecutor_RunWithOptions_Restorable(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()
	executor := NewExecutor()
	pipeline := NewMemPipeline()

	// Create a simple operation
	op := NewSimpleOperation("test", "create_file", "test.txt")
	fileItem := NewFile("test.txt").WithContent([]byte("content"))
	op.SetItem(fileItem)

	err := pipeline.Add(op)
	if err != nil {
		t.Fatalf("Failed to add operation to pipeline: %v", err)
	}

	// Test with restorable options
	opts := PipelineOptions{
		Restorable:      true,
		MaxBackupSizeMB: 5,
	}
	result := executor.RunWithOptions(ctx, pipeline, fs, opts)

	if !result.Success {
		t.Errorf("Expected success, got errors: %v", result.Errors)
	}

	if len(result.Operations) != 1 {
		t.Errorf("Expected 1 operation result, got %d", len(result.Operations))
	}

	if result.Budget == nil {
		t.Fatal("Expected budget to be initialized when restorable=true")
	}

	if result.Budget.TotalMB != 5.0 {
		t.Errorf("Expected budget total 5.0 MB, got %f", result.Budget.TotalMB)
	}

	if len(result.RestoreOps) != 1 {
		t.Errorf("Expected 1 restore operation when restorable=true, got %d", len(result.RestoreOps))
	}

	// Check operation result has backup data
	opResult := result.Operations[0]
	if opResult.BackupData == nil {
		t.Error("Expected backup data in operation result when restorable=true")
	}
}

func TestBatch_RunRestorable(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()
	batch := NewBatch().WithFileSystem(fs).WithContext(ctx)

	// Add a simple operation
	_, err := batch.CreateFile("test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Failed to add operation to batch: %v", err)
	}

	// Test restorable execution
	result, err := batch.RunRestorable()
	if err != nil {
		t.Fatalf("RunRestorable failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got errors: %v", result.Errors)
	}

	if result.Budget == nil {
		t.Error("Expected budget to be initialized in restorable mode")
	}

	if result.Budget.TotalMB != 10.0 {
		t.Errorf("Expected default budget 10.0 MB, got %f", result.Budget.TotalMB)
	}

	if len(result.RestoreOps) != 1 {
		t.Errorf("Expected 1 restore operation, got %d", len(result.RestoreOps))
	}
}

func TestBatch_RunRestorableWithBudget(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()
	batch := NewBatch().WithFileSystem(fs).WithContext(ctx)

	// Add a simple operation
	_, err := batch.CreateFile("test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Failed to add operation to batch: %v", err)
	}

	// Test restorable execution with custom budget
	result, err := batch.RunRestorableWithBudget(25)
	if err != nil {
		t.Fatalf("RunRestorableWithBudget failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got errors: %v", result.Errors)
	}

	if result.Budget == nil {
		t.Error("Expected budget to be initialized in restorable mode")
	}

	if result.Budget.TotalMB != 25.0 {
		t.Errorf("Expected custom budget 25.0 MB, got %f", result.Budget.TotalMB)
	}
}
