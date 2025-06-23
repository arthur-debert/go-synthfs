package synthfs

import (
	"context"
	"fmt"
	// "io/fs" // Removed unused import
)

// OperationID is a unique identifier for an operation.
// It's used for tracking dependencies and conflicts.
type OperationID string

// OperationDesc provides a human-readable description of an operation.
// This can be used for logging, display, or serialization.
// The exact structure might evolve based on needs for serialization.
type OperationDesc struct {
	// Type is the type of operation (e.g., "create_file", "create_dir").
	Type string
	// Path is the primary path targeted by the operation.
	Path string
	// Details provides additional structured information about the operation.
	// For example, for a CreateFile operation, this could include the file size.
	// For a Copy operation, it could include the source path.
	Details map[string]interface{}
}

// Operation defines a single abstract filesystem operation.
type Operation interface {
	// ID returns the unique identifier of the operation.
	ID() OperationID

	// Execute performs the operation on the given filesystem.
	// It should only be called after Validate has passed and dependencies are met.
	Execute(ctx context.Context, fsys FileSystem) error

	// Validate checks if the operation can be performed.
	// This might involve checking preconditions on the filesystem or
	// internal consistency of the operation's parameters.
	// For example, a Copy operation might validate that the source exists.
	Validate(ctx context.Context, fsys FileSystem) error

	// Dependencies returns a list of OperationIDs that must be successfully
	// executed before this operation can run.
	Dependencies() []OperationID

	// Conflicts returns a list of OperationIDs that cannot run concurrently
	// with this operation or that represent incompatible desired states.
	// The exact meaning of "conflict" might be refined (e.g., path-based conflicts).
	Conflicts() []OperationID

	// Rollback attempts to undo the effects of the Execute method.
	// This is crucial for transactional execution. If an operation
	// cannot be rolled back, it should return an error.
	Rollback(ctx context.Context, fsys FileSystem) error

	// Describe returns a human-readable description of the operation.
	Describe() OperationDesc
}

// ValidationError represents an error during operation validation.
type ValidationError struct {
	Operation Operation
	Reason    string
	Cause     error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation error for operation %s (%s): %s: %v",
			e.Operation.ID(), e.Operation.Describe().Path, e.Reason, e.Cause)
	}
	return fmt.Sprintf("validation error for operation %s (%s): %s",
		e.Operation.ID(), e.Operation.Describe().Path, e.Reason)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// DependencyError represents an error with operation dependencies.
type DependencyError struct {
	Operation    Operation
	Dependencies []OperationID
	Missing      []OperationID
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependency error for operation %s: missing dependencies %v (required: %v)",
		e.Operation.ID(), e.Missing, e.Dependencies)
}

// ConflictError represents an error when operations conflict with each other.
type ConflictError struct {
	Operation Operation
	Conflicts []OperationID
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict error for operation %s: conflicts with operations %v",
		e.Operation.ID(), e.Conflicts)
}
