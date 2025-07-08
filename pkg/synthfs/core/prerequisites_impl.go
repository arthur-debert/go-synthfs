package core

import (
	"fmt"
	"path/filepath"
)

// ParentDirPrerequisite represents a prerequisite that a parent directory must exist
type ParentDirPrerequisite struct {
	path string
}

// NewParentDirPrerequisite creates a new parent directory prerequisite
func NewParentDirPrerequisite(filePath string) *ParentDirPrerequisite {
	return &ParentDirPrerequisite{
		path: filepath.Dir(filePath),
	}
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
	// Skip validation for root and current directory
	if p.path == "." || p.path == "/" {
		return nil
	}

	// Try to get Stat method from filesystem
	type statter interface {
		Stat(name string) (interface{}, error)
	}

	if stat, ok := fsys.(statter); ok {
		if info, err := stat.Stat(p.path); err != nil {
			return fmt.Errorf("parent directory %s does not exist", p.path)
		} else {
			// Check if it's actually a directory
			if dirChecker, ok := info.(interface{ IsDir() bool }); ok {
				if !dirChecker.IsDir() {
					return fmt.Errorf("parent path %s exists but is not a directory", p.path)
				}
			}
		}
	}

	return nil
}

// NoConflictPrerequisite represents a prerequisite that no file should exist at the target path
type NoConflictPrerequisite struct {
	path string
}

// NewNoConflictPrerequisite creates a new no-conflict prerequisite
func NewNoConflictPrerequisite(path string) *NoConflictPrerequisite {
	return &NoConflictPrerequisite{
		path: path,
	}
}

// Type returns the prerequisite type
func (p *NoConflictPrerequisite) Type() string {
	return "no_conflict"
}

// Path returns the path this prerequisite relates to
func (p *NoConflictPrerequisite) Path() string {
	return p.path
}

// Validate checks if there's no existing file at the target path
func (p *NoConflictPrerequisite) Validate(fsys interface{}) error {
	// Try to get Stat method from filesystem
	type statter interface {
		Stat(name string) (interface{}, error)
	}

	if stat, ok := fsys.(statter); ok {
		if _, err := stat.Stat(p.path); err == nil {
			return fmt.Errorf("file already exists at %s", p.path)
		}
	}

	return nil
}

// SourceExistsPrerequisite represents a prerequisite that a source file must exist
type SourceExistsPrerequisite struct {
	path string
}

// NewSourceExistsPrerequisite creates a new source exists prerequisite
func NewSourceExistsPrerequisite(path string) *SourceExistsPrerequisite {
	return &SourceExistsPrerequisite{
		path: path,
	}
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
	type statter interface {
		Stat(name string) (interface{}, error)
	}

	if stat, ok := fsys.(statter); ok {
		if _, err := stat.Stat(p.path); err != nil {
			return fmt.Errorf("source file %s does not exist", p.path)
		}
	}

	return nil
}