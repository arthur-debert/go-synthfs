package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)

// computeFileChecksum computes MD5 checksum for a file
func (b *Batch) computeFileChecksum(filePath string) (*ChecksumRecord, error) {
	// Phase I, Milestone 3: Basic checksumming for copy/move operations
	return validation.ComputeFileChecksum(b.fs, filePath)
}

// Batch represents a collection of filesystem operations that can be validated and executed as a unit.
// It provides an imperative API with validate-as-you-go and automatic dependency resolution.
type Batch struct {
	operations  []Operation
	fs          FullFileSystem // Use FullFileSystem to have access to Stat method
	ctx         context.Context
	idCounter   int
	pathTracker *PathStateTracker     // Phase II: Track projected path state
	registry    core.OperationFactory // Phase 3: Operation registry for decoupling
}

// NewBatch creates a new operation batch with default filesystem and context.
func NewBatch() *Batch {
	fs := filesystem.NewOSFileSystem(".") // Use current directory as default root
	return &Batch{
		operations:  []Operation{},
		fs:          fs,
		ctx:         context.Background(),
		idCounter:   0,
		pathTracker: NewPathStateTracker(fs), // Phase II: Initialize path state tracker
		registry:    GetDefaultRegistry(),    // Phase 3: Use default operation registry
	}
}

// WithFileSystem sets the filesystem for the batch operations.
func (b *Batch) WithFileSystem(fs FullFileSystem) *Batch {
	b.fs = fs
	// Recreate pathTracker with new filesystem
	b.pathTracker = NewPathStateTracker(fs)
	return b
}

// WithContext sets the context for the batch operations.
func (b *Batch) WithContext(ctx context.Context) *Batch {
	b.ctx = ctx
	return b
}

// WithRegistry sets a custom operation registry for the batch.
func (b *Batch) WithRegistry(registry core.OperationFactory) *Batch {
	b.registry = registry
	return b
}

// Operations returns all operations currently in the batch.
func (b *Batch) Operations() []Operation {
	// Return a copy to prevent external modification
	opsCopy := make([]Operation, len(b.operations))
	copy(opsCopy, b.operations)
	return opsCopy
}

// add adds an operation to the batch and validates it against the projected filesystem state
func (b *Batch) add(op Operation) error {
	// First validate the operation itself (basic validation)
	if err := op.Validate(b.ctx, b.fs); err != nil {
		// For create operations, if the error is "file already exists" and the file
		// is scheduled for deletion, we should give a more specific error
		if validationErr, ok := err.(*ValidationError); ok {
			if validationErr.Reason == "file already exists" {
				if b.pathTracker.IsDeleted(op.Describe().Path) {
					return fmt.Errorf("validation error for operation %s (%s): path was scheduled for deletion",
						op.ID(), op.Describe().Path)
				}
			}
		}
		return err
	}

	// Phase II: Validate against projected state and update it.
	if err := b.pathTracker.UpdateState(op); err != nil {
		return err
	}

	// Auto-resolve dependencies (ensure parent directories exist)
	var opPath string
	// Check if it's an adapter wrapping an operations package operation
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		_, dst := adapter.opsOperation.GetPaths()
		if dst != "" {
			opPath = dst
		} else {
			opPath = op.Describe().Path
		}
	} else {
		opPath = op.Describe().Path
	}

	if parentDeps := b.ensureParentDirectories(opPath); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
	}

	b.operations = append(b.operations, op)
	return nil
}

// CreateDir adds a directory creation operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) CreateDir(path string, mode ...fs.FileMode) (Operation, error) {
	fileMode := fs.FileMode(0755) // Default directory mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation using the registry
	op, err := b.createOperation("create_directory", path)
	if err != nil {
		return nil, err
	}

	// Set the FsItem for this create operation
	dirItem := NewDirectory(path).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, dirItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	op.SetDescriptionDetail("mode", fileMode.String())

	// Add to batch first (which validates against projected state)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateDir(%s): %w", path, err)
	}

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
	op, err := b.createOperation("create_file", path)
	if err != nil {
		return nil, err
	}

	// Set the FsItem for this create operation
	fileItem := NewFile(path).WithContent(content).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, fileItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	// Set description details on the adapter
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("content_length", len(content))
		adapter.SetDescriptionDetail("mode", fileMode.String())
	}

	// Add to batch first (which validates against projected state)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateFile(%s): %w", path, err)
	}

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
	op, err := b.createOperation("copy", src)
	if err != nil {
		return nil, err
	}
	// Set description details and paths based on operation type
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("destination", dst)
		adapter.SetPaths(src, dst)
	}

	// Add to batch first (which validates the operation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Copy(%s, %s): %w", src, dst, err)
	}

	// Phase I, Milestone 3: Compute checksum for source file (after validation)
	if checksum, err := b.computeFileChecksum(src); err == nil && checksum != nil {
		if adapter, ok := op.(*OperationsPackageAdapter); ok {
			adapter.SetChecksum(src, checksum)
			adapter.SetDescriptionDetail("source_checksum", checksum.MD5)
		}
		Logger().Debug().
			Str("op_id", string(op.ID())).
			Str("src", src).
			Str("checksum", checksum.MD5).
			Msg("Computed source checksum for copy operation")
	}

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
	op, err := b.createOperation("move", src)
	if err != nil {
		return nil, err
	}
	// Set description details and paths based on operation type
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("destination", dst)
		adapter.SetPaths(src, dst)
	}

	// Add to batch first (which validates the operation)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Move(%s, %s): %w", src, dst, err)
	}

	// Phase I, Milestone 3: Compute checksum for source file (after validation)
	if checksum, err := b.computeFileChecksum(src); err == nil && checksum != nil {
		if adapter, ok := op.(*OperationsPackageAdapter); ok {
			adapter.SetChecksum(src, checksum)
			adapter.SetDescriptionDetail("source_checksum", checksum.MD5)
		}
		Logger().Debug().
			Str("op_id", string(op.ID())).
			Str("src", src).
			Str("checksum", checksum.MD5).
			Msg("Computed source checksum for move operation")
	}

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
	op, err := b.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	// We'll validate when adding to batch

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Delete(%s): %w", path, err)
	}

	// Add to batch (no dependency resolution needed for delete)
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("path", path).
		Msg("Delete operation added to batch")

	return op, nil
}

// CreateSymlink adds a symbolic link creation operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) CreateSymlink(target, linkPath string) (Operation, error) {
	// Create the operation using the registry
	op, err := b.createOperation("create_symlink", linkPath)
	if err != nil {
		return nil, err
	}

	// Set the SymlinkItem for this create operation
	symlinkItem := NewSymlink(linkPath, target)
	if err := b.registry.SetItemForOperation(op, symlinkItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	// Set description detail based on operation type
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("target", target)
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateSymlink(%s, %s): %w", target, linkPath, err)
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("target", target).
		Str("link_path", linkPath).
		Msg("CreateSymlink operation added to batch")

	return op, nil
}

// CreateArchive adds an archive creation operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) CreateArchive(archivePath string, format ArchiveFormat, sources ...string) (Operation, error) {
	// Validate inputs
	if len(sources) == 0 {
		return nil, fmt.Errorf("validation failed for CreateArchive(%s): must specify at least one source", archivePath)
	}

	// Create the operation using the registry
	op, err := b.createOperation("create_archive", archivePath)
	if err != nil {
		return nil, err
	}

	// Set the ArchiveItem for this create operation
	archiveItem := NewArchive(archivePath, format, sources)
	if err := b.registry.SetItemForOperation(op, archiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	// Set description details based on operation type
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("format", targets.ArchiveFormat(format).String())
		adapter.SetDescriptionDetail("sources", sources)
		adapter.SetDescriptionDetail("source_count", len(sources))
	}

	// Phase I, Milestone 3: Compute checksums for all source files
	for _, source := range sources {
		if checksum, err := b.computeFileChecksum(source); err != nil {
			return nil, fmt.Errorf("validation failed for CreateArchive(%s): failed to compute checksum for source %s: %w", archivePath, source, err)
		} else if checksum != nil {
			if adapter, ok := op.(*OperationsPackageAdapter); ok {
				adapter.SetChecksum(source, checksum)
			}
		}
	}
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("sources_checksummed", len(sources))
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add CreateArchive(%s): %w", archivePath, err)
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("archive_path", archivePath).
		Str("format", targets.ArchiveFormat(format).String()).
		Int("source_count", len(sources)).
		Msg("CreateArchive operation added to batch")

	return op, nil
}

// Unarchive adds an unarchive operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) Unarchive(archivePath, extractPath string) (Operation, error) {
	// Create the operation
	op, err := b.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	// Set the UnarchiveItem for this operation
	unarchiveItem := NewUnarchive(archivePath, extractPath)
	if err := b.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	// Set description detail based on operation type
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("extract_path", extractPath)
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add Unarchive(%s, %s): %w", archivePath, extractPath, err)
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("archive_path", archivePath).
		Str("extract_path", extractPath).
		Msg("Unarchive operation added to batch")

	return op, nil
}

// UnarchiveWithPatterns adds an unarchive operation with pattern filtering to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) UnarchiveWithPatterns(archivePath, extractPath string, patterns ...string) (Operation, error) {
	// Create the operation
	op, err := b.createOperation("unarchive", archivePath)
	if err != nil {
		return nil, err
	}

	// Set the UnarchiveItem for this operation with patterns
	unarchiveItem := NewUnarchive(archivePath, extractPath).WithPatterns(patterns...)
	if err := b.registry.SetItemForOperation(op, unarchiveItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	// Set description details based on operation type
	if adapter, ok := op.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("extract_path", extractPath)
		adapter.SetDescriptionDetail("patterns", patterns)
		adapter.SetDescriptionDetail("pattern_count", len(patterns))
	}

	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("failed to add UnarchiveWithPatterns(%s, %s): %w", archivePath, extractPath, err)
	}

	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("archive_path", archivePath).
		Str("extract_path", extractPath).
		Strs("patterns", patterns).
		Msg("UnarchiveWithPatterns operation added to batch")

	return op, nil
}

// Run runs all operations in the batch using the existing infrastructure.
func (b *Batch) Run() (*Result, error) {
	return b.RunWithOptions(DefaultPipelineOptions())
}

// RunWithOptions runs all operations in the batch with specified options (Phase III).
func (b *Batch) RunWithOptions(opts PipelineOptions) (*Result, error) {
	Logger().Info().
		Int("operation_count", len(b.operations)).
		Bool("restorable", opts.Restorable).
		Int("max_backup_mb", opts.MaxBackupSizeMB).
		Msg("executing batch")

	// Resolve implicit dependencies before execution
	if err := b.resolveImplicitDependencies(); err != nil {
		return nil, fmt.Errorf("failed to resolve implicit dependencies: %w", err)
	}

	// Create executor and pipeline
	executor := NewExecutor()
	pipeline := NewMemPipeline()

	// Add all operations to pipeline
	if err := pipeline.Add(b.operations...); err != nil {
		return nil, fmt.Errorf("failed to add operations to pipeline: %w", err)
	}

	// Run using Phase III infrastructure
	result := executor.RunWithOptions(b.ctx, pipeline, b.fs, opts)

	Logger().Info().
		Bool("success", result.Success).
		Int("operations_executed", len(result.Operations)).
		Int("restore_operations", len(result.RestoreOps)).
		Dur("duration", result.Duration).
		Msg("batch run completed")

	return result, nil
}

// RunRestorable runs all operations with backup enabled using the default 10MB budget (Phase III).
// This is a convenience method for the common case of wanting restorable execution.
func (b *Batch) RunRestorable() (*Result, error) {
	return b.RunWithOptions(PipelineOptions{
		Restorable:      true,
		MaxBackupSizeMB: 10,
	})
}

// RunRestorableWithBudget runs all operations with backup enabled using a custom budget (Phase III).
func (b *Batch) RunRestorableWithBudget(maxBackupMB int) (*Result, error) {
	return b.RunWithOptions(PipelineOptions{
		Restorable:      true,
		MaxBackupSizeMB: maxBackupMB,
	})
}

// generateID creates a unique operation ID based on type and path.
func (b *Batch) generateID(opType, path string) OperationID {
	b.idCounter++
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")
	return OperationID(fmt.Sprintf("batch_%d_%s_%s", b.idCounter, opType, cleanPath))
}

// createOperation is a helper method to create operations using the registry
func (b *Batch) createOperation(opType, path string) (Operation, error) {
	opID := b.generateID(opType, path)
	opInterface, err := b.registry.CreateOperation(opID, opType, path)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation: %w", err)
	}

	// The registry returns OperationsPackageAdapter
	// which implements the Operation interface
	if op, ok := opInterface.(Operation); ok {
		return op, nil
	}

	return nil, fmt.Errorf("registry returned unexpected type: %T", opInterface)
}

// ensureParentDirectories analyzes a path and adds CreateDir operations for missing parent directories.
// Returns the operation IDs of any auto-generated parent directory operations.
func (b *Batch) ensureParentDirectories(path string) []OperationID {
	// Clean and normalize the path
	cleanPath := filepath.Clean(path)
	parentDir := filepath.Dir(cleanPath)

	var dependencyIDs []OperationID

	// If parent is root or current directory, no parent needed
	if parentDir == "." || parentDir == "/" || parentDir == cleanPath {
		return dependencyIDs
	}

	// Phase II: Check if parent directory is projected to exist
	parentState, err := b.pathTracker.GetState(parentDir)
	if err == nil && parentState.WillExist {
		if parentState.CreatedBy != "" {
			// If it was created by a previous operation in this batch, we depend on it.
			dependencyIDs = append(dependencyIDs, parentState.CreatedBy)
		}
		// If it exists (either on disk or projected), we don't need to do anything else.
		return dependencyIDs
	}

	// Recursively ensure parent's parents exist
	parentDeps := b.ensureParentDirectories(parentDir)
	dependencyIDs = append(dependencyIDs, parentDeps...)

	// Create operation for the parent directory
	parentOp, err := b.createOperation("create_directory", parentDir)
	if err != nil {
		// If we can't create parent directory operation, skip it
		return dependencyIDs
	}
	parentDirItem := NewDirectory(parentDir).WithMode(0755)
	if err := b.registry.SetItemForOperation(parentOp, parentDirItem); err != nil {
		// If we can't set item, skip it
		return dependencyIDs
	}
	// Set description detail based on operation type
	if adapter, ok := parentOp.(*OperationsPackageAdapter); ok {
		adapter.SetDescriptionDetail("mode", "0755")
	}

	// Add dependencies from parent's parents
	for _, depID := range parentDeps {
		if adapter, ok := parentOp.(*OperationsPackageAdapter); ok {
			adapter.AddDependency(depID)
		}
	}

	// Validate the auto-generated parent operation and add it to the batch
	if err := b.add(parentOp); err != nil {
		// Log error but don't fail - might be resolved at execution time
		Logger().Warn().
			Err(err).
			Str("path", parentDir).
			Msg("validation warning for auto-generated parent directory")
		// Even if it fails validation (e.g., conflict), other operations might depend on its ID.
		// We'll still add the ID to dependencies. The conflict will be reported when the batch is run.
	}

	dependencyIDs = append(dependencyIDs, parentOp.ID())

	Logger().Info().
		Str("op_id", string(parentOp.ID())).
		Str("path", parentDir).
		Str("reason", "auto-generated for parent directory").
		Msg("CreateDir operation auto-added to batch")

	return dependencyIDs
}

// resolveImplicitDependencies analyzes all operations and adds dependencies to prevent conflicts.
// This ensures operations that depend on the same files are executed in the correct order.
func (b *Batch) resolveImplicitDependencies() error {
	Logger().Info().
		Int("operations", len(b.operations)).
		Msg("resolving implicit dependencies between operations")

	// Build maps of operations by the files they read/write/delete
	fileReaders := make(map[string][]int)    // path -> operation indices that read this file
	fileWriters := make(map[string][]int)    // path -> operation indices that write/create this file
	fileMovers := make(map[string][]int)     // path -> operation indices that move/delete this file
	symlinkTargets := make(map[string][]int) // target path -> operation indices that create symlinks to this target

	for i, op := range b.operations {
		desc := op.Describe()

		switch desc.Type {
		case "create_file", "create_directory":
			fileWriters[desc.Path] = append(fileWriters[desc.Path], i)

		case "copy":
			// Copy reads source and writes destination
			var srcPath, dstPath string
			if adapter, ok := op.(*OperationsPackageAdapter); ok {
				srcPath, dstPath = adapter.opsOperation.GetPaths()
			}
			if srcPath != "" {
				fileReaders[srcPath] = append(fileReaders[srcPath], i)
			}
			if dstPath != "" {
				fileWriters[dstPath] = append(fileWriters[dstPath], i)
			}

		case "move":
			// Move reads source and writes destination, then deletes source
			var srcPath, dstPath string
			if adapter, ok := op.(*OperationsPackageAdapter); ok {
				srcPath, dstPath = adapter.opsOperation.GetPaths()
			}
			if srcPath != "" {
				fileReaders[srcPath] = append(fileReaders[srcPath], i)
				fileMovers[srcPath] = append(fileMovers[srcPath], i)
			}
			if dstPath != "" {
				fileWriters[dstPath] = append(fileWriters[dstPath], i)
			}

		case "delete":
			fileMovers[desc.Path] = append(fileMovers[desc.Path], i)

		case "create_symlink":
			// Symlink creation depends on the target existing
			if target, ok := desc.Details["target"]; ok {
				if targetPath, ok := target.(string); ok {
					symlinkTargets[targetPath] = append(symlinkTargets[targetPath], i)
					// A symlink operation "reads" its target path, so it must happen
					// before the target is moved or deleted.
					fileReaders[targetPath] = append(fileReaders[targetPath], i)
				}
			}
			fileWriters[desc.Path] = append(fileWriters[desc.Path], i)

		case "create_archive":
			// Archive reads all source files
			if archiveItem := op.GetItem(); archiveItem != nil {
				if archive, ok := archiveItem.(*ArchiveItem); ok {
					for _, source := range archive.Sources() {
						fileReaders[source] = append(fileReaders[source], i)
					}
				}
			}
			fileWriters[desc.Path] = append(fileWriters[desc.Path], i)

		case "unarchive":
			// Unarchive reads archive file and writes extracted files
			fileReaders[desc.Path] = append(fileReaders[desc.Path], i)
			// Note: We can't easily predict all extracted files without opening the archive,
			// so we'll rely on explicit dependencies and validation at execution time
		}
	}

	// Now add dependencies to ensure correct ordering
	dependenciesAdded := 0

	// Rule 1: Operations that move/delete files must come after operations that read those files
	for filePath, movers := range fileMovers {
		if readers, hasReaders := fileReaders[filePath]; hasReaders {
			for _, moverIdx := range movers {
				for _, readerIdx := range readers {
					if readerIdx != moverIdx {
						// Reader must come before mover
						moverOp := b.operations[moverIdx]
						readerID := b.operations[readerIdx].ID()

						// Check if dependency already exists and add if not
						if adapter, ok := moverOp.(*OperationsPackageAdapter); ok {
							exists := false
							for _, dep := range adapter.Dependencies() {
								if dep == readerID {
									exists = true
									break
								}
							}
							if !exists {
								adapter.AddDependency(readerID)
								dependenciesAdded++
								Logger().Info().
									Str("operation", string(adapter.ID())).
									Str("depends_on", string(readerID)).
									Str("reason", fmt.Sprintf("mover depends on reader of %s", filePath)).
									Msg("added implicit dependency")
							}
						}
					}
				}
			}
		}
	}

	// Rule 2: Operations that create symlinks must come after operations that create their targets
	for targetPath, symlinkCreators := range symlinkTargets {
		if writers, hasWriters := fileWriters[targetPath]; hasWriters {
			for _, symlinkIdx := range symlinkCreators {
				for _, writerIdx := range writers {
					if writerIdx != symlinkIdx {
						// Writer must come before symlink creator
						symlinkOp := b.operations[symlinkIdx]
						writerID := b.operations[writerIdx].ID()

						// Check if dependency already exists and add if not
						if adapter, ok := symlinkOp.(*OperationsPackageAdapter); ok {
							exists := false
							for _, dep := range adapter.Dependencies() {
								if dep == writerID {
									exists = true
									break
								}
							}
							if !exists {
								adapter.AddDependency(writerID)
								dependenciesAdded++
								Logger().Info().
									Str("operation", string(adapter.ID())).
									Str("depends_on", string(writerID)).
									Str("reason", fmt.Sprintf("symlink depends on target creation %s", targetPath)).
									Msg("added implicit dependency")
							}
						}
					}
				}
			}
		}
	}

	Logger().Info().
		Int("dependencies_added", dependenciesAdded).
		Msg("implicit dependency resolution completed")

	return nil
}
