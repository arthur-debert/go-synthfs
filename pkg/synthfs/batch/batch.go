package batch

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

// Batch represents a collection of filesystem operations that can be validated and executed as a unit.
// It provides an imperative API with validate-as-you-go and automatic dependency resolution.
type Batch struct {
	operations  []OperationInterface
	fs          FilesystemInterface
	ctx         context.Context
	idCounter   int
	pathTracker PathStateTrackerInterface
	registry    core.OperationFactory
	logger      core.Logger
}

// OperationInterface defines what the batch package needs from operations
type OperationInterface interface {
	core.OperationMetadata
	core.DependencyAware
	core.ExecutableV2
	Validate(ctx context.Context, fsys FilesystemInterface) error
	SetDescriptionDetail(key string, value interface{})
	AddDependency(depID core.OperationID)
	SetPaths(src, dst string)
	GetItem() interface{}
}

// FilesystemInterface defines what the batch package needs from filesystems
type FilesystemInterface interface {
	Stat(name string) (interface{}, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(name string) error
	RemoveAll(name string) error
	Rename(oldpath, newpath string) error
	Symlink(oldname, newname string) error
}

// PathStateTrackerInterface defines what the batch package needs from path state tracking
type PathStateTrackerInterface interface {
	UpdateState(op OperationInterface) error
	IsDeleted(path string) bool
}

// NewBatch creates a new operation batch
func NewBatch(fs FilesystemInterface, registry core.OperationFactory, logger core.Logger, pathTracker PathStateTrackerInterface) *Batch {
	return &Batch{
		operations:  []OperationInterface{},
		fs:          fs,
		ctx:         context.Background(),
		idCounter:   0,
		pathTracker: pathTracker,
		registry:    registry,
		logger:      logger,
	}
}

// WithFileSystem sets the filesystem for the batch operations
func (b *Batch) WithFileSystem(fs FilesystemInterface) *Batch {
	b.fs = fs
	return b
}

// WithContext sets the context for the batch operations
func (b *Batch) WithContext(ctx context.Context) *Batch {
	b.ctx = ctx
	return b
}

// WithRegistry sets a custom operation registry for the batch
func (b *Batch) WithRegistry(registry core.OperationFactory) *Batch {
	b.registry = registry
	return b
}

// Operations returns all operations currently in the batch
func (b *Batch) Operations() []OperationInterface {
	// Return a copy to prevent external modification
	opsCopy := make([]OperationInterface, len(b.operations))
	copy(opsCopy, b.operations)
	return opsCopy
}

// add adds an operation to the batch and validates it against the projected filesystem state
func (b *Batch) add(op OperationInterface) error {
	// First validate the operation itself (basic validation)
	if err := op.Validate(b.ctx, b.fs); err != nil {
		// For create operations, if the error contains "file already exists" and the file
		// is scheduled for deletion, we should give a more specific error
		if b.pathTracker.IsDeleted(op.Describe().Path) {
			return fmt.Errorf("validation error for operation %s (%s): path was scheduled for deletion", 
				op.ID(), op.Describe().Path)
		}
		return err
	}

	// Validate against projected state and update it
	if err := b.pathTracker.UpdateState(op); err != nil {
		return err
	}

	// Auto-resolve dependencies (ensure parent directories exist)
	opPath := op.Describe().Path
	if parentDeps := b.ensureParentDirectories(opPath); len(parentDeps) > 0 {
		for _, depID := range parentDeps {
			op.AddDependency(depID)
		}
	}

	b.operations = append(b.operations, op)
	return nil
}

// CreateDir adds a directory creation operation to the batch
func (b *Batch) CreateDir(path string, mode ...fs.FileMode) (OperationInterface, error) {
	fileMode := fs.FileMode(0755) // Default directory mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation using the registry
	op, err := b.createOperation("create_directory", path)
	if err != nil {
		return nil, err
	}

	// Create directory item
	dirItem := targets.NewDirectory(path).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, dirItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	
	op.SetDescriptionDetail("mode", fileMode.String())

	// Add to batch (validates against projected state)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateDir(%s): %w", path, err)
	}

	b.logger.Info().
		Str("op_id", string(op.ID())).
		Str("path", path).
		Str("mode", fileMode.String()).
		Msg("CreateDir operation added to batch")

	return op, nil
}

// CreateFile adds a file creation operation to the batch
func (b *Batch) CreateFile(path string, content []byte, mode ...fs.FileMode) (OperationInterface, error) {
	fileMode := fs.FileMode(0644) // Default file mode
	if len(mode) > 0 {
		fileMode = mode[0]
	}

	// Create the operation using the registry
	op, err := b.createOperation("create_file", path)
	if err != nil {
		return nil, err
	}

	// Create file item
	fileItem := targets.NewFile(path).WithContent(content).WithMode(fileMode)
	if err := b.registry.SetItemForOperation(op, fileItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	
	op.SetDescriptionDetail("size", fmt.Sprintf("%d bytes", len(content)))
	op.SetDescriptionDetail("mode", fileMode.String())

	// Add to batch (validates against projected state)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateFile(%s): %w", path, err)
	}

	b.logger.Info().
		Str("op_id", string(op.ID())).
		Str("path", path).
		Int("content_size", len(content)).
		Str("mode", fileMode.String()).
		Msg("CreateFile operation added to batch")

	return op, nil
}

// Copy adds a copy operation to the batch
func (b *Batch) Copy(src, dst string) (OperationInterface, error) {
	// Create the operation using the registry
	op, err := b.createOperation("copy", src)
	if err != nil {
		return nil, err
	}

	// Set copy paths and details
	op.SetDescriptionDetail("destination", dst)
	op.SetPaths(src, dst)

	// Add to batch (validates against projected state)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Copy(%s, %s): %w", src, dst, err)
	}

	b.logger.Info().
		Str("op_id", string(op.ID())).
		Str("src", src).
		Str("dst", dst).
		Msg("Copy operation added to batch")

	return op, nil
}

// Move adds a move operation to the batch
func (b *Batch) Move(src, dst string) (OperationInterface, error) {
	// Create the operation using the registry
	op, err := b.createOperation("move", src)
	if err != nil {
		return nil, err
	}

	// Set move paths and details
	op.SetDescriptionDetail("destination", dst)
	op.SetPaths(src, dst)

	// Add to batch (validates against projected state)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Move(%s, %s): %w", src, dst, err)
	}

	b.logger.Info().
		Str("op_id", string(op.ID())).
		Str("src", src).
		Str("dst", dst).
		Msg("Move operation added to batch")

	return op, nil
}

// Delete adds a delete operation to the batch
func (b *Batch) Delete(path string) (OperationInterface, error) {
	// Create the operation using the registry
	op, err := b.createOperation("delete", path)
	if err != nil {
		return nil, err
	}

	// Delete operations don't need additional configuration

	// Add to batch (validates against projected state)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for Delete(%s): %w", path, err)
	}

	b.logger.Info().
		Str("op_id", string(op.ID())).
		Str("path", path).
		Msg("Delete operation added to batch")

	return op, nil
}

// CreateSymlink adds a symlink creation operation to the batch
func (b *Batch) CreateSymlink(target, linkPath string) (OperationInterface, error) {
	// Create the operation using the registry
	op, err := b.createOperation("create_symlink", linkPath)
	if err != nil {
		return nil, err
	}

	// Create symlink item
	symlinkItem := targets.NewSymlink(linkPath, target)
	if err := b.registry.SetItemForOperation(op, symlinkItem); err != nil {
		return nil, fmt.Errorf("failed to set item: %w", err)
	}
	
	op.SetDescriptionDetail("target", target)

	// Add to batch (validates against projected state)
	if err := b.add(op); err != nil {
		return nil, fmt.Errorf("validation failed for CreateSymlink(%s, %s): %w", target, linkPath, err)
	}

	b.logger.Info().
		Str("op_id", string(op.ID())).
		Str("link_path", linkPath).
		Str("target", target).
		Msg("CreateSymlink operation added to batch")

	return op, nil
}

// generateID generates a unique operation ID
func (b *Batch) generateID(opType, path string) core.OperationID {
	b.idCounter++
	return core.OperationID(fmt.Sprintf("batch_%d_%s_%s", b.idCounter, opType, filepath.Base(path)))
}

// createOperation creates a new operation using the registry
func (b *Batch) createOperation(opType, path string) (OperationInterface, error) {
	id := b.generateID(opType, path)
	op, err := b.registry.CreateOperation(id, opType, path)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s operation: %w", opType, err)
	}
	
	batchOp, ok := op.(OperationInterface)
	if !ok {
		return nil, fmt.Errorf("operation does not implement batch.OperationInterface")
	}
	
	return batchOp, nil
}

// ensureParentDirectories creates parent directory operations if needed
func (b *Batch) ensureParentDirectories(path string) []core.OperationID {
	var deps []core.OperationID
	dir := filepath.Dir(path)
	
	if dir == "." || dir == "/" {
		return deps
	}

	// Check if parent directory already exists or will be created
	for _, op := range b.operations {
		desc := op.Describe()
		if desc.Type == "create_directory" && desc.Path == dir {
			deps = append(deps, op.ID())
			return deps
		}
	}

	// Check if parent exists in filesystem
	if _, err := b.fs.Stat(dir); err != nil {
		// Parent doesn't exist, create it
		parentOp, err := b.CreateDir(dir)
		if err == nil {
			deps = append(deps, parentOp.ID())
		}
	}

	return deps
}