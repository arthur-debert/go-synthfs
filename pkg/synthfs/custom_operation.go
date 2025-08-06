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

// Execute runs the custom operation's execute function.
func (op *CustomOperation) Execute(ctx context.Context, fsys interface{}) error {
	if op.executeFunc == nil {
		return fmt.Errorf("custom operation %s: no execute function defined", op.ID())
	}

	// Type assert to filesystem.FileSystem
	fs, ok := fsys.(filesystem.FileSystem)
	if !ok {
		return fmt.Errorf("custom operation %s: invalid filesystem type", op.ID())
	}

	return op.executeFunc(ctx, fs)
}

// Validate runs the custom operation's validation function if defined.
// If no validation function is set, it returns nil (assumes valid).
func (op *CustomOperation) Validate(ctx context.Context, fsys interface{}) error {
	if op.validateFunc == nil {
		// No validation function means operation is always valid
		return nil
	}

	// Type assert to filesystem.FileSystem
	fs, ok := fsys.(filesystem.FileSystem)
	if !ok {
		return fmt.Errorf("custom operation %s: invalid filesystem type", op.ID())
	}

	return op.validateFunc(ctx, fs)
}

// Rollback runs the custom operation's rollback function if defined.
// If no rollback function is set, it returns nil (no-op rollback).
func (op *CustomOperation) Rollback(ctx context.Context, fsys interface{}) error {
	if op.rollbackFunc == nil {
		// No rollback function means nothing to rollback
		return nil
	}

	// Type assert to filesystem.FileSystem
	fs, ok := fsys.(filesystem.FileSystem)
	if !ok {
		return fmt.Errorf("custom operation %s: invalid filesystem type", op.ID())
	}

	return op.rollbackFunc(ctx, fs)
}

// ExecuteV2 implements the V2 execution interface
func (op *CustomOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Delegate to Execute with context type assertion
	if contextOp, ok := ctx.(context.Context); ok {
		return op.Execute(contextOp, fsys)
	}
	return fmt.Errorf("custom operation %s: invalid context type", op.ID())
}

// ValidateV2 implements the V2 validation interface
func (op *CustomOperation) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Delegate to Validate with context type assertion
	if contextOp, ok := ctx.(context.Context); ok {
		return op.Validate(contextOp, fsys)
	}
	return fmt.Errorf("custom operation %s: invalid context type", op.ID())
}

// ReverseOps returns the operations needed to reverse this custom operation.
// For custom operations, this creates a single operation that runs the rollback function.
func (op *CustomOperation) ReverseOps(ctx context.Context, fsys interface{}, budget interface{}) ([]interface{}, interface{}, error) {
	if op.rollbackFunc == nil {
		// No rollback means no reverse operations
		return []interface{}{}, nil, nil
	}

	// Create a reverse custom operation that runs the rollback function
	reverseOp := NewCustomOperation(
		fmt.Sprintf("reverse_%s", op.ID()),
		op.rollbackFunc,
	).WithDescription(fmt.Sprintf("Reverse of %s", op.ID()))

	return []interface{}{reverseOp}, nil, nil
}

// Ensure CustomOperation implements the Operation interface
var _ operations.Operation = (*CustomOperation)(nil)