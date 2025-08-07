package core

import (
	"fmt"
	"io/fs"
	"testing"
	"time"
)

// MockFileSystem implements the basic interfaces needed for prerequisite testing
type MockFileSystem struct {
	files map[string]MockFileInfo
}

// MockFileInfo implements the fs.FileInfo interface
type MockFileInfo struct {
	name  string
	isDir bool
	size  int64
	mode  fs.FileMode
}

func (f MockFileInfo) Name() string       { return f.name }
func (f MockFileInfo) Size() int64        { return f.size }
func (f MockFileInfo) Mode() fs.FileMode  { return f.mode }
func (f MockFileInfo) ModTime() time.Time { return time.Now() }
func (f MockFileInfo) IsDir() bool        { return f.isDir }
func (f MockFileInfo) Sys() interface{}   { return nil }

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files: make(map[string]MockFileInfo),
	}
}

func (fs *MockFileSystem) Stat(name string) (fs.FileInfo, error) {
	if info, exists := fs.files[name]; exists {
		return info, nil
	}
	return nil, fmt.Errorf("file not found: %s", name)
}

func (fs *MockFileSystem) AddFile(path string, isDir bool) {
	fs.files[path] = MockFileInfo{name: path, isDir: isDir}
}

func TestParentDirPrerequisite(t *testing.T) {
	fs := NewMockFileSystem()

	t.Run("Valid parent directory", func(t *testing.T) {
		// Setup: create parent directory
		fs.AddFile("parent", true)

		// Test: create prerequisite for a file in that directory
		prereq := NewParentDirPrerequisite("parent/file.txt")

		if prereq.Type() != "parent_dir" {
			t.Errorf("Expected type 'parent_dir', got '%s'", prereq.Type())
		}

		if prereq.Path() != "parent" {
			t.Errorf("Expected path 'parent', got '%s'", prereq.Path())
		}

		if err := prereq.Validate(fs); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("Missing parent directory", func(t *testing.T) {
		// Test: create prerequisite for a file with non-existent parent
		prereq := NewParentDirPrerequisite("nonexistent/file.txt")

		err := prereq.Validate(fs)
		if err == nil {
			t.Error("Expected validation error for missing parent directory")
		}
	})

	t.Run("Parent is file not directory", func(t *testing.T) {
		// Setup: create a file (not directory) at parent path
		fs.AddFile("parent-file", false)

		// Test: create prerequisite for a file in that "directory"
		prereq := NewParentDirPrerequisite("parent-file/file.txt")

		err := prereq.Validate(fs)
		if err == nil {
			t.Error("Expected validation error when parent is file not directory")
		}
	})

	t.Run("Root directory", func(t *testing.T) {
		// Test: root directory should always be valid
		prereq := NewParentDirPrerequisite("/file.txt")

		if err := prereq.Validate(fs); err != nil {
			t.Errorf("Root directory validation failed: %v", err)
		}
	})

	t.Run("Current directory", func(t *testing.T) {
		// Test: current directory should always be valid
		prereq := NewParentDirPrerequisite("file.txt")

		if err := prereq.Validate(fs); err != nil {
			t.Errorf("Current directory validation failed: %v", err)
		}
	})
}

func TestNoConflictPrerequisite(t *testing.T) {
	fs := NewMockFileSystem()

	t.Run("No existing file", func(t *testing.T) {
		// Test: path with no existing file should be valid
		prereq := NewNoConflictPrerequisite("new-file.txt")

		if prereq.Type() != "no_conflict" {
			t.Errorf("Expected type 'no_conflict', got '%s'", prereq.Type())
		}

		if prereq.Path() != "new-file.txt" {
			t.Errorf("Expected path 'new-file.txt', got '%s'", prereq.Path())
		}

		if err := prereq.Validate(fs); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("Existing file conflicts", func(t *testing.T) {
		// Setup: create a file
		fs.AddFile("existing-file.txt", false)

		// Test: should fail validation due to conflict
		prereq := NewNoConflictPrerequisite("existing-file.txt")

		err := prereq.Validate(fs)
		if err == nil {
			t.Error("Expected validation error for existing file conflict")
		}
	})

	t.Run("Existing directory conflicts", func(t *testing.T) {
		// Setup: create a directory
		fs.AddFile("existing-dir", true)

		// Test: should fail validation due to conflict
		prereq := NewNoConflictPrerequisite("existing-dir")

		err := prereq.Validate(fs)
		if err == nil {
			t.Error("Expected validation error for existing directory conflict")
		}
	})
}

func TestSourceExistsPrerequisite(t *testing.T) {
	fs := NewMockFileSystem()

	t.Run("Source file exists", func(t *testing.T) {
		// Setup: create source file
		fs.AddFile("source.txt", false)

		// Test: should pass validation
		prereq := NewSourceExistsPrerequisite("source.txt")

		if prereq.Type() != "source_exists" {
			t.Errorf("Expected type 'source_exists', got '%s'", prereq.Type())
		}

		if prereq.Path() != "source.txt" {
			t.Errorf("Expected path 'source.txt', got '%s'", prereq.Path())
		}

		if err := prereq.Validate(fs); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("Source directory exists", func(t *testing.T) {
		// Setup: create source directory
		fs.AddFile("source-dir", true)

		// Test: should pass validation
		prereq := NewSourceExistsPrerequisite("source-dir")

		if err := prereq.Validate(fs); err != nil {
			t.Errorf("Validation failed: %v", err)
		}
	})

	t.Run("Source does not exist", func(t *testing.T) {
		// Test: should fail validation
		prereq := NewSourceExistsPrerequisite("nonexistent.txt")

		err := prereq.Validate(fs)
		if err == nil {
			t.Error("Expected validation error for non-existent source")
		}
	})
}

func TestPrerequisiteTypes(t *testing.T) {
	t.Run("Prerequisite types are correct", func(t *testing.T) {
		parentPrereq := NewParentDirPrerequisite("dir/file.txt")
		if parentPrereq.Type() != "parent_dir" {
			t.Errorf("Expected parent_dir type, got %s", parentPrereq.Type())
		}

		conflictPrereq := NewNoConflictPrerequisite("file.txt")
		if conflictPrereq.Type() != "no_conflict" {
			t.Errorf("Expected no_conflict type, got %s", conflictPrereq.Type())
		}

		sourcePrereq := NewSourceExistsPrerequisite("source.txt")
		if sourcePrereq.Type() != "source_exists" {
			t.Errorf("Expected source_exists type, got %s", sourcePrereq.Type())
		}
	})
}
