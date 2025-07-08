package core

import (
	"fmt"
	"path/filepath"
)

// ParentDirPrerequisite represents a requirement for parent directory to exist
type ParentDirPrerequisite struct {
	path string
}

// NewParentDirPrerequisite creates a new parent directory prerequisite
func NewParentDirPrerequisite(path string) *ParentDirPrerequisite {
	return &ParentDirPrerequisite{path: filepath.Dir(path)}
}

// Type returns the prerequisite type
func (p *ParentDirPrerequisite) Type() string {
	return "parent_dir"
}

// Path returns the path this prerequisite relates to
func (p *ParentDirPrerequisite) Path() string {
	return p.path
}

// Validate checks if the parent directory exists
func (p *ParentDirPrerequisite) Validate(fsys interface{}) error {
	// Skip validation for root directory
	if p.path == "." || p.path == "/" {
		return nil
	}
	
	// Try to get Stat method from filesystem
	type statFS interface {
		Stat(name string) (interface{}, error)
	}
	
	fs, ok := fsys.(statFS)
	if !ok {
		return fmt.Errorf("filesystem does not support Stat operation")
	}
	
	_, err := fs.Stat(p.path)
	if err != nil {
		return fmt.Errorf("parent directory %s does not exist", p.path)
	}
	
	return nil
}

// NoConflictPrerequisite represents a requirement for no existing conflicting item
type NoConflictPrerequisite struct {
	path string
}

// NewNoConflictPrerequisite creates a new no-conflict prerequisite
func NewNoConflictPrerequisite(path string) *NoConflictPrerequisite {
	return &NoConflictPrerequisite{path: path}
}

// Type returns the prerequisite type
func (p *NoConflictPrerequisite) Type() string {
	return "no_conflict"
}

// Path returns the path this prerequisite relates to
func (p *NoConflictPrerequisite) Path() string {
	return p.path
}

// Validate checks if there's no conflicting item at the path
func (p *NoConflictPrerequisite) Validate(fsys interface{}) error {
	// Try to get Stat method from filesystem
	type statFS interface {
		Stat(name string) (interface{}, error)
	}
	
	fs, ok := fsys.(statFS)
	if !ok {
		return fmt.Errorf("filesystem does not support Stat operation")
	}
	
	_, err := fs.Stat(p.path)
	if err == nil {
		return fmt.Errorf("item already exists at path %s", p.path)
	}
	
	// If the error is "file not found", that's what we want
	// For other errors, we should return them as validation failures
	return nil
}

// SourceExistsPrerequisite represents a requirement for source file to exist
type SourceExistsPrerequisite struct {
	path string
}

// NewSourceExistsPrerequisite creates a new source exists prerequisite
func NewSourceExistsPrerequisite(path string) *SourceExistsPrerequisite {
	return &SourceExistsPrerequisite{path: path}
}

// Type returns the prerequisite type
func (p *SourceExistsPrerequisite) Type() string {
	return "source_exists"
}

// Path returns the path this prerequisite relates to
func (p *SourceExistsPrerequisite) Path() string {
	return p.path
}

// Validate checks if the source file exists
func (p *SourceExistsPrerequisite) Validate(fsys interface{}) error {
	// Try to get Stat method from filesystem
	type statFS interface {
		Stat(name string) (interface{}, error)
	}
	
	fs, ok := fsys.(statFS)
	if !ok {
		return fmt.Errorf("filesystem does not support Stat operation")
	}
	
	_, err := fs.Stat(p.path)
	if err != nil {
		return fmt.Errorf("source file %s does not exist", p.path)
	}
	
	return nil
}