package synthfs

import (
	"context"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
)

// PathState is a type alias for execution.PathState
type PathState = execution.PathState

// PathStateTracker is a wrapper around execution.PathStateTracker
type PathStateTracker struct {
	tracker *execution.PathStateTracker
}

// NewPathStateTracker creates a new tracker.
func NewPathStateTracker(fs FileSystem) *PathStateTracker {
	return &PathStateTracker{
		tracker: execution.NewPathStateTracker(fs),
	}
}

// GetState returns the projected state of a path.
func (pst *PathStateTracker) GetState(path string) (*PathState, error) {
	return pst.tracker.GetState(path)
}

// UpdateState applies the effect of an operation to the projected state of a path.
func (pst *PathStateTracker) UpdateState(op Operation) error {
	// Create a simple adapter to bridge to the execution package
	// This avoids the complex adapter system while maintaining functionality
	desc := op.Describe()
	opType := desc.Type
	
	// Map operation types to execution package expectations
	switch opType {
	case "create_file":
		return pst.tracker.UpdateState(&simpleOpAdapter{
			id: op.ID(),
			opType: "create_file", 
			path: desc.Path,
		})
	case "create_directory", "mkdir":
		return pst.tracker.UpdateState(&simpleOpAdapter{
			id: op.ID(),
			opType: "create_directory",
			path: desc.Path,
		})
	case "create_symlink":
		return pst.tracker.UpdateState(&simpleOpAdapter{
			id: op.ID(),
			opType: "create_symlink",
			path: desc.Path,
		})
	case "delete":
		return pst.tracker.UpdateState(&simpleOpAdapter{
			id: op.ID(),
			opType: "delete",
			path: desc.Path,
		})
	case "copy":
		src, dst := op.GetPaths()
		return pst.tracker.UpdateState(&simpleOpAdapter{
			id: op.ID(),
			opType: "copy",
			path: dst,
			srcPath: src,
		})
	case "move":
		src, dst := op.GetPaths()
		return pst.tracker.UpdateState(&simpleOpAdapter{
			id: op.ID(),
			opType: "move", 
			path: dst,
			srcPath: src,
		})
	case "create_archive":
		return pst.tracker.UpdateState(&simpleOpAdapter{
			id: op.ID(),
			opType: "create_file", // Archive is essentially a file
			path: desc.Path,
		})
	case "unarchive":
		// Unarchive creates multiple files/directories
		// For now, we'll just track the operation without specific state changes
		// This could be enhanced to track all extracted paths
		return nil
	default:
		// For unknown operation types, just return nil
		// This includes operations like write_template which might be added later
		return nil
	}
}

// IsDeleted returns true if the path is scheduled for deletion by any operation.
func (pst *PathStateTracker) IsDeleted(path string) bool {
	return pst.tracker.IsDeleted(path)
}

// simpleOpAdapter is a minimal adapter for PathStateTracker
// This avoids the complex adapter system while providing necessary functionality
type simpleOpAdapter struct {
	id      core.OperationID
	opType  string
	path    string
	srcPath string
}

func (soa *simpleOpAdapter) ID() core.OperationID { return soa.id }
func (soa *simpleOpAdapter) Describe() core.OperationDesc {
	return core.OperationDesc{
		Type: soa.opType,
		Path: soa.path,
	}
}
func (soa *simpleOpAdapter) GetSrcPath() string { return soa.srcPath }
func (soa *simpleOpAdapter) GetDstPath() string { return soa.path }

// Required methods to implement execution.OperationInterface
func (soa *simpleOpAdapter) AddDependency(depID core.OperationID) { /* no-op */ }
func (soa *simpleOpAdapter) Dependencies() []core.OperationID { return nil }
func (soa *simpleOpAdapter) Conflicts() []core.OperationID { return nil }
func (soa *simpleOpAdapter) Prerequisites() []core.Prerequisite { return nil }
func (soa *simpleOpAdapter) Execute(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error { return nil }
func (soa *simpleOpAdapter) Validate(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error { return nil }
func (soa *simpleOpAdapter) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) { return nil, nil, nil }
func (soa *simpleOpAdapter) Rollback(ctx context.Context, fsys interface{}) error { return nil }
func (soa *simpleOpAdapter) GetItem() interface{} { return nil }
func (soa *simpleOpAdapter) SetDescriptionDetail(key string, value interface{}) { /* no-op */ }
