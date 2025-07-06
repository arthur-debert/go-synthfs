package synthfs

import (
	"fmt"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// OperationRegistry implements the core.OperationFactory interface
type OperationRegistry struct{}

// NewOperationRegistry creates a new operation registry
func NewOperationRegistry() *OperationRegistry {
	return &OperationRegistry{}
}

// CreateOperation creates an operation based on type and path
func (r *OperationRegistry) CreateOperation(id core.OperationID, opType string, path string) (interface{}, error) {
	// For now, we just delegate to NewSimpleOperation
	// In the future, this could use a map of operation type to factory function
	op := NewSimpleOperation(id, opType, path)
	return op, nil
}

// SetItemForOperation sets the item for an operation
func (r *OperationRegistry) SetItemForOperation(op interface{}, item interface{}) error {
	operation, ok := op.(*SimpleOperation)
	if !ok {
		return fmt.Errorf("operation is not a SimpleOperation")
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