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
	synthfs.Logger().Trace().
		Interface("create_file_full_context", map[string]interface{}{
			"operation": map[string]interface{}{
				"id":           string(op.id),
				"path":         op.path,
				"mode":         op.mode.String(),
				"content_size": len(op.data),
				"content_preview": func() string {
					if len(op.data) <= 100 {
						return string(op.data)
					}
					return string(op.data[:100]) + "... (truncated)"
				}(),
				"content_hex": func() string {
					if len(op.data) <= 50 {
						return fmt.Sprintf("%x", op.data)
					}
					return fmt.Sprintf("%x... (truncated)", op.data[:50])
				}(),
				"dependencies": op.dependencies,
			},
			"context":    fmt.Sprintf("%+v", ctx),
			"filesystem": fmt.Sprintf("%T", fsys),
		}).
		Msg("executing CreateFile with complete data dump")

	synthfs.Logger().Info().
		Str("op_id", string(op.id)).
		Str("path", op.path).
		Int("content_size", len(op.data)).
		Str("mode", op.mode.String()).
		Msg("creating file")

	err := fsys.WriteFile(op.path, op.data, op.mode)
	if err != nil {
		synthfs.Logger().Trace().
			Interface("create_file_error_context", map[string]interface{}{
				"operation": map[string]interface{}{
					"id":           string(op.id),
					"path":         op.path,
					"mode":         op.mode.String(),
					"content_size": len(op.data),
				},
				"error":      err.Error(),
				"error_type": fmt.Sprintf("%T", err),
				"filesystem": fmt.Sprintf("%T", fsys),
			}).
			Msg("CreateFile execution failed - complete error context")

		synthfs.Logger().Info().
			Str("op_id", string(op.id)).
			Str("path", op.path).
			Err(err).
			Msg("file creation failed")
		return fmt.Errorf("failed to create file %s: %w", op.path, err)
	}

	synthfs.Logger().Info().
		Str("op_id", string(op.id)).
		Str("path", op.path).
		Msg("file created successfully")

	return nil
}

// Validate checks if the file creation is valid.
func (op *CreateFileOperation) Validate(ctx context.Context, fsys synthfs.FileSystem) error {
	synthfs.Logger().Debug().
		Str("op_id", string(op.id)).
		Str("path", op.path).
		Int("content_size", len(op.data)).
		Str("mode", op.mode.String()).
		Msg("starting file creation validation")

	// Validate path format
	if !fs.ValidPath(op.path) {
		synthfs.Logger().Debug().
			Str("op_id", string(op.id)).
			Str("path", op.path).
			Msg("path validation failed - invalid format")
		return fmt.Errorf("invalid path: %s", op.path)
	}

	// Validate path is not empty
	if op.path == "" {
		synthfs.Logger().Debug().
			Str("op_id", string(op.id)).
			Msg("path validation failed - empty path")
		return fmt.Errorf("CreateFileOperation: path cannot be empty")
	}

	// Validate file mode - only permission bits allowed
	if op.mode&^fs.ModePerm != 0 {
		synthfs.Logger().Debug().
			Str("op_id", string(op.id)).
			Str("path", op.path).
			Str("mode", op.mode.String()).
			Uint32("invalid_bits", uint32(op.mode&^fs.ModePerm)).
			Msg("mode validation failed - contains non-permission bits")
		return fmt.Errorf("CreateFileOperation: invalid file mode: %o", op.mode)
	}

	synthfs.Logger().Debug().
		Str("op_id", string(op.id)).
		Str("path", op.path).
		Msg("path format validation passed")

	// Check if file already exists and analyze the situation
	if file, err := fsys.Open(op.path); err == nil {
		synthfs.Logger().Debug().
			Str("op_id", string(op.id)).
			Str("path", op.path).
			Msg("target path already exists - checking if it's a file")

		if info, statErr := file.Stat(); statErr == nil {
			file.Close()
			if info.IsDir() {
				synthfs.Logger().Debug().
					Str("op_id", string(op.id)).
					Str("path", op.path).
					Bool("is_directory", true).
					Msg("target path is a directory - conflict will be handled during execution")
				// Don't fail validation here - let execution handle the conflict
			} else {
				synthfs.Logger().Debug().
					Str("op_id", string(op.id)).
					Str("path", op.path).
					Int64("existing_size", info.Size()).
					Str("existing_mode", info.Mode().String()).
					Int("new_content_size", len(op.data)).
					Str("new_mode", op.mode.String()).
					Msg("target path is an existing file - will be overwritten")
			}
		} else {
			file.Close()
			synthfs.Logger().Debug().
				Str("op_id", string(op.id)).
				Str("path", op.path).
				Err(statErr).
				Msg("could not stat existing file")
		}
	} else {
		synthfs.Logger().Debug().
			Str("op_id", string(op.id)).
			Str("path", op.path).
			Msg("target path does not exist - new file will be created")
	}

	// Validate content size
	if len(op.data) > 1024*1024*100 { // 100MB limit for example
		synthfs.Logger().Debug().
			Str("op_id", string(op.id)).
			Str("path", op.path).
			Int("content_size", len(op.data)).
			Int("max_allowed_size", 1024*1024*100).
			Msg("validation warning - large file content")
	}

	synthfs.Logger().Debug().
		Str("op_id", string(op.id)).
		Str("path", op.path).
		Msg("file creation validation completed successfully")

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
