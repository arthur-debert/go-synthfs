package operations_test

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestOperationPrerequisites(t *testing.T) {
	t.Run("CreateFileOperation prerequisites", func(t *testing.T) {
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "subdir/file.txt")
		
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
				if prereq.Path() != "subdir/file.txt" {
					t.Errorf("Expected parent_dir prerequisite path to be 'subdir/file.txt', got '%s'", prereq.Path())
				}
			case "no_conflict":
				hasNoConflict = true
				if prereq.Path() != "subdir/file.txt" {
					t.Errorf("Expected no_conflict prerequisite path to be 'subdir/file.txt', got '%s'", prereq.Path())
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

	t.Run("CreateDirectoryOperation prerequisites", func(t *testing.T) {
		op := operations.NewCreateDirectoryOperation(core.OperationID("test-op"), "parent/child")
		
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
			case "no_conflict":
				hasNoConflict = true
			}
		}
		
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
	})

	t.Run("CopyOperation prerequisites", func(t *testing.T) {
		op := operations.NewCopyOperation(core.OperationID("test-op"), "source.txt")
		op.SetPaths("source.txt", "dest/copy.txt")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 3 {
			t.Errorf("Expected 3 prerequisites, got %d", len(prereqs))
		}
		
		// Check for source exists, parent directory, and no conflict prerequisites
		hasSourceExists := false
		hasParentDir := false
		hasNoConflict := false
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "source_exists":
				hasSourceExists = true
				if prereq.Path() != "source.txt" {
					t.Errorf("Expected source_exists prerequisite path to be 'source.txt', got '%s'", prereq.Path())
				}
			case "parent_dir":
				hasParentDir = true
			case "no_conflict":
				hasNoConflict = true
			}
		}
		
		if !hasSourceExists {
			t.Error("Expected source_exists prerequisite")
		}
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
	})

	t.Run("DeleteOperation prerequisites", func(t *testing.T) {
		op := operations.NewDeleteOperation(core.OperationID("test-op"), "file.txt")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 1 {
			t.Errorf("Expected 1 prerequisite, got %d", len(prereqs))
		}
		
		// Check for source exists prerequisite
		if prereqs[0].Type() != "source_exists" {
			t.Errorf("Expected source_exists prerequisite, got %s", prereqs[0].Type())
		}
		if prereqs[0].Path() != "file.txt" {
			t.Errorf("Expected prerequisite path to be 'file.txt', got '%s'", prereqs[0].Path())
		}
	})

	t.Run("CreateSymlinkOperation prerequisites", func(t *testing.T) {
		op := operations.NewCreateSymlinkOperation(core.OperationID("test-op"), "subdir/link")
		
		prereqs := op.Prerequisites()
		if len(prereqs) != 2 {
			t.Errorf("Expected 2 prerequisites, got %d", len(prereqs))
		}
		
		// Check for parent directory and no conflict prerequisites
		hasParentDir := false
		hasNoConflict := false
		for _, prereq := range prereqs {
			switch prereq.Type() {
			case "parent_dir":
				hasParentDir = true
			case "no_conflict":
				hasNoConflict = true
			}
		}
		
		if !hasParentDir {
			t.Error("Expected parent_dir prerequisite")
		}
		if !hasNoConflict {
			t.Error("Expected no_conflict prerequisite")
		}
	})

	t.Run("Root path operations have no parent_dir prerequisite", func(t *testing.T) {
		op := operations.NewCreateFileOperation(core.OperationID("test-op"), "file.txt")
		
		prereqs := op.Prerequisites()
		
		// Should only have no_conflict prerequisite, not parent_dir
		if len(prereqs) != 1 {
			t.Errorf("Expected 1 prerequisite for root path, got %d", len(prereqs))
		}
		
		if prereqs[0].Type() != "no_conflict" {
			t.Errorf("Expected no_conflict prerequisite, got %s", prereqs[0].Type())
		}
	})
}

func TestPrerequisiteValidation(t *testing.T) {
	fs := NewMockFilesystem()
	
	t.Run("ParentDirPrerequisite validates correctly", func(t *testing.T) {
		// Create parent directory
		if err := fs.MkdirAll("parent", 0755); err != nil {
			t.Fatal(err)
		}
		
		prereq := core.NewParentDirPrerequisite("parent/child")
		err := prereq.Validate(fs)
		if err != nil {
			t.Errorf("Expected parent directory validation to pass, got error: %v", err)
		}
	})

	t.Run("ParentDirPrerequisite fails when parent missing", func(t *testing.T) {
		prereq := core.NewParentDirPrerequisite("nonexistent/child")
		err := prereq.Validate(fs)
		if err == nil {
			t.Error("Expected parent directory validation to fail when parent doesn't exist")
		}
	})

	t.Run("NoConflictPrerequisite validates correctly", func(t *testing.T) {
		prereq := core.NewNoConflictPrerequisite("newfile.txt")
		err := prereq.Validate(fs)
		if err != nil {
			t.Errorf("Expected no conflict validation to pass, got error: %v", err)
		}
	})

	t.Run("NoConflictPrerequisite fails when file exists", func(t *testing.T) {
		// Create a file
		if err := fs.WriteFile("existing.txt", []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
		
		prereq := core.NewNoConflictPrerequisite("existing.txt")
		err := prereq.Validate(fs)
		if err == nil {
			t.Error("Expected no conflict validation to fail when file exists")
		}
	})

	t.Run("SourceExistsPrerequisite validates correctly", func(t *testing.T) {
		// Create a file
		if err := fs.WriteFile("source.txt", []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
		
		prereq := core.NewSourceExistsPrerequisite("source.txt")
		err := prereq.Validate(fs)
		if err != nil {
			t.Errorf("Expected source exists validation to pass, got error: %v", err)
		}
	})

	t.Run("SourceExistsPrerequisite fails when source missing", func(t *testing.T) {
		prereq := core.NewSourceExistsPrerequisite("nonexistent.txt")
		err := prereq.Validate(fs)
		if err == nil {
			t.Error("Expected source exists validation to fail when source doesn't exist")
		}
	})
}