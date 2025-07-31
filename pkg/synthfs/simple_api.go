package synthfs

import (
	"context"
)

// Run executes a series of operations in sequence
func Run(ctx context.Context, fs FileSystem, ops ...Operation) (*Result, error) {
	return RunWithOptions(ctx, fs, DefaultPipelineOptions(), ops...)
}

// RunWithOptions executes operations with custom options
func RunWithOptions(ctx context.Context, fs FileSystem, options PipelineOptions, ops ...Operation) (*Result, error) {
	if len(ops) == 0 {
		return &Result{
			operations: []interface{}{},
			duration:   0,
		}, nil
	}
	
	// Create pipeline
	pipeline := NewMemPipeline()
	
	// Add operations
	for _, op := range ops {
		if err := pipeline.Add(op); err != nil {
			return nil, WrapOperationError(op, "add to pipeline", err)
		}
	}
	
	// Create executor and run
	executor := NewExecutor()
	result := executor.RunWithOptions(ctx, pipeline, fs, options)
	
	// Check for errors and enhance them
	for i, opResult := range result.GetOperations() {
		if opRes, ok := opResult.(OperationResult); ok && opRes.Error != nil {
			// Enhance the error with pipeline context
			var successfulOps []OperationID
			for j := 0; j < i; j++ {
				if prevRes, ok := result.GetOperations()[j].(OperationResult); ok && prevRes.Error == nil {
					successfulOps = append(successfulOps, prevRes.OperationID)
				}
			}
			
			return result, &PipelineError{
				FailedOp:      ops[i],
				FailedIndex:   i + 1,
				TotalOps:      len(ops),
				Err:           opRes.Error,
				SuccessfulOps: successfulOps,
			}
		}
	}
	
	return result, nil
}