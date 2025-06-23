package synthfs

import (
	"context"
	"fmt"
	"time"
)

// ExecuteConfig holds configuration options for the executor.
type ExecuteConfig struct {
	DryRun bool // If true, operations should not make actual changes.
	// Transactional and Concurrency options will be added in later phases.
}

// ExecuteOption defines a function that modifies ExecuteConfig.
type ExecuteOption func(*ExecuteConfig)

// WithDryRun sets the dry-run mode for execution.
// In dry-run mode, operations should be validated but not executed.
// The current basic operations do not fully support a sophisticated dry run
// beyond their Validate method. A true dry run might involve more detailed simulation.
func WithDryRun(enabled bool) ExecuteOption {
	return func(config *ExecuteConfig) {
		config.DryRun = enabled
	}
}

// OperationStatus indicates the outcome of an individual operation's execution.
type OperationStatus string

const (
	StatusSuccess    OperationStatus = "SUCCESS"
	StatusFailure    OperationStatus = "FAILURE"
	StatusSkipped    OperationStatus = "SKIPPED" // e.g., due to dry run or unmet dependencies
	StatusValidation OperationStatus = "VALIDATION_FAILURE"
)

// OperationResult holds the outcome of a single operation's execution.
type OperationResult struct {
	OperationID OperationID
	Operation   Operation // The operation that was executed
	Status      OperationStatus
	Error       error
	// Metrics     OperationMetrics // To be added later
	Duration time.Duration
}

// Result holds the overall outcome of executing a queue of operations.
type Result struct {
	Success    bool              // True if all operations were successful (or skipped appropriately)
	Operations []OperationResult // Results for each operation attempted
	Duration   time.Duration
	Errors     []error                     // Aggregated errors from operations that failed
	Rollback   func(context.Context) error // Rollback function for failed transactions
}

// Executor processes a queue of operations.
type Executor struct {
	// Executor might have its own configuration or state in the future,
	// e.g., default concurrency levels, logger.
}

// NewExecutor creates a new Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute executes all operations in the queue.
//
// Current transactional behavior:
// - If an error occurs during an individual operation's Execute method, the executor will log the error
//   and mark that operation as failed in the results.
// - However, the executor will CONTINUE to process subsequent operations in the queue.
// - After attempting all operations, a `Rollback` function is provided in the `Result`.
//   This function, if called, will attempt to roll back all operations that *successfully completed*
//   execution during this run.
// - Rollback is NOT automatically invoked by the executor upon encountering an error. The caller
//   is responsible for deciding whether to call the returned `Rollback` function based on the `Result`.
//
// Future enhancements might include options for stricter transactional atomicity (e.g., stop on first error
// and automatically attempt rollback).
func (e *Executor) Execute(ctx context.Context, queue Queue, fs FileSystem, opts ...ExecuteOption) *Result {
	config := &ExecuteConfig{}
	for _, opt := range opts {
		opt(config)
	}

	Logger().Trace().
		Interface("execute_config", config).
		Str("context", fmt.Sprintf("%+v", ctx)).
		Str("filesystem_type", fmt.Sprintf("%T", fs)).
		Msg("execute called with full context")

	Logger().Info().
		Int("operation_count", len(queue.Operations())).
		Bool("dry_run", config.DryRun).
		Msg("starting execution")

	start := time.Now()
	result := &Result{
		Operations: []OperationResult{},
		Errors:     []error{},
		Success:    true,
	}

	// Resolve dependencies first
	Logger().Info().Msg("resolving operation dependencies")
	if err := queue.Resolve(); err != nil {
		Logger().Trace().
			Interface("queue_operations", queue.Operations()).
			Err(err).
			Msg("dependency resolution failed - full queue state")

		Logger().Info().
			Err(err).
			Msg("dependency resolution failed")
		result.Success = false
		result.Errors = append(result.Errors, fmt.Errorf("dependency resolution failed: %w", err))
		result.Duration = time.Since(start)
		return result
	}
	Logger().Info().Msg("dependency resolution completed successfully")

	// Log the resolved queue state at trace level
	resolvedOps := queue.Operations()
	Logger().Trace().
		Int("resolved_operation_count", len(resolvedOps)).
		Interface("resolved_operations", func() []map[string]interface{} {
			var ops []map[string]interface{}
			for i, op := range resolvedOps {
				ops = append(ops, map[string]interface{}{
					"index":        i,
					"id":           string(op.ID()),
					"type":         op.Describe().Type,
					"path":         op.Describe().Path,
					"details":      op.Describe().Details,
					"dependencies": op.Dependencies(),
					"conflicts":    op.Conflicts(),
				})
			}
			return ops
		}()).
		Msg("complete resolved queue state")

	// Validate the queue
	Logger().Info().Msg("validating operation queue")
	if err := queue.Validate(ctx, fs); err != nil {
		Logger().Trace().
			Interface("validation_context", map[string]interface{}{
				"queue_state":      resolvedOps,
				"filesystem_type":  fmt.Sprintf("%T", fs),
				"validation_error": err.Error(),
				"error_type":       fmt.Sprintf("%T", err),
			}).
			Msg("queue validation failed - complete context dump")

		Logger().Debug().
			Err(err).
			Str("error_type", fmt.Sprintf("%T", err)).
			Msg("analyzing queue validation failure")

		// Debug: analyze the type of validation error for better understanding
		switch e := err.(type) {
		case *ValidationError:
			Logger().Debug().
				Str("failed_op_id", string(e.Operation.ID())).
				Str("failed_op_type", e.Operation.Describe().Type).
				Str("failed_op_path", e.Operation.Describe().Path).
				Str("validation_reason", e.Reason).
				Msg("individual operation validation failed")
		case *DependencyError:
			Logger().Debug().
				Str("failed_op_id", string(e.Operation.ID())).
				Interface("required_dependencies", e.Dependencies).
				Interface("missing_dependencies", e.Missing).
				Msg("dependency validation failed")
		case *ConflictError:
			Logger().Debug().
				Str("failed_op_id", string(e.Operation.ID())).
				Interface("conflicting_operations", e.Conflicts).
				Msg("conflict validation failed")
		default:
			Logger().Debug().
				Str("error_details", err.Error()).
				Msg("unknown validation error type")
		}

		Logger().Info().
			Err(err).
			Msg("queue validation failed")
		result.Success = false
		result.Errors = append(result.Errors, fmt.Errorf("queue validation failed: %w", err))
		result.Duration = time.Since(start)
		return result
	}
	Logger().Info().Msg("queue validation completed successfully")

	operations := queue.Operations()
	rollbackOps := make([]Operation, 0, len(operations))

	if config.DryRun {
		Logger().Trace().
			Interface("dry_run_operations", func() []map[string]interface{} {
				var ops []map[string]interface{}
				for _, op := range operations {
					ops = append(ops, map[string]interface{}{
						"id":           string(op.ID()),
						"type":         op.Describe().Type,
						"path":         op.Describe().Path,
						"details":      op.Describe().Details,
						"dependencies": op.Dependencies(),
						"conflicts":    op.Conflicts(),
					})
				}
				return ops
			}()).
			Msg("dry-run mode - complete operations that will be skipped")

		Logger().Info().Msg("executing in dry-run mode - no actual changes will be made")
		for _, op := range operations {
			Logger().Info().
				Str("op_id", string(op.ID())).
				Str("op_type", op.Describe().Type).
				Str("path", op.Describe().Path).
				Msg("dry-run: skipping operation execution")

			opResult := OperationResult{
				OperationID: op.ID(),
				Operation:   op,
				Status:      StatusSkipped,
				Duration:    0,
			}
			result.Operations = append(result.Operations, opResult)
		}
		result.Duration = time.Since(start)
		Logger().Info().
			Dur("total_duration", result.Duration).
			Int("operations_processed", len(operations)).
			Msg("dry-run execution completed")
		return result
	}

	Logger().Info().
		Int("operations_to_execute", len(operations)).
		Msg("beginning operation execution")

	// Execute operations
	for i, op := range operations {
		Logger().Trace().
			Interface("operation_full_context", map[string]interface{}{
				"index":           i,
				"total":           len(operations),
				"id":              string(op.ID()),
				"type":            op.Describe().Type,
				"path":            op.Describe().Path,
				"details":         op.Describe().Details,
				"dependencies":    op.Dependencies(),
				"conflicts":       op.Conflicts(),
				"executed_so_far": len(rollbackOps),
				"remaining":       len(operations) - i,
				"filesystem_type": fmt.Sprintf("%T", fs),
			}).
			Msg("about to execute operation - complete context")

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
			Logger().Trace().
				Interface("operation_failure_context", map[string]interface{}{
					"operation": map[string]interface{}{
						"id":           string(op.ID()),
						"type":         op.Describe().Type,
						"path":         op.Describe().Path,
						"details":      op.Describe().Details,
						"dependencies": op.Dependencies(),
						"conflicts":    op.Conflicts(),
					},
					"error":        err.Error(),
					"error_type":   fmt.Sprintf("%T", err),
					"duration":     opDuration,
					"executed_ops": len(rollbackOps),
					"rollback_ops": func() []string {
						var ids []string
						for _, rop := range rollbackOps {
							ids = append(ids, string(rop.ID()))
						}
						return ids
					}(),
					"filesystem_type": fmt.Sprintf("%T", fs),
				}).
				Msg("operation execution failed - complete failure context")

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
			Logger().Trace().
				Interface("operation_success_context", map[string]interface{}{
					"operation": map[string]interface{}{
						"id":      string(op.ID()),
						"type":    op.Describe().Type,
						"path":    op.Describe().Path,
						"details": op.Describe().Details,
					},
					"duration":        opDuration,
					"executed_ops":    len(rollbackOps) + 1,
					"filesystem_type": fmt.Sprintf("%T", fs),
				}).
				Msg("operation execution succeeded - complete success context")

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
