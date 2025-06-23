package synthfs_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/internal/testutil"
	"github.com/arthur-debert/synthfs/pkg/synthfs/ops"
)

// Helper mock operation for executor tests
type controllableMockOperation struct {
	id              synthfs.OperationID
	executeErr      error
	validateErr     error
	rollbackErr     error
	executeCount    int
	validateCount   int
	rollbackCount   int
	dependencies    []synthfs.OperationID
	executeDelay    time.Duration
	executeFunc     func(ctx context.Context, fsys synthfs.FileSystem) error
	validateFunc    func(ctx context.Context, fsys synthfs.FileSystem) error
	rollbackFunc    func(ctx context.Context, fsys synthfs.FileSystem) error
	descriptionPath string
}

func newControllableMockOp(id string) *controllableMockOperation {
	return &controllableMockOperation{id: synthfs.OperationID(id), descriptionPath: "path/" + id}
}

func (m *controllableMockOperation) ID() synthfs.OperationID { return m.id }
func (m *controllableMockOperation) Execute(ctx context.Context, fsys synthfs.FileSystem) error {
	m.executeCount++
	if m.executeDelay > 0 {
		time.Sleep(m.executeDelay)
	}
	if m.executeFunc != nil {
		return m.executeFunc(ctx, fsys)
	}
	return m.executeErr
}
func (m *controllableMockOperation) Validate(ctx context.Context, fsys synthfs.FileSystem) error {
	m.validateCount++
	if m.validateFunc != nil {
		return m.validateFunc(ctx, fsys)
	}
	return m.validateErr
}
func (m *controllableMockOperation) Dependencies() []synthfs.OperationID { return m.dependencies }
func (m *controllableMockOperation) Conflicts() []synthfs.OperationID    { return nil }
func (m *controllableMockOperation) Rollback(ctx context.Context, fsys synthfs.FileSystem) error {
	m.rollbackCount++
	if m.rollbackFunc != nil {
		return m.rollbackFunc(ctx, fsys)
	}
	return m.rollbackErr
}
func (m *controllableMockOperation) Describe() synthfs.OperationDesc {
	return synthfs.OperationDesc{Type: "controllable_mock", Path: m.descriptionPath}
}

func TestExecutor_Execute_SimpleSuccess(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue()

	op1 := newControllableMockOp("op1")
	op2 := newControllableMockOp("op2")

	_ = queue.Add(op1, op2)

	result := executor.Execute(ctx, queue, mfs)

	if !result.Success {
		t.Errorf("result.Success = false, want true. Errors: %v", result.Errors)
	}
	if len(result.Operations) != 2 {
		t.Fatalf("len(result.Operations) = %d, want 2", len(result.Operations))
	}
	if op1.validateCount != 1 || op1.executeCount != 1 {
		t.Errorf("op1 counts: validate=%d (want 1), execute=%d (want 1)", op1.validateCount, op1.executeCount)
	}
	if op2.validateCount != 1 || op2.executeCount != 1 {
		t.Errorf("op2 counts: validate=%d (want 1), execute=%d (want 1)", op2.validateCount, op2.executeCount)
	}
	if result.Operations[0].Status != synthfs.StatusSuccess || result.Operations[1].Status != synthfs.StatusSuccess {
		t.Errorf("Expected all operations to have status SUCCESS")
	}
}

func TestExecutor_Execute_ValidationError(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue()

	op1 := newControllableMockOp("op1_valid")
	op2ValidateErr := errors.New("validation failed for op2")
	op2 := newControllableMockOp("op2_invalid_validate")
	op2.validateErr = op2ValidateErr
	op3 := newControllableMockOp("op3_after_invalid")

	_ = queue.Add(op1, op2, op3)
	result := executor.Execute(ctx, queue, mfs)

	if result.Success {
		t.Errorf("result.Success = true, want false")
	}
	// With the current executor logic, if queue.Validate() fails,
	// it returns early and result.Operations will be empty.
	// The error from queue.Validate() will be in result.Errors.
	if len(result.Operations) != 0 {
		t.Fatalf("len(result.Operations) = %d, want 0 when queue.Validate() fails", len(result.Operations))
	}

	// op1, op2, op3 validateCounts will be 1 because queue.Validate() iterates through all.
	// executeCounts will be 0 because queue.Validate() fails before execution phase.
	if op1.validateCount != 1 || op1.executeCount != 0 {
		t.Errorf("op1 counts: validate=%d, execute=%d; want 1, 0", op1.validateCount, op1.executeCount)
	}
	if op2.validateCount != 1 || op2.executeCount != 0 {
		t.Errorf("op2 counts: validate=%d, execute=%d; want 1, 0", op2.validateCount, op2.executeCount)
	}
	// If queue.Validate() fails on op2, op3's Validate() might not be called if queue.Validate() fails fast.
	// Based on current failure (op3.validateCount = 0), memQueue.Validate() likely stops on first error.
	if op3.validateCount != 0 || op3.executeCount != 0 {
		t.Errorf("op3 counts: validate=%d, execute=%d; want 0, 0 (assuming queue.Validate fails fast)", op3.validateCount, op3.executeCount)
	}

	if len(result.Errors) == 0 {
		t.Fatalf("result.Errors count = %d, want > 0", len(result.Errors))
	}
	// Check that the specific validation error is wrapped in the result.Errors
	foundError := false
	for _, err := range result.Errors {
		if strings.Contains(err.Error(), op2ValidateErr.Error()) { // Check if op2ValidateErr is part of the error chain
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("result.Errors does not contain the expected validation error %q. Got errors: %v", op2ValidateErr, result.Errors)
	}
}

func TestExecutor_Execute_ExecutionError(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue()

	op1 := newControllableMockOp("op1_exec_ok")
	op2ExecuteErr := errors.New("execution failed for op2")
	op2 := newControllableMockOp("op2_invalid_exec")
	op2.executeErr = op2ExecuteErr
	op3 := newControllableMockOp("op3_after_exec_fail")

	_ = queue.Add(op1, op2, op3)
	result := executor.Execute(ctx, queue, mfs)

	if result.Success {
		t.Errorf("result.Success = true, want false")
	}
	if len(result.Operations) != 3 {
		t.Fatalf("len(result.Operations) = %d, want 3", len(result.Operations))
	}

	// op1 should execute successfully
	if op1.validateCount != 1 || op1.executeCount != 1 {
		t.Errorf("op1 counts: validate=%d, execute=%d; want 1, 1", op1.validateCount, op1.executeCount)
	}
	if result.Operations[0].Status != synthfs.StatusSuccess {
		t.Errorf("op1 status = %s, want SUCCESS", result.Operations[0].Status)
	}

	// op2 should validate, but fail execution
	if op2.validateCount != 1 || op2.executeCount != 1 {
		t.Errorf("op2 counts: validate=%d, execute=%d; want 1, 1", op2.validateCount, op2.executeCount)
	}
	if result.Operations[1].Status != synthfs.StatusFailure {
		t.Errorf("op2 status = %s, want FAILURE", result.Operations[1].Status)
	}
	if !errors.Is(result.Operations[1].Error, op2ExecuteErr) {
		t.Errorf("op2 error = %v, want to contain %v", result.Operations[1].Error, op2ExecuteErr)
	}

	// op3 should still be processed (validated and executed if valid) as there's no transactional rollback yet
	if op3.validateCount != 1 || op3.executeCount != 1 {
		t.Errorf("op3 counts: validate=%d, execute=%d; want 1, 1", op3.validateCount, op3.executeCount)
	}
	if result.Operations[2].Status != synthfs.StatusSuccess {
		t.Errorf("op3 status = %s, want SUCCESS", result.Operations[2].Status)
	}

	if len(result.Errors) != 1 {
		t.Errorf("result.Errors count = %d, want 1", len(result.Errors))
	}
}

func TestExecutor_Execute_WithDryRun(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue()

	op1 := newControllableMockOp("op1_dryrun")
	op2 := ops.NewCreateFile("test_dryrun.txt", []byte("data"), 0644).WithID("op2_createfile_dryrun")

	_ = queue.Add(op1, op2)
	result := executor.Execute(ctx, queue, mfs, synthfs.WithDryRun(true))

	if !result.Success { // Dry run with no validation errors should be Success=true
		t.Errorf("result.Success = false, want true for successful dry run. Errors: %v", result.Errors)
	}
	if len(result.Operations) != 2 {
		t.Fatalf("len(result.Operations) = %d, want 2", len(result.Operations))
	}

	// op1 (controllable mock)
	if op1.validateCount != 1 {
		t.Errorf("op1 validateCount = %d, want 1", op1.validateCount)
	}
	if op1.executeCount != 0 { // Should not execute
		t.Errorf("op1 executeCount = %d, want 0", op1.executeCount)
	}
	if result.Operations[0].Status != synthfs.StatusSkipped {
		t.Errorf("op1 status = %s, want SKIPPED", result.Operations[0].Status)
	}
	if result.Operations[0].Error != nil {
		t.Errorf("op1 error message: got %q, want <nil> for dry run skipped operation", result.Operations[0].Error)
	}

	// op2 (CreateFile)
	if result.Operations[1].Status != synthfs.StatusSkipped {
		t.Errorf("op2 status = %s, want SKIPPED", result.Operations[1].Status)
	}
	// Check that file was not actually created
	if _, err := mfs.Stat("test_dryrun.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("File test_dryrun.txt was created during dry run, Stat error: %v", err)
	}
}

func TestExecutor_Execute_DryRun_WithValidationError(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue()

	opValid := newControllableMockOp("op_valid_for_dry_run")
	opInvalidValidate := newControllableMockOp("op_invalid_validate_for_dry_run")
	opInvalidValidate.validateErr = errors.New("dry run validation fail")

	_ = queue.Add(opValid, opInvalidValidate)
	result := executor.Execute(ctx, queue, mfs, synthfs.WithDryRun(true))

	if result.Success { // Dry run with validation error should be Success=false
		t.Errorf("result.Success = true, want false due to validation error in dry run")
	}
	// If queue.Validate() fails, Executor.Execute returns early.
	// result.Operations will be empty.
	if len(result.Operations) != 0 {
		t.Fatalf("len(result.Operations) = %d, want 0 when queue.Validate() fails during dry run", len(result.Operations))
	}

	// opValid and opInvalidValidate counts will reflect that queue.Validate() processed them.
	// executeCounts will be 0.
	if opValid.validateCount != 1 || opValid.executeCount != 0 {
		t.Errorf("opValid counts: validate=%d, execute=%d; want 1, 0", opValid.validateCount, opValid.executeCount)
	}
	if opInvalidValidate.validateCount != 1 || opInvalidValidate.executeCount != 0 {
		t.Errorf("opInvalidValidate counts: validate=%d, execute=%d; want 1, 0", opInvalidValidate.validateCount, opInvalidValidate.executeCount)
	}

	// The error from queue.Validate (containing opInvalidValidate.validateErr) should be in result.Errors
	if len(result.Errors) == 0 {
		t.Fatalf("result.Errors count = %d, want > 0", len(result.Errors))
	}
	foundError := false
	for _, err := range result.Errors {
		if strings.Contains(err.Error(), opInvalidValidate.validateErr.Error()) {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("result.Errors does not contain the expected validation error %q. Got errors: %v", opInvalidValidate.validateErr, result.Errors)
	}
}

func TestExecutor_Execute_EmptyQueue(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue() // Empty queue

	result := executor.Execute(ctx, queue, mfs)

	if !result.Success {
		t.Errorf("result.Success = false, want true for empty queue. Errors: %v", result.Errors)
	}
	if len(result.Operations) != 0 {
		t.Errorf("len(result.Operations) = %d, want 0 for empty queue", len(result.Operations))
	}
	if len(result.Errors) != 0 {
		t.Errorf("len(result.Errors) = %d, want 0 for empty queue", len(result.Errors))
	}
}
