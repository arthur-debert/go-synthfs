package synthfs

import (
	"fmt"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// OperationRegistry implements the core.OperationFactory interface
type OperationRegistry struct {
	operationsFactory *operations.Factory
}

// NewOperationRegistry creates a new operation registry
func NewOperationRegistry() *OperationRegistry {
	return &OperationRegistry{
		operationsFactory: operations.NewFactory(),
	}
}

// CreateOperation creates an operation based on type and path
func (r *OperationRegistry) CreateOperation(id core.OperationID, opType string, path string) (interface{}, error) {
	// Always use the operations package
	opsOp, err := r.operationsFactory.CreateOperation(id, opType, path)
	if err != nil {
		return nil, err
	}
	// Return operations.Operation directly  
	return opsOp, nil
}

// SetItemForOperation sets the item for an operation
func (r *OperationRegistry) SetItemForOperation(op interface{}, item interface{}) error {
	// Handle operations package operation directly
	if opsOp, ok := op.(operations.Operation); ok {
		opsOp.SetItem(item)
		return nil
	}

	return fmt.Errorf("operation is not an operations.Operation")
}

// Global registry instance
var defaultRegistry = NewOperationRegistry()

// GetDefaultRegistry returns the default operation registry
func GetDefaultRegistry() core.OperationFactory {
	return defaultRegistry
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
