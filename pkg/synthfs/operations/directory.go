package operations

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// CreateDirectoryOperation represents a directory creation operation with clean interfaces.
type CreateDirectoryOperation struct {
	*BaseOperation
}

// NewCreateDirectoryOperation creates a new directory creation operation.
func NewCreateDirectoryOperation(id core.OperationID, path string) *CreateDirectoryOperation {
	return &CreateDirectoryOperation{
		BaseOperation: NewBaseOperation(id, "create_directory", path),
	}
}

// Execute creates the directory. The filesystem interface is generic to avoid coupling.
func (op *CreateDirectoryOperation) Execute(ctx context.Context, fsys interface{}) error {
	item := op.GetItem()
	if item == nil {
		return fmt.Errorf("create_directory operation requires an item")
	}

	// The item should implement our ItemInterface
	dirItem, ok := item.(ItemInterface)
	if !ok {
		return fmt.Errorf("item does not implement ItemInterface")
	}

	// Get filesystem methods through interface assertions
	mkdirAll, ok := getMkdirAllMethod(fsys)
	if !ok {
		return fmt.Errorf("filesystem does not support MkdirAll")
	}

	// Get mode from item or use default
	var mode interface{} = fs.FileMode(0755) // Default for directories

	// Try to get mode from item
	if modeGetter, ok := item.(interface{ Mode() fs.FileMode }); ok {
		mode = modeGetter.Mode()
	}

	// Create the directory with all parent directories
	if err := mkdirAll(dirItem.Path(), mode); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// ExecuteV2 performs the directory creation with execution context support.
func (op *CreateDirectoryOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the directory can be created.
func (op *CreateDirectoryOperation) Validate(ctx context.Context, fsys interface{}) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, fsys); err != nil {
		return err
	}

	item := op.GetItem()
	if item == nil {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "no item provided for create_directory operation",
		}
	}

	// Check if path already exists
	stat, ok := getStatMethod(fsys)
	if ok {
		if info, err := stat(op.description.Path); err == nil {
			if isDir, ok := info.(interface{ IsDir() bool }); ok && isDir.IsDir() {
				// Directory already exists - this is okay (idempotent)
				return nil
			} else {
				return &core.ValidationError{
					OperationID:   op.ID(),
					OperationDesc: op.Describe(),
					Reason:        "path exists but is not a directory",
				}
			}
		}
	}

	return nil
}

// Rollback removes the created directory.
func (op *CreateDirectoryOperation) Rollback(ctx context.Context, fsys interface{}) error {
	remove, ok := getRemoveMethod(fsys)
	if !ok {
		return fmt.Errorf("filesystem does not support Remove")
	}

	// Remove the directory
	if err := remove(op.description.Path); err != nil {
		// If it doesn't exist, that's fine
		return nil
	}

	return nil
}

// Helper function to get Stat method from filesystem
func getStatMethod(fsys interface{}) (func(string) (interface{}, error), bool) {
	// Try os.FileInfo version
	type statFSFileInfo interface {
		Stat(name string) (os.FileInfo, error)
	}

	if fs, ok := fsys.(statFSFileInfo); ok {
		return func(name string) (interface{}, error) {
			return fs.Stat(name)
		}, true
	}

	// Try fs.FileInfo version
	type statFSInfo interface {
		Stat(name string) (fs.FileInfo, error)
	}

	if fs, ok := fsys.(statFSInfo); ok {
		return func(name string) (interface{}, error) {
			return fs.Stat(name)
		}, true
	}

	// Try interface{} version
	type statFS interface {
		Stat(name string) (interface{}, error)
	}

	if fs, ok := fsys.(statFS); ok {
		return fs.Stat, true
	}
	return nil, false
}

// Helper function to get Remove method from filesystem
func getRemoveMethod(fsys interface{}) (func(string) error, bool) {
	type removeFS interface {
		Remove(name string) error
	}

	if fs, ok := fsys.(removeFS); ok {
		return fs.Remove, true
	}
	return nil, false
}
