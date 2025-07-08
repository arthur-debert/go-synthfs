package core

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

// ParentDirPrerequisite ensures a parent directory exists before creating a file/directory
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

// Path returns the path whose parent directory must exist
func (p *ParentDirPrerequisite) Path() string {
	return p.path
}

// Validate checks if the parent directory exists
func (p *ParentDirPrerequisite) Validate(fsys interface{}) error {
	parentPath := filepath.Dir(p.path)
	if parentPath == "." || parentPath == "/" {
		return nil // Root or current directory always exists
	}

	// Try to get a stat method from the filesystem
	if statFS, ok := fsys.(interface{ Stat(string) (fs.FileInfo, error) }); ok {
		info, err := statFS.Stat(parentPath)
		if err != nil {
			return fmt.Errorf("parent directory %s does not exist", parentPath)
		}
		if !info.IsDir() {
			return fmt.Errorf("parent path %s exists but is not a directory", parentPath)
		}
		return nil
	}

	// Alternative interface for stat
	if statFS, ok := fsys.(interface{ Stat(string) (interface{}, error) }); ok {
		info, err := statFS.Stat(parentPath)
		if err != nil {
			return fmt.Errorf("parent directory %s does not exist", parentPath)
		}
		if dirChecker, ok := info.(interface{ IsDir() bool }); ok {
			if !dirChecker.IsDir() {
				return fmt.Errorf("parent path %s exists but is not a directory", parentPath)
			}
		}
		return nil
	}

	return fmt.Errorf("filesystem does not support Stat operation")
}

// NoConflictPrerequisite ensures no file/directory exists at the target path
type NoConflictPrerequisite struct {
	path string
}

// NewNoConflictPrerequisite creates a new no conflict prerequisite
func NewNoConflictPrerequisite(path string) *NoConflictPrerequisite {
	return &NoConflictPrerequisite{path: path}
}

// Type returns the prerequisite type
func (p *NoConflictPrerequisite) Type() string {
	return "no_conflict"
}

// Path returns the path that must not exist
func (p *NoConflictPrerequisite) Path() string {
	return p.path
}

// Validate checks if the path does not exist
func (p *NoConflictPrerequisite) Validate(fsys interface{}) error {
	// Try to get a stat method from the filesystem
	if statFS, ok := fsys.(interface{ Stat(string) (fs.FileInfo, error) }); ok {
		_, err := statFS.Stat(p.path)
		if err == nil {
			return fmt.Errorf("path %s already exists", p.path)
		}
		return nil // Path doesn't exist - good
	}

	// Alternative interface for stat
	if statFS, ok := fsys.(interface{ Stat(string) (interface{}, error) }); ok {
		_, err := statFS.Stat(p.path)
		if err == nil {
			return fmt.Errorf("path %s already exists", p.path)
		}
		return nil // Path doesn't exist - good
	}

	return fmt.Errorf("filesystem does not support Stat operation")
}

// SourceExistsPrerequisite ensures a source file/directory exists
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

// Path returns the path that must exist
func (p *SourceExistsPrerequisite) Path() string {
	return p.path
}

// Validate checks if the path exists
func (p *SourceExistsPrerequisite) Validate(fsys interface{}) error {
	// Try to get a stat method from the filesystem
	if statFS, ok := fsys.(interface{ Stat(string) (fs.FileInfo, error) }); ok {
		_, err := statFS.Stat(p.path)
		if err != nil {
			return fmt.Errorf("source path %s does not exist", p.path)
		}
		return nil
	}

	// Alternative interface for stat
	if statFS, ok := fsys.(interface{ Stat(string) (interface{}, error) }); ok {
		_, err := statFS.Stat(p.path)
		if err != nil {
			return fmt.Errorf("source path %s does not exist", p.path)
		}
		return nil
	}

	return fmt.Errorf("filesystem does not support Stat operation")
}