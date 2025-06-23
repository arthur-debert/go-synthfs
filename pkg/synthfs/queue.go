package synthfs

import "fmt"

// Queue defines an interface for managing a sequence of operations.
type Queue interface {
	// Add appends one or more operations to the queue.
	// It may return an error, for example, if an operation with a duplicate ID
	// is added (though this basic queue does not check for duplicates yet).
	Add(ops ...Operation) error

	// Operations returns all operations currently in the queue.
	// The order is the order they were added.
	// Note: The design doc mentions Resolve() and Validate() on the Queue.
	// These will be added in later phases. For Phase 1, a simple list is sufficient.
	Operations() []Operation
}

// memQueue is an in-memory implementation of the Queue interface.
type memQueue struct {
	ops         []Operation
	idIndex map[OperationID]bool // Used to track IDs for uniqueness, if desired
}

// NewMemQueue creates a new in-memory operation queue.
func NewMemQueue() Queue {
	return &memQueue{
		ops:         make([]Operation, 0),
		idIndex: make(map[OperationID]bool),
	}
}

// Add appends operations to the queue.
// In this basic implementation, it simply appends.
// Future versions should handle ID uniqueness and potentially other validations.
func (mq *memQueue) Add(ops ...Operation) error {
	for _, op := range ops {
		if op == nil {
			return fmt.Errorf("cannot add a nil operation to the queue")
		}
		// Basic ID uniqueness check (can be made optional or more sophisticated)
		// For Phase 1, this might be too strict, or could be a configurable behavior.
		// The design doc doesn't explicitly state Add should validate this,
		// but it's a common place for such a check.
		// Let's include a simple check for now.
		if _, exists := mq.idIndex[op.ID()]; exists {
			return fmt.Errorf("operation with ID '%s' already exists in the queue", op.ID())
		}
		mq.idIndex[op.ID()] = true
		mq.ops = append(mq.ops, op)
	}
	return nil
}

// Operations returns all operations currently in the queue.
func (mq *memQueue) Operations() []Operation {
	// Return a copy to prevent external modification of the internal slice
	// although the operations themselves are pointers and can be modified.
	// This is a common trade-off.
	opsCopy := make([]Operation, len(mq.ops))
	copy(opsCopy, mq.ops)
	return opsCopy
}
