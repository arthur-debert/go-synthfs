package synthfs

import (
	"context"

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
	// Use direct execution instead of adapters
	ops := pipeline.Operations()
	result, err := RunWithOptions(ctx, fs, opts, ops...)
	if err != nil {
		// If RunWithOptions returned an error, create a result with that error
		if result == nil {
			result = &Result{
				Success: false,
				Errors:  []error{err},
			}
		}
	}
	return result
}

