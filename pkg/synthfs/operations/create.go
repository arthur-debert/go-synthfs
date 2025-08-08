package operations

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// CreateFileOperation represents a file creation operation with clean interfaces.
type CreateFileOperation struct {
	*BaseOperation
}

// NewCreateFileOperation creates a new file creation operation.
func NewCreateFileOperation(id core.OperationID, path string) *CreateFileOperation {
	return &CreateFileOperation{
		BaseOperation: NewBaseOperation(id, "create_file", path),
	}
}

// Prerequisites returns the prerequisites for creating a file.
func (op *CreateFileOperation) Prerequisites() []core.Prerequisite {
	var prereqs []core.Prerequisite

	// Parent directory prerequisite removed - Execute auto-creates parent directories
	// Only keep the no-conflict prerequisite
	prereqs = append(prereqs, core.NewNoConflictPrerequisite(op.description.Path))

	return prereqs
}

// Execute creates the file with event handling.
func (op *CreateFileOperation) Execute(ctx context.Context, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	// Execute with event handling if ExecutionContext is provided
	if execCtx != nil {
		return ExecuteWithEvents(op, ctx, execCtx, fsys, op.execute)
	}

	// Fallback to direct execution
	return op.execute(ctx, fsys)
}

// execute is the internal implementation without event handling
func (op *CreateFileOperation) execute(ctx context.Context, fsys filesystem.FileSystem) error {
	item := op.GetItem()
	if item == nil {
		return fmt.Errorf("create_file operation requires an item")
	}

	// The item should implement our ItemInterface
	fileItem, ok := item.(ItemInterface)
	if !ok {
		return fmt.Errorf("item does not implement ItemInterface")
	}

	// Create parent directory if needed
	dir := filepath.Dir(fileItem.Path())
	if dir != "." && dir != "/" {
		if err := fsys.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	// Get content and mode from item
	var content []byte
	var mode interface{} = fs.FileMode(0644) // Default

	// Try to get content from item
	if contentGetter, ok := item.(interface{ Content() []byte }); ok {
		content = contentGetter.Content()
	}

	// Try to get mode from item
	if modeGetter, ok := item.(interface{ Mode() fs.FileMode }); ok {
		mode = modeGetter.Mode()
	}

	// Write the file
	fileMode, ok := mode.(fs.FileMode)
	if !ok {
		fileMode = 0644 // Default
	}
	if err := fsys.WriteFile(fileItem.Path(), content, fileMode); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}



// Validate checks if the file can be created.
func (op *CreateFileOperation) Validate(ctx context.Context, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error {
	// First do base validation
	if err := op.BaseOperation.Validate(ctx, execCtx, fsys); err != nil {
		return err
	}

	item := op.GetItem()
	if item == nil {
		return &core.ValidationError{
			OperationID:   op.ID(),
			OperationDesc: op.Describe(),
			Reason:        "no item provided for create_file operation",
		}
	}

	// Check if item is a directory when we expect a file
	if typeGetter, ok := item.(interface{ Type() string }); ok {
		if typeGetter.Type() == "directory" {
			return &core.ValidationError{
				OperationID:   op.ID(),
				OperationDesc: op.Describe(),
				Reason:        "expected file item but got directory",
			}
		}
	}

	// Check if item implements IsDir
	if dirChecker, ok := item.(interface{ IsDir() bool }); ok {
		if dirChecker.IsDir() {
			return &core.ValidationError{
				OperationID:   op.ID(),
				OperationDesc: op.Describe(),
				Reason:        "cannot create file: item IsDir() returns true",
			}
		}
	}

	// Check if filesystem actually supports WriteFile by doing a simple test
	// We'll create a temporary file to check capability
	testPath := ".__synthfs_test_capability__"
	if err := fsys.WriteFile(testPath, []byte{}, 0644); err != nil {
		if strings.Contains(err.Error(), "does not support WriteFile") {
			return &core.ValidationError{
				OperationID:   op.ID(),
				OperationDesc: op.Describe(),
				Reason:        "filesystem does not support WriteFile",
			}
		}
	} else {
		// Clean up the test file if it was created successfully
		_ = fsys.Remove(testPath)
	}

	return nil
}


// Rollback removes the created file
func (op *CreateFileOperation) Rollback(ctx context.Context, fsys filesystem.FileSystem) error {
	return fsys.Remove(op.description.Path)
}

// ReverseOps for CreateFileOperation - returns a delete operation
func (op *CreateFileOperation) ReverseOps(ctx context.Context, fsys filesystem.FileSystem, budget interface{}) ([]Operation, interface{}, error) {
	// If file doesn't exist, reverse op is to delete the created file
	info, err := fsys.Stat(op.description.Path)
	if err != nil {
		// File does not exist, so reverse is a simple delete
		return []Operation{NewDeleteOperation(core.OperationID(fmt.Sprintf("reverse_%s", op.ID())), op.description.Path)}, nil, nil
	}

	// File exists, so we need to back it up
	file, err := fsys.Open(op.description.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open existing file for backup: %w", err)
	}
	if closer, ok := file.(io.Closer); ok {
		defer func() {
			if closeErr := closer.Close(); closeErr != nil {
				// Log error but don't fail the operation
				// The file was already read successfully
				_ = closeErr // Explicitly ignore the error
			}
		}()
	}

	content, err := io.ReadAll(file.(io.Reader))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read existing file for backup: %w", err)
	}

	backupData := &core.BackupData{
		OperationID:   op.ID(),
		BackupType:    "file",
		OriginalPath:  op.description.Path,
		BackupContent: content,
		SizeMB:        float64(len(content)) / (1024 * 1024),
		BackupTime:    time.Now(),
		Metadata: map[string]interface{}{
			"mode": info.Mode(),
		},
	}

	// Create a file creation operation to restore the backed up content
	reverseOp := NewCreateFileOperation(
		core.OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		op.description.Path,
	)
	reverseOp.SetItem(&MinimalItem{
		path:     op.description.Path,
		itemType: "file",
		content:  content,
		mode:     info.Mode(),
	})

	return []Operation{reverseOp}, backupData, nil
}

// ReverseOps for CreateDirectoryOperation - returns a delete operation
func (op *CreateDirectoryOperation) ReverseOps(ctx context.Context, fsys filesystem.FileSystem, budget interface{}) ([]Operation, interface{}, error) {
	// Create a delete operation to remove the directory
	reverseOp := NewDeleteOperation(
		core.OperationID(fmt.Sprintf("reverse_%s", op.ID())),
		op.description.Path,
	)

	return []Operation{reverseOp}, nil, nil
}
