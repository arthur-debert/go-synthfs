package synthfs

import (
	"fmt"
	"io/fs"
)

// PathStateType and related constants are defined in constants.go

// String returns the string representation of the PathStateType.
func (t PathStateType) String() string {
	switch t {
	case PathStateFile:
		return "file"
	case PathStateDir:
		return "directory"
	case PathStateSymlink:
		return "symlink"
	default:
		return "unknown"
	}
}

// PathState holds the projected state of a single path after all operations in a batch are applied.
// This is central to the Smart Dependencies feature (Phase II).
type PathState struct {
	// Path is the full path being tracked.
	Path string
	// WillExist is true if the path is expected to exist after the batch runs.
	WillExist bool
	// WillBeType is the projected type of the path (file, directory, etc.).
	WillBeType PathStateType
	// CreatedBy is the ID of the operation that creates this path.
	CreatedBy OperationID
	// DeletedBy is the ID of the operation that deletes this path.
	DeletedBy OperationID
	// ModifiedBy is a list of operation IDs that modify this path (e.g., copy to, unarchive to).
	ModifiedBy []OperationID
	// InitialState stores the fs.FileInfo of the path from the real filesystem at validation time.
	// It is nil if the path does not exist initially.
	InitialState fs.FileInfo
}

// PathStateTracker manages the projected state of all paths affected by a batch of operations.
type PathStateTracker struct {
	states map[string]*PathState
	fs     FullFileSystem
}

// NewPathStateTracker creates a new tracker.
func NewPathStateTracker(fs FullFileSystem) *PathStateTracker {
	return &PathStateTracker{
		states: make(map[string]*PathState),
		fs:     fs,
	}
}

// GetState returns the projected state of a path. If the path is not yet in the tracker,
// it queries the real filesystem and creates an initial state.
func (pst *PathStateTracker) GetState(path string) (*PathState, error) {
	if state, exists := pst.states[path]; exists {
		return state, nil
	}

	// If not tracked, check the real filesystem to create an initial state.
	info, err := pst.fs.Stat(path)
	if err != nil {
		// An error here means it doesn't exist. This is a valid state.
		initialState := &PathState{
			Path:      path,
			WillExist: false,
		}
		pst.states[path] = initialState
		return initialState, nil
	}

	// The path exists on the filesystem.
	var initialType PathStateType
	if info.IsDir() {
		initialType = PathStateDir
	} else if info.Mode()&fs.ModeSymlink != 0 {
		initialType = PathStateSymlink
	} else {
		initialType = PathStateFile
	}

	initialState := &PathState{
		Path:         path,
		WillExist:    true,
		WillBeType:   initialType,
		InitialState: info,
	}
	pst.states[path] = initialState
	return initialState, nil
}

// UpdateState applies the effect of an operation to the projected state of a path.
// This is where conflict detection will happen.
func (pst *PathStateTracker) UpdateState(op Operation) error {
	desc := op.Describe()
	opID := op.ID()

	switch desc.Type {
	case "create_file":
		return pst.updateStateForCreate(opID, desc.Path, PathStateFile)
	case "create_directory":
		return pst.updateStateForCreate(opID, desc.Path, PathStateDir)
	case "create_symlink":
		return pst.updateStateForCreate(opID, desc.Path, PathStateSymlink)
	case "create_archive":
		return pst.updateStateForCreate(opID, desc.Path, PathStateFile) // Archives are files

	case "delete":
		state, err := pst.GetState(desc.Path)
		if err != nil {
			return err
		}
		// In Phase II, it's a conflict to delete something that isn't projected to exist.
		if !state.WillExist {
			return fmt.Errorf("validation conflict for %s: path %s to be deleted is not projected to exist", opID, desc.Path)
		}
		// It's also a conflict to delete something that's already been deleted.
		if state.DeletedBy != "" {
			return fmt.Errorf("operation %s conflicts with %s: cannot delete %s, already scheduled for deletion", opID, state.DeletedBy, desc.Path)
		}
		// It is also a conflict to delete a path that was just created in this batch.
		if state.CreatedBy != "" {
			return fmt.Errorf("operation %s conflicts with %s: cannot delete %s, it was created in the same batch", opID, state.CreatedBy, desc.Path)
		}
		state.WillExist = false
		state.DeletedBy = opID

	case "copy":
		simpleOp, ok := op.(*SimpleOperation)
		if !ok {
			return fmt.Errorf("invalid operation type for copy: expected SimpleOperation")
		}
		srcPath := simpleOp.GetSrcPath()
		dstPath := simpleOp.GetDstPath()

		// Validate source
		srcState, err := pst.GetState(srcPath)
		if err != nil {
			return err
		}
		if !srcState.WillExist {
			return fmt.Errorf("validation conflict for %s: copy source %s does not exist", opID, srcPath)
		}

		// Update destination
		return pst.updateStateForCreate(opID, dstPath, srcState.WillBeType)

	case "move":
		simpleOp, ok := op.(*SimpleOperation)
		if !ok {
			return fmt.Errorf("invalid operation type for move: expected SimpleOperation")
		}
		srcPath := simpleOp.GetSrcPath()
		dstPath := simpleOp.GetDstPath()

		// Validate source
		srcState, err := pst.GetState(srcPath)
		if err != nil {
			return err
		}
		if !srcState.WillExist {
			return fmt.Errorf("validation conflict for %s: move source %s does not exist", opID, srcPath)
		}
		if srcState.DeletedBy != "" {
			return fmt.Errorf("operation %s conflicts with %s: cannot move %s, already scheduled for deletion", opID, srcState.DeletedBy, srcPath)
		}

		// Update source to be deleted
		srcState.WillExist = false
		srcState.DeletedBy = opID

		// Update destination to be created
		return pst.updateStateForCreate(opID, dstPath, srcState.WillBeType)

	case "unarchive":
		// This is more complex as it affects an unknown number of paths.
		// For now, we'll just check the source archive exists and treat the destination as modified.
		state, err := pst.GetState(desc.Path)
		if err != nil {
			return err
		}
		if !state.WillExist {
			return fmt.Errorf("validation conflict for %s: source archive %s does not exist", opID, desc.Path)
		}

		// Mark destination directory as modified. A more advanced implementation could
		// inspect the archive or handle conflicts at execution time.
		if unarchiveItem, ok := op.GetItem().(*UnarchiveItem); ok {
			destPath := unarchiveItem.ExtractPath()
			destState, err := pst.GetState(destPath)
			if err != nil {
				return err
			}
			destState.ModifiedBy = append(destState.ModifiedBy, opID)
		}
	}

	return nil
}

func (pst *PathStateTracker) updateStateForCreate(opID OperationID, path string, createType PathStateType) error {
	state, err := pst.GetState(path)
	if err != nil {
		return err
	}

	if state.WillExist {
		return fmt.Errorf("operation %s conflicts with existing state: cannot create %s because it is projected to already exist", opID, path)
	}
	if state.DeletedBy != "" {
		// A file was deleted, now we are creating it again. This should probably be a "modify" operation.
		// For now, let's treat it as a conflict to keep it simple.
		return fmt.Errorf("operation %s conflicts with %s: cannot create %s, path was scheduled for deletion", opID, state.DeletedBy, path)
	}

	state.WillExist = true
	state.WillBeType = createType
	state.CreatedBy = opID
	state.DeletedBy = "" // Reset deleted status
	return nil
}
