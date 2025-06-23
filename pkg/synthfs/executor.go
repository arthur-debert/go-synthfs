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
	Success     bool              // True if all operations were successful (or skipped appropriately)
	Operations  []OperationResult // Results for each operation attempted
	Duration    time.Duration
	Errors      []error // Aggregated errors from operations that failed
	// Rollback    func(context.Context) error // To be added with transactional support
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

// Execute processes the operations in the queue sequentially.
// For Phase 1, this is a very basic execution:
// - It iterates through operations from the queue.
// - It calls Validate on each operation.
// - If DryRun is true or Validate fails, it skips Execute.
// - Otherwise, it calls Execute on the operation.
// - Dependency resolution and conflict detection are not yet implemented.
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

	queuedOps := queue.Operations() // Get all operations from the queue

	for _, op := range queuedOps {
		opStartTime := time.Now()
		opResult := OperationResult{
			OperationID: op.ID(),
			Operation:   op,
		}

		// 1. Validate the operation
		err := op.Validate(ctx, fsys)
		if err != nil {
			opResult.Status = StatusValidation
			opResult.Error = fmt.Errorf("validation failed for operation %s (%s): %w", op.ID(), op.Describe().Path, err)
			opResult.Duration = time.Since(opStartTime)
			overallResult.Operations = append(overallResult.Operations, opResult)
			overallResult.Errors = append(overallResult.Errors, opResult.Error)
			overallResult.Success = false // Validation failure means overall failure
			continue                      // Move to the next operation
		}

		// 2. Check for DryRun
		if config.DryRun {
			opResult.Status = StatusSkipped
			opResult.Error = fmt.Errorf("operation %s (%s) skipped due to dry run", op.ID(), op.Describe().Path) // Not really an error, but informative
			opResult.Duration = time.Since(opStartTime)
			overallResult.Operations = append(overallResult.Operations, opResult)
			// Dry run doesn't necessarily mean overall failure if all ops would have been valid.
			// However, if any validation failed, Success is already false.
			// For now, let's say a full dry run that passes validation is a "successful" dry run.
			continue
		}

		// 3. Execute the operation
		// In later phases, dependency checks would happen before this.
		execErr := op.Execute(ctx, fsys)
		opResult.Duration = time.Since(opStartTime)

		if execErr != nil {
			opResult.Status = StatusFailure
			opResult.Error = fmt.Errorf("execution failed for operation %s (%s): %w", op.ID(), op.Describe().Path, execErr)
			overallResult.Errors = append(overallResult.Errors, opResult.Error)
			overallResult.Success = false
			// Rollback logic will be added in Phase 2 for transactional execution.
			// For now, we just record the failure and continue.
		} else {
			opResult.Status = StatusSuccess
		}
		overallResult.Operations = append(overallResult.Operations, opResult)
	}

	overallResult.Duration = time.Since(startTime)
	return overallResult
}
