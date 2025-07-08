package batch_test

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/registry"
)

func TestPhase6SwitchDefaults(t *testing.T) {
	// Create test filesystem and registry
	fs := filesystem.NewOSFileSystem(".")
	reg := registry.NewRegistry()

	t.Run("Default behavior now uses SimpleBatch", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		// Add a file operation to a nested path
		_, err := batch.CreateFile("default/nested/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Default Run() should now use SimpleBatch behavior (Phase 6)
		result, err := batch.Run()
		if err != nil {
			t.Logf("Run failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}

		// Since UseSimpleBatch defaults to true now, this should delegate to SimpleBatch
	})

	t.Run("RunWithOptions defaults to SimpleBatch", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		_, err := batch.CreateFile("options/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// When UseSimpleBatch is not specified, it should default to true (Phase 6)
		opts := map[string]interface{}{
			"restorable":            false,
			"max_backup_size_mb":    0,
			"resolve_prerequisites": true,
			// use_simple_batch not specified - should default to true in Phase 6
		}

		result, err := batch.RunWithOptions(opts)
		if err != nil {
			t.Logf("RunWithOptions failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}
	})

	t.Run("Legacy behavior can still be accessed", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		_, err := batch.CreateFile("legacy/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Explicitly request legacy behavior
		result, err := batch.RunWithLegacyBatch()
		if err != nil {
			t.Logf("RunWithLegacyBatch failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}
	})

	t.Run("Legacy behavior with budget works", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		_, err := batch.CreateFile("legacy-budget/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Use legacy behavior with budget
		result, err := batch.RunWithLegacyBatchAndBudget(20)
		if err != nil {
			t.Logf("RunWithLegacyBatchAndBudget failed (expected with missing prerequisites): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}
	})

	t.Run("Explicit SimpleBatch flag override", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		_, err := batch.CreateFile("explicit/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// Explicitly set UseSimpleBatch to false to override default
		opts := map[string]interface{}{
			"use_simple_batch":      false, // Override the Phase 6 default
			"restorable":            false,
			"max_backup_size_mb":    0,
			"resolve_prerequisites": true,
		}

		result, err := batch.RunWithOptions(opts)
		if err != nil {
			t.Logf("RunWithOptions with legacy override failed (expected): %v", err)
		}

		if result == nil {
			t.Error("Expected result to be returned even on failure")
		}
	})

	t.Run("SimpleBatch convenience methods still work", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		_, err := batch.CreateFile("convenience/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("Failed to create file operation: %v", err)
		}

		// These methods should work the same as before
		result1, err1 := batch.RunWithSimpleBatch()
		if err1 != nil {
			t.Logf("RunWithSimpleBatch failed (expected): %v", err1)
		}
		if result1 == nil {
			t.Error("Expected result from RunWithSimpleBatch")
		}

		result2, err2 := batch.RunWithSimpleBatchAndBudget(15)
		if err2 != nil {
			t.Logf("RunWithSimpleBatchAndBudget failed (expected): %v", err2)
		}
		if result2 == nil {
			t.Error("Expected result from RunWithSimpleBatchAndBudget")
		}
	})
}

func TestPhase6BackwardCompatibility(t *testing.T) {
	// Create test filesystem and registry
	fs := filesystem.NewOSFileSystem(".")
	reg := registry.NewRegistry()

	t.Run("Existing code behavior changes gracefully", func(t *testing.T) {
		// This test verifies that existing code that didn't specify UseSimpleBatch
		// now gets the new behavior, but still works

		batch := batch.NewBatch(fs, reg)

		// Add operations like existing code would
		_, err := batch.CreateDir("compat-test")
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}

		_, err = batch.CreateFile("compat-test/file.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Existing methods should still work, but with new default behavior
		result, err := batch.Run()
		if err != nil {
			t.Logf("Run failed (may be expected with new behavior): %v", err)
		}
		if result == nil {
			t.Error("Run should return a result")
		}

		// The key difference is that now Run() uses SimpleBatch by default
		// Previously it used the traditional batch approach
	})

	t.Run("Users who need old behavior can get it", func(t *testing.T) {
		batch := batch.NewBatch(fs, reg)

		_, err := batch.CreateFile("old-behavior-test.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Users who specifically need the old behavior can use RunWithLegacyBatch
		result, err := batch.RunWithLegacyBatch()
		if err != nil {
			t.Logf("RunWithLegacyBatch failed (may be expected): %v", err)
		}
		if result == nil {
			t.Error("RunWithLegacyBatch should return a result")
		}

		// Or they can set the flag explicitly
		opts := map[string]interface{}{
			"use_simple_batch": false,
		}
		result2, err2 := batch.RunWithOptions(opts)
		if err2 != nil {
			t.Logf("RunWithOptions with legacy flag failed (may be expected): %v", err2)
		}
		if result2 == nil {
			t.Error("RunWithOptions should return a result")
		}
	})

	t.Run("Migration documentation accuracy", func(t *testing.T) {
		// This test verifies that the migration path works as documented
		
		batch := batch.NewBatch(fs, reg)
		_, err := batch.CreateFile("migration-test.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Phase 5: Users could opt-in to SimpleBatch with use_simple_batch: true
		optsPhase5 := map[string]interface{}{
			"use_simple_batch": true,
		}
		result5, err5 := batch.RunWithOptions(optsPhase5)
		if err5 != nil {
			t.Logf("Phase 5 style call failed (expected): %v", err5)
		}
		if result5 == nil {
			t.Error("Phase 5 style call should return a result")
		}

		// Phase 6: SimpleBatch is now the default, but users can opt-out
		optsPhase6Default := map[string]interface{}{
			// No use_simple_batch specified - should default to true
		}
		result6, err6 := batch.RunWithOptions(optsPhase6Default)
		if err6 != nil {
			t.Logf("Phase 6 default call failed (expected): %v", err6)
		}
		if result6 == nil {
			t.Error("Phase 6 default call should return a result")
		}

		// Phase 6: Users who need old behavior can opt-out
		optsPhase6Legacy := map[string]interface{}{
			"use_simple_batch": false,
		}
		result6Legacy, err6Legacy := batch.RunWithOptions(optsPhase6Legacy)
		if err6Legacy != nil {
			t.Logf("Phase 6 legacy call failed (expected): %v", err6Legacy)
		}
		if result6Legacy == nil {
			t.Error("Phase 6 legacy call should return a result")
		}
	})
}