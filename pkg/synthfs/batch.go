package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// Batch represents a collection of filesystem operations that can be validated and executed as a unit.
// It provides an imperative API with validate-as-you-go and automatic dependency resolution.
type Batch struct {
	operations []Operation
	fs         FullFileSystem // Use FullFileSystem to have access to Stat method
	ctx        context.Context
	idCounter  int
}

// NewBatch creates a new operation batch with default filesystem and context.
func NewBatch() *Batch {
	return &Batch{
		operations: []Operation{},
		fs:         NewOSFileSystem("."), // Use current directory as default root
		ctx:        context.Background(),
		idCounter:  0,
	}
}

// WithFileSystem sets the filesystem for the batch operations.
func (b *Batch) WithFileSystem(fs FullFileSystem) *Batch {
	b.fs = fs
	return b
}

// WithContext sets the context for the batch operations.
func (b *Batch) WithContext(ctx context.Context) *Batch {
	b.ctx = ctx
	return b
}

// Operations returns all operations currently in the batch.
func (b *Batch) Operations() []Operation {
	// Return a copy to prevent external modification
	opsCopy := make([]Operation, len(b.operations))
	copy(opsCopy, b.operations)
	return opsCopy
}

// CreateDir adds a directory creation operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) CreateDir(path string, mode ...fs.FileMode) (Operation, error) {
	fileMode := fs.FileMode(0755) // Default directory mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation directly to avoid circular import
	opID := b.generateID("create_dir", path)
	op := NewSimpleOperation(opID, "create_directory", path)

	// Set the FsItem for this create operation
	dirItem := NewDirectory(path).WithMode(fileMode)
	op.SetItem(dirItem)
	op.SetDescriptionDetail("mode", fileMode.String())

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for CreateDir(%s): %w", path, err)
	}

	// Auto-resolve dependencies (ensure parent directories exist)
	if err := b.ensureParentDirectories(path); err != nil {
		return nil, fmt.Errorf("dependency resolution failed for CreateDir(%s): %w", path, err)
	}

	// Add to batch
	b.operations = append(b.operations, op)
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", path).
		Str("mode", fileMode.String()).
		Msg("CreateDir operation added to batch")

	return op, nil
}

// CreateFile adds a file creation operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) CreateFile(path string, content []byte, mode ...fs.FileMode) (Operation, error) {
	fileMode := fs.FileMode(0644) // Default file mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation directly to avoid circular import
	opID := b.generateID("create_file", path)
	op := NewSimpleOperation(opID, "create_file", path)

	// Set the FsItem for this create operation
	fileItem := NewFile(path).WithContent(content).WithMode(fileMode)
	op.SetItem(fileItem)
	op.SetDescriptionDetail("content_length", len(content))
	op.SetDescriptionDetail("mode", fileMode.String())

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for CreateFile(%s): %w", path, err)
	}

	// Auto-resolve dependencies (ensure parent directories exist)
	if err := b.ensureParentDirectories(path); err != nil {
		return nil, fmt.Errorf("dependency resolution failed for CreateFile(%s): %w", path, err)
	}

	// Add to batch
	b.operations = append(b.operations, op)
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", path).
		Int("content_length", len(content)).
		Str("mode", fileMode.String()).
		Msg("CreateFile operation added to batch")

	return op, nil
}

// Copy adds a copy operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) Copy(src, dst string) (Operation, error) {
	// Create the operation directly to avoid circular import
	opID := b.generateID("copy", src+"_to_"+dst)
	op := NewSimpleOperation(opID, "copy", src)
	op.SetDescriptionDetail("destination", dst)
	op.SetPaths(src, dst)

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for Copy(%s, %s): %w", src, dst, err)
	}

	// Auto-resolve dependencies (ensure destination parent directories exist)
	if err := b.ensureParentDirectories(dst); err != nil {
		return nil, fmt.Errorf("dependency resolution failed for Copy(%s, %s): %w", src, dst, err)
	}

	// Add to batch
	b.operations = append(b.operations, op)
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("src", src).
		Str("dst", dst).
		Msg("Copy operation added to batch")

	return op, nil
}

// Move adds a move operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) Move(src, dst string) (Operation, error) {
	// Create the operation directly to avoid circular import
	opID := b.generateID("move", src+"_to_"+dst)
	op := NewSimpleOperation(opID, "move", src)
	op.SetDescriptionDetail("destination", dst)
	op.SetPaths(src, dst)

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for Move(%s, %s): %w", src, dst, err)
	}

	// Auto-resolve dependencies (ensure destination parent directories exist)
	if err := b.ensureParentDirectories(dst); err != nil {
		return nil, fmt.Errorf("dependency resolution failed for Move(%s, %s): %w", src, dst, err)
	}

	// Add to batch
	b.operations = append(b.operations, op)
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("src", src).
		Str("dst", dst).
		Msg("Move operation added to batch")

	return op, nil
}

// Delete adds a delete operation to the batch.
// It validates the operation immediately.
func (b *Batch) Delete(path string) (Operation, error) {
	// Create the operation directly to avoid circular import
	opID := b.generateID("delete", path)
	op := NewSimpleOperation(opID, "delete", path)

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for Delete(%s): %w", path, err)
	}

	// Add to batch (no dependency resolution needed for delete)
	b.operations = append(b.operations, op)
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", path).
		Msg("Delete operation added to batch")

	return op, nil
}

// Execute runs all operations in the batch using the existing infrastructure.
func (b *Batch) Execute() (*Result, error) {
	Logger().Info().
		Int("operation_count", len(b.operations)).
		Msg("executing batch")

	// Create executor and queue
	executor := NewExecutor()
	queue := NewMemQueue()

	// Add all operations to queue
	if err := queue.Add(b.operations...); err != nil {
		return nil, fmt.Errorf("failed to add operations to queue: %w", err)
	}

	// Execute using existing infrastructure
	result := executor.Execute(b.ctx, queue, b.fs)

	Logger().Info().
		Bool("success", result.Success).
		Int("operations_executed", len(result.Operations)).
		Dur("duration", result.Duration).
		Msg("batch execution completed")

	return result, nil
}

// generateID creates a unique operation ID based on type and path.
func (b *Batch) generateID(opType, path string) OperationID {
	b.idCounter++
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")
	return OperationID(fmt.Sprintf("batch_%d_%s_%s", b.idCounter, opType, cleanPath))
}

// ensureParentDirectories analyzes a path and adds CreateDir operations for missing parent directories.
func (b *Batch) ensureParentDirectories(path string) error {
	// Clean and normalize the path
	cleanPath := filepath.Clean(path)
	parentDir := filepath.Dir(cleanPath)

	// If parent is root or current directory, no parent needed
	if parentDir == "." || parentDir == "/" || parentDir == cleanPath {
		return nil
	}

	// Check if parent directory already exists in filesystem
	if _, err := b.fs.Stat(parentDir); err == nil {
		// Parent exists, no need to create
		return nil
	}

	// Check if we already have a CreateDir operation for this parent
	for _, op := range b.operations {
		if op.Describe().Type == "create_directory" && op.Describe().Path == parentDir {
			// Already have an operation to create this parent
			return nil
		}
	}

	// Recursively ensure parent's parents exist
	if err := b.ensureParentDirectories(parentDir); err != nil {
		return err
	}

	// Create operation for the parent directory
	parentOpID := b.generateID("create_dir_auto", parentDir)
	parentOp := NewSimpleOperation(parentOpID, "create_directory", parentDir)
	parentDirItem := NewDirectory(parentDir).WithMode(0755)
	parentOp.SetItem(parentDirItem)
	parentOp.SetDescriptionDetail("mode", "0755")

	// Validate the auto-generated parent operation
	if err := parentOp.Validate(b.ctx, b.fs); err != nil {
		return fmt.Errorf("validation failed for auto-generated parent directory %s: %w", parentDir, err)
	}

	// Add to operations
	b.operations = append(b.operations, parentOp)
	Logger().Info().
		Str("op_id", string(parentOp.ID())).
		Str("path", parentDir).
		Str("reason", "auto-generated for parent directory").
		Msg("CreateDir operation auto-added to batch")

	return nil
}
