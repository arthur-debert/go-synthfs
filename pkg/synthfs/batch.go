THIS SHOULD BE A LINTER ERRORTHIS SHOULD BE A LINTER ERRORpackage synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Batch represents a collection of filesystem operations that can be validated and executed as a unit.
// This is a wrapper around the batch package implementation to maintain the public API.
type Batch struct {
	impl batch.Batch
}

// BatchOptions configures batch creation behavior
type BatchOptions struct {
	UseSimpleBatch bool // Whether to use SimpleBatch instead of traditional Batch (default: false, will change to true in future versions)
}

// NewSimpleBatch creates a new operation batch using the SimpleBatch implementation.
// This implementation relies on prerequisite resolution instead of automatic parent directory creation.
// This is the recommended approach for new code.
func NewSimpleBatch() *Batch {
	return NewBatchWithOptions(BatchOptions{UseSimpleBatch: true})
}

// NewLegacyBatch creates a new operation batch using the legacy implementation.
// DEPRECATED: This method is provided for backward compatibility only.
// Use NewBatch() or NewSimpleBatch() for new code, as they provide better
// extensibility through prerequisite resolution.
func NewLegacyBatch() *Batch {
	return NewBatchWithOptions(BatchOptions{UseSimpleBatch: false})
}

// NewBatch creates a new operation batch with default filesystem and context.
// This uses prerequisite resolution for dependency management instead of automatic 
// parent directory creation, providing better extensibility and testability.
// Now defaults to SimpleBatch implementation (Phase 6: Switch Defaults).
func NewBatch() *Batch {
	return NewBatchWithOptions(BatchOptions{UseSimpleBatch: true})
}

// NewBatchWithOptions creates a new operation batch with custom options.
func NewBatchWithOptions(opts BatchOptions) *Batch {
	fs := filesystem.NewOSFileSystem(".")
	registry := GetDefaultRegistry()
	logger := NewLoggerAdapter(Logger())
	
	var impl batch.Batch
	if opts.UseSimpleBatch {
		// Use SimpleBatch implementation with prerequisite resolution enabled by default
		impl = batch.NewSimpleBatch(fs, registry).
			WithContext(context.Background()).
			WithLogger(logger)
	} else {
		// Use traditional batch implementation
		impl = batch.NewBatch(fs, registry).
			WithContext(context.Background()).
			WithLogger(logger)
	}
	
	return &Batch{impl: impl}
}

// WithFileSystem sets the filesystem for the batch operations.
func (b *Batch) WithFileSystem(fs FullFileSystem) *Batch {
	b.impl = b.impl.WithFileSystem(fs)
	return b
}

// WithContext sets the context for the batch operations.
func (b *Batch) WithContext(ctx context.Context) *Batch {
	b.impl = b.impl.WithContext(ctx)
	return b
}

// WithRegistry sets a custom operation registry for the batch.
func (b *Batch) WithRegistry(registry core.OperationFactory) *Batch {
	b.impl = b.impl.WithRegistry(registry)
	return b
}

// WithLogger sets the logger for the batch.
func (b *Batch) WithLogger(logger core.Logger) *Batch {
	b.impl = b.impl.WithLogger(logger)
	return b
}

// Operations returns all operations currently in the batch.
func (b *Batch) Operations() []Operation {
	opsInterface := b.impl.Operations()
	var operations []Operation
	for _, op := range opsInterface {
		if opTyped, ok := op.(Operation); ok {
			operations = append(operations, opTyped)
		}
	}
	return operations
}

// CreateDir adds a directory creation operation to the batch.
func (b *Batch) CreateDir(path string, mode ...fs.FileMode) (Operation, error) {
	op, err := b.impl.CreateDir(path, mode...)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// CreateFile adds a file creation operation to the batch.
func (b *Batch) CreateFile(path string, content []byte, mode ...fs.FileMode) (Operation, error) {
	op, err := b.impl.CreateFile(path, content, mode...)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// Copy adds a copy operation to the batch.
func (b *Batch) Copy(src, dst string) (Operation, error) {
	op, err := b.impl.Copy(src, dst)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// Move adds a move operation to the batch.
func (b *Batch) Move(src, dst string) (Operation, error) {
	op, err := b.impl.Move(src, dst)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// Delete adds a delete operation to the batch.
func (b *Batch) Delete(path string) (Operation, error) {
	op, err := b.impl.Delete(path)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// CreateSymlink adds a symbolic link creation operation to the batch.
func (b *Batch) CreateSymlink(target, linkPath string) (Operation, error) {
	op, err := b.impl.CreateSymlink(target, linkPath)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// CreateArchive adds an archive creation operation to the batch.
func (b *Batch) CreateArchive(archivePath string, format ArchiveFormat, sources ...string) (Operation, error) {
	op, err := b.impl.CreateArchive(archivePath, format, sources...)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// Unarchive adds an unarchive operation to the batch.
func (b *Batch) Unarchive(archivePath, extractPath string) (Operation, error) {
	op, err := b.impl.Unarchive(archivePath, extractPath)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// UnarchiveWithPatterns adds an unarchive operation with pattern filtering to the batch.
func (b *Batch) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (Operation, error) {
	op, err := b.impl.UnarchiveWithPatterns(archivePath, extractPath, patterns...)
	if err != nil {
		return nil, err
	}
	if opTyped, ok := op.(Operation); ok {
		return opTyped, nil
	}
	return nil, fmt.Errorf("unexpected operation type: %T", op)
}

// Run runs all operations in the batch.
func (b *Batch) Run() (*Result, error) {
	batchResult, err := b.impl.Run()
	if err != nil {
		return nil, err
	}
	return ConvertBatchResult(batchResult), nil
}

// RunWithOptions runs all operations in the batch with specified options.
func (b *Batch) RunWithOptions(opts PipelineOptions) (*Result, error) {
	// Convert PipelineOptions to interface{} map for batch package
	optsMap := map[string]interface{}{
		"restorable":            opts.Restorable,
		"max_backup_size_mb":    opts.MaxBackupSizeMB,
		"resolve_prerequisites": true, // Always enable prerequisite resolution
	}
	
	batchResult, err := b.impl.RunWithOptions(optsMap)
	if err != nil {
		return nil, err
	}
	return ConvertBatchResult(batchResult), nil
}

// RunRestorable runs all operations with backup enabled using the default 10MB budget.
func (b *Batch) RunRestorable() (*Result, error) {
	batchResult, err := b.impl.RunRestorable()
	if err != nil {
		return nil, err
	}
	return ConvertBatchResult(batchResult), nil
}

// RunRestorableWithBudget runs all operations with backup enabled using a custom budget.
func (b *Batch) RunRestorableWithBudget(maxBackupMB int) (*Result, error) {
	batchResult, err := b.impl.RunRestorableWithBudget(maxBackupMB)
	if err != nil {
		return nil, err
	}
	return ConvertBatchResult(batchResult), nil
}

// ConvertBatchResult converts a batch package result to main package Result
func ConvertBatchResult(batchResult interface{}) *Result {
	// Extract fields from the batch result interface
	type resultGetter interface {
		IsSuccess() bool
		GetOperations() []interface{}
		GetRestoreOps() []interface{} 
		GetDuration() interface{}
		GetError() error
		GetBudget() interface{}
		GetRollback() interface{}
	}
	
	r, ok := batchResult.(resultGetter)
	if !ok {
		return nil
	}
	
	// Convert operations from []interface{} to []OperationResult
	var operationResults []OperationResult
	for _, op := range r.GetOperations() {
		// The operations are core.OperationResult from execution package
		if coreOpResult, ok := op.(*core.OperationResult); ok {
			// Extract the actual operation from the result
			if actualOp := coreOpResult.Operation; actualOp != nil {
				// Try to get the original operation from adapters
				var origOp Operation
				if adapter, ok := actualOp.(interface{ GetOriginalOperation() interface{} }); ok {
					if o, ok := adapter.GetOriginalOperation().(Operation); ok {
						origOp = o
					}
				} else if mainOp, ok := actualOp.(Operation); ok {
					origOp = mainOp
				}
				
				if origOp != nil {
					opResult := OperationResult{
						OperationID: coreOpResult.OperationID,
						Operation:   origOp,
						Status:      coreOpResult.Status,
						Error:       coreOpResult.Error,
						Duration:    coreOpResult.Duration,
						BackupData:  coreOpResult.BackupData,
						BackupSizeMB: coreOpResult.BackupSizeMB,
					}
					operationResults = append(operationResults, opResult)
				}
			}
		} else if coreOpResult, ok := op.(core.OperationResult); ok {
			// Handle non-pointer version
			if actualOp := coreOpResult.Operation; actualOp != nil {
				// Try to get the original operation from adapters
				var origOp Operation
				if adapter, ok := actualOp.(interface{ GetOriginalOperation() interface{} }); ok {
					if o, ok := adapter.GetOriginalOperation().(Operation); ok {
						origOp = o
					}
				} else if mainOp, ok := actualOp.(Operation); ok {
					origOp = mainOp
				}
				
				if origOp != nil {
					opResult := OperationResult{
						OperationID: coreOpResult.OperationID,
						Operation:   origOp,
						Status:      coreOpResult.Status,
						Error:       coreOpResult.Error,
						Duration:    coreOpResult.Duration,
						BackupData:  coreOpResult.BackupData,
						BackupSizeMB: coreOpResult.BackupSizeMB,
					}
					operationResults = append(operationResults, opResult)
				}
			}
		}
	}
	
	// Convert restore operations
	var restoreOps []Operation
	for _, op := range r.GetRestoreOps() {
		if mainOp, ok := op.(Operation); ok {
			restoreOps = append(restoreOps, mainOp)
		}
	}
	
	// Extract duration
	var duration time.Duration
	if d, ok := r.GetDuration().(time.Duration); ok {
		duration = d
	}
	
	// Extract budget
	var budget *BackupBudget
	if b := r.GetBudget(); b != nil {
		if bg, ok := b.(*core.BackupBudget); ok {
			budget = (*BackupBudget)(bg)
		}
	}
	
	// Convert rollback function signature
	var rollback func(context.Context) error
	if rb := r.GetRollback(); rb != nil {
		// Try func(context.Context) error first
		if rbFunc, ok := rb.(func(context.Context) error); ok {
			rollback = rbFunc
		} else if rbFunc, ok := rb.(func() error); ok {
			// Fall back to func() error
			rollback = func(ctx context.Context) error {
				return rbFunc()
			}
		}
	}
	
	// Create result
	result := &Result{
		Success:    r.IsSuccess(),
		Operations: operationResults,
		RestoreOps: restoreOps,
		Duration:   duration,
		Errors:     []error{},
		Budget:     budget,
		Rollback:   rollback,
	}
	
	// Add error if present
	if err := r.GetError(); err != nil {
		result.Errors = append(result.Errors, err)
	}
	
	// If the batch failed, mark operations as failed
	if !r.IsSuccess() && len(result.Errors) > 0 {
		for i := range result.Operations {
			if result.Operations[i].Status == StatusSuccess {
				result.Operations[i].Status = StatusFailure
				result.Operations[i].Error = result.Errors[0]
			}
		}
	}
	
	return result
}