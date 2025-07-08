package core

import (
	"fmt"
	"path/filepath"
)

// ParentDirPrerequisite represents a requirement for a parent directory to exist
type ParentDirPrerequisite struct {
	path string
}

// NewParentDirPrerequisite creates a new parent directory prerequisite
func NewParentDirPrerequisite(path string) *ParentDirPrerequisite {
	return &ParentDirPrerequisite{path: path}
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
	parentDir := filepath.Dir(p.path)
	
	// If parent is root or current directory, it always exists
	if parentDir == "." || parentDir == "/" || parentDir == p.path {
		return nil
	}
	
	// Try to stat the parent directory
	if stat, ok := fsys.(interface{ Stat(string) (interface{}, error) }); ok {
		info, err := stat.Stat(parentDir)
		if err != nil {
			return fmt.Errorf("parent directory %s does not exist: %w", parentDir, err)
		}
		
		// Check if it's actually a directory
		if dirChecker, ok := info.(interface{ IsDir() bool }); ok {
			if !dirChecker.IsDir() {
				return fmt.Errorf("parent path %s is not a directory", parentDir)
			}
		}
	}
	
	return nil
}

// NoConflictPrerequisite represents a requirement that a path does not conflict with existing files
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

// Validate checks if the path doesn't conflict with existing files
func (p *NoConflictPrerequisite) Validate(fsys interface{}) error {
	// Try to stat the path
	if stat, ok := fsys.(interface{ Stat(string) (interface{}, error) }); ok {
		if _, err := stat.Stat(p.path); err == nil {
			return fmt.Errorf("path %s already exists", p.path)
		}
		// If stat fails, the path doesn't exist, which is what we want
	}
	
	return nil
}

// SourceExistsPrerequisite represents a requirement that a source path exists
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

// Validate checks if the source path exists
func (p *SourceExistsPrerequisite) Validate(fsys interface{}) error {
	// Try to stat the source path
	if stat, ok := fsys.(interface{ Stat(string) (interface{}, error) }); ok {
		if _, err := stat.Stat(p.path); err != nil {
			return fmt.Errorf("source path %s does not exist: %w", p.path, err)
		}
	} else {
		return fmt.Errorf("filesystem does not support Stat operation")
	}
	
	return nil
}