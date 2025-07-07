package operations

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// CopyOperation represents a file/directory copy operation.
type CopyOperation struct {
	*BaseOperation
}

// NewCopyOperation creates a new copy operation.
func NewCopyOperation(id core.OperationID, srcPath string) *CopyOperation {
	return &CopyOperation{
		BaseOperation: NewBaseOperation(id, "copy", srcPath),
	}
}

// Execute performs the copy operation.
func (op *CopyOperation) Execute(ctx context.Context, fsys interface{}) error {
	src, dst := op.GetPaths()
	if src == "" || dst == "" {
		return fmt.Errorf("copy operation requires both source and destination paths")
	}

	// Get filesystem methods
	open, hasOpen := getOpenMethod(fsys)
	writeFile, hasWriteFile := getWriteFileMethod(fsys)
	stat, hasStat := getStatMethod(fsys)
	mkdirAll, hasMkdirAll := getMkdirAllMethod(fsys)

	if !hasOpen || !hasWriteFile || !hasStat {
		return fmt.Errorf("filesystem does not support required operations for copy")
	}

	// Check source exists and get info
	info, err := stat(src)
	if err != nil {
		return fmt.Errorf("source not found: %w", err)
	}

	// Create parent directory if needed
	if hasMkdirAll {
		dir := filepath.Dir(dst)
		if dir != "." && dir != "/" {
			if err := mkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
		}
	}

	// Check if source is a file
	if isRegular, ok := info.(interface{ Mode() interface{} }); ok {
		// Open source file
		srcFile, err := open(src)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer func() {
			if closer, ok := srcFile.(io.Closer); ok {
				_ = closer.Close()
			}
		}()

		// Read content
		var content []byte
		if reader, ok := srcFile.(io.Reader); ok {
			content, err = io.ReadAll(reader)
			if err != nil {
				return fmt.Errorf("failed to read source file: %w", err)
			}
		} else {
			return fmt.Errorf("source file does not implement io.Reader")
		}

		// Write to destination
		if err := writeFile(dst, content, isRegular.Mode()); err != nil {
			return fmt.Errorf("failed to write destination file: %w", err)
		}

		// TODO: Compute and store checksum for the copied file
	} else {
		// TODO: Handle directory copy
		return fmt.Errorf("directory copy not yet implemented")
	}

	return nil
}

// ExecuteV2 performs the copy with execution context support.
func (op *CopyOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the copy operation can be performed.
func (op *CopyOperation) Validate(ctx context.Context, fsys interface{}) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, fsys); err != nil {
		return err
	}

	src, dst := op.GetPaths()
	if src == "" {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "source path cannot be empty",
		}
	}

	if dst == "" {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "destination path cannot be empty",
		}
	}

	// Check if source exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(src); err != nil {
			return &core.ValidationError{
				OperationID:   op.ID(),
				OperationDesc: op.Describe(),
				Reason:        "source does not exist",
				Cause:         err,
			}
		}
	}

	return nil
}

// Rollback removes the copied file/directory.
func (op *CopyOperation) Rollback(ctx context.Context, fsys interface{}) error {
	_, dst := op.GetPaths()
	if dst == "" {
		return nil
	}

	remove, ok := getRemoveMethod(fsys)
	if !ok {
		return fmt.Errorf("filesystem does not support Remove")
	}

	// Remove the destination
	_ = remove(dst) // Ignore error - might not exist
	return nil
}

// MoveOperation represents a file/directory move operation.
type MoveOperation struct {
	*BaseOperation
}

// NewMoveOperation creates a new move operation.
func NewMoveOperation(id core.OperationID, srcPath string) *MoveOperation {
	return &MoveOperation{
		BaseOperation: NewBaseOperation(id, "move", srcPath),
	}
}

// Execute performs the move operation.
func (op *MoveOperation) Execute(ctx context.Context, fsys interface{}) error {
	src, dst := op.GetPaths()
	if src == "" || dst == "" {
		return fmt.Errorf("move operation requires both source and destination paths")
	}

	// Try rename first (most efficient)
	if rename, ok := getRenameMethod(fsys); ok {
		if err := rename(src, dst); err == nil {
			return nil // Success!
		}
		// If rename fails, fall back to copy+delete
	}

	// Fall back to copy + delete
	// First copy
	copyOp := NewCopyOperation(op.ID(), src)
	copyOp.SetPaths(src, dst)
	if err := copyOp.Execute(ctx, fsys); err != nil {
		return fmt.Errorf("move failed during copy: %w", err)
	}

	// Then delete source
	if remove, ok := getRemoveMethod(fsys); ok {
		if err := remove(src); err != nil {
			// Try to clean up the copy
			_ = remove(dst)
			return fmt.Errorf("move failed during delete: %w", err)
		}
	}

	return nil
}

// ExecuteV2 performs the move with execution context support.
func (op *MoveOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the move operation can be performed.
func (op *MoveOperation) Validate(ctx context.Context, fsys interface{}) error {
	// Use same validation as copy
	copyOp := &CopyOperation{BaseOperation: op.BaseOperation}
	return copyOp.Validate(ctx, fsys)
}

// Rollback attempts to restore the moved file to its original location.
func (op *MoveOperation) Rollback(ctx context.Context, fsys interface{}) error {
	src, dst := op.GetPaths()
	if src == "" || dst == "" {
		return nil
	}

	// Try to move it back
	if rename, ok := getRenameMethod(fsys); ok {
		_ = rename(dst, src) // Ignore error
	}

	return nil
}

// Helper function to get Open method from filesystem
func getOpenMethod(fsys interface{}) (func(string) (interface{}, error), bool) {
	// Try fs.File version (most common)
	type openFSFile interface {
		Open(name string) (fs.File, error)
	}

	if fs, ok := fsys.(openFSFile); ok {
		return func(name string) (interface{}, error) {
			return fs.Open(name)
		}, true
	}

	// Try interface{} version
	type openFS interface {
		Open(name string) (interface{}, error)
	}

	if fs, ok := fsys.(openFS); ok {
		return fs.Open, true
	}
	return nil, false
}

// Helper function to get Rename method from filesystem
func getRenameMethod(fsys interface{}) (func(string, string) error, bool) {
	type renameFS interface {
		Rename(oldpath, newpath string) error
	}

	if fs, ok := fsys.(renameFS); ok {
		return fs.Rename, true
	}
	return nil, false
}
