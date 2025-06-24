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

// Operation defines a single abstract filesystem operation in the v2 API.
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

	// WithID sets the operation's ID.
	// Returns the operation to allow chaining.
	WithID(id OperationID) Operation

	// WithDependency adds a dependency to the operation.
	// Returns the operation to allow chaining.
	WithDependency(depID OperationID) Operation

	// GetItem returns the FsItem associated with this operation, if any.
	// This is primarily relevant for Create operations.
	// Returns nil if no item is directly associated (e.g., for Delete, Copy, Move by path).
	GetItem() FsItem
}

// --- BaseOperation ---

// BaseOperation provides a basic implementation of common Operation methods.
// Specific operations can embed this to reduce boilerplate.
type BaseOperation struct {
	id           OperationID
	dependencies []OperationID
	description  OperationDesc
}

// NewBaseOperation creates a new BaseOperation.
// Typically called by specific operation constructors.
func NewBaseOperation(id OperationID, descType string, path string) BaseOperation {
	return BaseOperation{
		id: id,
		description: OperationDesc{
			Type:    descType,
			Path:    path,
			Details: make(map[string]interface{}),
		},
	}
}

// ID returns the operation's ID.
func (bo *BaseOperation) ID() OperationID {
	return bo.id
}

// Dependencies returns the list of operation dependencies.
func (bo *BaseOperation) Dependencies() []OperationID {
	return bo.dependencies
}

// Describe returns the operation's description.
func (bo *BaseOperation) Describe() OperationDesc {
	return bo.description
}

// WithID sets the operation's ID.
func (bo *BaseOperation) WithID(id OperationID) Operation {
	bo.id = id
	// This is tricky: BaseOperation itself doesn't implement Operation.
	// The embedding struct must return itself. This method here
	// is more of a template. Specific operations will need to implement this
	// to return their own type.
	// For now, returning nil to indicate it needs to be implemented by embedder.
	// A better approach would be to have WithID/WithDependency modify the BaseOperation
	// and the concrete type's WithID/WithDependency methods call this and return `self`.
	panic("WithID must be implemented by the embedding Operation type and return itself")
}

// WithDependency adds a dependency.
func (bo *BaseOperation) WithDependency(depID OperationID) Operation {
	bo.dependencies = append(bo.dependencies, depID)
	// Similar to WithID, this needs to be properly handled by the embedding type.
	panic("WithDependency must be implemented by the embedding Operation type and return itself")
}

// SetDescriptionDetail sets a detail in the operation's description.
func (bo *BaseOperation) SetDescriptionDetail(key string, value interface{}) {
	if bo.description.Details == nil {
		bo.description.Details = make(map[string]interface{})
	}
	bo.description.Details[key] = value
}

// --- Placeholder Operation Types (for generic constructors) ---

// GenericOperation is a placeholder for operations created by the unified constructors.
// It will embed BaseOperation and implement the Execute, Validate, Rollback methods
// based on the specific action (Create, Delete, Copy, Move) and FsItem type.
// For Phase 0, these methods can be stubs.
type GenericOperation struct {
	BaseOperation
	// Item field will be used for Create operations
	Item FsItem // The FsItem for Create operations
	// SrcPath/DstPath fields for Copy/Move
	SrcPath string
	DstPath string
}

// Ensure GenericOperation satisfies the Operation interface.
var _ Operation = (*GenericOperation)(nil)

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

// WithID sets the operation's ID for GenericOperation.
func (op *GenericOperation) WithID(id OperationID) Operation {
	op.id = id
	return op
}

// WithDependency adds a dependency for GenericOperation.
func (op *GenericOperation) WithDependency(depID OperationID) Operation {
	op.dependencies = append(op.dependencies, depID)
	return op
}

// GetItem returns the FsItem associated with this GenericOperation (primarily for Create).
func (op *GenericOperation) GetItem() FsItem {
	return op.Item
}

// Conflicts returns an empty list for GenericOperation (conflicts not implemented yet).
func (op *GenericOperation) Conflicts() []OperationID {
	return nil
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
