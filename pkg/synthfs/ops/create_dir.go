package ops

import (
	"context"
	"fmt"
	"io/fs"

	"go-synthfs/pkg/synthfs"
)

// CreateDirOperation represents an operation to create a directory.
type CreateDirOperation struct {
	id           synthfs.OperationID
	path         string
	mode         fs.FileMode
	dependencies []synthfs.OperationID
	// withParents indicates if MkdirAll behavior (creating parent dirs) is intended.
	// This is implicitly true because we use MkdirAll in Execute.
	// We might add an option if only Mkdir (no parents) is needed.
}

// NewCreateDir creates a new CreateDirOperation.
// The path is the full path to the directory to be created.
// Mode specifies the permissions for the new directory.
func NewCreateDir(path string, mode fs.FileMode) *CreateDirOperation {
	return &CreateDirOperation{
		path: path,
		mode: mode,
		// Default ID, can be overridden by WithID
		id: synthfs.OperationID(fmt.Sprintf("create_dir:%s", path)),
	}
}

// WithID sets a custom OperationID for the operation.
func (op *CreateDirOperation) WithID(id synthfs.OperationID) *CreateDirOperation {
	op.id = id
	return op
}

// WithDependency adds an OperationID that this operation depends on.
func (op *CreateDirOperation) WithDependency(dep synthfs.OperationID) *CreateDirOperation {
	op.dependencies = append(op.dependencies, dep)
	return op
}

// ID returns the operation's ID.
func (op *CreateDirOperation) ID() synthfs.OperationID {
	return op.id
}

// Execute creates the directory using MkdirAll.
func (op *CreateDirOperation) Execute(ctx context.Context, fsys synthfs.FileSystem) error {
	return fsys.MkdirAll(op.path, op.mode)
}

// Validate checks if the operation parameters are sensible.
func (op *CreateDirOperation) Validate(ctx context.Context, fsys synthfs.FileSystem) error {
	if op.path == "" {
		return fmt.Errorf("CreateDirOperation: path cannot be empty")
	}
	if op.mode&^fs.ModePerm != 0 { // Check if mode contains non-permission bits
		// Note: fs.ModeDir is also a non-permission bit, but MkdirAll expects it.
		// We are primarily concerned with bits outside of ModePerm and ModeDir.
		// However, standard practice is to just pass ModePerm bits to MkdirAll
		// and let the system handle setting ModeDir.
		// For simplicity, we'll assume mode is fs.ModePerm bits.
		// The Execute uses MkdirAll, which handles directory creation appropriately.
		return fmt.Errorf("CreateDirOperation: invalid directory mode: %o", op.mode)
	}
	return nil
}

// Dependencies returns the list of operations this one depends on.
func (op *CreateDirOperation) Dependencies() []synthfs.OperationID {
	return op.dependencies
}

// Conflicts returns an empty list for this basic operation.
func (op *CreateDirOperation) Conflicts() []synthfs.OperationID {
	return nil // No explicit conflicts defined for this basic version
}

// Rollback removes the directory that was created.
// This is a best-effort rollback. If MkdirAll created parent directories,
// this Rollback will only remove the target directory `op.path`.
// A more sophisticated rollback might need to track which parent directories
// were created by this specific operation, which is complex.
// Using RemoveAll is safer if the directory might contain items created by other means
// or if we want to ensure the specific path is gone, but it's aggressive.
// Using Remove is safer if we only want to remove it if it's empty and was the specific dir we made.
// For now, using Remove, assuming it was the leaf directory we intended to create.
// If the directory is not empty (e.g. other operations wrote into it), Remove will fail.
// This is a common challenge with rollbacks of directory creations.
func (op *CreateDirOperation) Rollback(ctx context.Context, fsys synthfs.FileSystem) error {
	// Attempt to remove the directory. This will fail if the directory is not empty.
	err := fsys.Remove(op.path)
	if err != nil {
		// If Remove fails, it might be because the directory is not empty,
		// or it doesn't exist. If it doesn't exist, that's fine for a rollback.
		// We'd need to check the error type (e.g., fs.ErrNotExist) if we want to be specific.
		// For now, we assume that if an error occurs, it's a potential issue,
		// but a "best effort" rollback might ignore some errors.
		// A more robust system might check fs.ErrNotExist and return nil for that.
		// If we want to be more aggressive and ensure it's gone (even if it has contents
		// that were NOT part of this op's dependent operations), we could use RemoveAll.
		// However, that could delete more than this operation created.
		// The design document mentions Remove(name string) error, which is safer.
		return fmt.Errorf("CreateDirOperation: rollback of MkdirAll for %s failed: %w. Directory might not be empty or parents were also created", op.path, err)
	}
	return nil
}

// Describe provides a human-readable description of the operation.
func (op *CreateDirOperation) Describe() synthfs.OperationDesc {
	return synthfs.OperationDesc{
		Type: "create_dir",
		Path: op.path,
		Details: map[string]interface{}{
			"mode": op.mode.String(),
		},
	}
}
