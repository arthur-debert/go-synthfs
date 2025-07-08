package batch_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/batch"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestSimpleBatch(t *testing.T) {
	fs := filesystem.NewOSFileSystem(".")
	registry := operations.NewFactory()
	
	t.Run("NewSimpleBatch creates batch", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		if sb == nil {
			t.Fatal("NewSimpleBatch should return a non-nil batch")
		}
		
		ops := sb.Operations()
		if len(ops) != 0 {
			t.Errorf("Expected 0 operations, got %d", len(ops))
		}
	})
	
	t.Run("SimpleBatch can add operations", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		// Add a file creation operation
		op, err := sb.CreateFile("test.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("CreateFile should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("SimpleBatch can chain configuration", func(t *testing.T) {
		testFS := testutil.NewMockFS()
		logger := &testutil.TestLogger{}
		
		sb := batch.NewSimpleBatch(fs, registry).
			WithFileSystem(testFS).
			WithContext(context.Background()).
			WithLogger(logger)
		
		if sb == nil {
			t.Fatal("Chained configuration should return a non-nil batch")
		}
		
		// Test that we can still add operations
		_, err := sb.CreateFile("test.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("CreateFile failed after chaining: %v", err)
		}
	})
	
	t.Run("SimpleBatch creates operations without parent directories", func(t *testing.T) {
		testFS := testutil.NewMockFS()
		sb := batch.NewSimpleBatch(testFS, registry)
		
		// Create a file in a nested directory without creating parent first
		op, err := sb.CreateFile("deep/nested/path/test.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("CreateFile should not fail for nested path: %v", err)
		}
		
		if op == nil {
			t.Fatal("CreateFile should return a non-nil operation")
		}
		
		// SimpleBatch should not create parent directories automatically
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation (no parent dir creation), got %d", len(ops))
		}
		
		// Verify it's a create_file operation
		if opImpl, ok := ops[0].(interface{ Describe() core.OperationDesc }); ok {
			desc := opImpl.Describe()
			if desc.Type != "create_file" {
				t.Errorf("Expected create_file operation, got %s", desc.Type)
			}
			if desc.Path != "deep/nested/path/test.txt" {
				t.Errorf("Expected path 'deep/nested/path/test.txt', got %s", desc.Path)
			}
		} else {
			t.Error("Operation should implement Describe method")
		}
	})
	
	t.Run("SimpleBatch Run() enables prerequisite resolution by default", func(t *testing.T) {
		testFS := testutil.NewMockFS()
		sb := batch.NewSimpleBatch(testFS, registry)
		
		// Create a file in a nested directory
		_, err := sb.CreateFile("parent/child.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		
		// Run should enable prerequisite resolution and create parent directories
		result, err := sb.Run()
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("Run should return a non-nil result")
		}
		
		// Check if result implements expected interface
		if resultImpl, ok := result.(interface{ IsSuccess() bool }); ok {
			if !resultImpl.IsSuccess() {
				t.Error("Run should succeed with prerequisite resolution")
			}
		} else {
			t.Error("Result should implement IsSuccess method")
		}
	})
}

func TestSimpleBatchOperations(t *testing.T) {
	fs := filesystem.NewOSFileSystem(".")
	registry := operations.NewFactory()
	
	t.Run("CreateDir", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		op, err := sb.CreateDir("testdir", 0755)
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("CreateDir should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("Copy", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		op, err := sb.Copy("src.txt", "dst.txt")
		if err != nil {
			t.Fatalf("Copy failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("Copy should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("Move", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		op, err := sb.Move("src.txt", "dst.txt")
		if err != nil {
			t.Fatalf("Move failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("Move should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("Delete", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		op, err := sb.Delete("file.txt")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("Delete should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("CreateSymlink", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		op, err := sb.CreateSymlink("target.txt", "link.txt")
		if err != nil {
			t.Fatalf("CreateSymlink failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("CreateSymlink should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("CreateArchive", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		op, err := sb.CreateArchive("archive.tar.gz", core.ArchiveFormatTarGz, "file1.txt", "file2.txt")
		if err != nil {
			t.Fatalf("CreateArchive failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("CreateArchive should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("Unarchive", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		op, err := sb.Unarchive("archive.tar.gz", "extracted/")
		if err != nil {
			t.Fatalf("Unarchive failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("Unarchive should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
	
	t.Run("UnarchiveWithPatterns", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		op, err := sb.UnarchiveWithPatterns("archive.tar.gz", "extracted/", "*.txt", "*.md")
		if err != nil {
			t.Fatalf("UnarchiveWithPatterns failed: %v", err)
		}
		
		if op == nil {
			t.Fatal("UnarchiveWithPatterns should return a non-nil operation")
		}
		
		ops := sb.Operations()
		if len(ops) != 1 {
			t.Errorf("Expected 1 operation, got %d", len(ops))
		}
	})
}

func TestSimpleBatchExecution(t *testing.T) {
	fs := filesystem.NewOSFileSystem(".")
	registry := operations.NewFactory()
	
	t.Run("Run with empty batch", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		result, err := sb.Run()
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("Run should return a non-nil result")
		}
		
		// Check success
		if resultImpl, ok := result.(interface{ IsSuccess() bool }); ok {
			if !resultImpl.IsSuccess() {
				t.Error("Empty batch should succeed")
			}
		}
	})
	
	t.Run("RunWithOptions", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		opts := map[string]interface{}{
			"restorable":            true,
			"max_backup_size_mb":    5,
			"resolve_prerequisites": true,
		}
		
		result, err := sb.RunWithOptions(opts)
		if err != nil {
			t.Fatalf("RunWithOptions failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("RunWithOptions should return a non-nil result")
		}
	})
	
	t.Run("RunRestorable", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		result, err := sb.RunRestorable()
		if err != nil {
			t.Fatalf("RunRestorable failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("RunRestorable should return a non-nil result")
		}
	})
	
	t.Run("RunRestorableWithBudget", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		result, err := sb.RunRestorableWithBudget(20)
		if err != nil {
			t.Fatalf("RunRestorableWithBudget failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("RunRestorableWithBudget should return a non-nil result")
		}
	})
	
	t.Run("RunWithPrerequisites", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		result, err := sb.RunWithPrerequisites()
		if err != nil {
			t.Fatalf("RunWithPrerequisites failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("RunWithPrerequisites should return a non-nil result")
		}
	})
	
	t.Run("RunWithPrerequisitesAndBudget", func(t *testing.T) {
		sb := batch.NewSimpleBatch(fs, registry)
		
		result, err := sb.RunWithPrerequisitesAndBudget(15)
		if err != nil {
			t.Fatalf("RunWithPrerequisitesAndBudget failed: %v", err)
		}
		
		if result == nil {
			t.Fatal("RunWithPrerequisitesAndBudget should return a non-nil result")
		}
	})
}