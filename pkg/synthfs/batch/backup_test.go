package batch_test

import (
	"context"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestBackupBudget(t *testing.T) {
	t.Run("Budget initialization", func(t *testing.T) {
		budget := &core.BackupBudget{
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
		budget := &core.BackupBudget{
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
		budget := &core.BackupBudget{
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
		budget := &core.BackupBudget{
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
		budget := &core.BackupBudget{
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
		opts := synthfs.DefaultPipelineOptions()

		if opts.Restorable != false {
			t.Errorf("Expected Restorable false, got %v", opts.Restorable)
		}
		if opts.MaxBackupSizeMB != 10 {
			t.Errorf("Expected MaxBackupSizeMB 10, got %d", opts.MaxBackupSizeMB)
		}
	})

	t.Run("Custom PipelineOptions", func(t *testing.T) {
		opts := core.PipelineOptions{
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
		backup := &core.BackupData{
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

func TestBatch_RunRestorable(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()
	registry := synthfs.GetDefaultRegistry()
	b := batch.NewBatch(fs, registry).WithFileSystem(fs).WithContext(ctx)

	// Add a simple operation
	_, err := b.CreateFile("test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Failed to add operation to batch: %v", err)
	}

	// Test restorable execution
	result, err := b.RunRestorable()
	if err != nil {
		t.Fatalf("RunRestorable failed: %v", err)
	}

	res := result.(batch.Result)

	if !res.IsSuccess() {
		t.Errorf("Expected success, got errors: %v", res.GetError())
	}

	if res.GetBudget() == nil {
		t.Error("Expected budget to be initialized in restorable mode")
	}

	if budget, ok := res.GetBudget().(*core.BackupBudget); ok {
		if budget.TotalMB != 10.0 {
			t.Errorf("Expected default budget 10.0 MB, got %f", budget.TotalMB)
		}
	} else {
		t.Error("Expected budget to be *core.BackupBudget type")
	}

	if len(res.GetRestoreOps()) != 1 {
		t.Errorf("Expected 1 restore operation, got %d", len(res.GetRestoreOps()))
	}
}

func TestBatch_RunRestorableWithBudget(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()
	registry := synthfs.GetDefaultRegistry()
	b := batch.NewBatch(fs, registry).WithFileSystem(fs).WithContext(ctx)

	// Add a simple operation
	_, err := b.CreateFile("test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("Failed to add operation to batch: %v", err)
	}

	// Test restorable execution with custom budget
	result, err := b.RunRestorableWithBudget(25)
	if err != nil {
		t.Fatalf("RunRestorableWithBudget failed: %v", err)
	}

	res := result.(batch.Result)

	if !res.IsSuccess() {
		t.Errorf("Expected success, got errors: %v", res.GetError())
	}

	if res.GetBudget() == nil {
		t.Error("Expected budget to be initialized in restorable mode")
	}

	if budget, ok := res.GetBudget().(*core.BackupBudget); ok {
		if budget.TotalMB != 25.0 {
			t.Errorf("Expected custom budget 25.0 MB, got %f", budget.TotalMB)
		}
	} else {
		t.Error("Expected budget to be of type *core.BackupBudget type")
	}
}
