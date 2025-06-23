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

// Execute processes the operations in the queue with dependency resolution.
func (e *Executor) Execute(ctx context.Context, queue Queue, fsys FileSystem, opts ...ExecuteOption) *Result {
	startTime := time.Now()

	config := &ExecuteConfig{}
	for _, opt := range opts {
		opt(config)
	}

	overallResult := &Result{
		Success:    true, // Assume success until a failure occurs
		Operations: []OperationResult{},
		Errors:     []error{},
	}

	// Phase 1: Resolve dependencies
	if err := queue.Resolve(); err != nil {
		overallResult.Success = false
		overallResult.Errors = append(overallResult.Errors, fmt.Errorf("dependency resolution failed: %w", err))
		overallResult.Duration = time.Since(startTime)
		return overallResult
	}

	// Phase 2: Validate the entire queue (dependencies, individual ops, conflicts)
	// Note: queue.Validate() is expected to be called after queue.Resolve()
	if err := queue.Validate(ctx, fsys); err != nil {
		overallResult.Success = false
		// Attempt to cast to specific error types to provide more granular results if needed
		// For now, append the raw error.
		// If err is a ValidationError, we might want to populate opResult for the specific op.
		overallResult.Errors = append(overallResult.Errors, fmt.Errorf("queue validation failed: %w", err))
		overallResult.Duration = time.Since(startTime)
		// TODO: If queue.Validate fails due to a specific operation,
		// we should ideally populate an OperationResult for that operation.
		// This requires queue.Validate to return more structured error information or
		// for the executor to iterate and validate if it needs to populate individual results.
		// For now, a general error is added.
		return overallResult
	}

	// Phase 3: Execute operations in dependency-resolved order
	queuedOps := queue.Operations() // Now in dependency-resolved order and validated
	executedOps := make([]Operation, 0)

	for _, op := range queuedOps {
		opStartTime := time.Now()
		opResult := OperationResult{
			OperationID: op.ID(),
			Operation:   op,
		}

		// Individual operation validation has already been done by queue.Validate().
		// We proceed to check for DryRun and then execute.

		// Check for DryRun
		if config.DryRun {
			opResult.Status = StatusSkipped
			// opResult.Error = nil; // No actual error occurred
			opResult.Duration = time.Since(opStartTime)
			overallResult.Operations = append(overallResult.Operations, opResult)
			// Dry run operations are considered "successful" in terms of queue processing flow
			continue
		}

		// Execute the operation
		execErr := op.Execute(ctx, fsys)
		opResult.Duration = time.Since(opStartTime)

		if execErr != nil {
			opResult.Status = StatusFailure
			opResult.Error = fmt.Errorf("execution failed for operation %s (%s): %w", op.ID(), op.Describe().Path, execErr)
			overallResult.Errors = append(overallResult.Errors, opResult.Error)
			overallResult.Success = false

			// Create rollback function for successfully executed operations
			overallResult.Rollback = e.createRollbackFunc(executedOps, fsys)
			// Continue processing remaining operations (don't break on failure)
		} else {
			opResult.Status = StatusSuccess
			executedOps = append(executedOps, op)
		}
		overallResult.Operations = append(overallResult.Operations, opResult)
	}

	// If all operations succeeded but we executed some, still create rollback function
	if overallResult.Success && len(executedOps) > 0 {
		overallResult.Rollback = e.createRollbackFunc(executedOps, fsys)
	}

	overallResult.Duration = time.Since(startTime)
	return overallResult
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
