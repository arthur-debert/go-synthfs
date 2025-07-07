package core

// OperationMetadata provides basic information about an operation
type OperationMetadata interface {
	ID() OperationID
	Describe() OperationDesc
}

// DependencyAware provides dependency and conflict information
type DependencyAware interface {
	Dependencies() []OperationID
	Conflicts() []OperationID
}

// Note: Executable interface will be defined in the main synthfs package
// because it depends on filesystem.FileSystem which would create a circular dependency

// ExecutableV2 defines execution capabilities using ExecutionContext
type ExecutableV2 interface {
	ExecuteV2(ctx interface{}, execCtx *ExecutionContext, fsys interface{}) error
	ValidateV2(ctx interface{}, execCtx *ExecutionContext, fsys interface{}) error
}

// OperationFactory creates operations based on type and item
// Note: The actual Operation interface is defined in the main synthfs package
type OperationFactory interface {
	CreateOperation(id OperationID, opType string, path string) (interface{}, error)
	SetItemForOperation(op interface{}, item interface{}) error
}
