package synthfs

import (
	"context"
	"time"
)

// Run executes a series of operations in sequence
func Run(ctx context.Context, fs FileSystem, ops ...Operation) (*Result, error) {
	return RunWithOptions(ctx, fs, DefaultPipelineOptions(), ops...)
}

// RunWithOptions executes operations with custom options
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
				OperationID:  op.ID(),
				Operation:    op,
				Status:       StatusValidation,
				Error:        err,
				Duration:     0,
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
			OperationID:  op.ID(),
			Operation:    op,
			Status:       StatusSuccess,
			Error:        err,
			Duration:     duration,
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