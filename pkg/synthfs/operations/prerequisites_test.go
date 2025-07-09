package operations

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestOperationPrerequisites(t *testing.T) {
	t.Run("CreateFileOperation Prerequisites", func(t *testing.T) {
		op := NewCreateFileOperation(core.OperationID("test-op"), "dir/file.txt")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 2 {
			t.Errorf("Expected 2 prerequisites, got %d", len(prereqs))
		}
		
		// Check for parent directory prerequisite
		hasParentDir := false
		hasNoConflict := false
		
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "parent_dir":
				hasParentDir = true
				if prereq.Path() != "dir" {
					t.Errorf("Expected parent_dir path 'dir', got '%s'", prereq.Path())
				}
			case "no_conflict":
				hasNoConflict = true
				if prereq.Path() != "dir/file.txt" {
					t.Errorf("Expected no_conflict path 'dir/file.txt', got '%s'", prereq.Path())
				}
			}
		}
		
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
	})
	
	t.Run("CreateFileOperation with root path", func(t *testing.T) {
		op := NewCreateFileOperation(core.OperationID("test-op"), "file.txt")
		
		prereqs := op.Prerequisites()
		// Should still have no_conflict but parent_dir is current directory
		if len(prereqs) != 2 {
			t.Errorf("Expected 2 prerequisites, got %d", len(prereqs))
		}
	})
	
	t.Run("CreateDirectoryOperation Prerequisites", func(t *testing.T) {
		op := NewCreateDirectoryOperation(core.OperationID("test-op"), "parent/newdir")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 2 {
			t.Errorf("Expected 2 prerequisites, got %d", len(prereqs))
		}
		
		// Check for parent directory prerequisite
		hasParentDir := false
		hasNoConflict := false
		
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "parent_dir":
				hasParentDir = true
				if prereq.Path() != "parent" {
					t.Errorf("Expected parent_dir path 'parent', got '%s'", prereq.Path())
				}
			case "no_conflict":
				hasNoConflict = true
				if prereq.Path() != "parent/newdir" {
					t.Errorf("Expected no_conflict path 'parent/newdir', got '%s'", prereq.Path())
				}
			}
		}
		
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
	})
	
	t.Run("CreateSymlinkOperation Prerequisites", func(t *testing.T) {
		op := NewCreateSymlinkOperation(core.OperationID("test-op"), "dir/symlink")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 2 {
			t.Errorf("Expected 2 prerequisites, got %d", len(prereqs))
		}
		
		// Check for parent directory prerequisite
		hasParentDir := false
		hasNoConflict := false
		
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "parent_dir":
				hasParentDir = true
				if prereq.Path() != "dir" {
					t.Errorf("Expected parent_dir path 'dir', got '%s'", prereq.Path())
				}
			case "no_conflict":
				hasNoConflict = true
				if prereq.Path() != "dir/symlink" {
					t.Errorf("Expected no_conflict path 'dir/symlink', got '%s'", prereq.Path())
				}
			}
		}
		
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
	})
	
	t.Run("DeleteOperation Prerequisites", func(t *testing.T) {
		op := NewDeleteOperation(core.OperationID("test-op"), "file.txt")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 1 {
			t.Errorf("Expected 1 prerequisite, got %d", len(prereqs))
		}
		
		// Check for source exists prerequisite
		hasSourceExists := false
		
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "source_exists":
				hasSourceExists = true
				if prereq.Path() != "file.txt" {
					t.Errorf("Expected source_exists path 'file.txt', got '%s'", prereq.Path())
				}
			}
		}
		
		if !hasSourceExists {
			t.Error("Expected source_exists prerequisite")
		}
	})
	
	t.Run("CreateArchiveOperation Prerequisites", func(t *testing.T) {
		op := NewCreateArchiveOperation(core.OperationID("test-op"), "dir/archive.tar.gz")
		
		prereqs := op.Prerequisites()
		if len(prereqs) < 2 {
			t.Errorf("Expected at least 2 prerequisites, got %d", len(prereqs))
		}
		
		// Check for parent directory prerequisite
		hasParentDir := false
		hasNoConflict := false
		
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "parent_dir":
				hasParentDir = true
				if prereq.Path() != "dir" {
					t.Errorf("Expected parent_dir path 'dir', got '%s'", prereq.Path())
				}
			case "no_conflict":
				hasNoConflict = true
				if prereq.Path() != "dir/archive.tar.gz" {
					t.Errorf("Expected no_conflict path 'dir/archive.tar.gz', got '%s'", prereq.Path())
				}
			}
		}
		
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
	})
	
	t.Run("CopyOperation Prerequisites", func(t *testing.T) {
		op := NewCopyOperation(core.OperationID("test-op"), "source.txt")
		op.SetPaths("source.txt", "dir/dest.txt")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 3 {
			t.Errorf("Expected 3 prerequisites, got %d", len(prereqs))
		}
		
		// Check for all expected prerequisites
		hasParentDir := false
		hasNoConflict := false
		hasSourceExists := false
		
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "parent_dir":
				hasParentDir = true
				if prereq.Path() != "dir" {
					t.Errorf("Expected parent_dir path 'dir', got '%s'", prereq.Path())
				}
			case "no_conflict":
				hasNoConflict = true
				if prereq.Path() != "dir/dest.txt" {
					t.Errorf("Expected no_conflict path 'dir/dest.txt', got '%s'", prereq.Path())
				}
			case "source_exists":
				hasSourceExists = true
				if prereq.Path() != "source.txt" {
					t.Errorf("Expected source_exists path 'source.txt', got '%s'", prereq.Path())
				}
			}
		}
		
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
		if !hasSourceExists {
			t.Error("Expected source_exists prerequisite")
		}
	})
	
	t.Run("MoveOperation Prerequisites", func(t *testing.T) {
		op := NewMoveOperation(core.OperationID("test-op"), "source.txt")
		op.SetPaths("source.txt", "dir/dest.txt")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 3 {
			t.Errorf("Expected 3 prerequisites, got %d", len(prereqs))
		}
		
		// Check for all expected prerequisites
		hasParentDir := false
		hasNoConflict := false
		hasSourceExists := false
		
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "parent_dir":
				hasParentDir = true
				if prereq.Path() != "dir" {
					t.Errorf("Expected parent_dir path 'dir', got '%s'", prereq.Path())
				}
			case "no_conflict":
				hasNoConflict = true
				if prereq.Path() != "dir/dest.txt" {
					t.Errorf("Expected no_conflict path 'dir/dest.txt', got '%s'", prereq.Path())
				}
			case "source_exists":
				hasSourceExists = true
				if prereq.Path() != "source.txt" {
					t.Errorf("Expected source_exists path 'source.txt', got '%s'", prereq.Path())
				}
			}
		}
		
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
		if !hasSourceExists {
			t.Error("Expected source_exists prerequisite")
		}
	})
	
	t.Run("BaseOperation default Prerequisites", func(t *testing.T) {
		op := NewBaseOperation(core.OperationID("test-op"), "test", "test-path")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 0 {
			t.Errorf("Expected 0 prerequisites for base operation, got %d", len(prereqs))
		}
	})
}