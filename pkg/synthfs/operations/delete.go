package operations

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// DeleteOperation represents a file/directory deletion operation.
type DeleteOperation struct {
	*BaseOperation
}

// NewDeleteOperation creates a new delete operation.
func NewDeleteOperation(id core.OperationID, path string) *DeleteOperation {
	return &DeleteOperation{
		BaseOperation: NewBaseOperation(id, "delete", path),
	}
}

// Prerequisites returns the prerequisites for deleting a file/directory
func (op *DeleteOperation) Prerequisites() []core.Prerequisite {
	var prereqs []core.Prerequisite

	// For delete operations, we need the source to exist
	// Note: This is optional for idempotent delete operations
	prereqs = append(prereqs, core.NewSourceExistsPrerequisite(op.description.Path))

	return prereqs
}

// Execute performs the deletion.
func (op *DeleteOperation) Execute(ctx context.Context, fsys filesystem.FileSystem) error {
	path := op.description.Path
	if path == "" {
		return fmt.Errorf("delete operation requires a path")
	}


	// Check if it's a directory
	info, err := fsys.Stat(path)
	if err != nil {
		// Already doesn't exist - that's okay
		return nil
	}

	// Check if it's a directory
	if info.IsDir() {
		// Use RemoveAll for directories
		return fsys.RemoveAll(path)
	}

	// Use regular Remove
	if err := fsys.Remove(path); err != nil {
		// If it doesn't exist, that's fine
		return nil
	}

	return nil
}

// ExecuteV2 performs the deletion with execution context support.
func (op *DeleteOperation) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	// Convert context
	context, ok := ctx.(context.Context)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	// Call the operation's Execute method with proper event handling
	return executeWithEvents(op, context, execCtx, fsys, op.Execute)
}

// ValidateV2 checks if the delete operation can be performed using ExecutionContext.
func (op *DeleteOperation) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	return validateV2Helper(op, ctx, execCtx, fsys)
}

// Validate checks if the deletion can be performed.
func (op *DeleteOperation) Validate(ctx context.Context, fsys filesystem.FileSystem) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, fsys); err != nil {
		return err
	}

	path := op.description.Path

	// Check if path exists
	if _, err := fsys.Stat(path); err != nil {
		// It's okay if it doesn't exist (idempotent)
		return nil
	}

	return nil
}

// Rollback for delete would require backup data, which isn't implemented yet.
func (op *DeleteOperation) Rollback(ctx context.Context, fsys filesystem.FileSystem) error {
	return fmt.Errorf("rollback not implemented for delete operations")
}

// ReverseOps generates operations to restore deleted files (requires backup).
func (op *DeleteOperation) ReverseOps(ctx context.Context, fsys filesystem.FileSystem, budget interface{}) ([]Operation, interface{}, error) {
	path := op.description.Path

	// Check if we have a budget
	backupBudget := budget

	// Check if file still exists (to create backup)
	info, err := fsys.Stat(path)
	if err != nil {
		// File doesn't exist anymore - can't create backup
		return nil, nil, fmt.Errorf("cannot reverse delete operation: path %s no longer exists and no backup available", path)
	}

	// Check if it's a directory
	isDir := info.IsDir()

	// Estimate backup size
	estimatedSizeMB := float64(0.001) // Default 1KB for empty files

	// Try to get actual size
	estimatedSizeMB = float64(info.Size()) / (1024 * 1024) // Convert bytes to MB
	if isDir {
		estimatedSizeMB = float64(5.0) // Default 5MB for directories
	}

	// For directories, we'll check budget per-file during the walk
	// For files, check budget upfront
	if !isDir && backupBudget != nil {
		if budget, ok := backupBudget.(*core.BackupBudget); ok {
			if err := budget.ConsumeBackup(estimatedSizeMB); err != nil {
				// Budget exceeded - return error
				return nil, nil, fmt.Errorf("budget exceeded: cannot backup file '%s' (%.2fMB): %w", path, estimatedSizeMB, err)
			}
		}
	}

	// Create proper backup data structure first
	backupData := &core.BackupData{
		OperationID:  op.ID(),
		BackupType:   "placeholder", // Would be "file" or "directory_tree" in real implementation
		OriginalPath: path,
		BackupTime:   time.Now(),
		SizeMB:       estimatedSizeMB,
		Metadata:     make(map[string]interface{}),
	}

	// For directories, walk the tree and backup all items
	if isDir {
		backupData.BackupType = "directory_tree"

		// Try to use ReadDir if the filesystem supports it
		rdFS, hasReadDir := fsys.(interface{ ReadDir(string) ([]fs.DirEntry, error) })
		if !hasReadDir {
			return nil, nil, fmt.Errorf("filesystem does not support ReadDir for directory backup")
		}

		// Walk directory tree to backup all items
		var items []interface{}
		totalBackedUpSize := int64(0)
		skippedFiles := 0

		// Recursive function to walk and backup directory tree
		var walkAndBackup func(absPath, relPath string) error
		walkAndBackup = func(absPath, relPath string) error {
			// First, add directory entry itself
			dirInfo, err := fsys.Stat(absPath)
			if err != nil {
				return fmt.Errorf("cannot stat directory %s: %w", absPath, err)
			}

			dirMode := dirInfo.Mode()

			modTime := dirInfo.ModTime()

			// Create directory item
			dirItem := map[string]interface{}{
				"RelativePath": relPath,
				"ItemType":     "directory",
				"Mode":         dirMode,
				"Content":      []byte{},
				"Size":         int64(0),
				"ModTime":      modTime,
			}
			items = append(items, dirItem)

			// Read directory entries
			entries, err := rdFS.ReadDir(absPath)
			if err != nil {
				return fmt.Errorf("cannot read directory %s: %w", absPath, err)
			}

			// Process all entries
			for _, entry := range entries {
				entryPath := filepath.Join(absPath, entry.Name())
				entryRelPath := filepath.Join(relPath, entry.Name())

				if entry.IsDir() {
					// Recurse into subdirectory
					if err := walkAndBackup(entryPath, entryRelPath); err != nil {
						return err
					}
				} else {
					// Regular file - backup content
					entryInfo, err := entry.Info()
					if err != nil {
						continue // Skip files we can't stat
					}

					fileSizeBytes := entryInfo.Size()
					fileSizeMB := float64(fileSizeBytes) / (1024 * 1024)

					// Check budget before reading file
					if backupBudget != nil {
						if budget, ok := backupBudget.(*core.BackupBudget); ok {
							if err := budget.ConsumeBackup(fileSizeMB); err != nil {
								// Budget exceeded - skip this file
								skippedFiles++
								continue
							}
						}
					}

					// Read file content
					var content []byte
					if file, err := fsys.Open(entryPath); err == nil {
						if reader, ok := file.(io.Reader); ok {
							content, _ = io.ReadAll(reader)
						}
						if closer, ok := file.(io.Closer); ok {
							_ = closer.Close()
						}
					}

					if content == nil && backupBudget != nil {
						// Restore budget if we couldn't read the file
						if budget, ok := backupBudget.(*core.BackupBudget); ok {
							budget.RestoreBackup(fileSizeMB)
						}
						skippedFiles++
						continue
					}

					fileItem := map[string]interface{}{
						"RelativePath": entryRelPath,
						"ItemType":     "file",
						"Mode":         entryInfo.Mode(),
						"Content":      content,
						"Size":         fileSizeBytes,
						"ModTime":      entryInfo.ModTime(),
					}
					items = append(items, fileItem)
					totalBackedUpSize += fileSizeBytes
				}
			}
			return nil
		}

		// Start the walk
		_ = walkAndBackup(path, ".")

		// Update backup data
		backupData.SizeMB = float64(totalBackedUpSize) / (1024 * 1024)
		backupData.Metadata["items"] = items
		backupData.Metadata["reverse_type"] = "recreate_directory_tree"
		backupData.Metadata["skipped_files"] = skippedFiles
	} else {
		backupData.BackupType = "file"
		// Try to read file content for backup
		if file, err := fsys.Open(path); err == nil {
			if reader, ok := file.(io.Reader); ok {
				if content, err := io.ReadAll(reader); err == nil {
					backupData.BackupContent = content
				}
			}
			if closer, ok := file.(io.Closer); ok {
				_ = closer.Close()
			}
		}
	}

	// Create reverse operations based on backed up data
	var reverseOps []Operation

	if isDir {
		// For directories, create operations to restore the entire tree
		if items, ok := backupData.Metadata["items"].([]interface{}); ok {
			// Create operations in the right order - directories first, then files
			for i, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					itemType, _ := itemMap["ItemType"].(string)
					relPath, _ := itemMap["RelativePath"].(string)

					if itemType == "directory" {
						// Create directory operation
						itemPath := path
						if relPath != "." {
							itemPath = filepath.Join(path, relPath)
						}

						dirOp := NewCreateDirectoryOperation(
							core.OperationID(fmt.Sprintf("reverse_%s_item_%d", op.ID(), i)),
							itemPath,
						)
						mode, _ := itemMap["Mode"].(fs.FileMode)
						dirOp.SetItem(&MinimalItem{
							path:     itemPath,
							itemType: "directory",
							mode:     mode,
						})
						reverseOps = append(reverseOps, dirOp)
					}
				}
			}

			// Then create file operations
			for i, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					itemType, _ := itemMap["ItemType"].(string)
					relPath, _ := itemMap["RelativePath"].(string)

					if itemType == "file" {
						itemPath := filepath.Join(path, relPath)
						content, _ := itemMap["Content"].([]byte)

						fileOp := NewCreateFileOperation(
							core.OperationID(fmt.Sprintf("reverse_%s_item_%d", op.ID(), i)),
							itemPath,
						)
						mode, _ := itemMap["Mode"].(fs.FileMode)
						fileOp.SetItem(&MinimalItem{
							path:     itemPath,
							itemType: "file",
							content:  content,
							mode:     mode,
						})
						reverseOps = append(reverseOps, fileOp)
					}
				}
			}
		}
	} else {
		// Create file operation with backed up content
		fileOp := NewCreateFileOperation(
			core.OperationID(fmt.Sprintf("reverse_%s", op.ID())),
			path,
		)
		// Set a minimal item with backed up content
		fileOp.SetItem(&MinimalItem{
			path:     path,
			itemType: "file",
			content:  backupData.BackupContent,
		})
		reverseOps = append(reverseOps, fileOp)
	}

	// Check if we skipped files due to budget
	if skippedFiles, ok := backupData.Metadata["skipped_files"].(int); ok && skippedFiles > 0 {
		return reverseOps, backupData, fmt.Errorf("budget exceeded: skipped %d files", skippedFiles)
	}

	return reverseOps, backupData, nil
}
