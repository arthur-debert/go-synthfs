package synthfs

import (
	"testing"
)

// TestRegistryOperationsPackage tests that the registry can create operations from the operations package
func TestRegistryOperationsPackage(t *testing.T) {
	t.Run("Registry creates operations package operations by default", func(t *testing.T) {
		// Create registry (operations package is enabled by default)
		registry := NewOperationRegistry()

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

	// Remove test for disabled operations package since it's always enabled now

	t.Run("SetItemForOperation works with operations package adapter", func(t *testing.T) {
		registry := NewOperationRegistry()

		// Test with operations package operation (wrapped in adapter)
		opsOp, _ := registry.CreateOperation("ops-op", "create_file", "/test.txt")
		fileItem := NewFile("/test.txt")
		err := registry.SetItemForOperation(opsOp, fileItem)
		if err != nil {
			t.Errorf("Failed to set item on operations package operation: %v", err)
		}
	})
}
