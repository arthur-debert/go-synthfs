package synthfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// PathTracker tracks operations by path to detect conflicts
type PathTracker struct {
	CreatedPaths  map[string]OperationID  // paths that will be created
	DeletedPaths  map[string]OperationID  // paths that will be deleted
	ModifiedPaths map[string]OperationID  // paths that will be modified
}

// NewPathTracker creates a new path tracker
func NewPathTracker() *PathTracker {
	return &PathTracker{
		CreatedPaths:  make(map[string]OperationID),
		DeletedPaths:  make(map[string]OperationID),
		ModifiedPaths: make(map[string]OperationID),
	}
}

// CheckPathConflict checks if adding an operation would conflict with existing operations
func (pt *PathTracker) CheckPathConflict(opID OperationID, opType, path string, destPath string) error {
	// Phase I, Milestone 2: Duplicate path detection
	switch opType {
	case "create_file", "create_directory":
		if existingOpID, exists := pt.CreatedPaths[path]; exists {
			return fmt.Errorf("operation %s conflicts with %s: cannot create %s - already scheduled for creation", 
				opID, existingOpID, path)
		}
		if existingOpID, exists := pt.DeletedPaths[path]; exists {
			return fmt.Errorf("operation %s conflicts with %s: cannot create %s - already scheduled for deletion", 
				opID, existingOpID, path)
		}
		
	case "delete":
		if existingOpID, exists := pt.DeletedPaths[path]; exists {
			return fmt.Errorf("operation %s conflicts with %s: cannot delete %s - already scheduled for deletion", 
				opID, existingOpID, path)
		}
		
	case "copy", "move":
		// Check destination conflicts
		if destPath != "" {
			if existingOpID, exists := pt.CreatedPaths[destPath]; exists {
				return fmt.Errorf("operation %s conflicts with %s: cannot %s to %s - already scheduled for creation", 
					opID, existingOpID, opType, destPath)
			}
			if existingOpID, exists := pt.ModifiedPaths[destPath]; exists {
				return fmt.Errorf("operation %s conflicts with %s: cannot %s to %s - already scheduled for modification", 
					opID, existingOpID, opType, destPath)
			}
		}
		
	case "create_symlink":
		if existingOpID, exists := pt.CreatedPaths[path]; exists {
			return fmt.Errorf("operation %s conflicts with %s: cannot create symlink %s - already scheduled for creation", 
				opID, existingOpID, path)
		}
		
	case "create_archive":
		if existingOpID, exists := pt.CreatedPaths[path]; exists {
			return fmt.Errorf("operation %s conflicts with %s: cannot create archive %s - already scheduled for creation", 
				opID, existingOpID, path)
		}
		
	case "unarchive":
		// Unarchive operations can conflict on extraction directory, but this is complex to predict
		// For now, we'll rely on execution-time validation
	}
	
	return nil
}

// RecordOperation records an operation in the path tracker
func (pt *PathTracker) RecordOperation(opID OperationID, opType, path string, destPath string) {
	// Phase I, Milestone 2: Record path usage
	switch opType {
	case "create_file", "create_directory", "create_symlink", "create_archive":
		pt.CreatedPaths[path] = opID
		
	case "delete":
		pt.DeletedPaths[path] = opID
		
	case "copy":
		if destPath != "" {
			pt.CreatedPaths[destPath] = opID
		}
		
	case "move":
		if destPath != "" {
			pt.CreatedPaths[destPath] = opID
		}
		pt.DeletedPaths[path] = opID
		
	case "unarchive":
		// Complex to predict all extracted paths, skip for now
	}
}

// Batch represents a collection of filesystem operations that can be validated and executed as a unit.
// It provides an imperative API with validate-as-you-go and automatic dependency resolution.
type Batch struct {
	operations  []Operation
	fs          FullFileSystem // Use FullFileSystem to have access to Stat method
	ctx         context.Context
	idCounter   int
	pathTracker *PathTracker // Phase I, Milestone 2: Track path conflicts
}

// NewBatch creates a new operation batch with default filesystem and context.
func NewBatch() *Batch {
	return &Batch{
		operations:  []Operation{},
		fs:          NewOSFileSystem("."), // Use current directory as default root
		ctx:         context.Background(),
		idCounter:   0,
		pathTracker: NewPathTracker(), // Phase I, Milestone 2: Initialize path tracker
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
	if parentDeps := b.ensureParentDirectories(path); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
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
	if parentDeps := b.ensureParentDirectories(path); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
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
	if parentDeps := b.ensureParentDirectories(dst); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
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
	if parentDeps := b.ensureParentDirectories(dst); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
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

// CreateSymlink adds a symbolic link creation operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) CreateSymlink(target, linkPath string) (Operation, error) {
	// Create the operation
	opID := b.generateID("create_symlink", linkPath)
	op := NewSimpleOperation(opID, "create_symlink", linkPath)

	// Set the SymlinkItem for this create operation
	symlinkItem := NewSymlink(linkPath, target)
	op.SetItem(symlinkItem)
	op.SetDescriptionDetail("target", target)

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for CreateSymlink(%s, %s): %w", target, linkPath, err)
	}

	// Auto-resolve dependencies (ensure parent directories exist)
	if parentDeps := b.ensureParentDirectories(linkPath); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
	}

	// Add to batch
	b.operations = append(b.operations, op)
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

	// Create the operation
	opID := b.generateID("create_archive", archivePath)
	op := NewSimpleOperation(opID, "create_archive", archivePath)

	// Set the ArchiveItem for this create operation
	archiveItem := NewArchive(archivePath, format, sources)
	op.SetItem(archiveItem)
	op.SetDescriptionDetail("format", format.String())
	op.SetDescriptionDetail("source_count", len(sources))

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for CreateArchive(%s): %w", archivePath, err)
	}

	// Auto-resolve dependencies (ensure parent directories exist)
	if parentDeps := b.ensureParentDirectories(archivePath); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
	}

	// Add to batch
	b.operations = append(b.operations, op)
	Logger().Info().
		Str("op_id", string(op.ID())).
		Str("archive_path", archivePath).
		Str("format", format.String()).
		Int("source_count", len(sources)).
		Msg("CreateArchive operation added to batch")

	return op, nil
}

// Unarchive adds an unarchive operation to the batch.
// It validates the operation immediately and resolves dependencies automatically.
func (b *Batch) Unarchive(archivePath, extractPath string) (Operation, error) {
	// Create the operation
	opID := b.generateID("unarchive", archivePath+"_to_"+extractPath)
	op := NewSimpleOperation(opID, "unarchive", archivePath)

	// Set the UnarchiveItem for this operation
	unarchiveItem := NewUnarchive(archivePath, extractPath)
	op.SetItem(unarchiveItem)
	op.SetDescriptionDetail("extract_path", extractPath)

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for Unarchive(%s, %s): %w", archivePath, extractPath, err)
	}

	// Auto-resolve dependencies (ensure extract path parent directories exist)
	if parentDeps := b.ensureParentDirectories(extractPath); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
	}

	// Add to batch
	b.operations = append(b.operations, op)
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
	opID := b.generateID("unarchive", archivePath+"_to_"+extractPath)
	op := NewSimpleOperation(opID, "unarchive", archivePath)

	// Set the UnarchiveItem for this operation with patterns
	unarchiveItem := NewUnarchive(archivePath, extractPath).WithPatterns(patterns...)
	op.SetItem(unarchiveItem)
	op.SetDescriptionDetail("extract_path", extractPath)
	op.SetDescriptionDetail("patterns", patterns)
	op.SetDescriptionDetail("pattern_count", len(patterns))

	// Validate immediately
	if err := op.Validate(b.ctx, b.fs); err != nil {
		return nil, fmt.Errorf("validation failed for UnarchiveWithPatterns(%s, %s): %w", archivePath, extractPath, err)
	}

	// Auto-resolve dependencies (ensure extract path parent directories exist)
	if parentDeps := b.ensureParentDirectories(extractPath); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
	}

	// Add to batch
	b.operations = append(b.operations, op)
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
	Logger().Info().
		Int("operation_count", len(b.operations)).
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

	// Run using existing infrastructure
	result := executor.Run(b.ctx, pipeline, b.fs)

	Logger().Info().
		Bool("success", result.Success).
		Int("operations_executed", len(result.Operations)).
		Dur("duration", result.Duration).
		Msg("batch run completed")

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

	// Check if parent directory already exists in filesystem
	if _, err := b.fs.Stat(parentDir); err == nil {
		// Parent exists, no need to create
		return dependencyIDs
	}

	// Check if we already have a CreateDir operation for this parent
	for _, op := range b.operations {
		if op.Describe().Type == "create_directory" && op.Describe().Path == parentDir {
			// Already have an operation to create this parent
			dependencyIDs = append(dependencyIDs, op.ID())
			return dependencyIDs
		}
	}

	// Recursively ensure parent's parents exist
	parentDeps := b.ensureParentDirectories(parentDir)
	dependencyIDs = append(dependencyIDs, parentDeps...)

	// Create operation for the parent directory
	parentOpID := b.generateID("create_dir_auto", parentDir)
	parentOp := NewSimpleOperation(parentOpID, "create_directory", parentDir)
	parentDirItem := NewDirectory(parentDir).WithMode(0755)
	parentOp.SetItem(parentDirItem)
	parentOp.SetDescriptionDetail("mode", "0755")

	// Add dependencies from parent's parents
	for _, depID := range parentDeps {
		parentOp.AddDependency(depID)
	}

	// Validate the auto-generated parent operation
	if err := parentOp.Validate(b.ctx, b.fs); err != nil {
		// Log error but don't fail - might be resolved at execution time
		Logger().Warn().
			Err(err).
			Str("path", parentDir).
			Msg("validation warning for auto-generated parent directory")
	}

	// Add to operations
	b.operations = append(b.operations, parentOp)
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
			if simpleOp, ok := op.(*SimpleOperation); ok {
				srcPath := simpleOp.GetSrcPath()
				dstPath := simpleOp.GetDstPath()
				if srcPath != "" {
					fileReaders[srcPath] = append(fileReaders[srcPath], i)
				}
				if dstPath != "" {
					fileWriters[dstPath] = append(fileWriters[dstPath], i)
				}
			}
			
		case "move":
			// Move reads source and writes destination, then deletes source
			if simpleOp, ok := op.(*SimpleOperation); ok {
				srcPath := simpleOp.GetSrcPath()
				dstPath := simpleOp.GetDstPath()
				if srcPath != "" {
					fileReaders[srcPath] = append(fileReaders[srcPath], i)
					fileMovers[srcPath] = append(fileMovers[srcPath], i)
				}
				if dstPath != "" {
					fileWriters[dstPath] = append(fileWriters[dstPath], i)
				}
			}
			
		case "delete":
			fileMovers[desc.Path] = append(fileMovers[desc.Path], i)
			
		case "create_symlink":
			// Symlink creation depends on the target existing
			if target, ok := desc.Details["target"]; ok {
				if targetPath, ok := target.(string); ok {
					symlinkTargets[targetPath] = append(symlinkTargets[targetPath], i)
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
						if simpleOp, ok := b.operations[moverIdx].(*SimpleOperation); ok {
							readerID := b.operations[readerIdx].ID()
							// Check if dependency already exists
							exists := false
							for _, dep := range simpleOp.Dependencies() {
								if dep == readerID {
									exists = true
									break
								}
							}
							if !exists {
								simpleOp.AddDependency(readerID)
								dependenciesAdded++
								Logger().Info().
									Str("operation", string(simpleOp.ID())).
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
						if simpleOp, ok := b.operations[symlinkIdx].(*SimpleOperation); ok {
							writerID := b.operations[writerIdx].ID()
							// Check if dependency already exists
							exists := false
							for _, dep := range simpleOp.Dependencies() {
								if dep == writerID {
									exists = true
									break
								}
							}
							if !exists {
								simpleOp.AddDependency(writerID)
								dependenciesAdded++
								Logger().Info().
									Str("operation", string(simpleOp.ID())).
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
