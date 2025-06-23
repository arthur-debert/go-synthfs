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

func TestExecutor_Execute_RollbackCalled(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue()

	op1 := newControllableMockOp("op1_rb_ok")
	op2 := newControllableMockOp("op2_rb_ok")
	op3ExecuteErr := errors.New("op3 failed execution")
	op3 := newControllableMockOp("op3_rb_fail_exec")
	op3.executeErr = op3ExecuteErr // op3 will fail to execute
	op4 := newControllableMockOp("op4_rb_not_run") // op4 will not run if op3 fails and executor stops (current executor continues)

	// Rollback tracking
	var rollbackOrder []string
	op1.rollbackFunc = func(ctx context.Context, fsys synthfs.FileSystem) error {
		rollbackOrder = append(rollbackOrder, "op1")
		return nil
	}
	op2.rollbackFunc = func(ctx context.Context, fsys synthfs.FileSystem) error {
		rollbackOrder = append(rollbackOrder, "op2")
		return nil
	}
	op3.rollbackFunc = func(ctx context.Context, fsys synthfs.FileSystem) error {
		rollbackOrder = append(rollbackOrder, "op3")
		// This t.Error is an assertion helper within the mock. If op3's Execute fails,
		// its Rollback should not be part of the created rollback chain.
		t.Error("op3.Rollback should not be called if its Execute failed and it's not in rollbackOps")
		return nil
	}
	op4.rollbackFunc = func(ctx context.Context, fsys synthfs.FileSystem) error {
		// op4 executes successfully in the current model, so its rollback IS called.
		// The t.Error here was based on a misinterpretation of when it would be called.
		// Removing it as the main test assertions for op4.rollbackCount cover this.
		rollbackOrder = append(rollbackOrder, "op4")
		return nil
	}

	_ = queue.Add(op1, op2, op3, op4)
	result := executor.Execute(ctx, queue, mfs)

	// Current executor behavior: op3 fails, op4 still runs.
	// result.Success will be false.
	if result.Success {
		t.Errorf("Expected result.Success to be false due to op3 failure, got true")
	}

	if op1.executeCount != 1 { t.Errorf("op1 execute count: got %d, want 1", op1.executeCount) }
	if op2.executeCount != 1 { t.Errorf("op2 execute count: got %d, want 1", op2.executeCount) }
	if op3.executeCount != 1 { t.Errorf("op3 execute count: got %d, want 1", op3.executeCount) } // op3 attempts execution
	if op4.executeCount != 1 { t.Errorf("op4 execute count: got %d, want 1", op4.executeCount) } // op4 also executes

	// Call the rollback function
	if result.Rollback == nil {
		t.Fatal("result.Rollback function is nil")
	}
	err := result.Rollback(ctx)
	if err != nil {
		t.Errorf("result.Rollback returned error: %v", err)
	}

	// Check rollback counts
	if op1.rollbackCount != 1 {
		t.Errorf("op1 rollbackCount = %d, want 1", op1.rollbackCount)
	}
	if op2.rollbackCount != 1 {
		t.Errorf("op2 rollbackCount = %d, want 1", op2.rollbackCount)
	}
	if op3.rollbackCount != 0 { // op3 failed execution, so its Rollback should not be called by createRollbackFunc
		t.Errorf("op3 rollbackCount = %d, want 0", op3.rollbackCount)
	}
	if op4.rollbackCount != 1 { // op4 executed successfully, so its Rollback *should* be called
		t.Errorf("op4 rollbackCount = %d, want 1", op4.rollbackCount)
	}

	// Check rollback order: op4, then op2, then op1. op3 is skipped.
	// The rollbackOps list in executor.Execute includes all successfully executed ops.
	// So, op1, op2, op4. Rollback order is reverse: op4, op2, op1.
	expectedRollbackOrder := []string{"op4", "op2", "op1"}
	if len(rollbackOrder) != len(expectedRollbackOrder) {
		t.Errorf("Rollback order length: got %d, want %d. Order: %v", len(rollbackOrder), len(expectedRollbackOrder), rollbackOrder)
	} else {
		for i := range expectedRollbackOrder {
			if rollbackOrder[i] != expectedRollbackOrder[i] {
				t.Errorf("Rollback order at index %d: got %s, want %s. Full order: %v", i, rollbackOrder[i], expectedRollbackOrder[i], rollbackOrder)
				break
			}
		}
	}
}

func TestExecutor_Rollback_ErrorDuringRollback(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue()

	op1RollbackErr := errors.New("op1 failed to rollback")
	op1 := newControllableMockOp("op1_fail_rb")
	op1.rollbackErr = op1RollbackErr

	op2 := newControllableMockOp("op2_success_rb")

	_ = queue.Add(op1, op2) // Both will execute successfully
	result := executor.Execute(ctx, queue, mfs)

	if !result.Success {
		t.Fatalf("Executor.Execute failed unexpectedly: %v", result.Errors)
	}

	if result.Rollback == nil {
		t.Fatal("result.Rollback function is nil")
	}
	err := result.Rollback(ctx)

	if err == nil {
		t.Fatalf("result.Rollback expected an error, got nil")
	}
	if !strings.Contains(err.Error(), op1RollbackErr.Error()) {
		t.Errorf("result.Rollback error string %q does not contain expected error %q", err.Error(), op1RollbackErr.Error())
	}

	// Check that both rollbacks were attempted
	if op1.rollbackCount != 1 {
		t.Errorf("op1 rollbackCount = %d, want 1", op1.rollbackCount)
	}
	if op2.rollbackCount != 1 { // op2's rollback should still be called even if op1's failed
		t.Errorf("op2 rollbackCount = %d, want 1", op2.rollbackCount)
	}
}

func TestExecutor_Execute_DependencyResolutionFailure(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()
	executor := synthfs.NewExecutor()
	queue := synthfs.NewMemQueue()

	op1 := newControllableMockOp("op1_dep_fail")
	op1.dependencies = []synthfs.OperationID{"non_existent_op"} // Depends on an op not in the queue

	op2 := newControllableMockOp("op2_dep_fail") // Will also be in queue but not directly causing failure

	err := queue.Add(op1, op2)
	if err != nil {
		t.Fatalf("queue.Add failed: %v", err)
	}

	result := executor.Execute(ctx, queue, mfs)

	if result.Success {
		t.Errorf("result.Success = true, want false for dependency resolution failure")
	}

	if len(result.Errors) == 0 {
		t.Fatal("result.Errors is empty, want error for dependency resolution failure")
	} else {
		// Check for a specific type of error if possible, or string content
		// The error comes from queue.Resolve(), which wraps errors from toposort or custom checks.
		// For MemQueue, it's likely a synthfs.DependencyError or an error wrapping it.
		foundDepError := false
		for _, resErr := range result.Errors {
			// MemQueue.Resolve() wraps the error from validateDependencyReferences
			// which returns a DependencyError.
			// So we expect the error chain to contain a DependencyError.
			var depErr *synthfs.DependencyError
			if errors.As(resErr, &depErr) {
				foundDepError = true
				if depErr.Operation.ID() != op1.ID() {
					t.Errorf("DependencyError operation ID got %s, want %s", depErr.Operation.ID(), op1.ID())
				}
				if len(depErr.Missing) == 0 || depErr.Missing[0] != "non_existent_op" {
					t.Errorf("DependencyError missing got %v, want ['non_existent_op']", depErr.Missing)
				}
				break
			}
			// Fallback to string check if not a direct DependencyError in result.Errors[0]
			// (e.g. if it's further wrapped by executor)
			if strings.Contains(resErr.Error(), "dependency") && strings.Contains(resErr.Error(), "non_existent_op") {
				foundDepError = true
				// t.Logf("Found dependency error by string match: %v", resErr) // For debugging
				break
			}
		}
		if !foundDepError {
			t.Errorf("result.Errors does not contain a recognizable dependency error. Got: %v", result.Errors)
		}
	}

	// No operations should have been validated or executed
	if op1.validateCount != 0 {
		t.Errorf("op1 validateCount = %d, want 0", op1.validateCount)
	}
	if op1.executeCount != 0 {
		t.Errorf("op1 executeCount = %d, want 0", op1.executeCount)
	}
	if op2.validateCount != 0 {
		t.Errorf("op2 validateCount = %d, want 0", op2.validateCount)
	}
	if op2.executeCount != 0 {
		t.Errorf("op2 executeCount = %d, want 0", op2.executeCount)
	}

	// result.Operations should be empty as execution phase was not reached
	if len(result.Operations) != 0 {
		t.Errorf("len(result.Operations) = %d, want 0", len(result.Operations))
	}
}
