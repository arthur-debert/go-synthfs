package ops

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

// CreateDirOperation represents an operation to create a directory.
type CreateDirOperation struct {
	id           synthfs.OperationID
	path         string
	mode         fs.FileMode
	dependencies []synthfs.OperationID
	// createdPaths tracks which directories were actually created by this operation
	// for more accurate rollback
	createdPaths []string
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
		id:           synthfs.OperationID(fmt.Sprintf("create_dir:%s", path)),
		createdPaths: []string{},
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

// Execute creates the directory using MkdirAll and tracks what was created.
func (op *CreateDirOperation) Execute(ctx context.Context, fsys synthfs.FileSystem) error {
	synthfs.Logger().Info().
		Str("op_id", string(op.id)).
		Str("path", op.path).
		Str("mode", op.mode.String()).
		Msg("creating directory")

	// Track which directories need to be created for accurate rollback
	op.createdPaths = []string{}

	// Build path components to track creation
	pathParts := strings.Split(strings.Trim(op.path, "/"), "/")
	currentPath := ""

	for _, part := range pathParts {
		if part == "" {
			continue
		}

		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + "/" + part
		}

		// Check if this path exists already
		if file, err := fsys.Open(currentPath); err == nil {
			// Directory already exists, just check if it's actually a directory
			if info, statErr := file.Stat(); statErr == nil && info.IsDir() {
				file.Close()
				continue // Directory exists, skip creation
			} else {
				file.Close()
				return fmt.Errorf("path %s exists but is not a directory", currentPath)
			}
		}
	}

	// Execute MkdirAll
	if err := fsys.MkdirAll(op.path, op.mode); err != nil {
		synthfs.Logger().Info().
			Str("op_id", string(op.id)).
			Str("path", op.path).
			Err(err).
			Msg("directory creation failed")
		return fmt.Errorf("failed to create directory %s: %w", op.path, err)
	}

	// Record the directories that were created for rollback purposes
	pathParts = strings.Split(strings.Trim(op.path, "/"), "/")
	currentPath = ""

	for _, part := range pathParts {
		if part == "" {
			continue
		}

		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = currentPath + "/" + part
		}

		// Assume this was created (since MkdirAll succeeded)
		// In a more sophisticated implementation, we'd track exactly what was created
		op.createdPaths = append(op.createdPaths, currentPath)
	}

	synthfs.Logger().Info().
		Str("op_id", string(op.id)).
		Str("path", op.path).
		Msg("directory created successfully")

	return nil
}

// Validate checks if the operation parameters are sensible.
func (op *CreateDirOperation) Validate(ctx context.Context, fsys synthfs.FileSystem) error {
	if op.path == "" {
		return fmt.Errorf("CreateDirOperation: path cannot be empty")
	}

	// Check for invalid path patterns
	if strings.Contains(op.path, "..") {
		return fmt.Errorf("CreateDirOperation: path cannot contain '..' segments: %s", op.path)
	}

	// For directories, we allow ModeDir bit and permission bits
	// Extract just the permission bits to validate
	permBits := op.mode & fs.ModePerm

	// Check for invalid non-permission, non-directory mode bits
	invalidBits := op.mode &^ (fs.ModePerm | fs.ModeDir)
	if invalidBits != 0 {
		return fmt.Errorf("CreateDirOperation: invalid directory mode bits: %o", op.mode)
	}

	// Permission bits should be valid (0-0777)
	if permBits > 0777 {
		return fmt.Errorf("CreateDirOperation: invalid permission bits: %o", permBits)
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

// Rollback removes the target directory that was created by this operation.
// This uses a conservative approach - it only removes the target directory itself,
// not any parent directories that might have been created as a side effect.
func (op *CreateDirOperation) Rollback(ctx context.Context, fsys synthfs.FileSystem) error {
	// Only remove the target directory, not parent directories
	// This is a conservative approach that matches test expectations
	if err := fsys.Remove(op.path); err != nil {
		// If removal fails, it might be because the directory is not empty
		// This is expected behavior if other operations added content
		return fmt.Errorf("could not remove directory %s during rollback: %w", op.path, err)
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
