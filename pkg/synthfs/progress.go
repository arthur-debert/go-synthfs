package synthfs

import (
	"context"
	"fmt"
	"time"
)

// ProgressReporter defines the interface for progress reporting during execution
type ProgressReporter interface {
	OnStart(op Operation)
	OnProgress(op Operation, current, total int64)
	OnComplete(op Operation, result OperationResult)
	OnFinish(totalOps int, successCount int, duration time.Duration)
}

// ProgressReportingExecutor extends the basic executor with progress reporting
type ProgressReportingExecutor struct {
	*Executor
	reporter ProgressReporter
}

// NewProgressReportingExecutor creates a new executor with progress reporting
func NewProgressReportingExecutor(reporter ProgressReporter) *ProgressReportingExecutor {
	return &ProgressReportingExecutor{
		Executor: NewExecutor(),
		reporter: reporter,
	}
}

// ExecuteWithProgress executes operations with progress reporting
func (e *ProgressReportingExecutor) ExecuteWithProgress(ctx context.Context, queue Queue, fsys FileSystem, opts ...ExecuteOption) *Result {
	startTime := time.Now()

	config := &ExecuteConfig{}
	for _, opt := range opts {
		opt(config)
	}

	overallResult := &Result{
		Success:    true,
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

	// Phase 2: Check for dependency and conflict issues only
	if err := queue.Validate(ctx, fsys); err != nil {
		overallResult.Success = false
		overallResult.Errors = append(overallResult.Errors, fmt.Errorf("queue validation failed: %w", err))
		overallResult.Duration = time.Since(startTime)
		return overallResult
	}

	// Phase 3: Execute operations with progress reporting
	queuedOps := queue.Operations()
	executedOps := make([]Operation, 0)
	successCount := 0

	for i, op := range queuedOps {
		e.reporter.OnStart(op)

		opStartTime := time.Now()
		opResult := OperationResult{
			OperationID: op.ID(),
			Operation:   op,
		}

		// Validate each operation individually
		err := op.Validate(ctx, fsys)
		if err != nil {
			opResult.Status = StatusValidation
			opResult.Error = fmt.Errorf("validation failed for operation %s (%s): %w", op.ID(), op.Describe().Path, err)
			opResult.Duration = time.Since(opStartTime)
			overallResult.Operations = append(overallResult.Operations, opResult)
			overallResult.Errors = append(overallResult.Errors, opResult.Error)
			overallResult.Success = false

			e.reporter.OnComplete(op, opResult)
			continue
		}

		// Check for DryRun
		if config.DryRun {
			opResult.Status = StatusSkipped
			opResult.Error = fmt.Errorf("operation %s (%s) skipped due to dry run", op.ID(), op.Describe().Path)
			opResult.Duration = time.Since(opStartTime)
			overallResult.Operations = append(overallResult.Operations, opResult)

			e.reporter.OnComplete(op, opResult)
			continue
		}

		// Report progress
		e.reporter.OnProgress(op, int64(i+1), int64(len(queuedOps)))

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
		} else {
			opResult.Status = StatusSuccess
			executedOps = append(executedOps, op)
			successCount++
		}

		overallResult.Operations = append(overallResult.Operations, opResult)
		e.reporter.OnComplete(op, opResult)
	}

	// If all operations succeeded but we executed some, still create rollback function
	if overallResult.Success && len(executedOps) > 0 {
		overallResult.Rollback = e.createRollbackFunc(executedOps, fsys)
	}

	overallResult.Duration = time.Since(startTime)
	e.reporter.OnFinish(len(queuedOps), successCount, overallResult.Duration)

	return overallResult
}

// ExecuteStream executes operations and streams results via a channel
func (e *Executor) ExecuteStream(ctx context.Context, queue Queue, fsys FileSystem, opts ...ExecuteOption) <-chan OperationResult {
	resultChan := make(chan OperationResult, 10) // Buffered channel

	go func() {
		defer close(resultChan)

		config := &ExecuteConfig{}
		for _, opt := range opts {
			opt(config)
		}

		// Phase 1: Resolve dependencies
		if err := queue.Resolve(); err != nil {
			resultChan <- OperationResult{
				OperationID: OperationID("__dependency_resolution__"),
				Status:      StatusFailure,
				Error:       fmt.Errorf("dependency resolution failed: %w", err),
			}
			return
		}

		// Phase 2: Check for dependency and conflict issues
		if err := queue.Validate(ctx, fsys); err != nil {
			resultChan <- OperationResult{
				OperationID: OperationID("__queue_validation__"),
				Status:      StatusFailure,
				Error:       fmt.Errorf("queue validation failed: %w", err),
			}
			return
		}

		// Phase 3: Execute operations and stream results
		queuedOps := queue.Operations()
		executedOps := make([]Operation, 0)

		for _, op := range queuedOps {
			// Check for cancellation
			select {
			case <-ctx.Done():
				resultChan <- OperationResult{
					OperationID: op.ID(),
					Operation:   op,
					Status:      StatusFailure,
					Error:       ctx.Err(),
				}
				return
			default:
			}

			opStartTime := time.Now()
			opResult := OperationResult{
				OperationID: op.ID(),
				Operation:   op,
			}

			// Validate
			err := op.Validate(ctx, fsys)
			if err != nil {
				opResult.Status = StatusValidation
				opResult.Error = fmt.Errorf("validation failed for operation %s (%s): %w", op.ID(), op.Describe().Path, err)
				opResult.Duration = time.Since(opStartTime)
				resultChan <- opResult
				continue
			}

			// Check for DryRun
			if config.DryRun {
				opResult.Status = StatusSkipped
				opResult.Error = fmt.Errorf("operation %s (%s) skipped due to dry run", op.ID(), op.Describe().Path)
				opResult.Duration = time.Since(opStartTime)
				resultChan <- opResult
				continue
			}

			// Execute
			execErr := op.Execute(ctx, fsys)
			opResult.Duration = time.Since(opStartTime)

			if execErr != nil {
				opResult.Status = StatusFailure
				opResult.Error = fmt.Errorf("execution failed for operation %s (%s): %w", op.ID(), op.Describe().Path, execErr)
			} else {
				opResult.Status = StatusSuccess
				executedOps = append(executedOps, op)
			}

			resultChan <- opResult
		}
	}()

	return resultChan
}

// ConsoleProgressReporter implements ProgressReporter for console output
type ConsoleProgressReporter struct {
	verbose bool
}

// NewConsoleProgressReporter creates a new console progress reporter
func NewConsoleProgressReporter(verbose bool) *ConsoleProgressReporter {
	return &ConsoleProgressReporter{verbose: verbose}
}

// OnStart implements ProgressReporter
func (r *ConsoleProgressReporter) OnStart(op Operation) {
	if r.verbose {
		desc := op.Describe()
		fmt.Printf("Starting: %s (%s)\n", desc.Path, desc.Type)
	}
}

// OnProgress implements ProgressReporter
func (r *ConsoleProgressReporter) OnProgress(op Operation, current, total int64) {
	fmt.Printf("Progress: %d/%d (%.1f%%) - %s\n",
		current, total, float64(current)/float64(total)*100, op.ID())
}

// OnComplete implements ProgressReporter
func (r *ConsoleProgressReporter) OnComplete(op Operation, result OperationResult) {
	status := "✓"
	if result.Status != StatusSuccess {
		status = "✗"
	}

	if r.verbose || result.Status != StatusSuccess {
		fmt.Printf("%s %s (%s) - %v\n", status, result.OperationID, result.Status, result.Duration)
		if result.Error != nil {
			fmt.Printf("  Error: %v\n", result.Error)
		}
	}
}

// OnFinish implements ProgressReporter
func (r *ConsoleProgressReporter) OnFinish(totalOps int, successCount int, duration time.Duration) {
	if successCount == totalOps {
		fmt.Printf("\n✓ All %d operations completed successfully in %v\n", totalOps, duration)
	} else {
		fmt.Printf("\n✗ %d/%d operations completed successfully in %v\n", successCount, totalOps, duration)
	}
}

// NoOpProgressReporter implements ProgressReporter with no output
type NoOpProgressReporter struct{}

// NewNoOpProgressReporter creates a no-op progress reporter
func NewNoOpProgressReporter() *NoOpProgressReporter {
	return &NoOpProgressReporter{}
}

// OnStart implements ProgressReporter (no-op)
func (r *NoOpProgressReporter) OnStart(op Operation) {}

// OnProgress implements ProgressReporter (no-op)
func (r *NoOpProgressReporter) OnProgress(op Operation, current, total int64) {}

// OnComplete implements ProgressReporter (no-op)
func (r *NoOpProgressReporter) OnComplete(op Operation, result OperationResult) {}

// OnFinish implements ProgressReporter (no-op)
func (r *NoOpProgressReporter) OnFinish(totalOps int, successCount int, duration time.Duration) {}
