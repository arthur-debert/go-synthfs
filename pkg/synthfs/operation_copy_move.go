package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// executeCopy copies a file or directory to a new location.
func (op *SimpleOperation) executeCopy(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" || op.dstPath == "" {
		return fmt.Errorf("copy operation requires both source and destination paths")
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("src", op.srcPath).
		Str("dst", op.dstPath).
		Msg("executing copy operation")

	// Phase I, Milestone 4: Verify source checksum before copy
	if err := op.verifyChecksums(ctx, fsys); err != nil {
		return fmt.Errorf("checksum verification failed before copy: %w", err)
	}

	// Check if source is a directory
	srcInfo, err := fs.Stat(fsys, op.srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return op.copyDirectory(ctx, fsys, op.srcPath, op.dstPath)
	}

	return op.copyFile(ctx, fsys, op.srcPath, op.dstPath, srcInfo.Mode())
}

// executeMove moves a file or directory to a new location.
func (op *SimpleOperation) executeMove(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" || op.dstPath == "" {
		return fmt.Errorf("move operation requires both source and destination paths")
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("src", op.srcPath).
		Str("dst", op.dstPath).
		Msg("executing move operation")

	// Phase I, Milestone 4: Verify source checksum before move
	if err := op.verifyChecksums(ctx, fsys); err != nil {
		return fmt.Errorf("checksum verification failed before move: %w", err)
	}

	// Try rename first (most efficient if on same filesystem)
	if err := fsys.Rename(op.srcPath, op.dstPath); err == nil {
		Logger().Debug().
			Str("op_id", string(op.ID())).
			Msg("move completed using rename")
		return nil
	}

	// Rename failed, fall back to copy + delete
	Logger().Debug().
		Str("op_id", string(op.ID())).
		Msg("rename failed, falling back to copy + delete")

	// Check if source is a directory
	srcInfo, err := fs.Stat(fsys, op.srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// Copy the source to destination
	if srcInfo.IsDir() {
		if err := op.copyDirectory(ctx, fsys, op.srcPath, op.dstPath); err != nil {
			return fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := op.copyFile(ctx, fsys, op.srcPath, op.dstPath, srcInfo.Mode()); err != nil {
			return fmt.Errorf("failed to copy file: %w", err)
		}
	}

	// Delete the source
	if err := fsys.RemoveAll(op.srcPath); err != nil {
		// Try to clean up the destination since we failed to complete the move
		_ = fsys.RemoveAll(op.dstPath)
		return fmt.Errorf("failed to remove source after copy: %w", err)
	}

	return nil
}

// validateCopy validates a copy operation.
func (op *SimpleOperation) validateCopy(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "copy source path cannot be empty",
		}
	}

	if op.dstPath == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "copy destination path cannot be empty",
		}
	}

	// Check if source exists
	srcInfo, err := fs.Stat(fsys, op.srcPath)
	if err != nil {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("copy source does not exist: %s", op.srcPath),
			Cause:     err,
		}
	}

	// Check if destination already exists
	if _, err := fs.Stat(fsys, op.dstPath); err == nil {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("destination path %s already exists", op.dstPath),
		}
	}

	// Check if destination parent directory exists
	dstDir := filepath.Dir(op.dstPath)
	if dstDir != "." && dstDir != "/" {
		if stat, err := fs.Stat(fsys, dstDir); err != nil {
			// Parent doesn't exist - this might be created by another operation
			Logger().Debug().
				Str("op_id", string(op.ID())).
				Str("parent", dstDir).
				Msg("destination parent directory does not exist yet")
		} else if !stat.IsDir() {
			return &ValidationError{
				Operation: op,
				Reason:    fmt.Sprintf("destination parent %s exists but is not a directory", dstDir),
			}
		}
	}

	// Additional validation for directories
	if srcInfo.IsDir() {
		// Check if destination is inside source (would create infinite loop)
		srcAbs := filepath.Clean(op.srcPath)
		dstAbs := filepath.Clean(op.dstPath)
		// Check if dstAbs starts with srcAbs followed by a separator
		if strings.HasPrefix(dstAbs, srcAbs+string(filepath.Separator)) {
			return &ValidationError{
				Operation: op,
				Reason:    "cannot copy directory into itself",
			}
		}
	}

	return nil
}

// validateMove validates a move operation.
func (op *SimpleOperation) validateMove(ctx context.Context, fsys FileSystem) error {
	if op.srcPath == "" || op.dstPath == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "move operation requires both source and destination paths",
		}
	}

	// Check if source exists
	srcInfo, err := fs.Stat(fsys, op.srcPath)
	if err != nil {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("move source does not exist: %s", op.srcPath),
			Cause:     err,
		}
	}

	// Check if destination already exists
	if _, err := fs.Stat(fsys, op.dstPath); err == nil {
		return &ValidationError{
			Operation: op,
			Reason:    fmt.Sprintf("destination path %s already exists", op.dstPath),
		}
	}

	// Check if destination parent directory exists
	dstDir := filepath.Dir(op.dstPath)
	if dstDir != "." && dstDir != "/" {
		if stat, err := fs.Stat(fsys, dstDir); err != nil {
			// Parent doesn't exist - this might be created by another operation
			Logger().Debug().
				Str("op_id", string(op.ID())).
				Str("parent", dstDir).
				Msg("destination parent directory does not exist yet")
		} else if !stat.IsDir() {
			return &ValidationError{
				Operation: op,
				Reason:    fmt.Sprintf("destination parent %s exists but is not a directory", dstDir),
			}
		}
	}

	// Additional validation for directories
	if srcInfo.IsDir() {
		// Check if destination is inside source (would create infinite loop)
		srcAbs := filepath.Clean(op.srcPath)
		dstAbs := filepath.Clean(op.dstPath)
		// Check if dstAbs starts with srcAbs followed by a separator
		if strings.HasPrefix(dstAbs, srcAbs+string(filepath.Separator)) {
			return &ValidationError{
				Operation: op,
				Reason:    "cannot move directory into itself",
			}
		}
	}

	return nil
}

// rollbackCopy rolls back a copy operation by removing the destination.
func (op *SimpleOperation) rollbackCopy(ctx context.Context, fsys FileSystem) error {
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("dst", op.dstPath).
		Msg("rolling back copy operation")

	// Remove the destination that was created
	if err := fsys.RemoveAll(op.dstPath); err != nil {
		Logger().Warn().
			Str("op_id", string(op.ID())).
			Str("dst", op.dstPath).
			Err(err).
			Msg("rollback remove failed (may be acceptable)")
	}

	return nil
}

// rollbackMove rolls back a move operation by moving back (if possible).
func (op *SimpleOperation) rollbackMove(ctx context.Context, fsys FileSystem) error {
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("src", op.srcPath).
		Str("dst", op.dstPath).
		Msg("rolling back move operation")

	// Check if destination exists (move might have succeeded)
	if _, err := fs.Stat(fsys, op.dstPath); err == nil {
		// Try to move back
		if err := fsys.Rename(op.dstPath, op.srcPath); err != nil {
			// Rename failed, try copy + delete
			if info, err2 := fs.Stat(fsys, op.dstPath); err2 == nil {
				if info.IsDir() {
					if err3 := op.copyDirectory(ctx, fsys, op.dstPath, op.srcPath); err3 == nil {
						_ = fsys.RemoveAll(op.dstPath)
					}
				} else {
					if err3 := op.copyFile(ctx, fsys, op.dstPath, op.srcPath, info.Mode()); err3 == nil {
						_ = fsys.Remove(op.dstPath)
					}
				}
			}
		}
	}

	return nil
}

// reverseCopy generates operations to reverse a copy.
func (op *SimpleOperation) reverseCopy(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// To reverse a copy, delete the destination
	reverseOp := NewSimpleOperation(
		OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		"delete",
		op.dstPath,
	)

	// Copy operations don't require backup data
	backupData := &BackupData{
		OperationID: op.ID(),
		BackupType:  "none",
		BackupTime:  time.Now(),
		SizeMB:      0,
	}

	return []Operation{reverseOp}, backupData, nil
}

// reverseMove generates operations to reverse a move.
func (op *SimpleOperation) reverseMove(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	// To reverse a move, move back from destination to source
	reverseOp := NewSimpleOperation(
		OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		"move",
		op.dstPath,
	)
	reverseOp.SetPaths(op.dstPath, op.srcPath)
	reverseOp.SetDescriptionDetail("destination", op.srcPath)

	// Move operations don't require backup data
	backupData := &BackupData{
		OperationID: op.ID(),
		BackupType:  "none",
		BackupTime:  time.Now(),
	}

	return []Operation{reverseOp}, backupData, nil
}

// Helper method to copy a single file
func (op *SimpleOperation) copyFile(ctx context.Context, fsys FileSystem, src, dst string, mode fs.FileMode) error {
	// Read source file
	data, err := fs.ReadFile(fsys, src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Create parent directory if needed
	dir := filepath.Dir(dst)
	if dir != "." && dir != "/" {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	// Write destination file
	if err := fsys.WriteFile(dst, data, mode); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	// Phase I, Milestone 3: Store checksum for the new copy
	if fullFS, ok := fsys.(FullFileSystem); ok {
		if checksum, err := ComputeFileChecksum(fullFS, dst); err == nil && checksum != nil {
			op.SetChecksum(dst, checksum)
			Logger().Debug().
				Str("op_id", string(op.ID())).
				Str("path", dst).
				Str("md5", checksum.MD5).
				Msg("stored checksum for copied file")
		}
	}

	return nil
}

// Helper method to copy a directory recursively
func (op *SimpleOperation) copyDirectory(ctx context.Context, fsys FileSystem, src, dst string) error {
	// Create destination directory
	srcInfo, err := fs.Stat(fsys, src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	if err := fsys.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Walk source directory
	entries, err := fs.ReadDir(fsys, src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := op.copyDirectory(ctx, fsys, srcPath, dstPath); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("failed to get file info for %s: %w", srcPath, err)
			}
			if err := op.copyFile(ctx, fsys, srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}

	return nil
}