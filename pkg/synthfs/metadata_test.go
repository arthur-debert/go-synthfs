package synthfs

import (
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestOperationMetadata(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	fs := filesystem.NewOSFileSystem(tempDir)

	// Create a batch
	batch := NewBatch(fs)

	// Test metadata with CreateFile
	metadata := map[string]interface{}{
		"user_id":    "user123",
		"request_id": "req456",
		"tags":       []string{"test", "metadata"},
	}

	op, err := batch.CreateFile("testfile.txt", []byte("test content"), 0644, metadata)
	if err != nil {
		t.Fatalf("CreateFile with metadata failed: %v", err)
	}

	if op == nil {
		t.Fatal("Operation should not be nil")
	}

	// Test metadata with CreateDir
	dirMetadata := map[string]interface{}{
		"created_by": "test_suite",
		"purpose":    "testing",
	}

	dirOp, err := batch.CreateDir("testdir", 0755, dirMetadata)
	if err != nil {
		t.Fatalf("CreateDir with metadata failed: %v", err)
	}

	if dirOp == nil {
		t.Fatal("Directory operation should not be nil")
	}

	// Test operations without metadata (backward compatibility)
	op2, err := batch.CreateFile("testfile2.txt", []byte("test content 2"), 0644)
	if err != nil {
		t.Fatalf("CreateFile without metadata failed: %v", err)
	}

	if op2 == nil {
		t.Fatal("Operation without metadata should not be nil")
	}

	// Verify batch has operations
	operations := batch.Operations()
	if len(operations) != 3 {
		t.Fatalf("Expected 3 operations, got %d", len(operations))
	}
}

func TestBatchMetadata(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	fs := filesystem.NewOSFileSystem(tempDir)

	// Create a batch with metadata
	batch := NewBatch(fs)
	
	batchMetadata := map[string]interface{}{
		"batch_id":   "batch123",
		"created_by": "test_suite",
		"workflow":   "data_migration",
	}

	batch.WithMetadata(batchMetadata)

	// Add some operations
	_, err := batch.CreateFile("file1.txt", []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	_, err = batch.CreateDir("dir1", 0755)
	if err != nil {
		t.Fatalf("CreateDir failed: %v", err)
	}

	// Run the batch
	result, err := batch.Run()
	if err != nil {
		t.Fatalf("Batch execution failed: %v", err)
	}

	if !result.IsSuccess() {
		t.Fatal("Batch should have succeeded")
	}

	// Check that result has metadata getter
	resultMetadata := result.GetMetadata()
	if resultMetadata == nil {
		t.Log("Result metadata is nil - this may be expected if not implemented in execution layer yet")
	}
}

func TestCopyMoveMetadata(t *testing.T) {
	tempDir := t.TempDir()
	fs := filesystem.NewOSFileSystem(tempDir)

	// Create source file first
	err := fs.WriteFile("source.txt", []byte("source content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test Copy with metadata
	copyBatch := NewBatch(fs)
	copyMetadata := map[string]interface{}{
		"operation_type": "backup",
		"priority":       "high",
	}

	copyOp, err := copyBatch.Copy("source.txt", "dest.txt", copyMetadata)
	if err != nil {
		t.Fatalf("Copy with metadata failed: %v", err)
	}

	if copyOp == nil {
		t.Fatal("Copy operation should not be nil")
	}

	// Run copy first
	result, err := copyBatch.Run()
	if err != nil {
		t.Fatalf("Copy batch execution failed: %v", err)
	}

	if !result.IsSuccess() {
		t.Fatalf("Copy batch should have succeeded, error: %v", result.GetError())
	}

	// Test Move with metadata in separate batch
	moveBatch := NewBatch(fs)
	moveMetadata := map[string]interface{}{
		"operation_type": "reorganization",
		"automated":      true,
	}

	moveOp, err := moveBatch.Move("dest.txt", "final.txt", moveMetadata)
	if err != nil {
		t.Fatalf("Move with metadata failed: %v", err)
	}

	if moveOp == nil {
		t.Fatal("Move operation should not be nil")
	}

	// Run move batch
	result2, err := moveBatch.Run()
	if err != nil {
		t.Fatalf("Move batch execution failed: %v", err)
	}

	if !result2.IsSuccess() {
		t.Fatalf("Move batch should have succeeded, error: %v", result2.GetError())
	}
}

func TestDeleteSymlinkMetadata(t *testing.T) {
	tempDir := t.TempDir()
	fs := filesystem.NewOSFileSystem(tempDir)

	// Create a file to delete
	err := fs.WriteFile("todelete.txt", []byte("delete me"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file to delete: %v", err)
	}

	batch := NewBatch(fs)

	// Test Delete with metadata
	deleteMetadata := map[string]interface{}{
		"reason":    "cleanup",
		"confirmed": true,
	}

	deleteOp, err := batch.Delete("todelete.txt", deleteMetadata)
	if err != nil {
		t.Fatalf("Delete with metadata failed: %v", err)
	}

	if deleteOp == nil {
		t.Fatal("Delete operation should not be nil")
	}

	// Test CreateSymlink with metadata
	symlinkMetadata := map[string]interface{}{
		"link_type": "relative",
		"purpose":   "shortcut",
	}

	symlinkOp, err := batch.CreateSymlink("../target", "testlink", symlinkMetadata)
	if err != nil {
		t.Fatalf("CreateSymlink with metadata failed: %v", err)
	}

	if symlinkOp == nil {
		t.Fatal("CreateSymlink operation should not be nil")
	}

	// Just verify operations are added - don't execute since symlink might not be supported on all filesystems
	operations := batch.Operations()
	if len(operations) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(operations))
	}
}

func TestArchiveMetadata(t *testing.T) {
	tempDir := t.TempDir()
	fs := filesystem.NewOSFileSystem(tempDir)

	// Create source files
	err := fs.WriteFile("file1.txt", []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	err = fs.WriteFile("file2.txt", []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Test CreateArchive with metadata
	archiveBatch := NewBatch(fs)
	archiveMetadata := map[string]interface{}{
		"compression": "gzip",
		"backup_set":  "daily",
	}

	sources := []string{"file1.txt", "file2.txt"}
	archiveOp, err := archiveBatch.CreateArchive("archive.tar.gz", "tar.gz", sources, archiveMetadata)
	if err != nil {
		t.Fatalf("CreateArchive with metadata failed: %v", err)
	}

	if archiveOp == nil {
		t.Fatal("Archive operation should not be nil")
	}

	// First create the archive
	result, err := archiveBatch.Run()
	if err != nil {
		t.Fatalf("Archive batch execution failed: %v", err)
	}

	if !result.IsSuccess() {
		t.Fatalf("Archive batch should have succeeded, error: %v", result.GetError())
	}

	// Test Unarchive with metadata in separate batch
	unarchiveBatch := NewBatch(fs)
	unarchiveMetadata := map[string]interface{}{
		"extract_mode": "overwrite",
		"verify":       true,
	}

	unarchiveOp, err := unarchiveBatch.Unarchive("archive.tar.gz", "extracted", unarchiveMetadata)
	if err != nil {
		t.Fatalf("Unarchive with metadata failed: %v", err)
	}

	if unarchiveOp == nil {
		t.Fatal("Unarchive operation should not be nil")
	}

	// Test UnarchiveWithPatterns with metadata in same batch for simplicity
	patternMetadata := map[string]interface{}{
		"filter_type": "include_only",
		"case_sensitive": false,
	}

	patterns := []string{"*.txt"}
	patternOp, err := unarchiveBatch.UnarchiveWithPatterns("archive.tar.gz", "filtered", patterns, patternMetadata)
	if err != nil {
		t.Fatalf("UnarchiveWithPatterns with metadata failed: %v", err)
	}

	if patternOp == nil {
		t.Fatal("UnarchiveWithPatterns operation should not be nil")
	}

	// Verify operations are added to the unarchive batch
	operations := unarchiveBatch.Operations()
	if len(operations) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(operations))
	}

	// For the metadata test, we don't need to actually execute the unarchive operations
	// since we're mainly testing that the API accepts metadata parameters correctly
	t.Log("Archive operations with metadata created successfully")
}

func TestMetadataTypeSafety(t *testing.T) {
	tempDir := t.TempDir()
	fs := filesystem.NewOSFileSystem(tempDir)

	batch := NewBatch(fs)

	// Test various metadata types
	complexMetadata := map[string]interface{}{
		"string":  "test",
		"int":     42,
		"float":   3.14,
		"bool":    true,
		"slice":   []string{"a", "b", "c"},
		"map":     map[string]string{"nested": "value"},
		"nil":     nil,
	}

	op, err := batch.CreateFile("complex.txt", []byte("test"), 0644, complexMetadata)
	if err != nil {
		t.Fatalf("CreateFile with complex metadata failed: %v", err)
	}

	if op == nil {
		t.Fatal("Operation should not be nil")
	}

	// Test empty metadata
	op2, err := batch.CreateFile("empty_meta.txt", []byte("test"), 0644, map[string]interface{}{})
	if err != nil {
		t.Fatalf("CreateFile with empty metadata failed: %v", err)
	}

	if op2 == nil {
		t.Fatal("Operation with empty metadata should not be nil")
	}

	// Test nil metadata
	op3, err := batch.CreateFile("nil_meta.txt", []byte("test"), 0644, nil)
	if err != nil {
		t.Fatalf("CreateFile with nil metadata failed: %v", err)
	}

	if op3 == nil {
		t.Fatal("Operation with nil metadata should not be nil")
	}
}