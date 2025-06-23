package synthfs

import (
	"context"
	"fmt"

	"github.com/gammazero/toposort"
)

// Queue defines an interface for managing a sequence of operations.
type Queue interface {
	// Add appends one or more operations to the queue.
	// It may return an error, for example, if an operation with a duplicate ID
	// is added.
	Add(ops ...Operation) error

	// Operations returns all operations currently in the queue.
	// After Resolve() is called, this returns operations in dependency-resolved order.
	Operations() []Operation

	// Resolve performs dependency resolution using topological sorting.
	// This must be called before execution to ensure operations are in correct order.
	// Returns error if circular dependencies are detected.
	Resolve() error

	// Validate checks if all operations in the queue are valid.
	// This includes validating individual operations and checking for dependency conflicts.
	Validate(ctx context.Context, fs FileSystem) error
}

// memQueue is an in-memory implementation of the Queue interface.
type memQueue struct {
	ops         []Operation
	idIndex     map[OperationID]int // Maps operation ID to index in ops slice
	resolved    bool                // Whether dependency resolution has been performed
}

// NewMemQueue creates a new in-memory operation queue.
func NewMemQueue() Queue {
	return &memQueue{
		ops:      make([]Operation, 0),
		idIndex:  make(map[OperationID]int),
		resolved: false,
	}
}

// Add appends operations to the queue.
func (mq *memQueue) Add(ops ...Operation) error {
	for _, op := range ops {
		if op == nil {
			return fmt.Errorf("cannot add a nil operation to the queue")
		}
		
		// Check for duplicate IDs
		if _, exists := mq.idIndex[op.ID()]; exists {
			return fmt.Errorf("operation with ID '%s' already exists in the queue", op.ID())
		}
		
		// Add operation to queue
		index := len(mq.ops)
		mq.ops = append(mq.ops, op)
		mq.idIndex[op.ID()] = index
		
		// Mark as unresolved since we added new operations
		mq.resolved = false
	}
	return nil
}

// Operations returns all operations currently in the queue.
func (mq *memQueue) Operations() []Operation {
	// Return a copy to prevent external modification
	opsCopy := make([]Operation, len(mq.ops))
	copy(opsCopy, mq.ops)
	return opsCopy
}

// Resolve performs dependency resolution using topological sorting.
func (mq *memQueue) Resolve() error {
	if len(mq.ops) == 0 {
		mq.resolved = true
		return nil
	}

	// Validate that all dependencies exist
	if err := mq.validateDependencies(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	// Build dependency graph using topological sort library
	edges := make([]toposort.Edge, 0)
	
	for _, op := range mq.ops {
		for _, depID := range op.Dependencies() {
			// Edge is [2]interface{} where element 0 comes before element 1
			// So dependency -> operation (dependency must come first)
			edges = append(edges, toposort.Edge{string(depID), string(op.ID())})
		}
	}

	// Perform topological sort
	sortedIDs, err := toposort.Toposort(edges)
	if err != nil {
		return fmt.Errorf("circular dependency detected: %w", err)
	}

	// Rebuild operations slice in topologically sorted order
	resolvedOps := make([]Operation, 0, len(mq.ops))
	newIdIndex := make(map[OperationID]int)
	
	// Add operations in dependency order
	for _, idInterface := range sortedIDs {
		idStr, ok := idInterface.(string)
		if !ok {
			return fmt.Errorf("unexpected type in topological sort result: %T", idInterface)
		}
		opID := OperationID(idStr)
		if oldIndex, exists := mq.idIndex[opID]; exists {
			newIndex := len(resolvedOps)
			resolvedOps = append(resolvedOps, mq.ops[oldIndex])
			newIdIndex[opID] = newIndex
		}
	}

	// Add any operations that weren't in the dependency graph (no dependencies or dependents)
	for _, op := range mq.ops {
		if _, alreadyAdded := newIdIndex[op.ID()]; !alreadyAdded {
			newIndex := len(resolvedOps)
			resolvedOps = append(resolvedOps, op)
			newIdIndex[op.ID()] = newIndex
		}
	}

	mq.ops = resolvedOps
	mq.idIndex = newIdIndex
	mq.resolved = true
	
	return nil
}

// Validate checks if all operations in the queue are valid.
func (mq *memQueue) Validate(ctx context.Context, fs FileSystem) error {
	// First validate dependencies exist
	if err := mq.validateDependencies(); err != nil {
		return err
	}

	// Validate each operation individually
	for _, op := range mq.ops {
		if err := op.Validate(ctx, fs); err != nil {
			return &ValidationError{
				Operation: op,
				Reason:    "operation validation failed",
				Cause:     err,
			}
		}
	}

	// Check for conflicts
	if err := mq.validateConflicts(); err != nil {
		return err
	}

	return nil
}

// validateDependencies ensures all referenced dependencies exist in the queue.
func (mq *memQueue) validateDependencies() error {
	for _, op := range mq.ops {
		for _, depID := range op.Dependencies() {
			if _, exists := mq.idIndex[depID]; !exists {
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
func (mq *memQueue) validateConflicts() error {
	for _, op := range mq.ops {
		for _, conflictID := range op.Conflicts() {
			if _, exists := mq.idIndex[conflictID]; exists {
				return &ConflictError{
					Operation: op,
					Conflicts: []OperationID{conflictID},
				}
			}
		}
	}
	return nil
}
