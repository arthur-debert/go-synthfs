package operations

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
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

// Prerequisites returns the prerequisites for copying a file/directory
func (op *CopyOperation) Prerequisites() []core.Prerequisite {
	var prereqs []core.Prerequisite

	// Need source to exist
	src, _ := op.GetPaths()
	if src != "" {
		prereqs = append(prereqs, core.NewSourceExistsPrerequisite(src))
	}

	// Need destination parent directory to exist
	_, dst := op.GetPaths()
	if dst != "" {
		if filepath.Dir(dst) != "." && filepath.Dir(dst) != "/" {
			prereqs = append(prereqs, core.NewParentDirPrerequisite(dst))
		}

		// Need no conflict with existing files at destination
		prereqs = append(prereqs, core.NewNoConflictPrerequisite(dst))
	}

	return prereqs
}

// Execute performs the copy operation.
func (op *CopyOperation) Execute(ctx context.Context, fsys filesystem.FileSystem) error {
	src, dst := op.GetPaths()
	if src == "" || dst == "" {
		return fmt.Errorf("copy operation requires both source and destination paths")
	}

	// Check source exists and get info
	info, err := fsys.Stat(src)
	if err != nil {
		return fmt.Errorf("source not found: %w", err)
	}

	// Create parent directory if needed
	dir := filepath.Dir(dst)
	if dir != "." && dir != "/" {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	// Check if source is a directory
	isDir := info.IsDir()

	if !isDir {
		// It's a file - copy it
		srcFile, err := fsys.Open(src)
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

		// Get file mode
		mode := info.Mode()

		// Write to destination
		if err := fsys.WriteFile(dst, content, mode); err != nil {
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
func (op *CopyOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the copy operation can be performed.
func (op *CopyOperation) Validate(ctx context.Context, fsys filesystem.FileSystem) error {
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
	if _, err := fsys.Stat(src); err != nil {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "copy source does not exist",
			Cause:         err,
		}
	}

	return nil
}

// ValidateV2 checks if the copy operation can be performed using ExecutionContext.
func (op *CopyOperation) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	return validateV2Helper(op, ctx, execCtx, fsys)
}

// Rollback removes the copied file/directory.
func (op *CopyOperation) Rollback(ctx context.Context, fsys filesystem.FileSystem) error {
	_, dst := op.GetPaths()
	if dst == "" {
		return nil
	}

	// Remove the destination
	_ = fsys.Remove(dst) // Ignore error - might not exist
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

// Prerequisites returns the prerequisites for moving a file/directory
func (op *MoveOperation) Prerequisites() []core.Prerequisite {
	var prereqs []core.Prerequisite

	// Need source to exist
	src, _ := op.GetPaths()
	if src != "" {
		prereqs = append(prereqs, core.NewSourceExistsPrerequisite(src))
	}

	// Need destination parent directory to exist
	_, dst := op.GetPaths()
	if dst != "" {
		if filepath.Dir(dst) != "." && filepath.Dir(dst) != "/" {
			prereqs = append(prereqs, core.NewParentDirPrerequisite(dst))
		}

		// Need no conflict with existing files at destination
		prereqs = append(prereqs, core.NewNoConflictPrerequisite(dst))
	}

	return prereqs
}

// Execute performs the move operation.
func (op *MoveOperation) Execute(ctx context.Context, fsys filesystem.FileSystem) error {
	src, dst := op.GetPaths()
	if src == "" || dst == "" {
		return fmt.Errorf("move operation requires both source and destination paths")
	}

	// Try rename first (most efficient)
	if err := fsys.Rename(src, dst); err == nil {
		return nil // Success!
	}
	// If rename fails, fall back to copy+delete

	// Fall back to copy + delete
	// First copy
	copyOp := NewCopyOperation(op.ID(), src)
	copyOp.SetPaths(src, dst)
	if err := copyOp.Execute(ctx, fsys); err != nil {
		return fmt.Errorf("move failed during copy: %w", err)
	}

	// Then delete source
	if err := fsys.Remove(src); err != nil {
		// Try to clean up the copy
		_ = fsys.Remove(dst)
		return fmt.Errorf("move failed during delete: %w", err)
	}

	return nil
}

// ExecuteV2 performs the move with execution context support.
func (op *MoveOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// Validate checks if the move operation can be performed.
func (op *MoveOperation) Validate(ctx context.Context, fsys filesystem.FileSystem) error {
	// Use same validation as copy
	copyOp := &CopyOperation{BaseOperation: op.BaseOperation}
	return copyOp.Validate(ctx, fsys)
}

// ValidateV2 checks if the move operation can be performed using ExecutionContext.
func (op *MoveOperation) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	return validateV2Helper(op, ctx, execCtx, fsys)
}

// Rollback attempts to restore the moved file to its original location.
func (op *MoveOperation) Rollback(ctx context.Context, fsys filesystem.FileSystem) error {
	src, dst := op.GetPaths()
	if src == "" || dst == "" {
		return fmt.Errorf("move operation missing source or destination path")
	}

	// Try to move it back
	if err := fsys.Rename(dst, src); err == nil {
		return nil
	}

	// Fallback to copy and delete
	// First copy back
	copyOp := NewCopyOperation(op.ID(), dst)
	copyOp.SetPaths(dst, src)
	if err := copyOp.Execute(ctx, fsys); err != nil {
		return fmt.Errorf("rollback failed during copy: %w", err)
	}

	// Then delete the destination
	if err := fsys.Remove(dst); err != nil {
		// Try to clean up the copy
		_ = fsys.Remove(src)
		return fmt.Errorf("rollback failed during delete: %w", err)
	}

	return nil
}


// ReverseOps for CopyOperation - returns a delete operation for the destination
func (op *CopyOperation) ReverseOps(ctx context.Context, fsys filesystem.FileSystem, budget interface{}) ([]Operation, interface{}, error) {
	_, dst := op.GetPaths()
	if dst == "" {
		return nil, nil, fmt.Errorf("copy operation has no destination path")
	}

	// Create a delete operation to remove the copied file
	reverseOp := NewDeleteOperation(
		core.OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		dst,
	)

	return []Operation{reverseOp}, nil, nil
}

// ReverseOps for MoveOperation - returns a move operation to restore the original
func (op *MoveOperation) ReverseOps(ctx context.Context, fsys filesystem.FileSystem, budget interface{}) ([]Operation, interface{}, error) {
	src, dst := op.GetPaths()
	if src == "" || dst == "" {
		return nil, nil, fmt.Errorf("move operation missing source or destination path")
	}

	// Create a move operation to restore the file to its original location
	reverseOp := NewMoveOperation(
		core.OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		dst, // Move from current location
	)
	reverseOp.SetPaths(dst, src) // Back to original location

	return []Operation{reverseOp}, nil, nil
}
