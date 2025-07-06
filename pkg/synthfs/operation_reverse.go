package synthfs

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// reverseDelete generates operations to reverse a deletion (requires backup).
func (op *SimpleOperation) reverseDelete(ctx context.Context, fsys FileSystem, budget *BackupBudget) ([]Operation, *BackupData, error) {
	path := op.description.Path

	// Check that filesystem supports Stat
	fullFS, ok := fsys.(FullFileSystem)
	if !ok {
		return nil, nil, fmt.Errorf("filesystem does not support Stat operation needed for backup")
	}

	// Check if path exists - might be partially deleted
	info, err := fullFS.Stat(path)
	if err == nil {
		// Path still exists - possibly partial delete
		Logger().Warn().
			Str("op_id", string(op.ID())).
			Str("path", path).
			Msg("path still exists when trying to reverse delete")
	}

	// To reverse a delete, we need the original content
	// This requires having created a backup before the delete
	var reverseOps []Operation
	var backupData *BackupData

	// If the file/directory doesn't exist anymore, we can't generate reverse operations
	// without having backed it up first
	if err != nil {
		return nil, nil, fmt.Errorf("cannot reverse delete operation: path %s no longer exists and no backup available", path)
	}

	// Create backup of existing content
	if info.IsDir() {
		// Directory backup - walk the tree and backup all files
		backedUpItems := []BackedUpItem{}
		totalBackedUpSize := int64(0)

		// Recursive function to walk and backup directory tree
		var firstBudgetError error
		var walkAndBackup func(absPath, relPath string) error
		walkAndBackup = func(absPath, relPath string) error {
			entries, readErr := fs.ReadDir(fullFS, absPath)
			if readErr != nil {
				return fmt.Errorf("cannot read directory %s: %w", absPath, readErr)
			}

			// First, create directory entry
			dirInfo, statErr := fullFS.Stat(absPath)
			if statErr != nil {
				return fmt.Errorf("cannot stat directory %s: %w", absPath, statErr)
			}

			backedUpItems = append(backedUpItems, BackedUpItem{
				RelativePath: relPath,
				ItemType:     "directory",
				Mode:         dirInfo.Mode(),
				Size:         0,
				ModTime:      dirInfo.ModTime(),
			})

			// Then process all entries
			for _, entry := range entries {
				entryPath := filepath.Join(absPath, entry.Name())
				entryRelPath := filepath.Join(relPath, entry.Name())

				entryInfo, infoErr := entry.Info()
				if infoErr != nil {
					// Log error but continue with other entries
					if firstBudgetError == nil {
						firstBudgetError = fmt.Errorf("cannot get info for %s: %w", entryPath, infoErr)
					}
					continue
				}

				if entry.IsDir() {
					// Recurse into subdirectory - always process directories
					if err := walkAndBackup(entryPath, entryRelPath); err != nil {
						// For directories, if it's not a budget error, return it
						if !strings.Contains(err.Error(), "budget exceeded") {
							return err
						}
						// Otherwise, continue processing other entries
					}
				} else {
					// Regular file - backup content
					fileSizeBytes := entryInfo.Size()
					fileSizeMB := float64(fileSizeBytes) / (1024 * 1024)

					// Check budget before reading file
					if budget != nil {
						if err := budget.ConsumeBackup(fileSizeMB); err != nil {
							if firstBudgetError == nil {
								firstBudgetError = fmt.Errorf("budget exceeded: cannot backup file '%s' (%.2fMB): %w", entryPath, fileSizeMB, err)
							}
							continue // Skip this file but continue with others
						}
					}

					content, readErr := readFileContent(fullFS, entryPath)
					if readErr != nil {
						if budget != nil {
							budget.RestoreBackup(fileSizeMB)
						}
						if firstBudgetError == nil {
							firstBudgetError = fmt.Errorf("cannot read file %s: %w", entryPath, readErr)
						}
						continue
					}

					backedUpItems = append(backedUpItems, BackedUpItem{
						RelativePath: entryRelPath,
						ItemType:     "file",
						Mode:         entryInfo.Mode(),
						Content:      content,
						Size:         fileSizeBytes,
						ModTime:      entryInfo.ModTime(),
					})
					totalBackedUpSize += fileSizeBytes
				}
			}
			return firstBudgetError
		}

		// Start the walk
		walkErr := walkAndBackup(path, ".")
		
		// Create backup data for the entire directory tree (even if partial)
		totalBackedUpSizeMB := float64(totalBackedUpSize) / (1024 * 1024)
		backupData = &BackupData{
			OperationID:   op.ID(),
			BackupType:    "directory_tree",
			OriginalPath:  path,
			BackupContent: nil,
			BackupMode:    info.Mode(),
			BackupTime:    time.Now(),
			SizeMB:        totalBackedUpSizeMB,
			Metadata: map[string]interface{}{
				"items":        backedUpItems,
				"reverse_type": "recreate_directory_tree",
			},
		}
		
		// Generate reverse operations from backed up items
		// Even if there was an error, we still generate ops for what we backed up
		for i, item := range backedUpItems {
			revOpID := OperationID(fmt.Sprintf("reverse_%s_item_%d", op.ID(), i))
			itemAbsPath := filepath.Join(path, item.RelativePath)
			if item.RelativePath == "." {
				itemAbsPath = path
			}

			if item.ItemType == "directory" {
				revOp := NewSimpleOperation(revOpID, "create_directory", itemAbsPath)
				dirItem := NewDirectory(itemAbsPath).WithMode(item.Mode)
				revOp.SetItem(dirItem)
				reverseOps = append(reverseOps, revOp)
			} else {
				revOp := NewSimpleOperation(revOpID, "create_file", itemAbsPath)
				fileItem := NewFile(itemAbsPath).WithContent(item.Content).WithMode(item.Mode)
				revOp.SetItem(fileItem)
				reverseOps = append(reverseOps, revOp)
			}
		}
		
		// If there was an error during walk, return it along with partial backup data
		if walkErr != nil {
			return reverseOps, backupData, walkErr
		}
	} else {
		// Regular file - backup content
		sizeMB := float64(info.Size()) / (1024 * 1024)

		if budget != nil {
			if err := budget.ConsumeBackup(sizeMB); err != nil {
				return nil, nil, fmt.Errorf("budget exceeded: cannot backup file '%s' (%.2fMB): %w", path, sizeMB, err)
			}
		}

		content, err := readFileContent(fullFS, path)
		if err != nil {
			if budget != nil {
				budget.RestoreBackup(sizeMB)
			}
			return nil, nil, fmt.Errorf("cannot read file content for backup: %w", err)
		}

		// Create reverse operation to recreate the file
		reverseOp := NewSimpleOperation(
			OperationID(fmt.Sprintf("reverse_%s", op.ID())),
			"create_file",
			path,
		)
		fileItem := NewFile(path).WithContent(content).WithMode(info.Mode())
		reverseOp.SetItem(fileItem)
		reverseOps = append(reverseOps, reverseOp)

		backupData = &BackupData{
			OperationID:   op.ID(),
			BackupType:    "file",
			OriginalPath:  path,
			BackupContent: content,
			BackupMode:    info.Mode(),
			BackupTime:    time.Now(),
			SizeMB:        sizeMB,
			Metadata:      map[string]interface{}{"reverse_type": "recreate_file"},
		}
	}

	return reverseOps, backupData, nil
}

// Helper function to read file content
func readFileContent(fsys FullFileSystem, path string) ([]byte, error) {
	file, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			Logger().Warn().Err(err).Str("path", path).Msg("failed to close file")
		}
	}()

	info, err := fsys.Stat(path)
	if err != nil {
		return nil, err
	}

	content := make([]byte, info.Size())
	_, err = io.ReadFull(file, content)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// determineArchiveFormat inspects the filename to determine the archive format.
func determineArchiveFormat(filename string) (ArchiveFormat, error) {
	lowerFilename := strings.ToLower(filename)
	if strings.HasSuffix(lowerFilename, ".tar.gz") || strings.HasSuffix(lowerFilename, ".tgz") {
		return ArchiveFormatTarGz, nil
	}
	if strings.HasSuffix(lowerFilename, ".zip") {
		return ArchiveFormatZip, nil
	}
	return -1, fmt.Errorf("unsupported archive format for file: %s", filename)
}