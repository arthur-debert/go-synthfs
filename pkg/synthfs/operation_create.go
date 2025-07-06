package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
)

// executeCreateFile creates a new file with content.
func (op *SimpleOperation) executeCreateFile(ctx context.Context, fsys FileSystem) error {
	fileItem, ok := op.item.(*FileItem)
	if !ok || fileItem == nil {
		return fmt.Errorf("create_file operation requires a FileItem")
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", fileItem.Path()).
		Int("content_size", len(fileItem.Content())).
		Msg("executing create file operation")

	// Create parent directory if needed
	dir := filepath.Dir(fileItem.Path())
	if dir != "." && dir != "/" {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	// Write file with specified content and mode
	if err := fsys.WriteFile(fileItem.Path(), fileItem.Content(), fileItem.Mode()); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Phase I, Milestone 3: Store checksum after successful creation
	if fullFS, ok := fsys.(FullFileSystem); ok {
		if checksum, err := ComputeFileChecksum(fullFS, fileItem.Path()); err == nil && checksum != nil {
			op.SetChecksum(fileItem.Path(), checksum)
			Logger().Debug().
				Str("op_id", string(op.ID())).
				Str("path", fileItem.Path()).
				Str("md5", checksum.MD5).
				Msg("stored checksum for created file")
		}
	}

	return nil
}

// executeCreateDirectory creates a new directory.
func (op *SimpleOperation) executeCreateDirectory(ctx context.Context, fsys FileSystem) error {
	dirItem, ok := op.item.(*DirectoryItem)
	if !ok || dirItem == nil {
		return fmt.Errorf("create_directory operation requires a DirectoryItem")
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", dirItem.Path()).
		Str("mode", dirItem.Mode().String()).
		Msg("executing create directory operation")

	// MkdirAll handles creation of parent directories as needed
	if err := fsys.MkdirAll(dirItem.Path(), dirItem.Mode()); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// executeCreateSymlink creates a symbolic link.
func (op *SimpleOperation) executeCreateSymlink(ctx context.Context, fsys FileSystem) error {
	symlinkItem, ok := op.item.(*SymlinkItem)
	if !ok || symlinkItem == nil {
		return fmt.Errorf("create_symlink operation requires a SymlinkItem")
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", symlinkItem.Path()).
		Str("target", symlinkItem.Target()).
		Msg("executing create symlink operation")

	// Create parent directory if needed
	dir := filepath.Dir(symlinkItem.Path())
	if dir != "." && dir != "/" {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	// Create the symlink
	if err := fsys.Symlink(symlinkItem.Target(), symlinkItem.Path()); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// validateCreateFile validates a file creation operation.
func (op *SimpleOperation) validateCreateFile(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return &ValidationError{
			Operation: op,
			Reason:    "no file item provided for create_file operation",
		}
	}

	fileItem, ok := op.item.(*FileItem)
	if !ok {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("expected FileItem for create_file operation, got %T", op.item),
		}
	}

	// Check if file already exists
	if _, err := fs.ReadFile(fsys, fileItem.Path()); err == nil {
		return &ValidationError{
			Operation: op,
			Reason:    "file already exists",
		}
	}

	// Check parent directory exists or can be created
	parentDir := filepath.Dir(fileItem.Path())
	if parentDir != "." && parentDir != "/" {
		if stat, err := fs.Stat(fsys, parentDir); err != nil {
			// Parent doesn't exist, but MkdirAll can create it during execution
			// This is not an error unless we want to enforce parent existence
		} else if !stat.IsDir() {
			return &ValidationError{
				Operation: op,
				Reason:    fmt.Sprintf("parent path %s exists but is not a directory", parentDir),
			}
		}
	}

	return nil
}

// validateCreateDirectory validates a directory creation operation.
func (op *SimpleOperation) validateCreateDirectory(ctx context.Context, fsys FileSystem) error {
	if op.item == nil {
		return &ValidationError{
			Operation: op,
			Reason:    "no directory item provided for create_directory operation",
		}
	}

	dirItem, ok := op.item.(*DirectoryItem)
	if !ok {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("expected DirectoryItem for create_directory operation, got %T", op.item),
		}
	}

	// Check if path already exists
	if stat, err := fs.Stat(fsys, dirItem.Path()); err == nil {
		if stat.IsDir() {
			// Directory already exists - not necessarily an error (idempotent)
			Logger().Debug().
				Str("op_id", string(op.ID())).
				Str("path", dirItem.Path()).
				Msg("directory already exists")
		} else {
			return &ValidationError{
				Operation: op,
				Reason:    "path exists but is not a directory",
			}
		}
	}

	return nil
}

// validateCreateSymlink validates a symlink creation operation.
func (op *SimpleOperation) validateCreateSymlink(ctx context.Context, fsys FileSystem) error {
	symlinkItem, ok := op.item.(*SymlinkItem)
	if !ok || symlinkItem == nil {
		return &ValidationError{
			Operation: op,
			Reason:    "create_symlink operation requires a SymlinkItem",
		}
	}

	if symlinkItem.Path() == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "symlink path cannot be empty",
		}
	}

	if symlinkItem.Target() == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "symlink target cannot be empty",
		}
	}

	// Check if symlink already exists
	if _, err := fs.Stat(fsys, symlinkItem.Path()); err == nil {
		return &ValidationError{
			Operation: op,
			Reason:    "symlink already exists",
		}
	}

	// Note: We don't validate if the target exists because:
	// 1. Symlinks can point to non-existent targets (dangling symlinks)
	// 2. The target might be created by a later operation

	return nil
}

// rollbackCreate rolls back any create operation by removing what was created.
func (op *SimpleOperation) rollbackCreate(ctx context.Context, fsys FileSystem) error {
	path := op.description.Path

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", path).
		Str("operation_type", op.description.Type).
		Msg("rolling back create operation")

	// Remove whatever was created
	if err := fsys.Remove(path); err != nil {
		// If it doesn't exist, that's fine - might have been cleaned up already
		Logger().Warn().
			Str("op_id", string(op.ID())).
			Str("path", path).
			Err(err).
			Msg("rollback remove failed (may be acceptable)")
		// We don't return the error because the rollback "succeeded" in ensuring
		// the file doesn't exist
	}

	return nil
}

// reverseCreateFile generates operations to reverse a file creation.
func (op *SimpleOperation) reverseCreateFile(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	path := op.description.Path

	// Create a delete operation to remove the file
	reverseOp := NewSimpleOperation(
		OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		"delete",
		path,
	)

	return []Operation{reverseOp}, nil, nil
}

// reverseCreateDirectory generates operations to reverse a directory creation.
func (op *SimpleOperation) reverseCreateDirectory(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	path := op.description.Path

	// Create a delete operation to remove the directory
	reverseOp := NewSimpleOperation(
		OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		"delete",
		path,
	)

	return []Operation{reverseOp}, nil, nil
}

// reverseCreateSymlink generates operations to reverse a symlink creation.
func (op *SimpleOperation) reverseCreateSymlink(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	path := op.description.Path

	// Create a delete operation to remove the symlink
	reverseOp := NewSimpleOperation(
		OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		"delete",
		path,
	)

	return []Operation{reverseOp}, nil, nil
}