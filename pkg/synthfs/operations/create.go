package operations

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// CreateFileOperation represents a file creation operation with clean interfaces.
type CreateFileOperation struct {
	*BaseOperation
}

// NewCreateFileOperation creates a new file creation operation.
func NewCreateFileOperation(id core.OperationID, path string) *CreateFileOperation {
	return &CreateFileOperation{
		BaseOperation: NewBaseOperation(id, "create_file", path),
	}
}

// Execute creates the file. The filesystem interface is generic to avoid coupling.
func (op *CreateFileOperation) Execute(ctx context.Context, fsys interface{}) error {
	item := op.GetItem()
	if item == nil {
		return fmt.Errorf("create_file operation requires an item")
	}

	// The item should implement our ItemInterface
	fileItem, ok := item.(ItemInterface)
	if !ok {
		return fmt.Errorf("item does not implement ItemInterface")
	}

	// Get filesystem methods through interface assertions
	// This allows us to work with any filesystem implementation
	writeFile, ok := getWriteFileMethod(fsys)
	if !ok {
		return fmt.Errorf("filesystem does not support WriteFile")
	}

	mkdirAll, ok := getMkdirAllMethod(fsys)
	if !ok {
		return fmt.Errorf("filesystem does not support MkdirAll")
	}

	// Create parent directory if needed
	dir := filepath.Dir(fileItem.Path())
	if dir != "." && dir != "/" {
		if err := mkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	// Get content and mode from item
	var content []byte
	var mode interface{} = fs.FileMode(0644) // Default

	// Try to get content from item
	if contentGetter, ok := item.(interface{ Content() []byte }); ok {
		content = contentGetter.Content()
	}

	// Try to get mode from item
	if modeGetter, ok := item.(interface{ Mode() fs.FileMode }); ok {
		mode = modeGetter.Mode()
	}

	// Write the file
	if err := writeFile(fileItem.Path(), content, mode); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ExecuteV2 performs the file creation with execution context support.
func (op *CreateFileOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the file can be created.
func (op *CreateFileOperation) Validate(ctx context.Context, fsys interface{}) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, fsys); err != nil {
		return err
	}

	item := op.GetItem()
	if item == nil {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "no item provided for create_file operation",
		}
	}

	// Check if filesystem supports required operations
	if _, ok := getWriteFileMethod(fsys); !ok {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "filesystem does not support WriteFile",
		}
	}

	return nil
}

// Helper functions to extract methods from filesystem interface
func getWriteFileMethod(fsys interface{}) (func(string, []byte, interface{}) error, bool) {
	// Try interface{} version first
	type writeFSInterface interface {
		WriteFile(name string, data []byte, perm interface{}) error
	}

	if fs, ok := fsys.(writeFSInterface); ok {
		return fs.WriteFile, true
	}

	// Try fs.FileMode version
	type writeFSFileMode interface {
		WriteFile(name string, data []byte, perm fs.FileMode) error
	}

	if fsFileMode, ok := fsys.(writeFSFileMode); ok {
		// Wrap to convert interface{} to fs.FileMode
		return func(name string, data []byte, perm interface{}) error {
			fileMode, ok := perm.(fs.FileMode)
			if !ok {
				// Try to convert from other types
				if mode, ok := perm.(int); ok {
					fileMode = fs.FileMode(mode)
				} else {
					fileMode = 0644 // Default
				}
			}
			return fsFileMode.WriteFile(name, data, fileMode)
		}, true
	}

	return nil, false
}

func getMkdirAllMethod(fsys interface{}) (func(string, interface{}) error, bool) {
	// Try interface{} version first
	type mkdirFSInterface interface {
		MkdirAll(path string, perm interface{}) error
	}

	if fs, ok := fsys.(mkdirFSInterface); ok {
		return fs.MkdirAll, true
	}

	// Try fs.FileMode version
	type mkdirFSFileMode interface {
		MkdirAll(path string, perm fs.FileMode) error
	}

	if fsFileMode, ok := fsys.(mkdirFSFileMode); ok {
		// Wrap to convert interface{} to fs.FileMode
		return func(path string, perm interface{}) error {
			fileMode, ok := perm.(fs.FileMode)
			if !ok {
				// Try to convert from other types
				if mode, ok := perm.(int); ok {
					fileMode = fs.FileMode(mode)
				} else {
					fileMode = 0755 // Default for directories
				}
			}
			return fsFileMode.MkdirAll(path, fileMode)
		}, true
	}

	return nil, false
}
