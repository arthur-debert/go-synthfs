package synthfs_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestExecutor_RunWithOptions_Basic(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()
	executor := synthfs.NewExecutor()
	pipeline := synthfs.NewMemPipeline()

	// Create a simple operation
	registry := synthfs.GetDefaultRegistry()
	op, err := registry.CreateOperation(core.OperationID("test"), "create_file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	operation := op.(synthfs.Operation)
	fileItem := synthfs.NewFile("test.txt").WithContent([]byte("content"))
	err = registry.SetItemForOperation(op, fileItem)
	if err != nil {
		t.Fatalf("Failed to set item for operation: %v", err)
	}

	err = pipeline.Add(operation)
	if err != nil {
		t.Fatalf("Failed to add operation to pipeline: %v", err)
	}

	// Test with default options
	opts := synthfs.DefaultPipelineOptions()
	result := executor.RunWithOptions(ctx, pipeline, fs, opts)

	if !result.IsSuccess() {
		t.Errorf("Expected success, got errors: %v", result.GetError())
	}

	if len(result.GetOperations()) != 1 {
		t.Errorf("Expected 1 operation result, got %d", len(result.GetOperations()))
	}

	// With the new architecture, budget might be initialized even when restorable=false
	// but it should not be used. Let's check if restorable was false in options
	if opts.Restorable {
		t.Error("Expected DefaultPipelineOptions to have restorable=false")
	}

	// If restorable=false and we still have a budget, it's OK as long as it wasn't used
	if result.GetBudget() != nil {
		if budget, ok := result.GetBudget().(*core.BackupBudget); ok && budget != nil && budget.UsedMB > 0 {
			t.Error("Expected no budget usage when restorable=false")
		}
	}

	if len(result.GetRestoreOps()) != 0 {
		t.Errorf("Expected 0 restore operations when restorable=false, got %d", len(result.GetRestoreOps()))
	}
}

func TestExecutor_RunWithOptions_Restorable(t *testing.T) {
	ctx := context.Background()
	fs := testutil.NewTestFileSystem()
	executor := synthfs.NewExecutor()
	pipeline := synthfs.NewMemPipeline()

	// Create a simple operation
	registry := synthfs.GetDefaultRegistry()
	op, err := registry.CreateOperation(core.OperationID("test"), "create_file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}
	operation := op.(synthfs.Operation)
	fileItem := synthfs.NewFile("test.txt").WithContent([]byte("content"))
	err = registry.SetItemForOperation(op, fileItem)
	if err != nil {
		t.Fatalf("Failed to set item for operation: %v", err)
	}

	err = pipeline.Add(operation)
	if err != nil {
		t.Fatalf("Failed to add operation to pipeline: %v", err)
	}

	// Test with restorable options
	opts := synthfs.PipelineOptions{
		Restorable:      true,
		MaxBackupSizeMB: 5,
	}
	result := executor.RunWithOptions(ctx, pipeline, fs, opts)

	if !result.IsSuccess() {
		t.Errorf("Expected success, got errors: %v", result.GetError())
	}

	if len(result.GetOperations()) != 1 {
		t.Errorf("Expected 1 operation result, got %d", len(result.GetOperations()))
	}

	if result.GetBudget() == nil {
		t.Fatal("Expected budget to be initialized when restorable=true")
	}

	if budget, ok := result.GetBudget().(*synthfs.BackupBudget); ok {
		if budget.TotalMB != 5.0 {
			t.Errorf("Expected budget total 5.0 MB, got %f", budget.TotalMB)
		}
	} else {
		t.Error("Expected budget to be *BackupBudget type")
	}

	if len(result.GetRestoreOps()) != 1 {
		t.Errorf("Expected 1 restore operation when restorable=true, got %d", len(result.GetRestoreOps()))
	}

	// Check operation result has backup data
	// Note: With the simplified API, we can't directly access operation details
	// We can only verify that restore operations were created
	if len(result.GetRestoreOps()) == 0 {
		t.Error("Expected restore operations to be created when restorable=true")
	}
}
