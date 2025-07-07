package synthfs

import (
	"io/fs"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// BatchAdapter adapts the batch package implementation to work with the main package types.
// This is a temporary adapter during migration.
type BatchAdapter struct {
	impl      batch.Batch
	mainBatch *Batch // Reference to main batch for operations we haven't migrated yet
}

// NewBatchAdapter creates a new batch adapter that delegates to the batch package.
func NewBatchAdapter(mainBatch *Batch) *BatchAdapter {
	// Create batch implementation with the filesystem and registry
	impl := batch.NewBatch(mainBatch.fs, mainBatch.registry)
	impl = impl.WithContext(mainBatch.ctx)

	return &BatchAdapter{
		impl:      impl,
		mainBatch: mainBatch,
	}
}

// CreateDir delegates to the batch package implementation.
func (a *BatchAdapter) CreateDir(path string, mode ...fs.FileMode) (Operation, error) {
	// Use the batch package to create the operation
	_, err := a.impl.CreateDir(path, mode...)
	if err != nil {
		return nil, err
	}

	// For now, we need to also create in main batch to maintain compatibility
	// This will be removed once we fully migrate
	return a.mainBatch.CreateDir(path, mode...)
}

// CreateFile delegates to the batch package implementation.
func (a *BatchAdapter) CreateFile(path string, content []byte, mode ...fs.FileMode) (Operation, error) {
	// Use the batch package to create the operation
	_, err := a.impl.CreateFile(path, content, mode...)
	if err != nil {
		return nil, err
	}

	// For now, we need to also create in main batch to maintain compatibility
	return a.mainBatch.CreateFile(path, content, mode...)
}

// Copy delegates to the batch package implementation.
func (a *BatchAdapter) Copy(src, dst string) (Operation, error) {
	// Use the batch package to create the operation
	_, err := a.impl.Copy(src, dst)
	if err != nil {
		return nil, err
	}

	// For now, we need to also create in main batch to maintain compatibility
	return a.mainBatch.Copy(src, dst)
}

// Move delegates to the batch package implementation.
func (a *BatchAdapter) Move(src, dst string) (Operation, error) {
	// Use the batch package to create the operation
	_, err := a.impl.Move(src, dst)
	if err != nil {
		return nil, err
	}

	// For now, we need to also create in main batch to maintain compatibility
	return a.mainBatch.Move(src, dst)
}

// Delete delegates to the batch package implementation.
func (a *BatchAdapter) Delete(path string) (Operation, error) {
	// Use the batch package to create the operation
	_, err := a.impl.Delete(path)
	if err != nil {
		return nil, err
	}

	// For now, we need to also create in main batch to maintain compatibility
	return a.mainBatch.Delete(path)
}

// ConvertBatchResult converts a batch.Result to a main package Result.
func ConvertBatchResult(batchResult interface{}) *Result {
	if batchResult == nil {
		return nil
	}

	result, ok := batchResult.(batch.Result)
	if !ok {
		return nil
	}

	// Convert operations from interface{} to Operation
	ops := result.GetOperations()
	operations := make([]Operation, len(ops))
	for i, op := range ops {
		if operation, ok := op.(Operation); ok {
			operations[i] = operation
		}
	}

	// Convert restore ops
	restoreOps := result.GetRestoreOps()
	restoreOperations := make([]Operation, len(restoreOps))
	for i, op := range restoreOps {
		if operation, ok := op.(Operation); ok {
			restoreOperations[i] = operation
		}
	}

	// Get duration
	var duration time.Duration
	if d := result.GetDuration(); d != nil {
		if dur, ok := d.(time.Duration); ok {
			duration = dur
		}
	}

	// Convert operations to OperationResult
	operationResults := make([]OperationResult, len(operations))
	for i, op := range operations {
		operationResults[i] = OperationResult{
			OperationID: op.ID(),
			Operation:   op,
			Status:      OperationStatus(core.StatusSuccess),
			Duration:    duration / time.Duration(len(operations)), // Approximate per-operation duration
		}
	}

	// Build errors list if execution failed
	var errors []error
	if err := result.GetError(); err != nil {
		errors = append(errors, err)
	}

	return &Result{
		Success:    result.IsSuccess(),
		Operations: operationResults,
		RestoreOps: restoreOperations,
		Duration:   duration,
		Errors:     errors,
	}
}
