package ops

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

// CreateFileOperation represents an operation to create a file.
type CreateFileOperation struct {
	id           synthfs.OperationID
	path         string
	data         []byte
	mode         fs.FileMode
	dependencies []synthfs.OperationID
	// conflicts are not explicitly handled in this basic version
}

// NewCreateFile creates a new CreateFileOperation.
// The path is the full path to the file to be created.
// Data is the content to write to the file.
// Mode specifies the permissions for the new file.
func NewCreateFile(path string, data []byte, mode fs.FileMode) *CreateFileOperation {
	return &CreateFileOperation{
		path: path,
		data: data,
		mode: mode,
		// Default ID, can be overridden by WithID
		id: synthfs.OperationID(fmt.Sprintf("create_file:%s", path)),
	}
}

// WithID sets a custom OperationID for the operation.
func (op *CreateFileOperation) WithID(id synthfs.OperationID) *CreateFileOperation {
	op.id = id
	return op
}

// WithDependency adds an OperationID that this operation depends on.
func (op *CreateFileOperation) WithDependency(dep synthfs.OperationID) *CreateFileOperation {
	op.dependencies = append(op.dependencies, dep)
	return op
}

// ID returns the operation's ID.
func (op *CreateFileOperation) ID() synthfs.OperationID {
	return op.id
}

// Execute creates the file with the specified data and mode.
func (op *CreateFileOperation) Execute(ctx context.Context, fsys synthfs.FileSystem) error {
	return fsys.WriteFile(op.path, op.data, op.mode)
}

// Validate checks if the operation parameters are sensible.
// For CreateFile, this is a basic check, more complex validation
// (e.g., path conflicts if not handled by dependencies) could be added.
func (op *CreateFileOperation) Validate(ctx context.Context, fsys synthfs.FileSystem) error {
	if op.path == "" {
		return fmt.Errorf("CreateFileOperation: path cannot be empty")
	}
	if op.mode&^fs.ModePerm != 0 { // Check if mode contains non-permission bits
		return fmt.Errorf("CreateFileOperation: invalid file mode: %o", op.mode)
	}
	// Further validation could involve checking if the parent directory
	// is expected to exist (if not managed by a dependency).
	return nil
}

// Dependencies returns the list of operations this one depends on.
func (op *CreateFileOperation) Dependencies() []synthfs.OperationID {
	return op.dependencies
}

// Conflicts returns an empty list for this basic operation.
// Conflict detection will be implemented in more advanced stages.
func (op *CreateFileOperation) Conflicts() []synthfs.OperationID {
	return nil // No explicit conflicts defined for this basic version
}

// Rollback removes the file that was created.
func (op *CreateFileOperation) Rollback(ctx context.Context, fsys synthfs.FileSystem) error {
	// Check if file exists before trying to remove, to make rollback idempotent
	// fs.Stat or similar would be needed, but ReadFS is not guaranteed to have it.
	// For now, we rely on fsys.Remove to handle non-existent file gracefully (e.g., return no error or specific error).
	// A more robust rollback might need to check if the file content is what we wrote.
	return fsys.Remove(op.path)
}

// Describe provides a human-readable description of the operation.
func (op *CreateFileOperation) Describe() synthfs.OperationDesc {
	return synthfs.OperationDesc{
		Type: "create_file",
		Path: op.path,
		Details: map[string]interface{}{
			"size": len(op.data),
			"mode": op.mode.String(),
		},
	}
}
