package synthfs

import (
	"context"
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

// Result holds the overall outcome of running a pipeline of operations
type Result struct {
	Success    bool
	Operations []OperationResult
	Duration   time.Duration
	Errors     []error
	Rollback   func(context.Context) error
	Budget     *BackupBudget
	RestoreOps []Operation
}

// DefaultPipelineOptions returns sensible defaults for pipeline execution
func DefaultPipelineOptions() PipelineOptions {
	return execution.DefaultPipelineOptions()
}

// Executor processes a pipeline of operations.
type Executor struct {
	executor *execution.Executor
}

// NewExecutor creates a new Executor.
func NewExecutor() *Executor {
	return &Executor{
		executor: execution.NewExecutor(NewLoggerAdapter(Logger())),
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
	result := &Result{
		Success:  coreResult.Success,
		Duration: coreResult.Duration,
		Errors:   coreResult.Errors,
		Budget:   coreResult.Budget,
		Rollback: coreResult.Rollback,
	}

	// Convert operation results
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
		} else if wrapper, ok := coreOpResult.Operation.(*operationWrapper); ok {
			opResult.Operation = wrapper.op
		}

		result.Operations = append(result.Operations, opResult)
	}

	// Convert restore operations
	for _, coreRestoreOp := range coreResult.RestoreOps {
		if op, ok := coreRestoreOp.(Operation); ok {
			result.RestoreOps = append(result.RestoreOps, op)
		}
	}

	return result
}

// pipelineWrapper wraps Pipeline to implement execution.PipelineInterface
type pipelineWrapper struct {
	pipeline Pipeline
}

func (pw *pipelineWrapper) Operations() []execution.OperationInterface {
	ops := pw.pipeline.Operations()
	var result []execution.OperationInterface
	for _, op := range ops {
		result = append(result, &operationWrapper{op: op})
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

// operationWrapper wraps Operation to implement execution.OperationInterface
type operationWrapper struct {
	op Operation
}

func (ow *operationWrapper) ID() core.OperationID {
	return ow.op.ID()
}

func (ow *operationWrapper) Describe() core.OperationDesc {
	return ow.op.Describe()
}

func (ow *operationWrapper) Dependencies() []core.OperationID {
	return ow.op.Dependencies()
}

func (ow *operationWrapper) Conflicts() []core.OperationID {
	return ow.op.Conflicts()
}

func (ow *operationWrapper) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Delegate to the original operation's ExecuteV2 if available, otherwise Execute
	if execV2Op, ok := ow.op.(interface {
		ExecuteV2(interface{}, *core.ExecutionContext, interface{}) error
	}); ok {
		return execV2Op.ExecuteV2(ctx, execCtx, fsys)
	}

	// Fallback to original Execute method
	if contextOp, ok := ctx.(context.Context); ok {
		if fsysOp, ok := fsys.(FileSystem); ok {
			return ow.op.Execute(contextOp, fsysOp)
		}
	}
	return nil
}

func (ow *operationWrapper) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Delegate to the original operation's ValidateV2 if available, otherwise Validate
	if validateV2Op, ok := ow.op.(interface {
		ValidateV2(interface{}, *core.ExecutionContext, interface{}) error
	}); ok {
		return validateV2Op.ValidateV2(ctx, execCtx, fsys)
	}

	// Fallback to original Validate method
	if contextOp, ok := ctx.(context.Context); ok {
		if fsysOp, ok := fsys.(FileSystem); ok {
			return ow.op.Validate(contextOp, fsysOp)
		}
	}
	return nil
}

func (ow *operationWrapper) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
	// Delegate to the original operation's ReverseOps
	if fsysOp, ok := fsys.(FileSystem); ok {
		reverseOps, backupData, err := ow.op.ReverseOps(ctx, fsysOp, budget)

		// Convert []Operation to []interface{} even if there's an error
		// This preserves partial backup data in case of budget exhaustion
		var result []interface{}
		for _, op := range reverseOps {
			result = append(result, op)
		}
		return result, backupData, err
	}
	return nil, nil, nil
}

func (ow *operationWrapper) Rollback(ctx context.Context, fsys interface{}) error {
	// Delegate to the original operation's Rollback
	if fsysOp, ok := fsys.(FileSystem); ok {
		return ow.op.Rollback(ctx, fsysOp)
	}
	return nil
}

func (ow *operationWrapper) GetItem() interface{} {
	return ow.op.GetItem()
}

func (ow *operationWrapper) GetSrcPath() string {
	if adapter, ok := ow.op.(*OperationsPackageAdapter); ok {
		src, _ := adapter.opsOperation.GetPaths()
		return src
	}
	if simpleOp, ok := ow.op.(*SimpleOperation); ok {
		return simpleOp.GetSrcPath()
	}
	return ""
}

func (ow *operationWrapper) GetDstPath() string {
	if adapter, ok := ow.op.(*OperationsPackageAdapter); ok {
		_, dst := adapter.opsOperation.GetPaths()
		return dst
	}
	if simpleOp, ok := ow.op.(*SimpleOperation); ok {
		return simpleOp.GetDstPath()
	}
	return ""
}
