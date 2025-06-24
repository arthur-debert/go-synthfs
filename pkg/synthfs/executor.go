package synthfs

import (
	"context"
	"fmt"
	"time"
)

// OperationStatus indicates the outcome of an individual operation's execution.
type OperationStatus string

const (
	StatusSuccess    OperationStatus = "SUCCESS"
	StatusFailure    OperationStatus = "FAILURE"
	StatusValidation OperationStatus = "VALIDATION_FAILURE"
)

// OperationResult holds the outcome of a single operation's execution.
type OperationResult struct {
	OperationID OperationID
	Operation   Operation // The operation that was executed
	Status      OperationStatus
	Error       error
	Duration    time.Duration
}

// Result holds the overall outcome of running a pipeline of operations.
type Result struct {
	Success    bool              // True if all operations were successful
	Operations []OperationResult // Results for each operation attempted
	Duration   time.Duration
	Errors     []error                     // Aggregated errors from operations that failed
	Rollback   func(context.Context) error // Rollback function for failed transactions
}

// Executor processes a pipeline of operations.
type Executor struct{}

// NewExecutor creates a new Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Run runs all operations in the pipeline.
//
// Behavior:
// - Resolves dependencies using topological sort
// - Validates all operations before execution
// - Executes operations in dependency order
// - Continues execution even if individual operations fail
// - Returns a Result with success/failure status and rollback function
// - Caller is responsible for calling Rollback if desired
func (e *Executor) Run(ctx context.Context, pipeline Pipeline, fs FileSystem) *Result {
	Logger().Info().
		Int("operation_count", len(pipeline.Operations())).
		Msg("starting execution")

	start := time.Now()
	result := &Result{
		Operations: []OperationResult{},
		Errors:     []error{},
		Success:    true,
	}

	// Resolve dependencies first
	Logger().Info().Msg("resolving operation dependencies")
	if err := pipeline.Resolve(); err != nil {
		Logger().Info().Err(err).Msg("dependency resolution failed")
		result.Success = false
		result.Errors = append(result.Errors, fmt.Errorf("dependency resolution failed: %w", err))
		result.Duration = time.Since(start)
		return result
	}
	Logger().Info().Msg("dependency resolution completed successfully")

	// Validate the pipeline
	Logger().Info().Msg("validating operation pipeline")
	if err := pipeline.Validate(ctx, fs); err != nil {
		Logger().Info().Err(err).Msg("pipeline validation failed")
		result.Success = false
		result.Errors = append(result.Errors, fmt.Errorf("pipeline validation failed: %w", err))
		result.Duration = time.Since(start)
		return result
	}
	Logger().Info().Msg("pipeline validation completed successfully")

	operations := pipeline.Operations()
	rollbackOps := make([]Operation, 0, len(operations))

	Logger().Info().
		Int("operations_to_execute", len(operations)).
		Msg("beginning operation execution")

	// Execute operations
	for i, op := range operations {
		Logger().Info().
			Str("op_id", string(op.ID())).
			Str("op_type", op.Describe().Type).
			Str("path", op.Describe().Path).
			Int("operation_index", i+1).
			Int("total_operations", len(operations)).
			Msg("executing operation")

		opStart := time.Now()
		err := op.Execute(ctx, fs)
		opDuration := time.Since(opStart)

		opResult := OperationResult{
			OperationID: op.ID(),
			Operation:   op,
			Duration:    opDuration,
		}

		if err != nil {
			Logger().Info().
				Str("op_id", string(op.ID())).
				Str("op_type", op.Describe().Type).
				Str("path", op.Describe().Path).
				Err(err).
				Dur("duration", opDuration).
				Msg("operation execution failed")

			opResult.Status = StatusFailure
			opResult.Error = err
			result.Success = false
			result.Errors = append(result.Errors, fmt.Errorf("operation %s failed: %w", op.ID(), err))
		} else {
			Logger().Info().
				Str("op_id", string(op.ID())).
				Str("op_type", op.Describe().Type).
				Str("path", op.Describe().Path).
				Dur("duration", opDuration).
				Msg("operation execution completed successfully")

			opResult.Status = StatusSuccess
			rollbackOps = append(rollbackOps, op)
		}

		result.Operations = append(result.Operations, opResult)
	}

	result.Duration = time.Since(start)
	result.Rollback = e.createRollbackFunc(rollbackOps, fs)

	Logger().Info().
		Bool("success", result.Success).
		Int("total_operations", len(operations)).
		Int("successful_operations", len(rollbackOps)).
		Int("failed_operations", len(result.Errors)).
		Dur("total_duration", result.Duration).
		Msg("execution completed")

	return result
}

// createRollbackFunc creates a rollback function that can undo executed operations.
func (e *Executor) createRollbackFunc(executedOps []Operation, fsys FileSystem) func(context.Context) error {
	if len(executedOps) == 0 {
		return func(ctx context.Context) error { return nil }
	}

	return func(ctx context.Context) error {
		// Rollback in reverse order
		var rollbackErrors []error
		for i := len(executedOps) - 1; i >= 0; i-- {
			op := executedOps[i]
			if err := op.Rollback(ctx, fsys); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("rollback failed for operation %s: %w", op.ID(), err))
			}
		}

		if len(rollbackErrors) > 0 {
			return fmt.Errorf("rollback errors: %v", rollbackErrors)
		}
		return nil
	}
}
