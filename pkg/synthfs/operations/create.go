package operations

import (
	"context"
	"fmt"
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
	
	// Get content and mode from item details
	content := op.description.Details["content"].([]byte)
	mode := op.description.Details["mode"].(interface{})
	
	// Write the file
	if err := writeFile(fileItem.Path(), content, mode); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
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
	type writeFS interface {
		WriteFile(name string, data []byte, perm interface{}) error
	}
	
	if fs, ok := fsys.(writeFS); ok {
		return fs.WriteFile, true
	}
	return nil, false
}

func getMkdirAllMethod(fsys interface{}) (func(string, interface{}) error, bool) {
	type mkdirFS interface {
		MkdirAll(path string, perm interface{}) error
	}
	
	if fs, ok := fsys.(mkdirFS); ok {
		return fs.MkdirAll, true
	}
	return nil, false
}