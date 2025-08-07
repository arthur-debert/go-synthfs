package synthfs

import (
	"context"
	"fmt"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Run executes a series of operations in sequence.
//
// Operations are executed in the order provided, with each operation's success required
// before proceeding. If an operation fails, subsequent operations are not executed.
// By default, this function does not perform a rollback. To enable rollback,
// use RunWithOptions with RollbackOnError set to true.
//
// Example - Simple sequential operations:
//
//	fs := synthfs.NewOSFileSystem("/tmp")
//	sfs := synthfs.New()
//	
//	result, err := synthfs.Run(ctx, fs,
//		sfs.CreateDir("project", 0755),
//		sfs.CreateFile("project/README.md", []byte("# Project"), 0644),
//		sfs.CreateFile("project/main.go", []byte("package main"), 0644),
//	)
//	
//	if err != nil {
//		log.Fatal(err)
//	}
//	log.Printf("Executed %d operations in %v", len(result.Operations), result.Duration)
//
// Example - Operations with dependencies:
//
//	result, err := synthfs.Run(ctx, fs,
//		sfs.CreateDir("data", 0755),                    // Must happen first
//		sfs.CreateFile("data/config.json", data, 0644), // Depends on directory existing
//		sfs.Copy("data/config.json", "backup.json"),    // Depends on file existing
//	)
//
// For complex dependency management, consider using BuildPipeline instead.
func Run(ctx context.Context, fs filesystem.FileSystem, ops ...Operation) (*Result, error) {
	return RunWithOptions(ctx, fs, DefaultPipelineOptions(), ops...)
}

// RunWithOptions executes operations with custom options.
// This function directly executes operations without using the pipeline/adapter system,
// providing a simpler and more direct execution path.
func RunWithOptions(ctx context.Context, fs filesystem.FileSystem, options PipelineOptions, ops ...Operation) (*Result, error) {
	// For the simple API, we disable prerequisite resolution by default to allow for the straightforward,
	// ordered execution of operations without requiring explicit dependency declarations.
	options.ResolvePrerequisites = false

	if options.DryRun {
		fs = NewDryRunFS()
	}

	if len(ops) == 0 {
		return &Result{
			Success:    true,
			Operations: []core.OperationResult{},
			Duration:   0,
		}, nil
	}

	// Check for duplicate operation IDs
	idsSeen := make(map[core.OperationID]bool)
	for _, op := range ops {
		id := op.ID()
		if idsSeen[id] {
			return nil, fmt.Errorf("operation with ID '%s' already exists", id)
		}
		idsSeen[id] = true
	}

	// For the simple API, we need to validate operations with projected state
	// to support sequential operations where later ops depend on earlier ones
	projectedFS := NewProjectedFileSystem(fs)
	
	// First, validate all operations with projected state
	for _, op := range ops {
		// Validate against projected filesystem state
		if err := op.Validate(ctx, nil, projectedFS); err != nil {
			// Return a failed result with the error
			return &Result{
				Success:    false,
				Operations: []core.OperationResult{},
				Duration:   0,
				Errors:     []error{err},
			}, err
		}
		// Update projected state to reflect this operation
		if err := projectedFS.UpdateProjectedState(op); err != nil {
			// Return a failed result with the error
			return &Result{
				Success:    false,
				Operations: []core.OperationResult{},
				Duration:   0,
				Errors:     []error{err},
			}, err
		}
	}
	
	// Execute operations directly
	result, err := executeOperationsDirect(ctx, fs, options, ops)
	
	// Wrap errors to match original batch API behavior
	if !result.Success && len(result.Errors) > 0 {
		err = wrapExecutionError(result.Errors[0], result, ops)
	}

	return result, err
}

// wrapExecutionError wraps execution errors to match original batch API behavior
func wrapExecutionError(execErr error, result *Result, ops []Operation) error {
	// RollbackError should be wrapped in PipelineError to maintain expected API
	if rollbackErr, ok := execErr.(*core.RollbackError); ok {
		// Find which operation failed to create PipelineError context
		failedIndex := -1
		var failedOp Operation
		var successfulOps []core.OperationID
		
		// Determine failed operation by examining result
		for i, op := range ops {
			if i < len(result.Operations) && result.Operations[i].Status == StatusSuccess {
				successfulOps = append(successfulOps, op.ID())
			} else {
				failedIndex = i + 1 // 1-based index
				failedOp = op
				break
			}
		}
		
		// Wrap in PipelineError
		return &PipelineError{
			FailedOp:      failedOp,
			FailedIndex:   failedIndex,
			TotalOps:      len(ops),
			Err:           rollbackErr,
			SuccessfulOps: successfulOps,
		}
	}
	
	// For other error types, return as-is
	return execErr
}

// executeOperationsDirect executes operations directly without pipeline adapters
func executeOperationsDirect(ctx context.Context, fs filesystem.FileSystem, options PipelineOptions, ops []Operation) (*Result, error) {
	start := time.Now()
	
	result := &Result{
		Operations: []core.OperationResult{},
		Errors:     []error{},
		Success:    true,
		RestoreOps: []interface{}{},
	}

	// Initialize backup budget if restorable mode is enabled
	var budget *core.BackupBudget
	if options.Restorable {
		budget = &core.BackupBudget{
			TotalMB:     float64(options.MaxBackupSizeMB),
			RemainingMB: float64(options.MaxBackupSizeMB),
			UsedMB:      0,
		}
		result.Budget = budget
	}

	// Create execution context
	logger := DefaultLogger()
	execCtx := &core.ExecutionContext{
		Logger:   NewLoggerAdapter(&logger),
		Budget:   budget,
		EventBus: nil, // We can add this later if needed
	}

	// Track successful operations for rollback
	var successfulOps []Operation
	var reverseOps []interface{}

	// Execute operations
	for _, op := range ops {
		// Generate reverse operations if restorable mode is enabled
		var backupData *core.BackupData
		var reverseErr error

		if options.Restorable {
			var opReverseOps []Operation
			var backupDataInterface interface{}
			
			opReverseOps, backupDataInterface, reverseErr = op.ReverseOps(ctx, fs, budget)
			if reverseErr != nil {
				// Log warning but continue - backup is nice-to-have
			} else {
				// Convert to interface{} slice for result
				for _, revOp := range opReverseOps {
					reverseOps = append(reverseOps, revOp)
				}
				
				// Extract backup data if available
				if backupDataInterface != nil {
					if bd, ok := backupDataInterface.(*core.BackupData); ok {
						backupData = bd
					}
				}
			}
		}

		opStart := time.Now()
		err := op.Execute(ctx, execCtx, fs)
		opDuration := time.Since(opStart)

		opResult := core.OperationResult{
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
			opResult.Status = core.StatusFailure
			opResult.Error = err
			result.Success = false
			result.Errors = append(result.Errors, fmt.Errorf("operation %s failed: %w", op.ID(), err))

			// Restore budget if operation failed and backup was created
			if options.Restorable && backupData != nil && budget != nil {
				budget.RestoreBackup(backupData.SizeMB)
			}
		} else {
			opResult.Status = core.StatusSuccess
			successfulOps = append(successfulOps, op)

			// Add reverse operations to result if available
			if options.Restorable && len(reverseOps) > 0 {
				result.RestoreOps = append(result.RestoreOps, reverseOps...)
			}
		}

		result.Operations = append(result.Operations, opResult)

		// Break after recording the failed operation if we should not continue on error
		if err != nil && !options.ContinueOnError {
			break
		}
	}

	result.Duration = time.Since(start)

	// Create rollback function
	result.Rollback = func(ctx context.Context) error {
		// Reverse the order of successful operations for rollback
		for i := len(successfulOps) - 1; i >= 0; i-- {
			if rollbackErr := successfulOps[i].Rollback(ctx, fs); rollbackErr != nil {
				return rollbackErr
			}
		}
		return nil
	}

	// Execute rollback if needed
	if !result.Success && options.RollbackOnError && len(successfulOps) > 0 {
		rollbackErrors := make([]error, 0)
		
		// Reverse the order of successful operations for rollback
		for i := len(successfulOps) - 1; i >= 0; i-- {
			if rollbackErr := successfulOps[i].Rollback(ctx, fs); rollbackErr != nil {
				rollbackErrors = append(rollbackErrors, rollbackErr)
			}
		}

		// If rollback had errors, wrap them
		if len(rollbackErrors) > 0 {
			originalErr := result.Errors[0] // Get the first (main) error
			
			// Build map from operation ID to rollback error
			rollbackErrMap := make(map[core.OperationID]error)
			for i, rollbackErr := range rollbackErrors {
				if i < len(successfulOps) {
					rollbackErrMap[successfulOps[len(successfulOps)-1-i].ID()] = rollbackErr
				}
			}
			
			rollbackErr := &core.RollbackError{
				OriginalErr:  originalErr,
				RollbackErrs: rollbackErrMap,
			}
			// Replace the first error with the rollback error
			result.Errors[0] = rollbackErr
		}
	}

	return result, nil
}
