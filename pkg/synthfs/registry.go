package synthfs

import (
	"fmt"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// OperationRegistry implements the core.OperationFactory interface
type OperationRegistry struct{
	useOperationsPackage bool
	operationsFactory    *operations.Factory
}

// NewOperationRegistry creates a new operation registry
func NewOperationRegistry() *OperationRegistry {
	return &OperationRegistry{
		useOperationsPackage: false, // Start with false for backward compatibility
		operationsFactory:    operations.NewFactory(),
	}
}

// CreateOperation creates an operation based on type and path
func (r *OperationRegistry) CreateOperation(id core.OperationID, opType string, path string) (interface{}, error) {
	if r.useOperationsPackage {
		// Use the new operations package
		opsOp, err := r.operationsFactory.CreateOperation(id, opType, path)
		if err != nil {
			return nil, err
		}
		// Wrap in adapter to maintain compatibility with main package Operation interface
		return NewOperationsPackageAdapter(opsOp), nil
	}
	
	// Fall back to old implementation for backward compatibility
	op := NewSimpleOperation(id, opType, path)
	return op, nil
}

// SetItemForOperation sets the item for an operation
func (r *OperationRegistry) SetItemForOperation(op interface{}, item interface{}) error {
	// Try operations package operation first
	if opsOp, ok := op.(operations.Operation); ok {
		return r.operationsFactory.SetItemForOperation(opsOp, item)
	}
	
	// Fall back to SimpleOperation
	operation, ok := op.(*SimpleOperation)
	if !ok {
		return fmt.Errorf("operation is not a SimpleOperation or operations.Operation")
	}
	
	fsItem, ok := item.(FsItem)
	if !ok {
		return fmt.Errorf("item is not an FsItem")
	}
	
	operation.SetItem(fsItem)
	return nil
}

// Global registry instance
var defaultRegistry = NewOperationRegistry()

// GetDefaultRegistry returns the default operation registry
func GetDefaultRegistry() core.OperationFactory {
	return defaultRegistry
}

// EnableOperationsPackage enables the use of the operations package for creating operations
func (r *OperationRegistry) EnableOperationsPackage() {
	r.useOperationsPackage = true
}

// RegisterFactory implements the OperationRegistrar interface
func (r *OperationRegistry) RegisterFactory(factory core.OperationFactory) {
	// For now, we don't need to do anything as we have a single factory
	// In the future, this could maintain a map of operation types to factories
}

// init function to initialize the operations package
func init() {
	operations.Initialize(defaultRegistry)
}