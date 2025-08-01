package synthfs

import (
	"context"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Direct execution methods for simple use cases

// WriteFile writes a file directly to the filesystem
func WriteFile(ctx context.Context, fs filesystem.FileSystem, path string, content []byte, mode fs.FileMode) error {
	op := New().CreateFile(path, content, mode)
	return executeDirectOp(ctx, fs, op)
}

// MkdirAll creates a directory and all necessary parents
func MkdirAll(ctx context.Context, fs filesystem.FileSystem, path string, mode fs.FileMode) error {
	op := New().CreateDir(path, mode)
	return executeDirectOp(ctx, fs, op)
}

// Remove removes a file or empty directory
func Remove(ctx context.Context, fs filesystem.FileSystem, path string) error {
	op := New().Delete(path)
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
