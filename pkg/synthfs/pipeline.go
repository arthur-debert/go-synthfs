package synthfs

import (
	"context"
	"fmt"

	"github.com/gammazero/toposort"
)

// Pipeline defines an interface for managing a sequence of operations.
type Pipeline interface {
	// Add appends one or more operations to the pipeline.
	Add(ops ...Operation) error

	// Operations returns all operations currently in the pipeline.
	Operations() []Operation

	// Resolve performs dependency resolution.
	Resolve() error

	// Validate checks if all operations in the pipeline are valid.
	Validate(ctx context.Context, fs FileSystem) error
}

// memPipeline is an in-memory implementation of the Pipeline interface.
type memPipeline struct {
	ops      []Operation
	idIndex  map[OperationID]int
	resolved bool
}

// NewMemPipeline creates a new in-memory operation pipeline.
func NewMemPipeline() Pipeline {
	return &memPipeline{
		ops:      make([]Operation, 0),
		idIndex:  make(map[OperationID]int),
		resolved: false,
	}
}

// Add appends operations to the pipeline.
func (mp *memPipeline) Add(ops ...Operation) error {
	for _, op := range ops {
		if op == nil {
			return fmt.Errorf("cannot add a nil operation to the pipeline")
		}
		if _, exists := mp.idIndex[op.ID()]; exists {
			return fmt.Errorf("operation with ID '%s' already exists in the pipeline", op.ID())
		}
		index := len(mp.ops)
		mp.ops = append(mp.ops, op)
		mp.idIndex[op.ID()] = index
		mp.resolved = false
	}
	return nil
}

// Operations returns all operations currently in the pipeline.
func (mp *memPipeline) Operations() []Operation {
	opsCopy := make([]Operation, len(mp.ops))
	copy(opsCopy, mp.ops)
	return opsCopy
}

// Resolve performs dependency resolution using topological sorting.
func (mp *memPipeline) Resolve() error {
	if len(mp.ops) == 0 {
		mp.resolved = true
		return nil
	}

	if mp.resolved {
		return nil
	}

	if err := mp.validateDependencies(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	edges := make([]toposort.Edge, 0)

	for _, op := range mp.ops {
		for _, depID := range op.Dependencies() {
			edges = append(edges, toposort.Edge{string(depID), string(op.ID())})
		}
	}

	sortedIDs, err := toposort.Toposort(edges)
	if err != nil {
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	resolvedOps := make([]Operation, 0, len(mp.ops))
	newIdIndex := make(map[OperationID]int)

	for _, idInterface := range sortedIDs {
		idStr, ok := idInterface.(string)
		if !ok {
			return fmt.Errorf("unexpected type in topological sort result: %T", idInterface)
		}
		opID := OperationID(idStr)
		if oldIndex, exists := mp.idIndex[opID]; exists {
			newIndex := len(resolvedOps)
			resolvedOps = append(resolvedOps, mp.ops[oldIndex])
			newIdIndex[opID] = newIndex
		}
	}

	for _, op := range mp.ops {
		if _, alreadyAdded := newIdIndex[op.ID()]; !alreadyAdded {
			newIndex := len(resolvedOps)
			resolvedOps = append(resolvedOps, op)
			newIdIndex[op.ID()] = newIndex
		}
	}

	mp.ops = resolvedOps
	mp.idIndex = newIdIndex
	mp.resolved = true

	return nil
}

// Validate checks if all operations in the pipeline are valid.
func (mp *memPipeline) Validate(ctx context.Context, fs FileSystem) error {
	if err := mp.validateDependencies(); err != nil {
		return err
	}

	for _, op := range mp.ops {
		if err := op.Validate(ctx, fs); err != nil {
			return &ValidationError{
				Operation: op,
				Reason:    "operation validation failed",
				Cause:     err,
			}
		}
	}

	if err := mp.validateConflicts(); err != nil {
		return err
	}

	return nil
}

// validateDependencies ensures all referenced dependencies exist in the pipeline.
func (mp *memPipeline) validateDependencies() error {
	for _, op := range mp.ops {
		deps := op.Dependencies()
		for _, depID := range deps {
			if _, exists := mp.idIndex[depID]; !exists {
				return &DependencyError{
					Operation:    op,
					Dependencies: op.Dependencies(),
					Missing:      []OperationID{depID},
				}
			}
		}
	}
	return nil
}

// validateConflicts checks for operations that conflict with each other.
func (mp *memPipeline) validateConflicts() error {
	for _, op := range mp.ops {
		conflicts := op.Conflicts()
		for _, conflictID := range conflicts {
			if _, exists := mp.idIndex[conflictID]; exists {
				return &ConflictError{
					Operation: op,
					Conflicts: []OperationID{conflictID},
				}
			}
		}
	}
	return nil
}
