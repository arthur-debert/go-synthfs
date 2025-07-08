package batch

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

// pathStateTracker wraps execution.PathStateTracker for the batch package
type pathStateTracker struct {
	tracker *execution.PathStateTracker
}

// newPathStateTracker creates a new path state tracker
func newPathStateTracker(fs interface{}) *pathStateTracker {
	// Wrap the filesystem to implement execution.FileSystemInterface
	fsWrapper := &fileSystemWrapper{fs: fs}
	return &pathStateTracker{
		tracker: execution.NewPathStateTracker(fsWrapper),
	}
}

// fileSystemWrapper wraps interface{} filesystem to implement execution.FileSystemInterface
type fileSystemWrapper struct {
	fs interface{}
}

func (f *fileSystemWrapper) Stat(path string) (fs.FileInfo, error) {
	// Try different Stat method signatures
	if stat, ok := f.fs.(interface{ Stat(string) (fs.FileInfo, error) }); ok {
		return stat.Stat(path)
	}
	
	// Try interface{} version and convert
	if stat, ok := f.fs.(interface{ Stat(string) (interface{}, error) }); ok {
		info, err := stat.Stat(path)
		if err != nil {
			return nil, err
		}
		if fi, ok := info.(fs.FileInfo); ok {
			return fi, nil
		}
	}
	
	return nil, fmt.Errorf("filesystem does not support Stat")
}

// updateState updates the projected state based on an operation
func (pst *pathStateTracker) updateState(op interface{}) error {
	// We need to wrap the operation to implement execution.OperationInterface
	wrapper := &operationWrapper{op: op}
	return pst.tracker.UpdateState(wrapper)
}

// isDeleted checks if a path is scheduled for deletion
func (pst *pathStateTracker) isDeleted(path string) bool {
	return pst.tracker.IsDeleted(path)
}

// getState returns the projected state of a path
func (pst *pathStateTracker) getState(path string) (*execution.PathState, error) {
	return pst.tracker.GetState(path)
}

// operationWrapper wraps an interface{} operation to implement execution.OperationInterface
type operationWrapper struct {
	op interface{}
}

func (w *operationWrapper) ID() core.OperationID {
	if op, ok := w.op.(interface{ ID() core.OperationID }); ok {
		return op.ID()
	}
	return ""
}

func (w *operationWrapper) Describe() core.OperationDesc {
	if op, ok := w.op.(interface{ Describe() core.OperationDesc }); ok {
		return op.Describe()
	}
	return core.OperationDesc{}
}

func (w *operationWrapper) Dependencies() []core.OperationID {
	if op, ok := w.op.(interface{ Dependencies() []core.OperationID }); ok {
		return op.Dependencies()
	}
	return nil
}

func (w *operationWrapper) Conflicts() []core.OperationID {
	if op, ok := w.op.(interface{ Conflicts() []core.OperationID }); ok {
		return op.Conflicts()
	}
	return nil
}

func (w *operationWrapper) AddDependency(depID core.OperationID) {
	if op, ok := w.op.(interface{ AddDependency(core.OperationID) }); ok {
		op.AddDependency(depID)
	}
}

func (w *operationWrapper) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Not needed for state tracking
	return fmt.Errorf("ExecuteV2 not implemented for operation wrapper")
}

func (w *operationWrapper) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	// Not needed for state tracking
	return nil
}

func (w *operationWrapper) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
	// Not needed for state tracking
	return nil, nil, nil
}

func (w *operationWrapper) Rollback(ctx context.Context, fsys interface{}) error {
	// Not needed for state tracking
	return nil
}

func (w *operationWrapper) GetItem() interface{} {
	if op, ok := w.op.(interface{ GetItem() interface{} }); ok {
		return op.GetItem()
	}
	return nil
}

func (w *operationWrapper) SetDescriptionDetail(key string, value interface{}) {
	if op, ok := w.op.(interface{ SetDescriptionDetail(string, interface{}) }); ok {
		op.SetDescriptionDetail(key, value)
	}
}

// ensureParentDirectories generates CreateDir operations for missing parent directories
func ensureParentDirectories(b *BatchImpl, path string) ([]core.OperationID, error) {
	// Clean and normalize the path
	cleanPath := filepath.Clean(path)
	parentDir := filepath.Dir(cleanPath)

	var dependencyIDs []core.OperationID

	// If parent is root or current directory, no parent needed
	if parentDir == "." || parentDir == "/" || parentDir == cleanPath {
		return dependencyIDs, nil
	}

	// Check if parent directory exists in the filesystem
	if stat, ok := b.fs.(interface{ Stat(string) (interface{}, error) }); ok {
		if _, err := stat.Stat(parentDir); err == nil {
			// Parent exists in filesystem
			return dependencyIDs, nil
		}
	}

	// Check if parent directory is already projected to exist
	if b.pathTracker != nil {
		state, err := b.pathTracker.getState(parentDir)
		if err == nil && state != nil && state.WillExist && state.WillBeType == core.PathStateDir {
			// Parent is projected to exist
			return dependencyIDs, nil
		}
	}

	// Check if we already have a CreateDir operation for this parent
	for _, op := range b.operations {
		if desc, ok := op.(interface{ Describe() core.OperationDesc }); ok {
			opDesc := desc.Describe()
			if opDesc.Type == "create_directory" && opDesc.Path == parentDir {
				if idGetter, ok := op.(interface{ ID() core.OperationID }); ok {
					return []core.OperationID{idGetter.ID()}, nil
				}
			}
		}
	}

	// Recursively ensure parent's parents exist
	parentDeps, err := ensureParentDirectories(b, parentDir)
	if err != nil {
		return nil, err
	}
	dependencyIDs = append(dependencyIDs, parentDeps...)

	// Create operation for the parent directory
	parentOp, err := b.createOperation("create_directory", parentDir)
	if err != nil {
		return dependencyIDs, fmt.Errorf("failed to create parent directory operation: %w", err)
	}

	// Set directory item
	// Use targets package directly to create directory item
	dirItem := targets.NewDirectory(parentDir).WithMode(0755)
	if err := b.registry.SetItemForOperation(parentOp, dirItem); err != nil {
		return dependencyIDs, fmt.Errorf("failed to set directory item: %w", err)
	}

	// Set directory mode
	if err := b.setOperationDetails(parentOp, map[string]interface{}{
		"mode":         "0755",
		"auto_created": true,
	}); err != nil {
		return dependencyIDs, fmt.Errorf("failed to set operation details: %w", err)
	}

	// Add dependencies from parent's parents
	if depAdder, ok := parentOp.(interface{ AddDependency(core.OperationID) }); ok {
		for _, depID := range parentDeps {
			depAdder.AddDependency(depID)
		}
	}

	// Add to batch (without auto-parent creation to avoid infinite recursion)
	if err := b.addWithoutAutoParent(parentOp); err != nil {
		return dependencyIDs, fmt.Errorf("failed to add parent directory operation: %w", err)
	}

	// Get operation ID
	if idGetter, ok := parentOp.(interface{ ID() core.OperationID }); ok {
		dependencyIDs = append(dependencyIDs, idGetter.ID())
	}

	return dependencyIDs, nil
}