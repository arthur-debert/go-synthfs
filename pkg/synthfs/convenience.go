package synthfs

import (
	"context"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

// CreateFile creates a file creation operation with auto-generated ID
func CreateFile(path string, content []byte, mode fs.FileMode) Operation {
	id := GenerateID("create_file", path)
	op := operations.NewCreateFileOperation(id, path)
	item := targets.NewFile(path).WithContent(content).WithMode(mode)
	op.SetItem(item)
	return NewOperationsPackageAdapter(op)
}

// CreateFileWithID creates a file creation operation with explicit ID
func CreateFileWithID(id string, path string, content []byte, mode fs.FileMode) Operation {
	op := operations.NewCreateFileOperation(core.OperationID(id), path)
	item := targets.NewFile(path).WithContent(content).WithMode(mode)
	op.SetItem(item)
	return NewOperationsPackageAdapter(op)
}

// CreateDir creates a directory creation operation with auto-generated ID
func CreateDir(path string, mode fs.FileMode) Operation {
	id := GenerateID("create_directory", path)
	op := operations.NewCreateDirectoryOperation(id, path)
	item := targets.NewDirectory(path).WithMode(mode)
	op.SetItem(item)
	return NewOperationsPackageAdapter(op)
}

// CreateDirWithID creates a directory creation operation with explicit ID
func CreateDirWithID(id string, path string, mode fs.FileMode) Operation {
	op := operations.NewCreateDirectoryOperation(core.OperationID(id), path)
	item := targets.NewDirectory(path).WithMode(mode)
	op.SetItem(item)
	return NewOperationsPackageAdapter(op)
}

// Delete creates a delete operation with auto-generated ID
func Delete(path string) Operation {
	id := GenerateID("delete", path)
	return NewOperationsPackageAdapter(operations.NewDeleteOperation(id, path))
}

// DeleteWithID creates a delete operation with explicit ID
func DeleteWithID(id string, path string) Operation {
	return NewOperationsPackageAdapter(operations.NewDeleteOperation(core.OperationID(id), path))
}

// Copy creates a copy operation with auto-generated ID
func Copy(src, dst string) Operation {
	id := GenerateID("copy", src)
	op := operations.NewCopyOperation(id, src)
	op.SetPaths(src, dst)
	return NewOperationsPackageAdapter(op)
}

// CopyWithID creates a copy operation with explicit ID
func CopyWithID(id string, src, dst string) Operation {
	op := operations.NewCopyOperation(core.OperationID(id), src)
	op.SetPaths(src, dst)
	return NewOperationsPackageAdapter(op)
}

// Move creates a move operation with auto-generated ID
func Move(src, dst string) Operation {
	id := GenerateID("move", src)
	op := operations.NewMoveOperation(id, src)
	op.SetPaths(src, dst)
	return NewOperationsPackageAdapter(op)
}

// MoveWithID creates a move operation with explicit ID
func MoveWithID(id string, src, dst string) Operation {
	op := operations.NewMoveOperation(core.OperationID(id), src)
	op.SetPaths(src, dst)
	return NewOperationsPackageAdapter(op)
}

// CreateSymlink creates a symlink operation with auto-generated ID
func CreateSymlink(target, linkPath string) Operation {
	id := GenerateID("create_symlink", linkPath)
	op := operations.NewCreateSymlinkOperation(id, linkPath)
	item := targets.NewSymlink(linkPath, target)
	op.SetItem(item)
	return NewOperationsPackageAdapter(op)
}

// CreateSymlinkWithID creates a symlink operation with explicit ID
func CreateSymlinkWithID(id string, target, linkPath string) Operation {
	op := operations.NewCreateSymlinkOperation(core.OperationID(id), linkPath)
	item := targets.NewSymlink(linkPath, target)
	op.SetItem(item)
	return NewOperationsPackageAdapter(op)
}

// Direct execution methods for simple use cases

// WriteFile writes a file directly to the filesystem
func WriteFile(ctx context.Context, fs filesystem.FileSystem, path string, content []byte, mode fs.FileMode) error {
	op := CreateFile(path, content, mode)
	return executeDirectOp(ctx, fs, op)
}

// MkdirAll creates a directory and all necessary parents
func MkdirAll(ctx context.Context, fs filesystem.FileSystem, path string, mode fs.FileMode) error {
	op := CreateDir(path, mode)
	return executeDirectOp(ctx, fs, op)
}

// Remove removes a file or empty directory
func Remove(ctx context.Context, fs filesystem.FileSystem, path string) error {
	op := Delete(path)
	return executeDirectOp(ctx, fs, op)
}

// executeDirectOp executes a single operation directly
func executeDirectOp(ctx context.Context, fs filesystem.FileSystem, op Operation) error {
	// Get operation description for error context
	desc := op.Describe()
	action := getOperationAction(desc.Type)
	
	// Validate first
	if err := op.Validate(ctx, fs); err != nil {
		return WrapOperationError(op, action, err)
	}
	
	// Execute
	if err := op.Execute(ctx, fs); err != nil {
		return WrapOperationError(op, action, err)
	}
	
	return nil
}