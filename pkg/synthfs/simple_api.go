package synthfs

import (
	"context"
	"time"
)

// Run executes a series of operations in sequence.
//
// Operations are executed in the order provided, with each operation's success required
// before proceeding. If an operation fails, subsequent operations are not executed.
// This function does not perform rollback of successful operations.
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
func Run(ctx context.Context, fs FileSystem, ops ...Operation) (*Result, error) {
	return RunWithOptions(ctx, fs, DefaultPipelineOptions(), ops...)
}

// RunWithOptions executes operations with custom options.
// Note: This simplified runner does not currently support all pipeline options (e.g., DryRun).
// It executes operations sequentially and does not perform a pre-validation step for the entire pipeline.
func RunWithOptions(ctx context.Context, fs FileSystem, options PipelineOptions, ops ...Operation) (*Result, error) {
	if len(ops) == 0 {
		return &Result{
			success:    true,
			operations: []interface{}{},
			duration:   0,
		}, nil
	}

	// For simple API, execute operations sequentially without pipeline validation
	// This allows operations like Copy to work when the source is created by a previous operation
	results := make([]interface{}, 0, len(ops))
	startTime := time.Now()

	for i, op := range ops {
		// Validate before execute
		if err := op.Validate(ctx, fs); err != nil {
			// Create failed operation result
			opResult := OperationResult{
				OperationID: op.ID(),
				Operation:   op,
				Status:      StatusValidation,
				Error:       err,
				Duration:    0,
			}

			// Get successful operations
			var successfulOps []OperationID
			for j := 0; j < i; j++ {
				if res, ok := results[j].(OperationResult); ok && res.Error == nil {
					successfulOps = append(successfulOps, res.OperationID)
				}
			}

			// Return partial result with error
			return &Result{
					success:    false,
					operations: append(results, opResult),
					duration:   time.Since(startTime),
					err:        err,
				}, &PipelineError{
					FailedOp:      op,
					FailedIndex:   i + 1,
					TotalOps:      len(ops),
					Err:           err,
					SuccessfulOps: successfulOps,
				}
		}

		// Execute operation
		startOpTime := time.Now()
		err := op.Execute(ctx, fs)
		duration := time.Since(startOpTime)

		// Create operation result
		opResult := OperationResult{
			OperationID: op.ID(),
			Operation:   op,
			Status:      StatusSuccess,
			Error:       err,
			Duration:    duration,
		}

		if err != nil {
			opResult.Status = StatusFailure

			// Get successful operations
			var successfulOps []OperationID
			for j := 0; j < i; j++ {
				if res, ok := results[j].(OperationResult); ok && res.Error == nil {
					successfulOps = append(successfulOps, res.OperationID)
				}
			}

			// Return partial result with error
			return &Result{
					success:    false,
					operations: append(results, opResult),
					duration:   time.Since(startTime),
					err:        err,
				}, &PipelineError{
					FailedOp:      op,
					FailedIndex:   i + 1,
					TotalOps:      len(ops),
					Err:           err,
					SuccessfulOps: successfulOps,
				}
		}

		results = append(results, opResult)
	}

	return &Result{
		success:    true,
		operations: results,
		duration:   time.Since(startTime),
	}, nil
}
