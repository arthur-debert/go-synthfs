package core

import (
	"fmt"
	"path/filepath"
)

// ParentDirPrerequisite requires that the parent directory of a path exists
type ParentDirPrerequisite struct {
	path string
}

// NewParentDirPrerequisite creates a new parent directory prerequisite
func NewParentDirPrerequisite(path string) Prerequisite {
	return &ParentDirPrerequisite{path: path}
}

// Type returns the prerequisite type identifier
func (p *ParentDirPrerequisite) Type() string {
	return "parent_dir"
}

// Path returns the path this prerequisite relates to
func (p *ParentDirPrerequisite) Path() string {
	return p.path
}

// Validate checks if the parent directory exists
func (p *ParentDirPrerequisite) Validate(fsys interface{}) error {
	parentDir := filepath.Dir(p.path)
	if parentDir == "." || parentDir == "/" {
		return nil // No parent directory required
	}

	// Check if parent directory exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(parentDir); err != nil {
			return fmt.Errorf("parent directory does not exist: %s", parentDir)
		}
	}

	return nil
}

// NoConflictPrerequisite requires that no conflicting file exists at the path
type NoConflictPrerequisite struct {
	path string
}

// NewNoConflictPrerequisite creates a new no-conflict prerequisite
func NewNoConflictPrerequisite(path string) Prerequisite {
	return &NoConflictPrerequisite{path: path}
}

// Type returns the prerequisite type identifier
func (p *NoConflictPrerequisite) Type() string {
	return "no_conflict"
}

// Path returns the path this prerequisite relates to
func (p *NoConflictPrerequisite) Path() string {
	return p.path
}

// Validate checks if no conflicting file exists
func (p *NoConflictPrerequisite) Validate(fsys interface{}) error {
	// Check if file already exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(p.path); err == nil {
			return fmt.Errorf("path already exists: %s", p.path)
		}
	}

	return nil
}

// SourceExistsPrerequisite requires that a source file/directory exists
type SourceExistsPrerequisite struct {
	path string
}

// NewSourceExistsPrerequisite creates a new source exists prerequisite
func NewSourceExistsPrerequisite(path string) Prerequisite {
	return &SourceExistsPrerequisite{path: path}
}

// Type returns the prerequisite type identifier
func (p *SourceExistsPrerequisite) Type() string {
	return "source_exists"
}

// Path returns the path this prerequisite relates to
func (p *SourceExistsPrerequisite) Path() string {
	return p.path
}

// Validate checks if the source exists
func (p *SourceExistsPrerequisite) Validate(fsys interface{}) error {
	// Check if source exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(p.path); err != nil {
			return fmt.Errorf("source does not exist: %s", p.path)
		}
	}

	return nil
}

// Helper function to get Stat method from filesystem interface
func getStatMethod(fsys interface{}) (func(string) (interface{}, error), bool) {
	// Try interface{} version first
	type statFS interface {
		Stat(name string) (interface{}, error)
	}

	if fs, ok := fsys.(statFS); ok {
		return fs.Stat, true
	}

	// Try os.FileInfo version
	type statFSFileInfo interface {
		Stat(name string) (interface{}, error)
	}

	if fs, ok := fsys.(statFSFileInfo); ok {
		return fs.Stat, true
	}

	return nil, false
}