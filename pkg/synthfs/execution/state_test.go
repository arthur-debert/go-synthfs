package execution_test

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/execution"
)

// TestPathStateTracker_GetState tests the GetState functionality
func TestPathStateTracker_GetState(t *testing.T) {
	t.Run("GetState for existing tracked path", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)

		// Create initial state manually
		state, err := tracker.GetState("existing.txt")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Should return same state on second call
		state2, err := tracker.GetState("existing.txt")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if state != state2 {
			t.Error("Expected same state object for tracked path")
		}
	})

	t.Run("GetState for path that exists on filesystem - file", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("real_file.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)

		state, err := tracker.GetState("real_file.txt")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !state.WillExist {
			t.Error("Expected WillExist to be true for existing file")
		}
		if state.WillBeType != core.PathStateFile {
			t.Errorf("Expected WillBeType to be PathStateFile, got: %v", state.WillBeType)
		}
		if state.Path != "real_file.txt" {
			t.Errorf("Expected Path to be 'real_file.txt', got: %s", state.Path)
		}
		if state.InitialState == nil {
			t.Error("Expected InitialState to be set for existing file")
		}
	})

	t.Run("GetState for path that exists on filesystem - directory", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddDirectory("real_dir")
		tracker := execution.NewPathStateTracker(fs)

		state, err := tracker.GetState("real_dir")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !state.WillExist {
			t.Error("Expected WillExist to be true for existing directory")
		}
		if state.WillBeType != core.PathStateDir {
			t.Errorf("Expected WillBeType to be PathStateDir, got: %v", state.WillBeType)
		}
	})

	t.Run("GetState for path that exists on filesystem - symlink", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddSymlink("real_link.txt", "target.txt")
		tracker := execution.NewPathStateTracker(fs)

		state, err := tracker.GetState("real_link.txt")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !state.WillExist {
			t.Error("Expected WillExist to be true for existing symlink")
		}
		if state.WillBeType != core.PathStateSymlink {
			t.Errorf("Expected WillBeType to be PathStateSymlink, got: %v", state.WillBeType)
		}
	})

	t.Run("GetState for path that doesn't exist on filesystem", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)

		state, err := tracker.GetState("nonexistent.txt")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if state.WillExist {
			t.Error("Expected WillExist to be false for non-existent file")
		}
		if state.Path != "nonexistent.txt" {
			t.Errorf("Expected Path to be 'nonexistent.txt', got: %s", state.Path)
		}
		if state.InitialState != nil {
			t.Error("Expected InitialState to be nil for non-existent file")
		}
	})
}

// TestPathStateTracker_UpdateState_CreateOperations tests create operation state updates
func TestPathStateTracker_UpdateState_CreateOperations(t *testing.T) {
	t.Run("UpdateState for create_file operation", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "create_file", "new_file.txt")

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		state, _ := tracker.GetState("new_file.txt")
		if !state.WillExist {
			t.Error("Expected WillExist to be true after create_file")
		}
		if state.WillBeType != core.PathStateFile {
			t.Errorf("Expected WillBeType to be PathStateFile, got: %v", state.WillBeType)
		}
		if state.CreatedBy != "op1" {
			t.Errorf("Expected CreatedBy to be 'op1', got: %s", state.CreatedBy)
		}
	})

	t.Run("UpdateState for create_directory operation", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "create_directory", "new_dir")

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		state, _ := tracker.GetState("new_dir")
		if state.WillBeType != core.PathStateDir {
			t.Errorf("Expected WillBeType to be PathStateDir, got: %v", state.WillBeType)
		}
	})

	t.Run("UpdateState for create_symlink operation", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "create_symlink", "new_link.txt")

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		state, _ := tracker.GetState("new_link.txt")
		if state.WillBeType != core.PathStateSymlink {
			t.Errorf("Expected WillBeType to be PathStateSymlink, got: %v", state.WillBeType)
		}
	})

	t.Run("UpdateState for create_archive operation", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "create_archive", "archive.tar.gz")

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		state, _ := tracker.GetState("archive.tar.gz")
		if state.WillBeType != core.PathStateFile {
			t.Errorf("Expected WillBeType to be PathStateFile for archive, got: %v", state.WillBeType)
		}
	})

	t.Run("UpdateState create conflict - path already exists", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("existing.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "create_file", "existing.txt")

		err := tracker.UpdateState(op)
		if err == nil {
			t.Error("Expected error for creating on existing path")
		}

		expectedErr := "operation op1 conflicts with existing state: cannot create existing.txt because it is projected to already exist"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("UpdateState create conflict - path scheduled for deletion", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("to_delete.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)

		// First delete the file
		deleteOp := NewMockOperationInterface("op1", "delete", "to_delete.txt")
		err := tracker.UpdateState(deleteOp)
		if err != nil {
			t.Fatalf("Delete operation should succeed, got: %v", err)
		}

		// Try to create on the deleted path
		createOp := NewMockOperationInterface("op2", "create_file", "to_delete.txt")
		err = tracker.UpdateState(createOp)
		if err == nil {
			t.Error("Expected error for creating on path scheduled for deletion")
		}

		expectedErr := "operation op2 conflicts with op1: cannot create to_delete.txt, path was scheduled for deletion"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})
}

// TestPathStateTracker_UpdateState_DeleteOperations tests delete operation state updates
func TestPathStateTracker_UpdateState_DeleteOperations(t *testing.T) {
	t.Run("UpdateState for delete operation - success", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("to_delete.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "delete", "to_delete.txt")

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		state, _ := tracker.GetState("to_delete.txt")
		if state.WillExist {
			t.Error("Expected WillExist to be false after delete")
		}
		if state.DeletedBy != "op1" {
			t.Errorf("Expected DeletedBy to be 'op1', got: %s", state.DeletedBy)
		}
	})

	t.Run("UpdateState delete conflict - path doesn't exist", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "delete", "nonexistent.txt")

		err := tracker.UpdateState(op)
		if err == nil {
			t.Error("Expected error for deleting non-existent path")
		}

		expectedErr := "validation conflict for op1: path nonexistent.txt to be deleted is not projected to exist"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("UpdateState delete conflict - already deleted", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("to_delete.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)

		// First delete
		op1 := NewMockOperationInterface("op1", "delete", "to_delete.txt")
		err := tracker.UpdateState(op1)
		if err != nil {
			t.Fatalf("First delete should succeed, got: %v", err)
		}

		// Second delete should fail - it will be caught by "path doesn't exist" check
		// because WillExist becomes false after deletion
		op2 := NewMockOperationInterface("op2", "delete", "to_delete.txt")
		err = tracker.UpdateState(op2)
		if err == nil {
			t.Error("Expected error for deleting already deleted path")
		}

		expectedErr := "validation conflict for op2: path to_delete.txt to be deleted is not projected to exist"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("UpdateState delete conflict - path created in same batch", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)

		// Create file
		createOp := NewMockOperationInterface("op1", "create_file", "new_file.txt")
		err := tracker.UpdateState(createOp)
		if err != nil {
			t.Fatalf("Create operation should succeed, got: %v", err)
		}

		// Try to delete the just-created file
		deleteOp := NewMockOperationInterface("op2", "delete", "new_file.txt")
		err = tracker.UpdateState(deleteOp)
		if err == nil {
			t.Error("Expected error for deleting file created in same batch")
		}

		expectedErr := "operation op2 conflicts with op1: cannot delete new_file.txt, it was created in the same batch"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})
}

// TestPathStateTracker_UpdateState_CopyOperations tests copy operation state updates
func TestPathStateTracker_UpdateState_CopyOperations(t *testing.T) {
	t.Run("UpdateState for copy operation - success", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("source.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)

		op := NewMockOperationInterface("op1", "copy", "")
		op.SetDetail("src", "source.txt")
		op.SetDetail("dst", "dest.txt")

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check destination was created
		destState, _ := tracker.GetState("dest.txt")
		if !destState.WillExist {
			t.Error("Expected destination to exist after copy")
		}
		if destState.WillBeType != core.PathStateFile {
			t.Errorf("Expected destination type to match source (file), got: %v", destState.WillBeType)
		}
		if destState.CreatedBy != "op1" {
			t.Errorf("Expected destination CreatedBy to be 'op1', got: %s", destState.CreatedBy)
		}

		// Check source still exists
		srcState, _ := tracker.GetState("source.txt")
		if !srcState.WillExist {
			t.Error("Expected source to still exist after copy")
		}
	})

	t.Run("UpdateState copy - missing source path", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "copy", "")

		err := tracker.UpdateState(op)
		if err == nil {
			t.Error("Expected error for copy with missing source path")
		}

		expectedErr := "copy operation op1 missing source or destination path"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("UpdateState copy - source doesn't exist", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)

		op := NewMockOperationInterface("op1", "copy", "")
		op.SetDetail("src", "nonexistent.txt")
		op.SetDetail("dst", "dest.txt")

		err := tracker.UpdateState(op)
		if err == nil {
			t.Error("Expected error for copy with non-existent source")
		}

		expectedErr := "copy source nonexistent.txt does not exist"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("UpdateState copy with path getter methods", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddDirectory("source_dir")
		tracker := execution.NewPathStateTracker(fs)

		op := NewMockOperationInterfaceWithPaths("op1", "copy", "", "source_dir", "dest_dir")

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check destination inherits source type (directory)
		destState, _ := tracker.GetState("dest_dir")
		if destState.WillBeType != core.PathStateDir {
			t.Errorf("Expected destination type to match source (directory), got: %v", destState.WillBeType)
		}
	})
}

// TestPathStateTracker_UpdateState_MoveOperations tests move operation state updates
func TestPathStateTracker_UpdateState_MoveOperations(t *testing.T) {
	t.Run("UpdateState for move operation - success", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("source.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)

		op := NewMockOperationInterface("op1", "move", "")
		op.SetDetail("src", "source.txt")
		op.SetDetail("dst", "dest.txt")

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check source was deleted
		srcState, _ := tracker.GetState("source.txt")
		if srcState.WillExist {
			t.Error("Expected source to not exist after move")
		}
		if srcState.DeletedBy != "op1" {
			t.Errorf("Expected source DeletedBy to be 'op1', got: %s", srcState.DeletedBy)
		}

		// Check destination was created
		destState, _ := tracker.GetState("dest.txt")
		if !destState.WillExist {
			t.Error("Expected destination to exist after move")
		}
		if destState.CreatedBy != "op1" {
			t.Errorf("Expected destination CreatedBy to be 'op1', got: %s", destState.CreatedBy)
		}
	})

	t.Run("UpdateState move - source doesn't exist", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)

		op := NewMockOperationInterface("op1", "move", "")
		op.SetDetail("src", "nonexistent.txt")
		op.SetDetail("dst", "dest.txt")

		err := tracker.UpdateState(op)
		if err == nil {
			t.Error("Expected error for move with non-existent source")
		}

		expectedErr := "validation conflict for op1: move source nonexistent.txt does not exist"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("UpdateState move - source already deleted", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("source.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)

		// Delete source first
		deleteOp := NewMockOperationInterface("op1", "delete", "source.txt")
		err := tracker.UpdateState(deleteOp)
		if err != nil {
			t.Fatalf("Delete should succeed, got: %v", err)
		}

		// Try to move deleted source - will be caught by "source doesn't exist" check
		// because WillExist becomes false after deletion
		moveOp := NewMockOperationInterface("op2", "move", "")
		moveOp.SetDetail("src", "source.txt")
		moveOp.SetDetail("dst", "dest.txt")

		err = tracker.UpdateState(moveOp)
		if err == nil {
			t.Error("Expected error for move with deleted source")
		}

		expectedErr := "validation conflict for op2: move source source.txt does not exist"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})
}

// TestPathStateTracker_UpdateState_UnarchiveOperations tests unarchive operation state updates
func TestPathStateTracker_UpdateState_UnarchiveOperations(t *testing.T) {
	t.Run("UpdateState for unarchive operation - success", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("archive.tar.gz", 1000, 0644)
		tracker := execution.NewPathStateTracker(fs)

		op := NewMockOperationInterface("op1", "unarchive", "archive.tar.gz")
		unarchiveItem := NewMockUnarchiveItem("archive.tar.gz", "extract_dir")
		op.SetItem(unarchiveItem)

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Check destination directory is marked as modified
		destState, _ := tracker.GetState("extract_dir")
		if len(destState.ModifiedBy) == 0 {
			t.Error("Expected destination to be marked as modified")
		}
		if destState.ModifiedBy[0] != "op1" {
			t.Errorf("Expected destination ModifiedBy to contain 'op1', got: %v", destState.ModifiedBy)
		}
	})

	t.Run("UpdateState unarchive - source archive doesn't exist", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "unarchive", "nonexistent.tar.gz")

		err := tracker.UpdateState(op)
		if err == nil {
			t.Error("Expected error for unarchive with non-existent source")
		}

		expectedErr := "validation conflict for op1: source archive nonexistent.tar.gz does not exist"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got: %s", expectedErr, err.Error())
		}
	})

	t.Run("UpdateState unarchive - no item", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("archive.tar.gz", 1000, 0644)
		tracker := execution.NewPathStateTracker(fs)

		op := NewMockOperationInterface("op1", "unarchive", "archive.tar.gz")
		// Don't set item

		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Expected no error when no item is set, got: %v", err)
		}
	})
}

// TestPathStateTracker_IsDeleted tests the IsDeleted functionality
func TestPathStateTracker_IsDeleted(t *testing.T) {
	t.Run("IsDeleted returns false for non-existent path", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)

		if tracker.IsDeleted("nonexistent.txt") {
			t.Error("Expected IsDeleted to return false for non-existent path")
		}
	})

	t.Run("IsDeleted returns false for existing non-deleted path", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("existing.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)

		if tracker.IsDeleted("existing.txt") {
			t.Error("Expected IsDeleted to return false for existing non-deleted path")
		}
	})

	t.Run("IsDeleted returns true for deleted path", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		fs.AddFile("to_delete.txt", 100, 0644)
		tracker := execution.NewPathStateTracker(fs)

		op := NewMockOperationInterface("op1", "delete", "to_delete.txt")
		err := tracker.UpdateState(op)
		if err != nil {
			t.Fatalf("Delete should succeed, got: %v", err)
		}

		if !tracker.IsDeleted("to_delete.txt") {
			t.Error("Expected IsDeleted to return true for deleted path")
		}
	})

	t.Run("IsDeleted handles GetState error gracefully", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)

		// This should not panic even if GetState encounters issues
		result := tracker.IsDeleted("any_path")
		if result {
			t.Error("Expected IsDeleted to return false when GetState fails")
		}
	})
}

// TestPathStateTracker_UnsupportedOperations tests unknown operation types
func TestPathStateTracker_UnsupportedOperations(t *testing.T) {
	t.Run("UpdateState for unknown operation type", func(t *testing.T) {
		fs := NewMockFileSystemInterface()
		tracker := execution.NewPathStateTracker(fs)
		op := NewMockOperationInterface("op1", "unknown_operation", "some_path")

		err := tracker.UpdateState(op)
		// Should succeed (no error) as unknown operations are ignored
		if err != nil {
			t.Errorf("Expected no error for unknown operation type, got: %v", err)
		}
	})
}

// Mock types for testing

type MockFileSystemInterface struct {
	files map[string]*MockFileInfo
}

func NewMockFileSystemInterface() *MockFileSystemInterface {
	return &MockFileSystemInterface{
		files: make(map[string]*MockFileInfo),
	}
}

func (m *MockFileSystemInterface) AddFile(path string, size int64, mode fs.FileMode) {
	m.files[path] = &MockFileInfo{
		name:  path,
		size:  size,
		mode:  mode,
		isDir: false,
	}
}

func (m *MockFileSystemInterface) AddDirectory(path string) {
	m.files[path] = &MockFileInfo{
		name:  path,
		size:  0,
		mode:  fs.ModeDir | 0755,
		isDir: true,
	}
}

func (m *MockFileSystemInterface) AddSymlink(path, target string) {
	m.files[path] = &MockFileInfo{
		name:  path,
		size:  int64(len(target)),
		mode:  fs.ModeSymlink | 0644,
		isDir: false,
	}
}

func (m *MockFileSystemInterface) Stat(path string) (fs.FileInfo, error) {
	if info, exists := m.files[path]; exists {
		return info, nil
	}
	return nil, fs.ErrNotExist
}

type MockFileInfo struct {
	name  string
	size  int64
	mode  fs.FileMode
	isDir bool
}

func (m *MockFileInfo) Name() string       { return m.name }
func (m *MockFileInfo) Size() int64        { return m.size }
func (m *MockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m *MockFileInfo) ModTime() time.Time { return time.Now() }
func (m *MockFileInfo) IsDir() bool        { return m.isDir }
func (m *MockFileInfo) Sys() interface{}   { return nil }

type MockOperationInterface struct {
	id      core.OperationID
	opType  string
	path    string
	details map[string]interface{}
	item    interface{}
	srcPath string
	dstPath string
}

func NewMockOperationInterface(id, opType, path string) *MockOperationInterface {
	return &MockOperationInterface{
		id:      core.OperationID(id),
		opType:  opType,
		path:    path,
		details: make(map[string]interface{}),
	}
}

func NewMockOperationInterfaceWithPaths(id, opType, path, srcPath, dstPath string) *MockOperationInterface {
	op := NewMockOperationInterface(id, opType, path)
	op.srcPath = srcPath
	op.dstPath = dstPath
	return op
}

type MockOperationInterfaceWithPaths struct {
	*MockOperationInterface
	srcPath string
	dstPath string
}

func (m *MockOperationInterfaceWithPaths) GetSrcPath() string { return m.srcPath }
func (m *MockOperationInterfaceWithPaths) GetDstPath() string { return m.dstPath }

func (m *MockOperationInterface) ID() core.OperationID { return m.id }
func (m *MockOperationInterface) Describe() core.OperationDesc {
	return core.OperationDesc{
		Type:    m.opType,
		Path:    m.path,
		Details: m.details,
	}
}
func (m *MockOperationInterface) Dependencies() []core.OperationID     { return []core.OperationID{} }
func (m *MockOperationInterface) Conflicts() []core.OperationID        { return []core.OperationID{} }
func (m *MockOperationInterface) Prerequisites() []core.Prerequisite   { return []core.Prerequisite{} }
func (m *MockOperationInterface) AddDependency(depID core.OperationID) { /* no-op for mock */ }
func (m *MockOperationInterface) ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return nil
}
func (m *MockOperationInterface) ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys interface{}) error {
	return nil
}
func (m *MockOperationInterface) ReverseOps(ctx context.Context, fsys interface{}, budget *core.BackupBudget) ([]interface{}, *core.BackupData, error) {
	return nil, nil, nil
}
func (m *MockOperationInterface) Rollback(ctx context.Context, fsys interface{}) error { return nil }
func (m *MockOperationInterface) SetDescriptionDetail(key string, value interface{}) { /* no-op for mock */
}
func (m *MockOperationInterface) SetDetail(key string, value interface{}) {
	m.details[key] = value
}
func (m *MockOperationInterface) GetItem() interface{}     { return m.item }
func (m *MockOperationInterface) SetItem(item interface{}) { m.item = item }

// Add path getter methods for copy/move operations
func (m *MockOperationInterface) GetSrcPath() string {
	if m.srcPath != "" {
		return m.srcPath
	}
	if src, ok := m.details["src"].(string); ok {
		return src
	}
	return ""
}

func (m *MockOperationInterface) GetDstPath() string {
	if m.dstPath != "" {
		return m.dstPath
	}
	if dst, ok := m.details["dst"].(string); ok {
		return dst
	}
	return ""
}

type MockUnarchiveItem struct {
	path        string
	extractPath string
}

func NewMockUnarchiveItem(path, extractPath string) *MockUnarchiveItem {
	return &MockUnarchiveItem{
		path:        path,
		extractPath: extractPath,
	}
}

func (m *MockUnarchiveItem) Path() string        { return m.path }
func (m *MockUnarchiveItem) ExtractPath() string { return m.extractPath }
