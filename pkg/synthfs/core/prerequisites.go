package core

// Prerequisite represents a condition that must be met before an operation executes
type Prerequisite interface {
	Type() string        // "parent_dir", "no_conflict", "source_exists"
	Path() string        // Path this prerequisite relates to
	Validate(fsys interface{}) error
}

// PrerequisiteResolver can create operations to satisfy prerequisites
type PrerequisiteResolver interface {
	CanResolve(prereq Prerequisite) bool
	Resolve(prereq Prerequisite) ([]interface{}, error) // Returns operations
}