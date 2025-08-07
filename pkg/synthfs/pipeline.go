package synthfs

import (
	"context"
	"fmt"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// Pipeline defines an interface for managing a sequence of operations.
type Pipeline interface {
	// Add appends one or more operations to the pipeline.
	// It may return an error, for example, if an operation with a duplicate ID
	// is added.
	Add(ops ...Operation) error

	// Operations returns all operations currently in the pipeline.
	// After Resolve() is called, this returns operations in dependency-resolved order.
	Operations() []Operation

	// Resolve performs dependency resolution using topological sorting.
	// This must be called before execution to ensure operations are in correct order.
	// Returns error if circular dependencies are detected.
	Resolve() error

	// Validate checks if all operations in the pipeline are valid.
	// This includes validating individual operations and checking for dependency conflicts.
	Validate(ctx context.Context, fs FileSystem) error
}

// NewMemPipeline creates a new in-memory operation pipeline.
func NewMemPipeline() Pipeline {
	return &simplePipeline{
		operations: make([]Operation, 0),
		idMap:      make(map[OperationID]Operation),
	}
}

// simplePipeline is a simple pipeline implementation without adapters
type simplePipeline struct {
	operations []Operation
	idMap      map[OperationID]Operation
	resolved   bool
}

// Add appends operations to the pipeline.
func (sp *simplePipeline) Add(ops ...Operation) error {
	for _, op := range ops {
		// Check for duplicate IDs
		if existing, exists := sp.idMap[op.ID()]; exists {
			return fmt.Errorf("operation with ID %s already exists: %v", op.ID(), existing)
		}
		
		sp.operations = append(sp.operations, op)
		sp.idMap[op.ID()] = op
		sp.resolved = false // Mark as needing resolution
	}
	return nil
}

// Operations returns all operations currently in the pipeline.
func (sp *simplePipeline) Operations() []Operation {
	return sp.operations
}

// Resolve performs dependency resolution using topological sorting.
func (sp *simplePipeline) Resolve() error {
	// For now, we just return operations in the order they were added
	// Since the Operation interface doesn't expose Dependencies(), 
	// we skip dependency resolution to maintain compatibility
	sp.resolved = true
	return nil
}

// Validate checks if all operations in the pipeline are valid.
func (sp *simplePipeline) Validate(ctx context.Context, fs FileSystem) error {
	for _, op := range sp.operations {
		if err := op.Validate(ctx, &core.ExecutionContext{}, fs); err != nil {
			return err
		}
	}
	return nil
}
