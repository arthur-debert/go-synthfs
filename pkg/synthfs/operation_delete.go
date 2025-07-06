package synthfs

import (
	"context"
	"fmt"
	"io/fs"
)

// executeDelete removes a file or directory.
func (op *SimpleOperation) executeDelete(ctx context.Context, fsys FileSystem) error {
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", op.description.Path).
		Msg("executing delete operation")

	// Try Remove first (works for files and empty directories)
	err := fsys.Remove(op.description.Path)
	if err != nil {
		// Check if it's a non-empty directory error, then try RemoveAll
		// But if it's a "not exist" error, we should return it
		if pathErr, ok := err.(*fs.PathError); ok && pathErr.Err == fs.ErrNotExist {
			return fmt.Errorf("failed to delete %s: %w", op.description.Path, err)
		}
		
		// For other errors (like directory not empty), try RemoveAll
		if err2 := fsys.RemoveAll(op.description.Path); err2 != nil {
			return fmt.Errorf("failed to delete %s: %w", op.description.Path, err2)
		}
	}

	return nil
}

// validateDelete validates a delete operation.
func (op *SimpleOperation) validateDelete(ctx context.Context, fsys FileSystem) error {
	// For delete operations, we allow non-existent paths (idempotent behavior)
	// The actual deletion will handle the case where the file doesn't exist
	return nil
}

