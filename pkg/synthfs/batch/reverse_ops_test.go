package batch_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestReverseOperations_CreateFile(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()

	registry := synthfs.GetDefaultRegistry()
	op, err := registry.CreateOperation(core.OperationID("test-create-file"), "create_file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	operation := op.(synthfs.Operation)
	budget := &core.BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	reverseOps, backupData, err := operation.ReverseOps(ctx, fs, budget)
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
	fs := testutil.NewTestFileSystem()

	// Create a test file first
	content := []byte("test file content")
	err := fs.WriteFile("test.txt", content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	registry := synthfs.GetDefaultRegistry()
	op, err := registry.CreateOperation(core.OperationID("test-delete"), "delete", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	operation := op.(synthfs.Operation)
	budget := &core.BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	reverseOps, backupData, err := operation.ReverseOps(ctx, fs, budget)
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
	fs := testutil.NewTestFileSystem()

	// Create a large test file that exceeds budget
	largeContent := make([]byte, 11*1024*1024) // 11MB
	err := fs.WriteFile("large.txt", largeContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	registry := synthfs.GetDefaultRegistry()
	op, err := registry.CreateOperation(core.OperationID("test-delete"), "delete", "large.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	operation := op.(synthfs.Operation)
	budget := &core.BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	_, _, err = operation.ReverseOps(ctx, fs, budget)
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
	fs := testutil.NewTestFileSystem()

	registry := synthfs.GetDefaultRegistry()
	op, err := registry.CreateOperation(core.OperationID("test-move"), "move", "src.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	operation := op.(synthfs.Operation)
	operation.SetPaths("src.txt", "dst.txt")
	budget := &core.BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	reverseOps, backupData, err := operation.ReverseOps(ctx, fs, budget)
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
	reverseDesc := reverseOp.Describe()
	if reverseDesc.Path != "dst.txt" {
		t.Errorf("Expected reverse src path 'dst.txt', got '%s'", reverseDesc.Path)
	}
	// For move operations, we need to check the operation type
	if reverseOp, ok := reverseOp.(interface {
		GetSrcPath() string
		GetDstPath() string
	}); ok {
		if reverseOp.GetSrcPath() != "dst.txt" {
			t.Errorf("Expected reverse src path 'dst.txt', got '%s'", reverseOp.GetSrcPath())
		}
		if reverseOp.GetDstPath() != "src.txt" {
			t.Errorf("Expected reverse dst path 'src.txt', got '%s'", reverseOp.GetDstPath())
		}
	}

	if backupData.BackupType != "none" {
		t.Errorf("Expected backup type 'none' for move operation, got '%s'", backupData.BackupType)
	}
}

func TestReverseOperations_Copy(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()

	registry := synthfs.GetDefaultRegistry()
	op, err := registry.CreateOperation(core.OperationID("test-copy"), "copy", "src.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	operation := op.(synthfs.Operation)
	operation.SetPaths("src.txt", "dst.txt")
	budget := &core.BackupBudget{TotalMB: 10.0, RemainingMB: 10.0, UsedMB: 0.0}

	reverseOps, backupData, err := operation.ReverseOps(ctx, fs, budget)
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
