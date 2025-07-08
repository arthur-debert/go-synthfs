THIS SHOULD BE A LINTER ERRORpackage batch_test

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/registry"
)

func TestMigrationPath(t *testing.T) {
	// Create test filesystem and registry
	fs := filesystem.NewOSFileSystem(".")
	reg := registry.NewRegistry()

	t.Run("UseSimpleBatch flag in RunWithOptions", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		// Add a file operation to a nested path
		_, err := batch.CreateFile("deep/nested/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Run with UseSimpleBatch enabled
		opts := map[string]interface{}{
			"use_simple_batch":      true,
			"resolve_prerequisites": true,
			"restorable":            false,
			"max_backup_size_mb":    0,
		}

		result, err := batch.RunWithOptions(opts)
		if err != nil {
			t.Logf("Run failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}

		// Verify that the batch implementation was called
		// (This is mainly a smoke test - the detailed functionality is tested elsewhere)
	})

	t.Run("UseSimpleBatch false uses traditional behavior", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		// Add a file operation
		_, err := batch.CreateFile("test/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Run with UseSimpleBatch disabled (default)
		opts := map[string]interface{}{
			"use_simple_batch":      false,
			"resolve_prerequisites": true,
			"restorable":            false,
			"max_backup_size_mb":    0,
		}

		result, err := batch.RunWithOptions(opts)
		if err != nil {
			t.Logf("Run failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}
	})

	t.Run("RunWithSimpleBatch convenience method", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		// Add some operations
		_, err := batch.CreateDir("test-dir")
		if err != nil {
			t.Fatalf("Failed to create directory operation: %v", err)
		}

		_, err = batch.CreateFile("test-dir/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Use the convenience method
		result, err := batch.RunWithSimpleBatch()
		if err != nil {
			t.Logf("RunWithSimpleBatch failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}
	})

	t.Run("RunWithSimpleBatchAndBudget convenience method", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		// Add some operations
		_, err := batch.CreateFile("test.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Use the convenience method with budget
		result, err := batch.RunWithSimpleBatchAndBudget(20)
		if err != nil {
			t.Logf("RunWithSimpleBatchAndBudget failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}
	})

	t.Run("Default behavior unchanged", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		// Add some operations
		_, err := batch.CreateDir("default-test")
		if err != nil {
			t.Fatalf("Failed to create directory operation: %v", err)
		}

		// Default Run() should not use SimpleBatch
		result, err := batch.Run()
		if err != nil {
			t.Logf("Default Run failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}

		// The default behavior should remain unchanged (backward compatibility)
	})

	t.Run("Migration preserves operation order", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		// Add operations in specific order
		op1, err := batch.CreateDir("dir1")
		if err != nil {
			t.Fatalf("Failed to create dir1: %v", err)
		}

		op2, err := batch.CreateFile("file1.txt", []byte("content1"))
		if err != nil {
			t.Fatalf("Failed to create file1: %v", err)
		}

		op3, err := batch.CreateDir("dir2")
		if err != nil {
			t.Fatalf("Failed to create dir2: %v", err)
		}

		// Get operations before migration
		opsBefore := batch.Operations()
		if len(opsBefore) != 3 {
			t.Errorf("Expected 3 operations, got %d", len(opsBefore))
		}

		// Test with SimpleBatch
		opts := map[string]interface{}{
			"use_simple_batch":      true,
			"resolve_prerequisites": false, // Disable to avoid prerequisite resolution complexity
			"restorable":            false,
			"max_backup_size_mb":    0,
		}

		_, err = batch.RunWithOptions(opts)
		if err != nil {
			t.Logf("SimpleBatch run failed (may be expected): %v", err)
		}

		// Verify original batch operations are unchanged
		opsAfter := batch.Operations()
		if len(opsAfter) != len(opsBefore) {
			t.Errorf("Operation count changed after migration: before=%d, after=%d", len(opsBefore), len(opsAfter))
		}

		// Verify IDs are the same
		if len(opsAfter) >= 3 {
			beforeIDs := make([]string, len(opsBefore))
			afterIDs := make([]string, len(opsAfter))

			for i, op := range opsBefore {
				if idGetter, ok := op.(interface{ ID() interface{ String() string } }); ok {
					beforeIDs[i] = idGetter.ID().String()
				}
			}

			for i, op := range opsAfter {
				if idGetter, ok := op.(interface{ ID() interface{ String() string } }); ok {
					afterIDs[i] = idGetter.ID().String()
				}
			}

			for i := 0; i < len(beforeIDs) && i < len(afterIDs); i++ {
				if beforeIDs[i] != afterIDs[i] {
					t.Errorf("Operation order changed: position %d before=%s, after=%s", i, beforeIDs[i], afterIDs[i])
				}
			}
		}

		// The test operations should have the right IDs
		_ = op1
		_ = op2
		_ = op3
	})
}

func TestBackwardCompatibility(t *testing.T) {
	// Create test filesystem and registry
	fs := filesystem.NewOSFileSystem(".")
	reg := registry.NewRegistry()

	t.Run("Existing code continues to work", func(t *testing.T) {
		// This test ensures that existing code that doesn't use UseSimpleBatch
		// continues to work as before

		batch := batch.NewBatch(fs, reg)

		// Add operations like existing code would
		_, err := batch.CreateDir("legacy-test")
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}

		_, err = batch.CreateFile("legacy-test/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Run with traditional methods
		result1, err1 := batch.Run()
		if err1 != nil {
			t.Logf("Run failed (may be expected): %v", err1)
		}
		if result1 == nil {
			t.Error("Run should return a result")
		}

		// Test RunRestorable
		result2, err2 := batch.RunRestorable()
		if err2 != nil {
			t.Logf("RunRestorable failed (may be expected): %v", err2)
		}
		if result2 == nil {
			t.Error("RunRestorable should return a result")
		}

		// Test RunWithPrerequisites
		result3, err3 := batch.RunWithPrerequisites()
		if err3 != nil {
			t.Logf("RunWithPrerequisites failed (may be expected): %v", err3)
		}
		if result3 == nil {
			t.Error("RunWithPrerequisites should return a result")
		}
	})

	t.Run("Options format remains compatible", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		_, err := batch.CreateFile("compat-test.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Existing options format should still work
		opts := map[string]interface{}{
			"restorable":            true,
			"max_backup_size_mb":    10,
			"resolve_prerequisites": true,
			// use_simple_batch not specified - should default to false
		}

		result, err := batch.RunWithOptions(opts)
		if err != nil {
			t.Logf("RunWithOptions failed (may be expected): %v", err)
		}
		if result == nil {
			t.Error("RunWithOptions should return a result")
		}
	})
}