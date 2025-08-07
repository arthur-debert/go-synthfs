package synthfs

import (
	"context"
	"fmt"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
)

// PipelineOptions controls how operations are executed
type PipelineOptions = core.PipelineOptions

// OperationResult holds the outcome of a single operation's execution
type OperationResult struct {
	OperationID  OperationID
	Operation    Operation // The operation that was executed
	Status       OperationStatus
	Error        error
	Duration     time.Duration
	BackupData   *BackupData
	BackupSizeMB float64
}

// DefaultPipelineOptions returns a new PipelineOptions with default values.
func DefaultPipelineOptions() PipelineOptions {
	return PipelineOptions{
		DryRun:                 false,
		RollbackOnError:        false,
		ContinueOnError:        false,
		Restorable:             false,
		MaxBackupSizeMB:        10,
		ResolvePrerequisites:   true,
		UseSimpleBatch:         true,
	}
}

// Executor processes a pipeline of operations.
type Executor struct {
	executor *execution.Executor
}

// NewExecutor creates a new Executor.
func NewExecutor() *Executor {
	logger := DefaultLogger()
	return &Executor{
		executor: execution.NewExecutor(NewLoggerAdapter(&logger)),
	}
}

// EventBus returns the executor's event bus for subscription
func (e *Executor) EventBus() core.EventBus {
	return e.executor.EventBus()
}

// Run runs all operations in the pipeline with default options.
func (e *Executor) Run(ctx context.Context, pipeline Pipeline, fs FileSystem) *Result {
	return e.RunWithOptions(ctx, pipeline, fs, DefaultPipelineOptions())
}

// RunWithOptions runs all operations in the pipeline with specified options.
func (e *Executor) RunWithOptions(ctx context.Context, pipeline Pipeline, fs FileSystem, opts PipelineOptions) *Result {
	// Create pipeline wrapper
	wrapper := &pipelineWrapper{pipeline: pipeline}
	coreResult := e.executor.RunWithOptions(ctx, wrapper, fs, opts)

	// Convert core.Result back to main package Result
	return e.convertResult(coreResult)
}

// convertResult converts core.Result to main package Result
func (e *Executor) convertResult(coreResult *core.Result) *Result {
	// Create operations list
	operations := make([]interface{}, 0, len(coreResult.Operations))
	for _, coreOpResult := range coreResult.Operations {
		opResult := OperationResult{
			OperationID:  coreOpResult.OperationID,
			Status:       coreOpResult.Status,
			Error:        coreOpResult.Error,
			Duration:     coreOpResult.Duration,
			BackupData:   coreOpResult.BackupData,
			BackupSizeMB: coreOpResult.BackupSizeMB,
		}

		// Convert operation from interface{} back to Operation
		if op, ok := coreOpResult.Operation.(Operation); ok {
			opResult.Operation = op
		} else if adapter, ok := coreOpResult.Operation.(*operationInterfaceAdapter); ok {
			opResult.Operation = adapter.Operation
		}

		operations = append(operations, opResult)
	}

	// Convert restore operations
	restoreOps := make([]interface{}, 0, len(coreResult.RestoreOps))
	restoreOps = append(restoreOps, coreResult.RestoreOps...)

	// Get first error if any and wrap it appropriately
	var firstErr error
	if len(coreResult.Errors) > 0 {
		firstErr = coreResult.Errors[0]

		// Check if this is a rollback error by type assertion
		if rollbackErr, ok := firstErr.(*core.RollbackError); ok {
			// Find which operation failed
			var failedOp Operation
			var failedIndex int
			for i, opResult := range coreResult.Operations {
				if opResult.Status == StatusFailure {
					failedIndex = i + 1
					if op, ok := opResult.Operation.(Operation); ok {
						failedOp = op
					} else if adapter, ok := opResult.Operation.(*operationInterfaceAdapter); ok {
						failedOp = adapter.Operation
					}
					break
				}
			}

			// Create a rich RollbackError
			rbErr := &RollbackError{
				OriginalErr:  rollbackErr.OriginalErr,
				RollbackErrs: make(map[OperationID]error),
			}
			// For now, we don't have per-operation rollback errors, so we just add the aggregate.
			// This can be improved later if the core executor provides more granular errors.
			if len(rollbackErr.RollbackErrs) > 0 {
				rbErr.RollbackErrs["rollback"] = rollbackErr.RollbackErrs[0]
			}

			// Wrap in PipelineError
			pipelineErr := &PipelineError{
				FailedOp:      failedOp,
				FailedIndex:   failedIndex,
				TotalOps:      len(coreResult.Operations),
				Err:           rbErr,
				SuccessfulOps: make([]OperationID, 0),
			}

			// Add successful operation IDs
			for i, opResult := range coreResult.Operations {
				if i >= failedIndex-1 {
					break
				}
				if opResult.Status == StatusSuccess {
					pipelineErr.SuccessfulOps = append(pipelineErr.SuccessfulOps, opResult.OperationID)
				}
			}
			firstErr = pipelineErr
		}
	}

	// Create the result using the simplified structure
	return &Result{
		success:    coreResult.Success,
		operations: operations,
		restoreOps: restoreOps,
		duration:   coreResult.Duration,
		err:        firstErr,
		budget:     coreResult.Budget,
		rollback:   coreResult.Rollback,
	}
}

// pipelineWrapper wraps Pipeline to implement execution.PipelineInterface
type pipelineWrapper struct {
	pipeline Pipeline
}

func (pw *pipelineWrapper) Add(ops ...interface{}) error {
	// Convert operations and add to pipeline
	for _, op := range ops {
		if opTyped, ok := op.(Operation); ok {
			if err := pw.pipeline.Add(opTyped); err != nil {
				return err
			}
		}
	}
	return nil
}

func (pw *pipelineWrapper) Operations() []interface{} {
	ops := pw.pipeline.Operations()
	var result []interface{}
	for _, op := range ops {
		result = append(result, &operationInterfaceAdapter{Operation: op})
	}
	return result
}

func (pw *pipelineWrapper) Resolve() error {
	return pw.pipeline.Resolve()
}

func (pw *pipelineWrapper) Validate(ctx context.Context, fs interface{}) error {
	fsys, ok := fs.(FileSystem)
	if !ok {
		return nil // Skip validation if filesystem interface doesn't match
	}
	return pw.pipeline.Validate(ctx, fsys)
}

func (pw *pipelineWrapper) ResolvePrerequisites(resolver core.PrerequisiteResolver, fs interface{}) error {
	// The main package pipeline doesn't support prerequisite resolution yet
	// This is a no-op for now
	return nil
}

// operationInterfaceAdapter implements execution.OperationInterface for synthfs.Operation
// This is a temporary adapter that will be removed once all operations
// implement the execution interface directly.
type operationInterfaceAdapter struct {
	Operation
}

// Execute adapts the concrete types to interface{} types
func (a *operationInterfaceAdapter) Execute(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	contextObj, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("expected context.Context, got %T", ctx)
	}
	fsysObj, ok := fsys.(FileSystem)
	if !ok {
		return fmt.Errorf("expected FileSystem, got %T", fsys)
	}
	return a.Operation.Execute(contextObj, execCtx, fsysObj)
}

// Validate adapts the concrete types to interface{} types  
func (a *operationInterfaceAdapter) Validate(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	contextObj, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("expected context.Context, got %T", ctx)
	}
	fsysObj, ok := fsys.(FileSystem)
	if !ok {
		return fmt.Errorf("expected FileSystem, got %T", fsys)
	}
	return a.Operation.Validate(contextObj, execCtx, fsysObj)
}

// ReverseOps adapts the return types
func (a *operationInterfaceAdapter) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
	fsysObj, ok := fsys.(FileSystem)
	if !ok {
		return nil, nil, fmt.Errorf("expected FileSystem, got %T", fsys)
	}
	
	ops, backupData, err := a.Operation.ReverseOps(ctx, fsysObj, budget)
	
	// Convert []Operation to []interface{}
	var result []interface{}
	for _, op := range ops {
		result = append(result, &operationInterfaceAdapter{Operation: op})
	}
	
	return result, backupData, err
}

// Rollback adapts the filesystem type
func (a *operationInterfaceAdapter) Rollback(ctx context.Context, fsys interface{}) error {
	fsysObj, ok := fsys.(FileSystem)
	if !ok {
		return fmt.Errorf("expected FileSystem, got %T", fsys)
	}
	return a.Operation.Rollback(ctx, fsysObj)
}

// GetItem returns the item as interface{}
func (a *operationInterfaceAdapter) GetItem() interface{} {
	return a.Operation.GetItem()
}

// GetSrcPath returns the source path for copy/move operations
func (a *operationInterfaceAdapter) GetSrcPath() string {
	if adapter, ok := a.Operation.(*OperationsPackageAdapter); ok {
		src, _ := adapter.opsOperation.GetPaths()
		return src
	}
	return ""
}

// GetDstPath returns the destination path for copy/move operations
func (a *operationInterfaceAdapter) GetDstPath() string {
	if adapter, ok := a.Operation.(*OperationsPackageAdapter); ok {
		_, dst := adapter.opsOperation.GetPaths()
		return dst
	}
	return ""
}
