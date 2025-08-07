package operations

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
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

// Prerequisites returns the prerequisites for creating a directory.
func (op *CreateDirectoryOperation) Prerequisites() []core.Prerequisite {
	var prereqs []core.Prerequisite

	// Always need parent directory to exist (even if it's current directory)
	prereqs = append(prereqs, core.NewParentDirPrerequisite(op.description.Path))

	// Note: We don't use NoConflictPrerequisite because directory creation is idempotent.
	// If a directory already exists, the operation should succeed (like mkdir -p).
	// The individual Validate() method handles conflict detection properly.

	return prereqs
}

// Execute creates the directory with event handling.
func (op *CreateDirectoryOperation) Execute(ctx context.Context, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	// Execute with event handling if ExecutionContext is provided
	if execCtx != nil {
		return ExecuteWithEvents(op, ctx, execCtx, fsys, op.execute)
	}

	// Fallback to direct execution
	return op.execute(ctx, fsys)
}

// execute is the internal implementation without event handling
func (op *CreateDirectoryOperation) execute(ctx context.Context, fsys filesystem.FileSystem) error {
	item := op.GetItem()
	if item == nil {
		return fmt.Errorf("create_directory operation requires an item")
	}

	// The item should implement our ItemInterface
	dirItem, ok := item.(ItemInterface)
	if !ok {
		return fmt.Errorf("item does not implement ItemInterface")
	}

	// Create the directory with all parent directories

	// Get mode from item or use default
	var mode interface{} = fs.FileMode(0755) // Default for directories

	// Try to get mode from item
	if modeGetter, ok := item.(interface{ Mode() fs.FileMode }); ok {
		mode = modeGetter.Mode()
	}

	// Create the directory with all parent directories
	if err := fsys.MkdirAll(dirItem.Path(), mode.(fs.FileMode)); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}



// Validate checks if the directory can be created.
func (op *CreateDirectoryOperation) Validate(ctx context.Context, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, execCtx, fsys); err != nil {
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

	// Check if item is a file when we expect a directory
	if typeGetter, ok := item.(interface{ Type() string }); ok {
		if typeGetter.Type() != "directory" {
			return &core.ValidationError{
				OperationID:   op.ID(),
				OperationDesc: op.Describe(),
				Reason:        "expected directory item but got " + typeGetter.Type(),
			}
		}
	}

	// Check if item implements IsDir
	if dirChecker, ok := item.(interface{ IsDir() bool }); ok {
		if !dirChecker.IsDir() {
			return &core.ValidationError{
				OperationID:   op.ID(),
				OperationDesc: op.Describe(),
				Reason:        "cannot create directory: item IsDir() returns false",
			}
		}
	}

	// Check if path already exists
	if info, err := fsys.Stat(op.description.Path); err == nil {
		if info.IsDir() {
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

	return nil
}

// Rollback removes the created directory.
func (op *CreateDirectoryOperation) Rollback(ctx context.Context, fsys filesystem.FileSystem) error {
	// Remove the directory
	if err := fsys.Remove(op.description.Path); err != nil {
		// If it doesn't exist, that's fine
		return nil
	}

	return nil
}

