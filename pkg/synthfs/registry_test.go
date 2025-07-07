package synthfs

import (
	"testing"
)

// TestRegistryOperationsPackage tests that the registry can create operations from the operations package
func TestRegistryOperationsPackage(t *testing.T) {
	t.Run("Registry creates operations package operations when enabled", func(t *testing.T) {
		// Create registry and enable operations package
		registry := NewOperationRegistry()
		registry.EnableOperationsPackage()
		
		// Test each operation type
		opTypes := []string{
			"create_file",
			"create_directory", 
			"copy",
			"move",
			"delete",
			"create_symlink",
			"create_archive",
			"unarchive",
		}
		
		for _, opType := range opTypes {
			op, err := registry.CreateOperation("test-op", opType, "/test/path")
			if err != nil {
				t.Errorf("Failed to create %s operation: %v", opType, err)
				continue
			}
			
			// Verify it's wrapped in adapter
			if _, ok := op.(*OperationsPackageAdapter); !ok {
				t.Errorf("Expected OperationsPackageAdapter for %s, got %T", opType, op)
			}
			
			// Verify it implements Operation interface
			if _, ok := op.(Operation); !ok {
				t.Errorf("Operation %s does not implement Operation interface", opType)
			}
		}
	})
	
	t.Run("Registry creates SimpleOperation when operations package disabled", func(t *testing.T) {
		// Create registry without enabling operations package
		registry := NewOperationRegistry()
		
		op, err := registry.CreateOperation("test-op", "create_file", "/test.txt")
		if err != nil {
			t.Fatalf("Failed to create operation: %v", err)
		}
		
		// Verify it's SimpleOperation
		if _, ok := op.(*SimpleOperation); !ok {
			t.Errorf("Expected SimpleOperation, got %T", op)
		}
	})
	
	t.Run("SetItemForOperation works with both operation types", func(t *testing.T) {
		registry := NewOperationRegistry()
		
		// Test with SimpleOperation
		simpleOp, _ := registry.CreateOperation("simple-op", "create_file", "/test1.txt")
		fileItem := NewFile("/test1.txt")
		err := registry.SetItemForOperation(simpleOp, fileItem)
		if err != nil {
			t.Errorf("Failed to set item on SimpleOperation: %v", err)
		}
		
		// Test with operations package operation
		registry.EnableOperationsPackage()
		opsOp, _ := registry.CreateOperation("ops-op", "create_file", "/test2.txt")
		err = registry.SetItemForOperation(opsOp, fileItem)
		if err != nil {
			t.Errorf("Failed to set item on operations package operation: %v", err)
		}
	})
}