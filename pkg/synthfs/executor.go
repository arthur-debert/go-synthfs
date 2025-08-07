package synthfs

import (
	"context"
	"fmt"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
)

// PipelineOptions controls how operations are executed
type PipelineOptions = core.PipelineOptions

// OperationResult is defined as a type alias in types.go

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
	return e.executor.RunWithOptions(ctx, wrapper, fs, opts)
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
	
	// Convert backupData interface{} to *core.BackupData
	var backupDataPtr *core.BackupData
	if backupData != nil {
		if bd, ok := backupData.(*core.BackupData); ok {
			backupDataPtr = bd
		}
	}
	
	return result, backupDataPtr, err
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
	src, _ := a.GetPaths()
	return src
}

// GetDstPath returns the destination path for copy/move operations
func (a *operationInterfaceAdapter) GetDstPath() string {
	_, dst := a.GetPaths()
	return dst
}
