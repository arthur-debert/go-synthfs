package synthfs

import (
	"context"
	"fmt"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// CustomOperationFunc is the signature for custom operation functions
type CustomOperationFunc func(ctx context.Context, fs filesystem.FileSystem) error

// CustomOperationWithOutputFunc is the signature for custom operations that can store output
type CustomOperationWithOutputFunc func(ctx context.Context, fs filesystem.FileSystem, storeOutput func(string, interface{})) error

// CustomOperation allows users to define their own operations that integrate
// with SynthFS's pipeline system. It provides access to all standard operation
// features including validation, dependencies, and rollback.
type CustomOperation struct {
	*operations.BaseOperation
	executeFunc  CustomOperationFunc
	validateFunc CustomOperationFunc
	rollbackFunc CustomOperationFunc
}

// NewCustomOperation creates a new custom operation with the given ID and execute function.
// The operation type will be "custom" and the path will be the operation ID.
func NewCustomOperation(id string, executeFunc CustomOperationFunc) *CustomOperation {
	return &CustomOperation{
		BaseOperation: operations.NewBaseOperation(
			core.OperationID(id),
			"custom",
			id, // Use ID as path for custom operations
		),
		executeFunc: executeFunc,
	}
}

// NewCustomOperationWithOutput creates a new custom operation that can store output.
// The storeOutput function passed to the execute function can be used to store any
// output that should be available after execution.
func NewCustomOperationWithOutput(id string, executeFunc CustomOperationWithOutputFunc) *CustomOperation {
	op := &CustomOperation{
		BaseOperation: operations.NewBaseOperation(
			core.OperationID(id),
			"custom",
			id,
		),
	}
	
	// Wrap the function to provide the storeOutput helper
	op.executeFunc = func(ctx context.Context, fs filesystem.FileSystem) error {
		return executeFunc(ctx, fs, func(key string, value interface{}) {
			op.StoreOutput(key, value)
		})
	}
	
	return op
}

// WithValidation sets a custom validation function for the operation.
func (op *CustomOperation) WithValidation(validateFunc CustomOperationFunc) *CustomOperation {
	op.validateFunc = validateFunc
	return op
}

// WithRollback sets a custom rollback function for the operation.
func (op *CustomOperation) WithRollback(rollbackFunc CustomOperationFunc) *CustomOperation {
	op.rollbackFunc = rollbackFunc
	return op
}

// WithDescription sets a detailed description for the operation.
func (op *CustomOperation) WithDescription(description string) *CustomOperation {
	op.SetDescriptionDetail("description", description)
	return op
}

// StoreOutput is a helper method that custom operations can use to store output.
// This allows custom operations to make their output available after execution.
func (op *CustomOperation) StoreOutput(key string, value interface{}) {
	op.SetDescriptionDetail(key, value)
}

// Execute runs the custom operation's execute function with event handling.
func (op *CustomOperation) Execute(ctx context.Context, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	if op.executeFunc == nil {
		return fmt.Errorf("custom operation %s: no execute function defined", op.ID())
	}

	// Execute with event handling if ExecutionContext is provided
	if execCtx != nil {
		return operations.ExecuteWithEvents(op, ctx, execCtx, fsys, func(ctx context.Context, fsys filesystem.FileSystem) error {
			return op.executeFunc(ctx, fsys)
		})
	}

	// Fallback to direct execution
	return op.executeFunc(ctx, fsys)
}

// Validate runs the custom operation's validation function if defined.
// If no validation function is set, it returns nil (assumes valid).
func (op *CustomOperation) Validate(ctx context.Context, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	if op.validateFunc == nil {
		// No validation function means operation is always valid
		return nil
	}

	return op.validateFunc(ctx, fsys)
}

// Rollback runs the custom operation's rollback function if defined.
// If no rollback function is set, it returns nil (no-op rollback).
func (op *CustomOperation) Rollback(ctx context.Context, fsys filesystem.FileSystem) error {
	if op.rollbackFunc == nil {
		// No rollback function means nothing to rollback
		return nil
	}

	return op.rollbackFunc(ctx, fsys)
}


// ReverseOps returns the operations needed to reverse this custom operation.
// For custom operations, this creates a single operation that runs the rollback function.
func (op *CustomOperation) ReverseOps(ctx context.Context, fsys filesystem.FileSystem, budget interface{}) ([]operations.Operation, interface{}, error) {
	if op.rollbackFunc == nil {
		// No rollback means no reverse operations
		return []operations.Operation{}, nil, nil
	}

	// Create a reverse custom operation that runs the rollback function
	reverseOp := NewCustomOperation(
		fmt.Sprintf("reverse_%s", op.ID()),
		op.rollbackFunc,
	).WithDescription(fmt.Sprintf("Reverse of %s", op.ID()))

	return []operations.Operation{reverseOp}, nil, nil
}

// Ensure CustomOperation implements the Operation interface
var _ operations.Operation = (*CustomOperation)(nil)