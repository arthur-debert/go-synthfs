package execution_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
)

// TestNewExecutor tests executor creation
func TestNewExecutor(t *testing.T) {
	t.Run("NewExecutor creates executor with logger", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		if executor == nil {
			t.Fatal("Expected executor to be created")
		}

		// EventBus should be available
		eventBus := executor.EventBus()
		if eventBus == nil {
			t.Error("Expected EventBus to be available")
		}
	})

	t.Run("NewExecutor creates executor with nil logger", func(t *testing.T) {
		executor := execution.NewExecutor(nil)

		if executor == nil {
			t.Fatal("Expected executor to be created even with nil logger")
		}

		// Should still have an event bus
		eventBus := executor.EventBus()
		if eventBus == nil {
			t.Error("Expected EventBus to be available even with nil logger")
		}
	})
}

// TestDefaultPipelineOptions tests default option values
func TestDefaultPipelineOptions(t *testing.T) {
	opts := execution.DefaultPipelineOptions()

	if opts.Restorable {
		t.Error("Expected Restorable to be false by default")
	}

	if opts.MaxBackupSizeMB != core.DefaultMaxBackupMB {
		t.Errorf("Expected MaxBackupSizeMB to be %d, got %d", core.DefaultMaxBackupMB, opts.MaxBackupSizeMB)
	}

	if !opts.ResolvePrerequisites {
		t.Error("Expected ResolvePrerequisites to be true by default")
	}

	if !opts.UseSimpleBatch {
		t.Error("Expected UseSimpleBatch to be true by default")
	}
}

// TestExecutor_Run tests basic execution with default options
func TestExecutor_Run(t *testing.T) {
	t.Run("Run with successful operations", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test.txt")
		op2 := NewMockOperation("op2", "create_directory", "testdir")
		pipeline.AddOperations(op1, op2)

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.Run(ctx, pipeline, fs)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		if len(result.Operations) != 2 {
			t.Errorf("Expected 2 operation results, got %d", len(result.Operations))
		}

		if result.Duration == 0 {
			t.Error("Expected non-zero duration")
		}

		// Check rollback function was created
		if result.Rollback == nil {
			t.Error("Expected rollback function to be created")
		}
	})

	t.Run("Run with empty pipeline", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.Run(ctx, pipeline, fs)

		if !result.Success {
			t.Errorf("Expected success=true for empty pipeline, got success=%v, errors=%v", result.Success, result.Errors)
		}

		if len(result.Operations) != 0 {
			t.Errorf("Expected 0 operation results for empty pipeline, got %d", len(result.Operations))
		}
	})
}

// TestExecutor_RunWithOptions tests execution with custom options
func TestExecutor_RunWithOptions(t *testing.T) {
	t.Run("RunWithOptions with restorable execution", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test.txt")
		op1.SetReverseOps([]interface{}{NewMockOperation("rev1", "delete", "test.txt")}, &core.BackupData{
			SizeMB:     1.5,
			BackupType: "file_content",
		})
		pipeline.AddOperations(op1)

		fs := NewMockFileSystem()
		ctx := context.Background()

		opts := core.PipelineOptions{
			Restorable:           true,
			MaxBackupSizeMB:      50,
			ResolvePrerequisites: true,
		}

		result := executor.RunWithOptions(ctx, pipeline, fs, opts)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Check budget was initialized
		if result.Budget == nil {
			t.Error("Expected budget to be initialized for restorable execution")
		}

		if result.Budget.TotalMB != 50 {
			t.Errorf("Expected budget total to be 50MB, got %f", result.Budget.TotalMB)
		}

		// Check restore operations were collected
		if len(result.RestoreOps) == 0 {
			t.Error("Expected restore operations to be collected")
		}

		// Check operation result has backup info
		if len(result.Operations) > 0 {
			opResult := result.Operations[0]
			if opResult.BackupSizeMB == 0 {
				t.Error("Expected backup size to be recorded")
			}
		}
	})

	t.Run("RunWithOptions with prerequisite resolution disabled", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test.txt")
		pipeline.AddOperations(op1)

		fs := NewMockFileSystem()
		ctx := context.Background()

		opts := core.PipelineOptions{
			Restorable:           false,
			ResolvePrerequisites: false,
		}

		result := executor.RunWithOptions(ctx, pipeline, fs, opts)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Verify pipeline methods were called correctly
		if pipeline.resolvePrerequisitesCalled {
			t.Error("Expected ResolvePrerequisites to not be called when disabled")
		}
	})
}

// TestExecutor_RunWithOptionsAndResolver tests execution with custom resolver
func TestExecutor_RunWithOptionsAndResolver(t *testing.T) {
	t.Run("RunWithOptionsAndResolver with custom resolver", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test.txt")
		pipeline.AddOperations(op1)

		fs := NewMockFileSystem()
		ctx := context.Background()
		resolver := NewMockPrerequisiteResolver()

		opts := core.PipelineOptions{
			ResolvePrerequisites: true,
		}

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, opts, resolver)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Verify prerequisite resolution was called
		if !pipeline.resolvePrerequisitesCalled {
			t.Error("Expected ResolvePrerequisites to be called when enabled")
		}
	})

	t.Run("RunWithOptionsAndResolver with nil resolver", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test.txt")
		pipeline.AddOperations(op1)

		fs := NewMockFileSystem()
		ctx := context.Background()

		opts := core.PipelineOptions{
			ResolvePrerequisites: true,
		}

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, opts, nil)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Should skip prerequisite resolution gracefully
		if pipeline.resolvePrerequisitesCalled {
			t.Error("Expected ResolvePrerequisites to be skipped with nil resolver")
		}
	})
}

// TestExecutor_ErrorPaths tests various error scenarios
func TestExecutor_ErrorPaths(t *testing.T) {
	t.Run("Invalid pipeline type", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		// Pass an invalid pipeline type
		invalidPipeline := "not a pipeline"
		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, invalidPipeline, fs, execution.DefaultPipelineOptions(), nil)

		if result.Success {
			t.Error("Expected failure for invalid pipeline type")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected error for invalid pipeline type")
		}

		expectedErr := "invalid pipeline type: string"
		if result.Errors[0].Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, result.Errors[0].Error())
		}
	})

	t.Run("Prerequisite resolution failure", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		pipeline.SetResolvePrerequisitesError(errors.New("prerequisite resolution failed"))

		fs := NewMockFileSystem()
		ctx := context.Background()
		resolver := NewMockPrerequisiteResolver()

		opts := core.PipelineOptions{
			ResolvePrerequisites: true,
		}

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, opts, resolver)

		if result.Success {
			t.Error("Expected failure for prerequisite resolution error")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected error for prerequisite resolution failure")
		}

		if !strings.Contains(fmt.Sprintf("%v", result.Errors[0]), "prerequisite resolution failed") {
			t.Errorf("Expected prerequisite resolution error, got: %v", result.Errors[0])
		}
	})

	t.Run("Dependency resolution failure", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		pipeline.SetResolveError(errors.New("circular dependency detected"))

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		if result.Success {
			t.Error("Expected failure for dependency resolution error")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected error for dependency resolution failure")
		}

		if !strings.Contains(fmt.Sprintf("%v", result.Errors[0]), "dependency resolution failed") {
			t.Errorf("Expected dependency resolution error, got: %v", result.Errors[0])
		}
	})

	t.Run("Pipeline validation failure", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		pipeline.SetValidateError(errors.New("validation failed: conflicting operations"))

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		if result.Success {
			t.Error("Expected failure for pipeline validation error")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected error for pipeline validation failure")
		}

		if !strings.Contains(fmt.Sprintf("%v", result.Errors[0]), "pipeline validation failed") {
			t.Errorf("Expected pipeline validation error, got: %v", result.Errors[0])
		}
	})

	t.Run("Operation execution failure", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		failingOp := NewMockOperation("failing_op", "create_file", "test.txt")
		failingOp.SetExecuteError(errors.New("filesystem error"))
		successOp := NewMockOperation("success_op", "create_directory", "testdir")
		pipeline.AddOperations(failingOp, successOp)

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		if result.Success {
			t.Error("Expected failure when operation execution fails")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected error for operation execution failure")
		}

		// Check that first operation failed
		if len(result.Operations) < 1 {
			t.Fatal("Expected at least one operation result")
		}

		if result.Operations[0].Status != core.StatusFailure {
			t.Errorf("Expected first operation to have failed status, got: %v", result.Operations[0].Status)
		}

		if result.Operations[0].Error == nil {
			t.Error("Expected first operation to have error recorded")
		}

		// Second operation should still be attempted and succeed
		if len(result.Operations) >= 2 {
			if result.Operations[1].Status != core.StatusSuccess {
				t.Errorf("Expected second operation to succeed, got: %v", result.Operations[1].Status)
			}
		}
	})

	t.Run("Operation with non-OperationInterface type", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		// Add an invalid operation that doesn't implement OperationInterface
		pipeline.AddOperationsRaw("invalid operation")

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		// Should succeed but skip the invalid operation
		if !result.Success {
			t.Errorf("Expected success=true when skipping invalid operations, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Should have no operation results since the invalid operation was skipped
		if len(result.Operations) != 0 {
			t.Errorf("Expected 0 operation results for skipped operations, got %d", len(result.Operations))
		}
	})
}

// TestExecutor_BudgetManagement tests budget handling scenarios
func TestExecutor_BudgetManagement(t *testing.T) {
	t.Run("Budget tracking with successful operations", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test1.txt")
		op1.SetReverseOps([]interface{}{NewMockOperation("rev1", "delete", "test1.txt")}, &core.BackupData{
			SizeMB:     2.5,
			BackupType: "file_content",
		})
		op2 := NewMockOperation("op2", "create_file", "test2.txt")
		op2.SetReverseOps([]interface{}{NewMockOperation("rev2", "delete", "test2.txt")}, &core.BackupData{
			SizeMB:     1.0,
			BackupType: "file_content",
		})
		pipeline.AddOperations(op1, op2)

		fs := NewMockFileSystem()
		ctx := context.Background()

		opts := core.PipelineOptions{
			Restorable:      true,
			MaxBackupSizeMB: 10,
		}

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, opts, nil)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		if result.Budget == nil {
			t.Fatal("Expected budget to be initialized")
		}

		// Check budget usage
		expectedUsed := 3.5 // 2.5 + 1.0
		if result.Budget.UsedMB != expectedUsed {
			t.Errorf("Expected budget used to be %f, got %f", expectedUsed, result.Budget.UsedMB)
		}

		expectedRemaining := 6.5 // 10 - 3.5
		if result.Budget.RemainingMB != expectedRemaining {
			t.Errorf("Expected budget remaining to be %f, got %f", expectedRemaining, result.Budget.RemainingMB)
		}
	})

	t.Run("Budget restoration on operation failure", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		failingOp := NewMockOperation("failing_op", "create_file", "test.txt")
		failingOp.SetReverseOps([]interface{}{NewMockOperation("rev1", "delete", "test.txt")}, &core.BackupData{
			SizeMB:     3.0,
			BackupType: "file_content",
		})
		failingOp.SetExecuteError(errors.New("execution failed"))
		pipeline.AddOperations(failingOp)

		fs := NewMockFileSystem()
		ctx := context.Background()

		opts := core.PipelineOptions{
			Restorable:      true,
			MaxBackupSizeMB: 10,
		}

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, opts, nil)

		if result.Success {
			t.Error("Expected failure when operation execution fails")
		}

		if result.Budget == nil {
			t.Fatal("Expected budget to be initialized")
		}

		// Budget should be restored since operation failed
		if result.Budget.UsedMB != 0 {
			t.Errorf("Expected budget used to be 0 after failure, got %f", result.Budget.UsedMB)
		}

		if result.Budget.RemainingMB != 10 {
			t.Errorf("Expected budget remaining to be 10 after restoration, got %f", result.Budget.RemainingMB)
		}
	})

	t.Run("ReverseOps failure handling", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test.txt")
		op1.SetReverseOpsError(errors.New("reverse ops failed"))
		pipeline.AddOperations(op1)

		fs := NewMockFileSystem()
		ctx := context.Background()

		opts := core.PipelineOptions{
			Restorable:      true,
			MaxBackupSizeMB: 10,
		}

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, opts, nil)

		// Should still succeed despite reverse ops failure
		if !result.Success {
			t.Errorf("Expected success=true despite reverse ops failure, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Operation should still be recorded as successful
		if len(result.Operations) != 1 {
			t.Fatalf("Expected 1 operation result, got %d", len(result.Operations))
		}

		if result.Operations[0].Status != core.StatusSuccess {
			t.Errorf("Expected operation to succeed despite reverse ops failure, got: %v", result.Operations[0].Status)
		}

		// Should have no backup data due to reverse ops failure
		if result.Operations[0].BackupData != nil {
			t.Error("Expected no backup data when reverse ops fails")
		}
	})
}

// TestExecutor_RollbackFunction tests rollback functionality
func TestExecutor_RollbackFunction(t *testing.T) {
	t.Run("Rollback function with successful operations", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test1.txt")
		op2 := NewMockOperation("op2", "create_directory", "testdir")
		pipeline.AddOperations(op1, op2)

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		if result.Rollback == nil {
			t.Fatal("Expected rollback function to be created")
		}

		// Execute rollback
		rollbackErr := result.Rollback(ctx)
		if rollbackErr != nil {
			t.Errorf("Expected rollback to succeed, got error: %v", rollbackErr)
		}

		// Check that rollback was called on operations in reverse order
		if !op2.rollbackCalled {
			t.Error("Expected op2 rollback to be called")
		}
		if !op1.rollbackCalled {
			t.Error("Expected op1 rollback to be called")
		}
	})

	t.Run("Rollback function with failed operations", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		successOp := NewMockOperation("success_op", "create_file", "test.txt")
		failingOp := NewMockOperation("failing_op", "create_directory", "testdir")
		failingOp.SetExecuteError(errors.New("execution failed"))
		pipeline.AddOperations(successOp, failingOp)

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		if result.Success {
			t.Error("Expected failure when operation execution fails")
		}

		if result.Rollback == nil {
			t.Fatal("Expected rollback function to be created")
		}

		// Execute rollback - should only rollback successful operations
		rollbackErr := result.Rollback(ctx)
		if rollbackErr != nil {
			t.Errorf("Expected rollback to succeed, got error: %v", rollbackErr)
		}

		// Only successful operation should be rolled back
		if !successOp.rollbackCalled {
			t.Error("Expected successful operation rollback to be called")
		}
		if failingOp.rollbackCalled {
			t.Error("Expected failed operation rollback to NOT be called")
		}
	})

	t.Run("Rollback function with rollback failures", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		op1 := NewMockOperation("op1", "create_file", "test1.txt")
		op2 := NewMockOperation("op2", "create_directory", "testdir")
		op2.SetRollbackError(errors.New("rollback failed"))
		pipeline.AddOperations(op1, op2)

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Execute rollback
		rollbackErr := result.Rollback(ctx)
		if rollbackErr == nil {
			t.Error("Expected rollback to fail when individual rollback fails")
		}

		if !strings.Contains(fmt.Sprintf("%v", rollbackErr), "rollback errors") {
			t.Errorf("Expected rollback error message, got: %v", rollbackErr)
		}
	})

	t.Run("Rollback function with no executed operations", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		pipeline := NewMockPipelineInterface()
		// Empty pipeline

		fs := NewMockFileSystem()
		ctx := context.Background()

		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		if !result.Success {
			t.Errorf("Expected success=true for empty pipeline, got success=%v, errors=%v", result.Success, result.Errors)
		}

		if result.Rollback == nil {
			t.Fatal("Expected rollback function to be created")
		}

		// Execute rollback on empty set
		rollbackErr := result.Rollback(ctx)
		if rollbackErr != nil {
			t.Errorf("Expected rollback to succeed with no operations, got error: %v", rollbackErr)
		}
	})
}

// TestPipelineAdapter tests the pipeline adapter functionality
func TestPipelineAdapter(t *testing.T) {
	t.Run("Pipeline adapter with execution.Pipeline", func(t *testing.T) {
		logger := NewMockLogger()
		execPipeline := execution.NewMemPipeline(logger)

		// Test through the adapter by using a Pipeline type instead of PipelineInterface
		executor := execution.NewExecutor(logger)
		fs := NewMockFileSystemInterface()
		ctx := context.Background()

		// Add operation to pipeline
		op1 := NewMockOperationInterface("op1", "create_file", "test.txt")
		err := execPipeline.Add(op1)
		if err != nil {
			t.Fatalf("Failed to add operation to pipeline: %v", err)
		}

		// This should trigger the pipelineAdapter code path since execPipeline is a Pipeline, not PipelineInterface
		result := executor.RunWithOptionsAndResolver(ctx, execPipeline, fs, execution.DefaultPipelineOptions(), nil)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Verify that operations were processed
		if len(result.Operations) != 1 {
			t.Errorf("Expected 1 operation result, got %d", len(result.Operations))
		}
	})

	t.Run("Pipeline adapter through MockPipeline", func(t *testing.T) {
		logger := NewMockLogger()
		executor := execution.NewExecutor(logger)

		// Create a mock pipeline that implements the execution.Pipeline interface
		pipeline := NewMockPipeline()
		op1 := NewMockOperationInterface("op1", "create_file", "test.txt")
		pipeline.SetOperations([]interface{}{op1})

		fs := NewMockFileSystemInterface()
		ctx := context.Background()

		// This tests the adapter conversion from Pipeline to PipelineInterface
		result := executor.RunWithOptionsAndResolver(ctx, pipeline, fs, execution.DefaultPipelineOptions(), nil)

		if !result.Success {
			t.Errorf("Expected success=true with Pipeline adapter, got success=%v, errors=%v", result.Success, result.Errors)
		}

		// Check that pipeline methods were called through adapter
		if !pipeline.resolveCalled {
			t.Error("Expected Resolve to be called through adapter")
		}
		if !pipeline.validateCalled {
			t.Error("Expected Validate to be called through adapter")
		}
		// Note: Add is not called during execution, only Operations, Resolve, ResolvePrerequisites, and Validate
	})
}

// Mock implementations for testing

type MockLogger struct {
	entries []string
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		entries: make([]string, 0),
	}
}

func (m *MockLogger) Info() core.LogEvent  { return &MockLogEvent{logger: m} }
func (m *MockLogger) Debug() core.LogEvent { return &MockLogEvent{logger: m} }
func (m *MockLogger) Warn() core.LogEvent  { return &MockLogEvent{logger: m} }
func (m *MockLogger) Error() core.LogEvent { return &MockLogEvent{logger: m} }
func (m *MockLogger) Trace() core.LogEvent { return &MockLogEvent{logger: m} }

type MockLogEvent struct {
	logger *MockLogger
}

func (e *MockLogEvent) Str(key, val string) core.LogEvent                   { return e }
func (e *MockLogEvent) Int(key string, val int) core.LogEvent               { return e }
func (e *MockLogEvent) Bool(key string, val bool) core.LogEvent             { return e }
func (e *MockLogEvent) Float64(key string, val float64) core.LogEvent       { return e }
func (e *MockLogEvent) Dur(key string, val interface{}) core.LogEvent       { return e }
func (e *MockLogEvent) Interface(key string, val interface{}) core.LogEvent { return e }
func (e *MockLogEvent) Err(err error) core.LogEvent                         { return e }
func (e *MockLogEvent) Msg(msg string)                                      { e.logger.entries = append(e.logger.entries, msg) }

type MockPipelineInterface struct {
	operations                 []interface{}
	resolveError               error
	validateError              error
	resolvePrerequisitesError  error
	resolveCalled              bool
	validateCalled             bool
	resolvePrerequisitesCalled bool
}

func NewMockPipelineInterface() *MockPipelineInterface {
	return &MockPipelineInterface{
		operations: make([]interface{}, 0),
	}
}

func (m *MockPipelineInterface) Add(ops ...interface{}) error {
	m.operations = append(m.operations, ops...)
	return nil
}

func (m *MockPipelineInterface) AddOperations(ops ...execution.OperationInterface) {
	for _, op := range ops {
		m.operations = append(m.operations, op)
	}
}

func (m *MockPipelineInterface) AddOperationsRaw(ops ...interface{}) {
	m.operations = append(m.operations, ops...)
}

func (m *MockPipelineInterface) Operations() []interface{} {
	return m.operations
}

func (m *MockPipelineInterface) Resolve() error {
	m.resolveCalled = true
	return m.resolveError
}

func (m *MockPipelineInterface) ResolvePrerequisites(resolver core.PrerequisiteResolver, fs interface{}) error {
	m.resolvePrerequisitesCalled = true
	return m.resolvePrerequisitesError
}

func (m *MockPipelineInterface) Validate(ctx context.Context, fs interface{}) error {
	m.validateCalled = true
	return m.validateError
}

func (m *MockPipelineInterface) SetResolveError(err error)  { m.resolveError = err }
func (m *MockPipelineInterface) SetValidateError(err error) { m.validateError = err }
func (m *MockPipelineInterface) SetResolvePrerequisitesError(err error) {
	m.resolvePrerequisitesError = err
}

type MockPipeline struct {
	operations                 []interface{}
	resolveCalled              bool
	validateCalled             bool
	resolvePrerequisitesCalled bool
	addCalled                  bool
}

func NewMockPipeline() *MockPipeline {
	return &MockPipeline{
		operations: make([]interface{}, 0),
	}
}

func (m *MockPipeline) Add(ops ...interface{}) error {
	m.operations = append(m.operations, ops...)
	m.addCalled = true
	return nil
}

func (m *MockPipeline) SetOperations(ops []interface{}) {
	m.operations = ops
}

func (m *MockPipeline) Operations() []interface{} {
	return m.operations
}

func (m *MockPipeline) Resolve() error {
	m.resolveCalled = true
	return nil
}

func (m *MockPipeline) ResolvePrerequisites(resolver core.PrerequisiteResolver, fs interface{}) error {
	m.resolvePrerequisitesCalled = true
	return nil
}

func (m *MockPipeline) Validate(ctx context.Context, fs interface{}) error {
	m.validateCalled = true
	return nil
}

type MockOperation struct {
	id              core.OperationID
	opType          string
	path            string
	executeError    error
	validateError   error
	rollbackError   error
	reverseOpsError error
	reverseOps      []interface{}
	backupData      *core.BackupData
	rollbackCalled  bool
}

func NewMockOperation(id, opType, path string) *MockOperation {
	return &MockOperation{
		id:     core.OperationID(id),
		opType: opType,
		path:   path,
	}
}

func (m *MockOperation) ID() core.OperationID { return m.id }
func (m *MockOperation) Describe() core.OperationDesc {
	return core.OperationDesc{
		Type: m.opType,
		Path: m.path,
	}
}
func (m *MockOperation) Dependencies() []core.OperationID     { return []core.OperationID{} }
func (m *MockOperation) Conflicts() []core.OperationID        { return []core.OperationID{} }
func (m *MockOperation) Prerequisites() []core.Prerequisite   { return []core.Prerequisite{} }
func (m *MockOperation) AddDependency(depID core.OperationID) { /* no-op for mock */ }

func (m *MockOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return m.executeError
}

func (m *MockOperation) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return m.validateError
}

func (m *MockOperation) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
	if m.reverseOpsError != nil {
		return nil, nil, m.reverseOpsError
	}

	// Apply budget usage if backup data is provided
	if m.backupData != nil && budget != nil {
		budget.UsedMB += m.backupData.SizeMB
		budget.RemainingMB -= m.backupData.SizeMB
	}

	return m.reverseOps, m.backupData, nil
}

func (m *MockOperation) Rollback(ctx context.Context, fsys interface{}) error {
	m.rollbackCalled = true
	return m.rollbackError
}

func (m *MockOperation) GetItem() interface{}                               { return nil }
func (m *MockOperation) SetDescriptionDetail(key string, value interface{}) { /* no-op for mock */ }

func (m *MockOperation) SetExecuteError(err error)    { m.executeError = err }
func (m *MockOperation) SetValidateError(err error)   { m.validateError = err }
func (m *MockOperation) SetRollbackError(err error)   { m.rollbackError = err }
func (m *MockOperation) SetReverseOpsError(err error) { m.reverseOpsError = err }
func (m *MockOperation) SetReverseOps(ops []interface{}, backup *core.BackupData) {
	m.reverseOps = ops
	m.backupData = backup
}

type MockFileSystem struct {
	files map[string][]byte
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files: make(map[string][]byte),
	}
}

type MockPrerequisiteResolver struct{}

func NewMockPrerequisiteResolver() *MockPrerequisiteResolver {
	return &MockPrerequisiteResolver{}
}

func (r *MockPrerequisiteResolver) CanResolve(prereq core.Prerequisite) bool {
	return true
}

func (r *MockPrerequisiteResolver) Resolve(prereq core.Prerequisite) ([]interface{}, error) {
	return []interface{}{}, nil
}
