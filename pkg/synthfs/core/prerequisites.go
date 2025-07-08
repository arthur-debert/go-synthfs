// Package core provides prerequisite interfaces and types for the synthfs system.
package core

// Prerequisite represents a condition that must be met before an operation executes
type Prerequisite interface {
	Type() string                          // "parent_dir", "no_conflict", "source_exists"
	Path() string                          // Path this prerequisite relates to
	Validate(fsys interface{}) error       // Check if prerequisite is already satisfied
}

// PrerequisiteResolver can create operations to satisfy prerequisites
type PrerequisiteResolver interface {
	CanResolve(prereq Prerequisite) bool                   // Check if this resolver can handle the prerequisite
	Resolve(prereq Prerequisite) ([]interface{}, error)   // Returns operations to satisfy the prerequisite
}