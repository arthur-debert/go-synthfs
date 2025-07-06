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