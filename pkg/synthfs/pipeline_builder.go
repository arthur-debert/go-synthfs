package synthfs

import (
	"context"
)

// PipelineBuilder provides a fluent API for building and executing pipelines
type PipelineBuilder struct {
	pipeline     Pipeline
	dependencies map[OperationID][]OperationID
	lastOp       Operation
}

// BuildPipeline creates a new pipeline with the given operations
func BuildPipeline(ops ...Operation) *PipelineBuilder {
	pb := &PipelineBuilder{
		pipeline:     NewMemPipeline(),
		dependencies: make(map[OperationID][]OperationID),
	}
	
	for _, op := range ops {
		if err := pb.pipeline.Add(op); err == nil {
			pb.lastOp = op
		}
	}
	
	return pb
}

// NewPipelineBuilder creates a new empty pipeline builder
func NewPipelineBuilder() *PipelineBuilder {
	return &PipelineBuilder{
		pipeline:     NewMemPipeline(),
		dependencies: make(map[OperationID][]OperationID),
	}
}

// Add adds an operation to the pipeline
func (pb *PipelineBuilder) Add(op Operation) *PipelineBuilder {
	if err := pb.pipeline.Add(op); err == nil {
		pb.lastOp = op
	}
	return pb
}

// After specifies that the last added operation depends on the given operations
func (pb *PipelineBuilder) After(deps ...Operation) *PipelineBuilder {
	if pb.lastOp != nil {
		for _, dep := range deps {
			pb.lastOp.AddDependency(dep.ID())
		}
	}
	return pb
}

// WithDependency adds a dependency between two operations
func (pb *PipelineBuilder) WithDependency(dependent, dependency Operation) *PipelineBuilder {
	dependent.AddDependency(dependency.ID())
	return pb
}

// Build returns the built pipeline
func (pb *PipelineBuilder) Build() Pipeline {
	return pb.pipeline
}

// Execute runs the pipeline with the given context and filesystem
func (pb *PipelineBuilder) Execute(ctx context.Context, fs FileSystem) (*Result, error) {
	// Resolve pipeline dependencies first
	if err := pb.pipeline.Resolve(); err != nil {
		return nil, err
	}
	
	executor := NewExecutor()
	result := executor.Run(ctx, pb.pipeline, fs)
	
	// Check for errors and return enhanced error
	for i, opResult := range result.GetOperations() {
		if opRes, ok := opResult.(OperationResult); ok && opRes.Error != nil {
			// Collect successful operations
			var successfulOps []OperationID
			for j := 0; j < i; j++ {
				if prevRes, ok := result.GetOperations()[j].(OperationResult); ok && prevRes.Error == nil {
					successfulOps = append(successfulOps, prevRes.OperationID)
				}
			}
			
			ops := pb.pipeline.Operations()
			if i < len(ops) {
				return result, &PipelineError{
					FailedOp:      ops[i],
					FailedIndex:   i + 1,
					TotalOps:      len(ops),
					Err:           opRes.Error,
					SuccessfulOps: successfulOps,
				}
			}
			return result, opRes.Error
		}
	}
	
	return result, nil
}

// ExecuteWith runs the pipeline with a custom executor
func (pb *PipelineBuilder) ExecuteWith(ctx context.Context, fs FileSystem, executor *Executor) (*Result, error) {
	result := executor.Run(ctx, pb.pipeline, fs)
	
	// Check for errors (same as Execute)
	for i, opResult := range result.GetOperations() {
		if opRes, ok := opResult.(OperationResult); ok && opRes.Error != nil {
			var successfulOps []OperationID
			for j := 0; j < i; j++ {
				if prevRes, ok := result.GetOperations()[j].(OperationResult); ok && prevRes.Error == nil {
					successfulOps = append(successfulOps, prevRes.OperationID)
				}
			}
			
			ops := pb.pipeline.Operations()
			if i < len(ops) {
				return result, &PipelineError{
					FailedOp:      ops[i],
					FailedIndex:   i + 1,
					TotalOps:      len(ops),
					Err:           opRes.Error,
					SuccessfulOps: successfulOps,
				}
			}
			return result, opRes.Error
		}
	}
	
	return result, nil
}

// WithOptions sets pipeline options and executes
func (pb *PipelineBuilder) WithOptions(options PipelineOptions) *PipelineExecutor {
	return &PipelineExecutor{
		pipeline: pb.pipeline,
		options:  options,
	}
}

// PipelineExecutor handles execution with options
type PipelineExecutor struct {
	pipeline Pipeline
	options  PipelineOptions
}

// Execute runs the pipeline with the configured options
func (pe *PipelineExecutor) Execute(ctx context.Context, fs FileSystem) (*Result, error) {
	executor := NewExecutor()
	result := executor.RunWithOptions(ctx, pe.pipeline, fs, pe.options)
	
	// Check for errors
	for i, opResult := range result.GetOperations() {
		if opRes, ok := opResult.(OperationResult); ok && opRes.Error != nil {
			var successfulOps []OperationID
			for j := 0; j < i; j++ {
				if prevRes, ok := result.GetOperations()[j].(OperationResult); ok && prevRes.Error == nil {
					successfulOps = append(successfulOps, prevRes.OperationID)
				}
			}
			
			ops := pe.pipeline.Operations()
			if i < len(ops) {
				return result, &PipelineError{
					FailedOp:      ops[i],
					FailedIndex:   i + 1,
					TotalOps:      len(ops),
					Err:           opRes.Error,
					SuccessfulOps: successfulOps,
				}
			}
			return result, opRes.Error
		}
	}
	
	return result, nil
}