package core

import (
	"fmt"
	"path/filepath"
)

// ParentDirPrerequisite ensures the parent directory exists
type ParentDirPrerequisite struct {
	path string
}

func NewParentDirPrerequisite(path string) Prerequisite {
	return &ParentDirPrerequisite{
		path: filepath.Dir(path),
	}
}

func (p *ParentDirPrerequisite) Type() string {
	return "parent_dir"
}

func (p *ParentDirPrerequisite) Path() string {
	return p.path
}

func (p *ParentDirPrerequisite) Validate(fsys interface{}) error {
	// Check if parent directory exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(p.path); err != nil {
			return fmt.Errorf("parent directory does not exist: %s", p.path)
		}
	}
	return nil
}

// NoConflictPrerequisite ensures no file exists at the target path
type NoConflictPrerequisite struct {
	path string
}

func NewNoConflictPrerequisite(path string) Prerequisite {
	return &NoConflictPrerequisite{
		path: path,
	}
}

func (p *NoConflictPrerequisite) Type() string {
	return "no_conflict"
}

func (p *NoConflictPrerequisite) Path() string {
	return p.path
}

func (p *NoConflictPrerequisite) Validate(fsys interface{}) error {
	// Check if file already exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(p.path); err == nil {
			return fmt.Errorf("file already exists: %s", p.path)
		}
	}
	return nil
}

// SourceExistsPrerequisite ensures a source file exists
type SourceExistsPrerequisite struct {
	path string
}

func NewSourceExistsPrerequisite(path string) Prerequisite {
	return &SourceExistsPrerequisite{
		path: path,
	}
}

func (p *SourceExistsPrerequisite) Type() string {
	return "source_exists"
}

func (p *SourceExistsPrerequisite) Path() string {
	return p.path
}

func (p *SourceExistsPrerequisite) Validate(fsys interface{}) error {
	// Check if source file exists
	if stat, ok := getStatMethod(fsys); ok {
		if _, err := stat(p.path); err != nil {
			return fmt.Errorf("source does not exist: %s", p.path)
		}
	}
	return nil
}

// Helper function to get Stat method from filesystem interface
func getStatMethod(fsys interface{}) (func(string) (interface{}, error), bool) {
	type statFS interface {
		Stat(name string) (interface{}, error)
	}

	if fs, ok := fsys.(statFS); ok {
		return fs.Stat, true
	}
	return nil, false
}