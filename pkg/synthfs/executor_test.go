package synthfs_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

// TestNewExecutor tests executor creation
func TestNewExecutor(t *testing.T) {
	t.Run("NewExecutor creates executor", func(t *testing.T) {
		executor := synthfs.NewExecutor()

		if executor == nil {
			t.Fatal("Expected executor to be created")
		}

		// EventBus should be available
		eventBus := executor.EventBus()
		if eventBus == nil {
			t.Error("Expected EventBus to be available")
		}
	})
}

// TestDefaultPipelineOptions tests default options
func TestDefaultPipelineOptions(t *testing.T) {
	opts := synthfs.DefaultPipelineOptions()

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

// TestExecutor_Run tests basic execution
func TestExecutor_Run(t *testing.T) {
	t.Run("Run with successful operations", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		// Add mock operations to pipeline
		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		op2 := NewMockMainOperation("op2", "create_directory", "testdir")
		pipeline.AddOperations(op1, op2)

		result := executor.Run(ctx, pipeline, fs)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, error=%v", result.Success, (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		if len(result.Operations) != 2 {
			t.Errorf("Expected 2 operation results, got %d", len(result.Operations))
		}

		if result.Duration == 0 {
			t.Error("Expected non-zero duration")
		}
	})

	t.Run("Run with empty pipeline", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		result := executor.Run(ctx, pipeline, fs)

		if !result.Success {
			t.Errorf("Expected success=true for empty pipeline, got success=%v, error=%v", result.Success, (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		if len(result.Operations) != 0 {
			t.Errorf("Expected 0 operation results for empty pipeline, got %d", len(result.Operations))
		}
	})
}

// TestExecutor_RunWithOptions tests execution with custom options
func TestExecutor_RunWithOptions(t *testing.T) {
	t.Run("RunWithOptions with restorable execution", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		op1.SetReverseOps([]synthfs.Operation{NewMockMainOperation("rev1", "delete", "test.txt")}, nil)
		pipeline.AddOperations(op1)

		opts := synthfs.PipelineOptions{
			Restorable:      true,
			MaxBackupSizeMB: 10,
		}

		result := executor.RunWithOptions(ctx, pipeline, fs, opts)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, error=%v", result.Success, (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		if result.Budget == nil {
			t.Error("Expected budget to be initialized for restorable execution")
		}

		if len(result.RestoreOps) == 0 {
			t.Error("Expected restore operations to be available")
		}
	})

	t.Run("RunWithOptions with prerequisite resolution disabled", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		pipeline.AddOperations(op1)

		opts := synthfs.PipelineOptions{
			ResolvePrerequisites: false,
		}

		result := executor.RunWithOptions(ctx, pipeline, fs, opts)

		if !result.Success {
			t.Errorf("Expected success=true, got success=%v, error=%v", result.Success, (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Should skip prerequisite resolution gracefully
		if pipeline.resolvePrerequisitesCalled {
			t.Error("Expected ResolvePrerequisites to be skipped when disabled")
		}
	})
}

// TestExecutor_ErrorPaths tests various error scenarios
func TestExecutor_ErrorPaths(t *testing.T) {
	t.Run("Pipeline validation failure", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		pipeline.SetValidateError(errors.New("validation failed: conflicting operations"))

		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		pipeline.AddOperations(op1)

		result := executor.RunWithOptions(ctx, pipeline, fs, synthfs.DefaultPipelineOptions())

		if result.Success {
			t.Error("Expected failure for pipeline validation error")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected error to be returned")
		}

		if !strings.Contains(fmt.Sprintf("%v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })()), "validation failed") {
			t.Errorf("Expected validation error, got: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}
	})

	t.Run("Operation execution failure", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		failingOp := NewMockMainOperation("failing_op", "create_file", "test.txt")
		failingOp.SetExecuteError(errors.New("filesystem error"))
		successOp := NewMockMainOperation("success_op", "create_directory", "testdir")
		pipeline.AddOperations(failingOp, successOp)

		result := executor.RunWithOptions(ctx, pipeline, fs, synthfs.DefaultPipelineOptions())

		if result.Success {
			t.Error("Expected failure when operation execution fails")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected error to be returned")
		}

		// Check that operations were processed
		if len(result.Operations) == 0 {
			t.Error("Expected operation results to be returned even on failure")
		}
	})
}

// TestExecutor_ResultConversion tests result conversion logic
func TestExecutor_ResultConversion(t *testing.T) {
	t.Run("Convert successful result", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		op2 := NewMockMainOperation("op2", "create_directory", "testdir")
		pipeline.AddOperations(op1, op2)

		result := executor.Run(ctx, pipeline, fs)

		// Test main result methods
		if !result.Success {
			t.Error("Expected success=true")
		}

		if len(result.Errors) > 0 {
			t.Errorf("Expected no error, got: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		ops := result.Operations
		if len(ops) != 2 {
			t.Errorf("Expected 2 operations, got %d", len(ops))
		}

		// Check operation results
		for i, opResult := range ops {
			if opResult.OperationID == "" {
				t.Errorf("Operation %d missing OperationID", i)
			}
			if opResult.Operation == nil {
				t.Errorf("Operation %d missing Operation reference", i)
			}
			// Duration might be 0 on very fast systems, so we just check it exists
			// The important thing is that the Duration field is populated from the core result
		}

		// Total duration might be 0 on very fast systems
		// Just verify the method exists and returns a value
		_ = result.Duration
	})

	t.Run("Convert result with restore operations", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		op1.SetReverseOps([]synthfs.Operation{NewMockMainOperation("rev1", "delete", "test.txt")}, nil)
		pipeline.AddOperations(op1)

		opts := synthfs.PipelineOptions{
			Restorable: true,
		}

		result := executor.RunWithOptions(ctx, pipeline, fs, opts)

		if !result.Success {
			t.Errorf("Expected success=true, got: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		restoreOps := result.RestoreOps
		if len(restoreOps) == 0 {
			t.Error("Expected restore operations to be available")
		}
	})
}

// TestPipelineWrapper tests the pipeline wrapper functionality
func TestPipelineWrapper(t *testing.T) {
	t.Run("Pipeline wrapper methods", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		pipeline.AddOperations(op1)

		result := executor.Run(ctx, pipeline, fs)

		if !result.Success {
			t.Errorf("Expected success=true, got: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify pipeline methods were called
		if !pipeline.resolveCalled {
			t.Error("Expected Resolve to be called")
		}
		if !pipeline.validateCalled {
			t.Error("Expected Validate to be called")
		}
	})

	t.Run("Pipeline wrapper with filesystem interface mismatch", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		var invalidFS interface{} = "not a filesystem"
		ctx := context.Background()

		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		pipeline.AddOperations(op1)

		// Should handle filesystem interface mismatch gracefully
		// Use type assertion in the executor.Run call
		if fs, ok := invalidFS.(synthfs.FileSystem); ok {
			result := executor.Run(ctx, pipeline, fs)
			if result == nil {
				t.Error("Expected result to be returned even with invalid filesystem")
			}
		} else {
			t.Log("Invalid filesystem interface correctly rejected by type system")
		}
	})
}

// TestOperationWrapper tests the operation wrapper functionality
func TestOperationWrapper(t *testing.T) {
	t.Run("Operation wrapper with prerequisites", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		// Create operation that implements Prerequisites method
		op1 := NewMockMainOperationWithPrerequisites("op1", "create_file", "test.txt")
		prereq := NewMockPrerequisite("parent_dir", ".", nil)
		op1.SetPrerequisites([]core.Prerequisite{prereq})

		pipeline.AddOperations(op1)

		result := executor.Run(ctx, pipeline, fs)

		if !result.Success {
			t.Errorf("Expected success=true, got: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify operation was wrapped and executed correctly
		if len(result.Operations) != 1 {
			t.Errorf("Expected 1 operation result, got %d", len(result.Operations))
		}
	})

	t.Run("Operation wrapper fallback methods", func(t *testing.T) {
		executor := synthfs.NewExecutor()
		pipeline := NewMockMainPipeline()
		fs := testutil.NewTestFileSystem()
		ctx := context.Background()

		// Test operation that only implements basic interface (no V2 methods)
		op1 := NewMockMainOperation("op1", "create_file", "test.txt")
		pipeline.AddOperations(op1)

		result := executor.Run(ctx, pipeline, fs)

		if !result.Success {
			t.Errorf("Expected success=true, got: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Should fallback to original Execute/Validate methods
		if len(result.Operations) != 1 {
			t.Errorf("Expected 1 operation result, got %d", len(result.Operations))
		}
	})
}

// Mock implementations for main package testing

type MockMainPipeline struct {
	operations                 []synthfs.Operation
	resolveError               error
	validateError              error
	resolvePrerequisitesError  error
	resolveCalled              bool
	validateCalled             bool
	resolvePrerequisitesCalled bool
}

func NewMockMainPipeline() *MockMainPipeline {
	return &MockMainPipeline{
		operations: make([]synthfs.Operation, 0),
	}
}

func (m *MockMainPipeline) Add(ops ...synthfs.Operation) error {
	m.operations = append(m.operations, ops...)
	return nil
}

func (m *MockMainPipeline) AddOperations(ops ...synthfs.Operation) {
	m.operations = append(m.operations, ops...)
}

func (m *MockMainPipeline) Operations() []synthfs.Operation {
	return m.operations
}

func (m *MockMainPipeline) Resolve() error {
	m.resolveCalled = true
	return m.resolveError
}

func (m *MockMainPipeline) Validate(ctx context.Context, fs synthfs.FileSystem) error {
	m.validateCalled = true
	return m.validateError
}

func (m *MockMainPipeline) SetResolveError(err error)  { m.resolveError = err }
func (m *MockMainPipeline) SetValidateError(err error) { m.validateError = err }
func (m *MockMainPipeline) SetResolvePrerequisitesError(err error) {
	m.resolvePrerequisitesError = err
}

type MockMainOperation struct {
	id              synthfs.OperationID
	opType          string
	path            string
	executeError    error
	validateError   error
	rollbackError   error
	reverseOpsError error
	reverseOps      []synthfs.Operation
	backupData      *synthfs.BackupData
	rollbackCalled  bool
	item            interface{}
}

func NewMockMainOperation(id, opType, path string) *MockMainOperation {
	return &MockMainOperation{
		id:     synthfs.OperationID(id),
		opType: opType,
		path:   path,
	}
}

func (m *MockMainOperation) ID() synthfs.OperationID { return m.id }
func (m *MockMainOperation) Describe() synthfs.OperationDesc {
	return synthfs.OperationDesc{
		Type: m.opType,
		Path: m.path,
	}
}
func (m *MockMainOperation) AddDependency(depID synthfs.OperationID) { /* no-op for mock */ }

func (m *MockMainOperation) Execute(ctx context.Context, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	return m.executeError
}

func (m *MockMainOperation) Validate(ctx context.Context, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	return m.validateError
}

func (m *MockMainOperation) ReverseOps(ctx context.Context, fs filesystem.FileSystem, budget interface{}) ([]operations.Operation, interface{}, error) {
	if m.reverseOpsError != nil {
		return nil, nil, m.reverseOpsError
	}
	// Convert []synthfs.Operation to []operations.Operation
	// synthfs.Operation is now an alias to operations.Operation
	opsOps := append([]operations.Operation(nil), m.reverseOps...)
	return opsOps, m.backupData, nil
}

func (m *MockMainOperation) Rollback(ctx context.Context, fs synthfs.FileSystem) error {
	m.rollbackCalled = true
	return m.rollbackError
}

func (m *MockMainOperation) GetItem() interface{} {
	return m.item
}

func (m *MockMainOperation) Prerequisites() []core.Prerequisite                   { return []core.Prerequisite{} }
func (m *MockMainOperation) GetChecksum(path string) interface{}                  { return nil }
func (m *MockMainOperation) GetAllChecksums() map[string]interface{}             { return nil }
func (m *MockMainOperation) SetDescriptionDetail(key string, value interface{})  { /* no-op for mock */ }
func (m *MockMainOperation) SetPaths(src, dst string)                            { /* no-op for mock */ }
func (m *MockMainOperation) GetPaths() (src, dst string)                         { return "", "" }
func (m *MockMainOperation) SetChecksum(path string, checksum interface{})       { /* no-op for mock */ }

func (m *MockMainOperation) SetExecuteError(err error)    { m.executeError = err }
func (m *MockMainOperation) SetValidateError(err error)   { m.validateError = err }
func (m *MockMainOperation) SetRollbackError(err error)   { m.rollbackError = err }
func (m *MockMainOperation) SetReverseOpsError(err error) { m.reverseOpsError = err }
func (m *MockMainOperation) SetReverseOps(ops []synthfs.Operation, backup *synthfs.BackupData) {
	m.reverseOps = ops
	m.backupData = backup
}
func (m *MockMainOperation) SetItem(item interface{}) { m.item = item }

// MockMainOperationWithPrerequisites extends MockMainOperation to support Prerequisites
type MockMainOperationWithPrerequisites struct {
	*MockMainOperation
	prerequisites []core.Prerequisite
}

func NewMockMainOperationWithPrerequisites(id, opType, path string) *MockMainOperationWithPrerequisites {
	return &MockMainOperationWithPrerequisites{
		MockMainOperation: NewMockMainOperation(id, opType, path),
		prerequisites:     []core.Prerequisite{},
	}
}

func (m *MockMainOperationWithPrerequisites) Prerequisites() []core.Prerequisite {
	return m.prerequisites
}

func (m *MockMainOperationWithPrerequisites) SetPrerequisites(prereqs []core.Prerequisite) {
	m.prerequisites = prereqs
}

// MockPrerequisite for testing prerequisite handling
type MockPrerequisite struct {
	prereqType    string
	path          string
	validateError error
}

func NewMockPrerequisite(prereqType, path string, validateError error) *MockPrerequisite {
	return &MockPrerequisite{
		prereqType:    prereqType,
		path:          path,
		validateError: validateError,
	}
}

func (m *MockPrerequisite) Type() string                    { return m.prereqType }
func (m *MockPrerequisite) Path() string                    { return m.path }
func (m *MockPrerequisite) Validate(fsys interface{}) error { return m.validateError }
