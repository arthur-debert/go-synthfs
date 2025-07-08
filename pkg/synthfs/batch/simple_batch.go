package batch

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)

// SimpleBatchImpl is a simplified batch implementation that doesn't handle prerequisites
// like parent directories. It relies on the pipeline to resolve prerequisites.
type SimpleBatchImpl struct {
	operations []interface{}
	fs         interface{} // Filesystem interface
	ctx        context.Context
	idCounter  int
	registry   core.OperationFactory
	logger     core.Logger
}

// NewSimpleBatch creates a new simplified operation batch.
func NewSimpleBatch(fs interface{}, registry core.OperationFactory) Batch {
	return &SimpleBatchImpl{
		operations: []interface{}{},
		fs:         fs,
		ctx:        context.Background(),
		idCounter:  0,
		registry:   registry,
		logger:     nil, // Will be set by WithLogger method
	}
}

// Operations returns all operations currently in the batch.
func (b *SimpleBatchImpl) Operations() []interface{} {
	// Return a copy to prevent external modification
	opsCopy := make([]interface{}, len(b.operations))
	copy(opsCopy, b.operations)
	return opsCopy
}

// WithFileSystem sets the filesystem for the batch operations.
func (b *SimpleBatchImpl) WithFileSystem(fs interface{}) Batch {
	b.fs = fs
	return b
}

// WithContext sets the context for the batch operations.
func (b *SimpleBatchImpl) WithContext(ctx context.Context) Batch {
	b.ctx = ctx
	return b
}

// WithRegistry sets a custom operation registry for the batch.
func (b *SimpleBatchImpl) WithRegistry(registry core.OperationFactory) Batch {
	b.registry = registry
	return b
}

// WithLogger sets the logger for the batch.
func (b *SimpleBatchImpl) WithLogger(logger core.Logger) Batch {
	b.logger = logger
	return b
}

// add adds an operation to the batch - simplified version without parent directory handling
func (b *SimpleBatchImpl) add(op interface{}) error {
	// Just validate the operation - no parent directory creation or conflict checking
	if err := b.validateOperation(op); err != nil {
		return err
	}

	b.operations = append(b.operations, op)
	return nil
}

// validateOperation validates an operation - same as BatchImpl
func (b *SimpleBatchImpl) validateOperation(op interface{}) error {
	// Try to validate the operation
	// First check if it has a Validate method that accepts interface{}
	type validator interface {
		Validate(ctx context.Context, fsys interface{}) error
	}

	validated := false
	if v, ok := op.(validator); ok {
		if err := v.Validate(b.ctx, b.fs); err != nil {
			return err
		}
		validated = true
	}

	// If not validated yet, try ValidateV2 which operations should have
	if !validated {
		type validatorV2 interface {
			ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error
		}

		if v, ok := op.(validatorV2); ok {
			// Create a minimal ExecutionContext for validation
			execCtx := &core.ExecutionContext{}
			if err := v.ValidateV2(b.ctx, execCtx, b.fs); err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateDir adds a directory creation operation to the batch.
func (b *SimpleBatchImpl) CreateDir(path string, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0755) // Default directory mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation using the registry
	op, err := b.createOperation("create_directory", path)
	if err != nil {
		return nil, err
	}

	// Create and set the directory item for this operation
	dirItem := targets.NewDirectory(path).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, dirItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateDir operation: %w", err)
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"mode": fileMode.String(),
	}); err != nil {
		return nil, err
	}

	// Add to batch (which validates)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateDir(%s): %w", path, err)
	}

	return op, nil
}

// CreateFile adds a file creation operation to the batch.
func (b *SimpleBatchImpl) CreateFile(path string, content []byte, mode ...fs.FileMode) (interface{}, error) {
	fileMode := fs.FileMode(0644) // Default file mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation
	op, err := b.createOperation("create_file", path)
	if err != nil {
		return nil, err
	}

	// Create and set the file item for this operation
	fileItem := targets.NewFile(path).WithContent(content).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, fileItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateFile operation: %w", err)
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"content_length": len(content),
		"mode":           fileMode.String(),
		"content":        content,
	}); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateFile(%s): %w", path, err)
	}

	return op, nil
}

// Copy adds a copy operation to the batch.
func (b *SimpleBatchImpl) Copy(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("copy", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := b.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Copy(%s, %s): %w", src, dst, err)
	}

	// Compute checksum for source file (after validation passes)
	if fs, ok := b.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			// Set checksum on operation
			type checksumSetter interface {
				SetChecksum(path string, checksum *validation.ChecksumRecord)
			}
			if setter, ok := op.(checksumSetter); ok {
				setter.SetChecksum(src, checksum)
			}
			// Set source_checksum in details
			_ = b.setOperationDetails(op, map[string]interface{}{
				"source_checksum": checksum.MD5,
			})
		}
	}

	return op, nil
}

// Move adds a move operation to the batch.
func (b *SimpleBatchImpl) Move(src, dst string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("move", src)
	if err != nil {
		return nil, err
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"destination": dst,
		"src":         src,
		"dst":         dst,
	}); err != nil {
		return nil, err
	}

	// Set paths
	if err := b.setOperationPaths(op, src, dst); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Move(%s, %s): %w", src, dst, err)
	}

	// Compute checksum for source file (after validation passes)
	if fs, ok := b.fs.(filesystem.FullFileSystem); ok {
		if checksum, err := validation.ComputeFileChecksum(fs, src); err == nil && checksum != nil {
			// Set checksum on operation
			type checksumSetter interface {
				SetChecksum(path string, checksum *validation.ChecksumRecord)
			}
			if setter, ok := op.(checksumSetter); ok {
				setter.SetChecksum(src, checksum)
			}
			// Set source_checksum in details
			_ = b.setOperationDetails(op, map[string]interface{}{
				"source_checksum": checksum.MD5,
			})
		}
	}

	return op, nil
}

// Delete adds a delete operation to the batch.
func (b *SimpleBatchImpl) Delete(path string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Delete(%s): %w", path, err)
	}

	return op, nil
}

// CreateSymlink adds a symbolic link creation operation to the batch.
func (b *SimpleBatchImpl) CreateSymlink(target, linkPath string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("create_symlink", linkPath)
	if err != nil {
		return nil, err
	}

	// Create and set the symlink item for this operation
	symlinkItem := targets.NewSymlink(linkPath, target)
	if err := b.registry.SetItemForOperation(op, symlinkItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateSymlink operation: %w", err)
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"target": target,
	}); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateSymlink(%s, %s): %w", target, linkPath, err)
	}

	return op, nil
}

// CreateArchive adds an archive creation operation to the batch.
func (b *SimpleBatchImpl) CreateArchive(archivePath string, format interface{}, sources ...string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("create_archive", archivePath)
	if err != nil {
		return nil, err
	}

	// Create and set the archive item for this operation
	archiveItem := targets.NewArchive(archivePath, format, sources...)
	if err := b.registry.SetItemForOperation(op, archiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for CreateArchive operation: %w", err)
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"format":  format,
		"sources": sources,
	}); err != nil {
		return nil, err
	}

	// Compute checksums for source files (after validation passes)
	if fs, ok := b.fs.(filesystem.FullFileSystem); ok {
		for _, source := range sources {
			if checksum, err := validation.ComputeFileChecksum(fs, source); err == nil && checksum != nil {
				// Set checksum on operation
				type checksumSetter interface {
					SetChecksum(path string, checksum *validation.ChecksumRecord)
				}
				if setter, ok := op.(checksumSetter); ok {
					setter.SetChecksum(source, checksum)
				}
			}
		}
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateArchive(%s): %w", archivePath, err)
	}

	return op, nil
}

// Unarchive adds an unarchive operation to the batch.
func (b *SimpleBatchImpl) Unarchive(archivePath, extractPath string) (interface{}, error) {
	return b.UnarchiveWithPatterns(archivePath, extractPath)
}

// UnarchiveWithPatterns adds an unarchive operation with patterns to the batch.
func (b *SimpleBatchImpl) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (interface{}, error) {
	// Create the operation
	op, err := b.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	// Create and set the unarchive item for this operation
	unarchiveItem := targets.NewUnarchive(archivePath, extractPath, patterns...)
	if err := b.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item for Unarchive operation: %w", err)
	}

	// Set operation details
	if err := b.setOperationDetails(op, map[string]interface{}{
		"extract_path": extractPath,
		"patterns":     patterns,
	}); err != nil {
		return nil, err
	}

	// Add to batch
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Unarchive(%s, %s): %w", archivePath, extractPath, err)
	}

	return op, nil
}

// Run executes all operations in the batch with default options.
func (b *SimpleBatchImpl) Run() (interface{}, error) {
	return b.RunWithOptions(core.PipelineOptions{})
}

// RunWithOptions executes all operations in the batch with specified options.
func (b *SimpleBatchImpl) RunWithOptions(opts interface{}) (interface{}, error) {
	// Convert options to PipelineOptions
	pipelineOpts, ok := opts.(core.PipelineOptions)
	if !ok {
		pipelineOpts = core.PipelineOptions{}
	}

	// Create execution pipeline
	logger := b.logger
	if logger == nil {
		logger = &noOpLogger{}
	}

	pipeline := execution.NewMemPipeline(logger)
	
	// Add operations to pipeline
	if err := pipeline.Add(b.operations...); err != nil {
		return nil, fmt.Errorf("failed to add operations to pipeline: %w", err)
	}

	// Create executor with our operations
	executor := execution.NewExecutor(pipeline, logger)
	
	// Execute operations
	result := executor.RunWithOptions(b.fs, pipelineOpts)
	
	return NewResult(result), nil
}

// RunRestorable executes all operations in the batch with restoration enabled.
func (b *SimpleBatchImpl) RunRestorable() (interface{}, error) {
	return b.RunRestorableWithBudget(core.DefaultMaxBackupMB)
}

// RunRestorableWithBudget executes all operations in the batch with restoration and custom budget.
func (b *SimpleBatchImpl) RunRestorableWithBudget(maxBackupMB int) (interface{}, error) {
	opts := core.PipelineOptions{
		Restorable:      true,
		MaxBackupSizeMB: maxBackupMB,
	}
	return b.RunWithOptions(opts)
}

// RunWithPrerequisites executes all operations in the batch with prerequisite resolution enabled.
func (b *SimpleBatchImpl) RunWithPrerequisites() (interface{}, error) {
	return b.RunWithPrerequisitesAndBudget(core.DefaultMaxBackupMB)
}

// RunWithPrerequisitesAndBudget executes all operations with prerequisite resolution and custom budget.
func (b *SimpleBatchImpl) RunWithPrerequisitesAndBudget(maxBackupMB int) (interface{}, error) {
	opts := core.PipelineOptions{
		Restorable:           true,
		MaxBackupSizeMB:     maxBackupMB,
		ResolvePrerequisites: true,
	}
	return b.RunWithOptions(opts)
}

// Helper methods (same as BatchImpl)
func (b *SimpleBatchImpl) generateID(opType, path string) core.OperationID {
	b.idCounter++
	return core.OperationID(fmt.Sprintf("%s_%d_%s", opType, b.idCounter, path))
}

func (b *SimpleBatchImpl) createOperation(opType, path string) (interface{}, error) {
	id := b.generateID(opType, path)
	return b.registry.CreateOperation(id, opType, path)
}

func (b *SimpleBatchImpl) setOperationDetails(op interface{}, details map[string]interface{}) error {
	type detailSetter interface {
		SetDescriptionDetail(key string, value interface{})
	}

	if setter, ok := op.(detailSetter); ok {
		for key, value := range details {
			setter.SetDescriptionDetail(key, value)
		}
		return nil
	}

	return fmt.Errorf("operation does not support setting details")
}

func (b *SimpleBatchImpl) setOperationPaths(op interface{}, src, dst string) error {
	type pathSetter interface {
		SetPaths(src, dst string)
	}

	if setter, ok := op.(pathSetter); ok {
		setter.SetPaths(src, dst)
		return nil
	}

	return fmt.Errorf("operation does not support setting paths")
}