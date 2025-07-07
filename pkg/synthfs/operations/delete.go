package operations

import (
	"context"
	"fmt"
	
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// DeleteOperation represents a file/directory deletion operation.
type DeleteOperation struct {
	*BaseOperation
}

// NewDeleteOperation creates a new delete operation.
func NewDeleteOperation(id core.OperationID, path string) *DeleteOperation {
	return &DeleteOperation{
		BaseOperation: NewBaseOperation(id, "delete", path),
	}
}

// Execute performs the deletion.
func (op *DeleteOperation) Execute(ctx context.Context, fsys interface{}) error {
	path := op.description.Path
	if path == "" {
		return fmt.Errorf("delete operation requires a path")
	}
	
	// Get filesystem methods
	stat, hasStat := getStatMethod(fsys)
	remove, hasRemove := getRemoveMethod(fsys)
	removeAll, hasRemoveAll := getRemoveAllMethod(fsys)
	
	if !hasRemove {
		return fmt.Errorf("filesystem does not support Remove")
	}
	
	// Check if it's a directory
	if hasStat {
		info, err := stat(path)
		if err != nil {
			// Already doesn't exist - that's okay
			return nil
		}
		
		// Check if it's a directory
		if isDir, ok := info.(interface{ IsDir() bool }); ok && isDir.IsDir() {
			// Use RemoveAll for directories if available
			if hasRemoveAll {
				return removeAll(path)
			}
		}
	}
	
	// Use regular Remove
	if err := remove(path); err != nil {
		// If it doesn't exist, that's fine
		return nil
	}
	
	return nil
}

// ExecuteV2 performs the deletion with execution context support.
func (op *DeleteOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}
	
	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the deletion can be performed.
func (op *DeleteOperation) Validate(ctx context.Context, fsys interface{}) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, fsys); err != nil {
		return err
	}
	
	path := op.description.Path
	
	// Check if path exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(path); err != nil {
			// It's okay if it doesn't exist (idempotent)
			return nil
		}
	}
	
	return nil
}

// Rollback for delete would require backup data, which isn't implemented yet.
func (op *DeleteOperation) Rollback(ctx context.Context, fsys interface{}) error {
	return fmt.Errorf("rollback of delete operations not yet implemented")
}

// ReverseOps generates operations to restore deleted files (requires backup).
func (op *DeleteOperation) ReverseOps(ctx context.Context, fsys interface{}, budget interface{}) ([]interface{}, interface{}, error) {
	// TODO: Implement when backup functionality is added
	return nil, nil, fmt.Errorf("reverse operations for delete not yet implemented")
}

// Helper function to get RemoveAll method from filesystem
func getRemoveAllMethod(fsys interface{}) (func(string) error, bool) {
	type removeAllFS interface {
		RemoveAll(path string) error
	}
	
	if fs, ok := fsys.(removeAllFS); ok {
		return fs.RemoveAll, true
	}
	return nil, false
}