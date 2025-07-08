package execution

import (
	"context"
	"fmt"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// Pipeline defines the interface for operation pipeline management
type Pipeline interface {
	Add(op interface{}) error
	Operations() []interface{}
	Resolve() error
	ResolvePrerequisites(resolver core.PrerequisiteResolver, fs interface{}) error
	Validate(ctx context.Context, fs interface{}) error
}

// MemPipeline is an in-memory implementation of Pipeline
type MemPipeline struct {
	operations []interface{}
	logger     core.Logger
}

// NewMemPipeline creates a new in-memory pipeline
func NewMemPipeline(logger core.Logger) Pipeline {
	return &MemPipeline{
		operations: []interface{}{},
		logger:     logger,
	}
}

// Add adds an operation to the pipeline
func (p *MemPipeline) Add(op interface{}) error {
	if op == nil {
		return fmt.Errorf("cannot add nil operation to pipeline")
	}
	
	p.operations = append(p.operations, op)
	return nil
}

// Operations returns all operations in the pipeline
func (p *MemPipeline) Operations() []interface{} {
	// Return a copy to prevent external modification
	result := make([]interface{}, len(p.operations))
	copy(result, p.operations)
	return result
}

// Resolve resolves operation dependencies
func (p *MemPipeline) Resolve() error {
	// For now, this is a no-op since dependency resolution is not yet implemented
	// This method is required by the PipelineInterface
	p.logger.Debug().Msg("dependency resolution completed (no-op)")
	return nil
}

// ResolvePrerequisites resolves operation prerequisites by creating additional operations
func (p *MemPipeline) ResolvePrerequisites(resolver core.PrerequisiteResolver, fs interface{}) error {
	if resolver == nil {
		p.logger.Debug().Msg("no prerequisite resolver provided")
		return nil
	}
	
	p.logger.Debug().
		Int("original_operation_count", len(p.operations)).
		Msg("starting prerequisite resolution")
	
	// Keep track of prerequisites we've already resolved to avoid duplicates
	resolvedPrereqs := make(map[string]bool)
	
	// Process operations in order, but be aware that we might add new operations
	originalCount := len(p.operations)
	for i := 0; i < originalCount; i++ {
		op := p.operations[i]
		
		// Check if operation has prerequisites
		type prereqProvider interface {
			Prerequisites() []core.Prerequisite
		}
		
		if prereqOp, ok := op.(prereqProvider); ok {
			prereqs := prereqOp.Prerequisites()
			p.logger.Debug().
				Str("operation_id", p.getOperationID(op)).
				Int("prerequisite_count", len(prereqs)).
				Msg("processing operation prerequisites")
			
			for _, prereq := range prereqs {
				prereqKey := fmt.Sprintf("%s:%s", prereq.Type(), prereq.Path())
				
				// Skip if we've already resolved this prerequisite
				if resolvedPrereqs[prereqKey] {
					p.logger.Debug().
						Str("prerequisite_type", prereq.Type()).
						Str("prerequisite_path", prereq.Path()).
						Msg("prerequisite already resolved, skipping")
					continue
				}
				
				// Check if prerequisite can be resolved
				if !resolver.CanResolve(prereq) {
					p.logger.Debug().
						Str("prerequisite_type", prereq.Type()).
						Str("prerequisite_path", prereq.Path()).
						Msg("prerequisite cannot be resolved by this resolver")
					continue
				}
				
				// Check if prerequisite is already satisfied
				if err := prereq.Validate(fs); err == nil {
					p.logger.Debug().
						Str("prerequisite_type", prereq.Type()).
						Str("prerequisite_path", prereq.Path()).
						Msg("prerequisite already satisfied")
					resolvedPrereqs[prereqKey] = true
					continue
				}
				
				// Resolve the prerequisite
				p.logger.Debug().
					Str("prerequisite_type", prereq.Type()).
					Str("prerequisite_path", prereq.Path()).
					Msg("resolving prerequisite")
				
				newOps, err := resolver.Resolve(prereq)
				if err != nil {
					return fmt.Errorf("failed to resolve prerequisite %s:%s: %w", 
						prereq.Type(), prereq.Path(), err)
				}
				
				// Add new operations at the beginning (before dependencies)
				if len(newOps) > 0 {
					p.logger.Debug().
						Str("prerequisite_type", prereq.Type()).
						Str("prerequisite_path", prereq.Path()).
						Int("new_operations_count", len(newOps)).
						Msg("adding prerequisite operations")
					
					// Insert new operations at the beginning
					p.operations = append(newOps, p.operations...)
					
					// Update originalCount to account for new operations
					originalCount += len(newOps)
					i += len(newOps) // Adjust index to account for inserted operations
				}
				
				resolvedPrereqs[prereqKey] = true
			}
		}
	}
	
	p.logger.Debug().
		Int("final_operation_count", len(p.operations)).
		Int("prerequisites_resolved", len(resolvedPrereqs)).
		Msg("prerequisite resolution completed")
	
	return nil
}

// Validate validates all operations in the pipeline
func (p *MemPipeline) Validate(ctx context.Context, fs interface{}) error {
	p.logger.Debug().
		Int("operation_count", len(p.operations)).
		Msg("starting pipeline validation")
	
	for i, op := range p.operations {
		p.logger.Debug().
			Str("operation_id", p.getOperationID(op)).
			Int("operation_index", i).
			Msg("validating operation")
		
		// Try ValidateV2 first
		type validatorV2 interface {
			ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error
		}
		
		if v2, ok := op.(validatorV2); ok {
			execCtx := &core.ExecutionContext{
				Logger: p.logger,
			}
			if err := v2.ValidateV2(ctx, execCtx, fs); err != nil {
				return fmt.Errorf("validation failed for operation %s: %w", p.getOperationID(op), err)
			}
			continue
		}
		
		// Fallback to Validate
		type validator interface {
			Validate(ctx context.Context, fsys interface{}) error
		}
		
		if v, ok := op.(validator); ok {
			if err := v.Validate(ctx, fs); err != nil {
				return fmt.Errorf("validation failed for operation %s: %w", p.getOperationID(op), err)
			}
		}
	}
	
	p.logger.Debug().Msg("pipeline validation completed successfully")
	return nil
}

// getOperationID is a helper to extract operation ID for logging
func (p *MemPipeline) getOperationID(op interface{}) string {
	type idProvider interface {
		ID() core.OperationID
	}
	
	if idOp, ok := op.(idProvider); ok {
		return string(idOp.ID())
	}
	
	return "unknown"
}
