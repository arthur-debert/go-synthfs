package operations

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// CreateSymlinkOperation represents a symbolic link creation operation.
type CreateSymlinkOperation struct {
	*BaseOperation
}

// NewCreateSymlinkOperation creates a new symlink creation operation.
func NewCreateSymlinkOperation(id core.OperationID, linkPath string) *CreateSymlinkOperation {
	return &CreateSymlinkOperation{
		BaseOperation: NewBaseOperation(id, "create_symlink", linkPath),
	}
}

// Prerequisites returns the prerequisites for creating a symlink.
func (op *CreateSymlinkOperation) Prerequisites() []core.Prerequisite {
	var prereqs []core.Prerequisite

	// Need parent directory to exist
	if filepath.Dir(op.description.Path) != "." && filepath.Dir(op.description.Path) != "/" {
		prereqs = append(prereqs, core.NewParentDirPrerequisite(op.description.Path))
	}

	// Need no conflict with existing files
	prereqs = append(prereqs, core.NewNoConflictPrerequisite(op.description.Path))

	return prereqs
}

// Execute creates the symbolic link.
func (op *CreateSymlinkOperation) Execute(ctx context.Context, fsys filesystem.FileSystem) error {
	item := op.GetItem()
	if item == nil {
		return fmt.Errorf("create_symlink operation requires an item")
	}

	// Get target from description details
	target, ok := op.description.Details["target"].(string)
	if !ok || target == "" {
		return fmt.Errorf("create_symlink operation requires a target")
	}

	// The item should implement our ItemInterface
	linkItem, ok := item.(ItemInterface)
	if !ok {
		return fmt.Errorf("item does not implement ItemInterface")
	}


	// Create parent directory if needed
	dir := filepath.Dir(linkItem.Path())
	if dir != "." && dir != "/" {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			// Check if it's just that MkdirAll is not supported
			if strings.Contains(err.Error(), "does not support MkdirAll") {
				// MkdirAll not supported - that's okay, continue with symlink creation
			} else {
				// Other MkdirAll error - this is a real failure
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
		}
	}

	// Create the symlink
	if err := fsys.Symlink(target, linkItem.Path()); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// ExecuteV2 performs the symlink creation with execution context support.
func (op *CreateSymlinkOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// ValidateV2 checks if the symlink can be created using ExecutionContext.
func (op *CreateSymlinkOperation) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	return validateV2Helper(op, ctx, execCtx, fsys)
}

// Validate checks if the symlink can be created.
func (op *CreateSymlinkOperation) Validate(ctx context.Context, fsys filesystem.FileSystem) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, fsys); err != nil {
		return err
	}

	item := op.GetItem()
	if item == nil {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "no item provided for create_symlink operation",
		}
	}

	// Check target
	target, ok := op.description.Details["target"].(string)
	if !ok || target == "" {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "symlink target cannot be empty",
		}
	}

	// Check if symlink already exists
	if _, err := fsys.Stat(op.description.Path); err == nil {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "symlink already exists",
		}
	}

	// Note: We don't validate if the target exists because:
	// 1. Symlinks can point to non-existent targets (dangling symlinks)
	// 2. The target might be created by a later operation

	return nil
}

// Rollback removes the created symlink.
func (op *CreateSymlinkOperation) Rollback(ctx context.Context, fsys filesystem.FileSystem) error {
	// Remove the symlink
	_ = fsys.Remove(op.description.Path) // Ignore error - might not exist
	return nil
}

// ReverseOps generates operations to remove the symlink.
func (op *CreateSymlinkOperation) ReverseOps(ctx context.Context, fsys filesystem.FileSystem, budget interface{}) ([]Operation, interface{}, error) {
	// Create a delete operation to remove the symlink
	reverseOp := NewDeleteOperation(
		core.OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		op.description.Path,
	)

	return []Operation{reverseOp}, nil, nil
}
