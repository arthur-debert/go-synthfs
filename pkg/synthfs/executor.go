package synthfs

import (
	"context"
	"fmt"
	"time"

)

// OperationStatus and related constants are defined in constants.go

// PipelineOptions controls how operations are executed (Phase III)
type PipelineOptions struct {
	Restorable      bool // Whether to enable reversible operations with backup
	MaxBackupSizeMB int  // Maximum backup size in MB (default: 10MB)
}

// DefaultPipelineOptions returns sensible defaults for pipeline execution
func DefaultPipelineOptions() PipelineOptions {
	return PipelineOptions{
		Restorable:      false,              // No backup overhead by default
		MaxBackupSizeMB: DefaultMaxBackupMB, // Default budget - perfect for config files
	}
}

// OperationResult holds the outcome of a single operation's execution.
type OperationResult struct {
	OperationID  OperationID
	Operation    Operation // The operation that was executed
	Status       OperationStatus
	Error        error
	Duration     time.Duration
	BackupData   *BackupData // Phase III: Backup data for restoration (only if restorable=true)
	BackupSizeMB float64          // Phase III: Actual backup size consumed
}

// Result holds the overall outcome of running a pipeline of operations.
type Result struct {
	Success    bool              // True if all operations were successful
	Operations []OperationResult // Results for each operation attempted
	Duration   time.Duration
	Errors     []error                     // Aggregated errors from operations that failed
	Rollback   func(context.Context) error // Rollback function for failed transactions

	// Phase III: Enhanced restoration functionality
	Budget     *BackupBudget // Backup budget information (only if restorable=true)
	RestoreOps []Operation   // Generated reverse operations for restoration
}

// Executor processes a pipeline of operations.
type Executor struct{}

// NewExecutor creates a new Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Run runs all operations in the pipeline with default options.
// This is a convenience method that calls RunWithOptions using DefaultPipelineOptions().
func (e *Executor) Run(ctx context.Context, pipeline Pipeline, fs FileSystem) *Result {
	return e.RunWithOptions(ctx, pipeline, fs, DefaultPipelineOptions())
}

// RunWithOptions runs all operations in the pipeline with specified options (Phase III).
//
// Behavior:
// - Resolves dependencies using topological sort
// - Validates all operations before execution
// - Executes operations in dependency order
// - Optionally generates backup data for restoration (if opts.Restorable=true)
// - Continues execution even if individual operations fail
// - Returns a Result with success/failure status and backup/restore information
// - Caller is responsible for calling Rollback if desired
func (e *Executor) RunWithOptions(ctx context.Context, pipeline Pipeline, fs FileSystem, opts PipelineOptions) *Result {
	Logger().Info().
		Int("operation_count", len(pipeline.Operations())).
		Bool("restorable", opts.Restorable).
		Int("max_backup_mb", opts.MaxBackupSizeMB).
		Msg("starting execution")

	start := time.Now()
	result := &Result{
		Operations: []OperationResult{},
		Errors:     []error{},
		Success:    true,
		RestoreOps: []Operation{},
	}

	// Phase III: Initialize budget if restorable mode is enabled
	var budget *BackupBudget
	if opts.Restorable {
		budget = &BackupBudget{
			TotalMB:     float64(opts.MaxBackupSizeMB),
			RemainingMB: float64(opts.MaxBackupSizeMB),
			UsedMB:      0,
		}
		result.Budget = budget

		Logger().Info().
			Float64("total_budget_mb", budget.TotalMB).
			Msg("backup budget initialized for restorable execution")
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

		// Phase III: Generate reverse operations if restorable mode is enabled
		var reverseOps []Operation
		var backupData *BackupData
		var reverseErr error

		if opts.Restorable {
			Logger().Debug().
				Str("op_id", string(op.ID())).
				Float64("remaining_budget_mb", budget.RemainingMB).
				Msg("generating reverse operations for backup")

			reverseOps, backupData, reverseErr = op.ReverseOps(ctx, fs, budget)
			if reverseErr != nil {
				Logger().Warn().
					Str("op_id", string(op.ID())).
					Err(reverseErr).
					Msg("failed to generate reverse operations - operation will execute without backup")
				// Continue execution even if reverse ops generation fails
			} else if backupData != nil {
				Logger().Debug().
					Str("op_id", string(op.ID())).
					Float64("backup_size_mb", backupData.SizeMB).
					Float64("remaining_budget_mb", budget.RemainingMB).
					Str("backup_type", backupData.BackupType).
					Msg("backup data generated successfully")
			}
		}

		opStart := time.Now()
		err := op.Execute(ctx, fs)
		opDuration := time.Since(opStart)

		opResult := OperationResult{
			OperationID:  op.ID(),
			Operation:    op,
			Duration:     opDuration,
			BackupData:   backupData,
			BackupSizeMB: 0,
		}

		if backupData != nil {
			opResult.BackupSizeMB = backupData.SizeMB
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

			// Phase III: Restore budget if operation failed and backup was created
			if opts.Restorable && backupData != nil && budget != nil {
				budget.RestoreBackup(backupData.SizeMB)
				Logger().Debug().
					Str("op_id", string(op.ID())).
					Float64("restored_budget_mb", backupData.SizeMB).
					Float64("remaining_budget_mb", budget.RemainingMB).
					Msg("restored backup budget due to operation failure")
			}
		} else {
			Logger().Info().
				Str("op_id", string(op.ID())).
				Str("op_type", op.Describe().Type).
				Str("path", op.Describe().Path).
				Dur("duration", opDuration).
				Msg("operation execution completed successfully")

			opResult.Status = StatusSuccess
			rollbackOps = append(rollbackOps, op)

			// Phase III: Add reverse operations to result if available
			if opts.Restorable && reverseOps != nil {
				result.RestoreOps = append(result.RestoreOps, reverseOps...)
				Logger().Debug().
					Str("op_id", string(op.ID())).
					Int("reverse_ops_count", len(reverseOps)).
					Msg("added reverse operations for restoration")
			}
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
		Int("restore_operations", len(result.RestoreOps)).
		Dur("total_duration", result.Duration).
		Msg("execution completed")

	// Phase III: Log budget usage summary
	if opts.Restorable && budget != nil {
		Logger().Info().
			Float64("total_budget_mb", budget.TotalMB).
			Float64("used_budget_mb", budget.UsedMB).
			Float64("remaining_budget_mb", budget.RemainingMB).
			Msg("backup budget usage summary")
	}

	return result
}

// createRollbackFunc creates a rollback function that can undo executed operations.
func (e *Executor) createRollbackFunc(executedOps []Operation, fsys FileSystem) func(context.Context) error {
	if len(executedOps) == 0 {
		return func(ctx context.Context) error { return nil }
	}

	return func(ctx context.Context) error {
		Logger().Info().Int("count", len(executedOps)).Msg("starting rollback")
		var firstErr error
		// Rollback in reverse order
		for i := len(executedOps) - 1; i >= 0; i-- {
			op := executedOps[i]
			if err := op.Rollback(ctx, fsys); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				Logger().Info().
					Str("op_id", string(op.ID())).
					Err(err).
					Msg("rollback failed for operation")
			}
		}

		if firstErr != nil {
			return fmt.Errorf("rollback errors: %w", firstErr)
		}
		return nil
	}
}
