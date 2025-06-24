package synthfs

import (
	"context"
	"fmt"
)

// OperationID is a unique identifier for an operation.
type OperationID string

// FileSystem interface is defined in fs.go

// OperationDesc provides a human-readable description of an operation.
type OperationDesc struct {
	Type    string                 // e.g., "create_file", "delete_directory"
	Path    string                 // Primary path affected
	Details map[string]interface{} // Additional operation-specific details
}

// Operation defines a single abstract filesystem operation.
type Operation interface {
	// ID returns the unique identifier of the operation.
	ID() OperationID

	// Dependencies returns a list of OperationIDs that must be successfully
	// executed before this operation can run.
	Dependencies() []OperationID

	// Conflicts returns a list of OperationIDs that cannot run concurrently
	// with this operation or that represent incompatible desired states.
	Conflicts() []OperationID

	// Execute performs the operation on the given filesystem.
	Execute(ctx context.Context, fsys FileSystem) error

	// Validate checks if the operation can be performed.
	Validate(ctx context.Context, fsys FileSystem) error

	// Rollback attempts to undo the effects of the Execute method.
	Rollback(ctx context.Context, fsys FileSystem) error

	// Describe returns a structured description of the operation.
	Describe() OperationDesc

	// GetItem returns the FsItem associated with this operation, if any.
	// This is primarily relevant for Create operations.
	// Returns nil if no item is directly associated (e.g., for Delete, Copy, Move by path).
	GetItem() FsItem
}

// --- SimpleOperation: Basic Operation Implementation ---

// SimpleOperation provides a straightforward implementation of Operation.
// Operations are created complete and immutable - no post-creation modification.
type SimpleOperation struct {
	id           OperationID
	dependencies []OperationID
	description  OperationDesc
	item         FsItem // For Create operations
	srcPath      string // For Copy/Move operations
	dstPath      string // For Copy/Move operations
}

// NewSimpleOperation creates a new simple operation.
func NewSimpleOperation(id OperationID, descType string, path string) *SimpleOperation {
	return &SimpleOperation{
		id: id,
		description: OperationDesc{
			Type:    descType,
			Path:    path,
			Details: make(map[string]interface{}),
		},
		dependencies: []OperationID{},
	}
}

// ID returns the operation's ID.
func (op *SimpleOperation) ID() OperationID {
	return op.id
}

// Dependencies returns the list of operation dependencies.
func (op *SimpleOperation) Dependencies() []OperationID {
	return op.dependencies
}

// Conflicts returns an empty list (conflicts not implemented yet).
func (op *SimpleOperation) Conflicts() []OperationID {
	return nil
}

// Describe returns the operation's description.
func (op *SimpleOperation) Describe() OperationDesc {
	return op.description
}

// GetItem returns the FsItem associated with this operation.
func (op *SimpleOperation) GetItem() FsItem {
	return op.item
}

// SetItem sets the FsItem for Create operations.
func (op *SimpleOperation) SetItem(item FsItem) {
	op.item = item
}

// SetPaths sets source and destination paths for Copy/Move operations.
func (op *SimpleOperation) SetPaths(src, dst string) {
	op.srcPath = src
	op.dstPath = dst
}

// AddDependency adds a dependency to the operation.
func (op *SimpleOperation) AddDependency(depID OperationID) {
	op.dependencies = append(op.dependencies, depID)
}

// SetDescriptionDetail sets a detail in the operation's description.
func (op *SimpleOperation) SetDescriptionDetail(key string, value interface{}) {
	if op.description.Details == nil {
		op.description.Details = make(map[string]interface{})
	}
	op.description.Details[key] = value
}

// Execute (stub for Phase 0)
func (op *SimpleOperation) Execute(ctx context.Context, fsys FileSystem) error {
	fmt.Printf("Execute (stub): %s %s\n", op.description.Type, op.description.Path)
	if op.item != nil {
		fmt.Printf("  Item Type: %s\n", op.item.Type())
	}
	if op.srcPath != "" {
		fmt.Printf("  Source: %s\n", op.srcPath)
	}
	if op.dstPath != "" {
		fmt.Printf("  Destination: %s\n", op.dstPath)
	}
	return nil // Placeholder
}

// Validate (stub for Phase 0)
func (op *SimpleOperation) Validate(ctx context.Context, fsys FileSystem) error {
	fmt.Printf("Validate (stub): %s %s\n", op.description.Type, op.description.Path)

	// Basic validation: reject empty paths for Phase 0
	if op.description.Path == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "path cannot be empty",
			Cause:     nil,
		}
	}

	return nil // Placeholder
}

// Rollback (stub for Phase 0)
func (op *SimpleOperation) Rollback(ctx context.Context, fsys FileSystem) error {
	fmt.Printf("Rollback (stub): %s %s\n", op.description.Type, op.description.Path)
	return nil // Placeholder
}

// --- Legacy GenericOperation (for backward compatibility during transition) ---

// GenericOperation is kept for backward compatibility but should be replaced with SimpleOperation.
type GenericOperation struct {
	id           OperationID
	dependencies []OperationID
	description  OperationDesc
	Item         FsItem // The FsItem for Create operations
	SrcPath      string // Source path for Copy/Move
	DstPath      string // Destination path for Copy/Move
}

// Ensure GenericOperation satisfies the Operation interface.
var _ Operation = (*GenericOperation)(nil)

func (op *GenericOperation) ID() OperationID             { return op.id }
func (op *GenericOperation) Dependencies() []OperationID { return op.dependencies }
func (op *GenericOperation) Conflicts() []OperationID    { return nil }
func (op *GenericOperation) Describe() OperationDesc     { return op.description }
func (op *GenericOperation) GetItem() FsItem             { return op.Item }

// Execute (stub for Phase 0)
func (op *GenericOperation) Execute(ctx context.Context, fsys FileSystem) error {
	fmt.Printf("Execute (stub): %s %s\n", op.description.Type, op.description.Path)
	if op.Item != nil {
		fmt.Printf("  Item Type: %s\n", op.Item.Type())
	}
	if op.SrcPath != "" {
		fmt.Printf("  Source: %s\n", op.SrcPath)
	}
	if op.DstPath != "" {
		fmt.Printf("  Destination: %s\n", op.DstPath)
	}
	return nil // Placeholder
}

// Validate (stub for Phase 0)
func (op *GenericOperation) Validate(ctx context.Context, fsys FileSystem) error {
	fmt.Printf("Validate (stub): %s %s\n", op.description.Type, op.description.Path)

	// Basic validation: reject empty paths for Phase 0
	if op.description.Path == "" {
		return &ValidationError{
			Operation: op,
			Reason:    "path cannot be empty",
			Cause:     nil,
		}
	}

	return nil // Placeholder
}

// Rollback (stub for Phase 0)
func (op *GenericOperation) Rollback(ctx context.Context, fsys FileSystem) error {
	fmt.Printf("Rollback (stub): %s %s\n", op.description.Type, op.description.Path)
	return nil // Placeholder
}

// --- Error Types ---

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
