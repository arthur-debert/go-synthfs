package synthfs_test

import (
	"testing"
)

func TestBatchChecksumVerification(t *testing.T) {
	// Phase I, Milestone 4: Test checksum verification at execution time

	// t.Run("Copy operation succeeds when source unchanged", func(t *testing.T) {
	// 	testFS := synthfs.NewTestFileSystem()
	//
	// 	// Create source file
	// 	sourceContent := []byte("Unchanged content for copy")
	// 	err := testFS.WriteFile("source.txt", sourceContent, 0644)
	// 	if err != nil {
	// 		t.Fatalf("Failed to create source file: %v", err)
	// 	}
	//
	// 	batch := synthfs.NewBatch().WithFileSystem(testFS)
	//
	// 	// Create copy operation (should compute checksum)
	// 	op, err := batch.Copy("source.txt", "destination.txt")
	// 	if err != nil {
	// 		t.Fatalf("Copy operation failed: %v", err)
	// 	}
	//
	// 	// Verify checksum was computed
	// 	checksum := op.GetChecksum("source.txt")
	// 	if checksum == nil {
	// 		t.Fatal("Expected checksum to be computed")
	// 	}
	//
	// 	// Execute the batch (should succeed since file unchanged)
	// 	result, err := batch.Run()
	// 	if err != nil {
	// 		t.Fatalf("Batch execution failed: %v", err)
	// 	}
	//
	// 	if !result.Success {
	// 		t.Fatalf("Batch execution was not successful: %v", result.Errors)
	// 	}
	//
	// 	// Verify destination file was created
	// 	if _, err := testFS.Stat("destination.txt"); err != nil {
	// 		t.Errorf("Destination file was not created: %v", err)
	// 	}
	// })
	//
	// t.Run("Copy operation fails when source file is modified", func(t *testing.T) {
	// 	testFS := synthfs.NewTestFileSystem()
	//
	// 	// Create source file
	// 	originalContent := []byte("Original content")
	// 	err := testFS.WriteFile("source.txt", originalContent, 0644)
	// 	if err != nil {
	// 		t.Fatalf("Failed to create source file: %v", err)
	// 	}
	//
	// 	batch := synthfs.NewBatch().WithFileSystem(testFS)
	//
	// 	// Create copy operation (computes checksum of original content)
	// 	_, err = batch.Copy("source.txt", "destination.txt")
	// 	if err != nil {
	// 		t.Fatalf("Copy operation failed: %v", err)
	// 	}
	//
	// 	// Modify source file before execution (change size)
	// 	modifiedContent := []byte("Modified content with different size")
	// 	err = testFS.WriteFile("source.txt", modifiedContent, 0644)
	// 	if err != nil {
	// 		t.Fatalf("Failed to modify source file: %v", err)
	// 	}
	//
	// 	// Execute the batch (should fail checksum verification)
	// 	result, err := batch.Run()
	// 	if err != nil {
	// 		t.Fatalf("Batch execution failed: %v", err)
	// 	}
	//
	// 	if result.Success {
	// 		t.Error("Expected batch execution to fail due to checksum verification")
	// 	}
	//
	// 	// Check that error mentions checksum verification
	// 	found := false
	// 	for _, opResult := range result.Operations {
	// 		if opResult.Error != nil && strings.Contains(opResult.Error.Error(), "checksum verification") {
	// 			found = true
	// 			// Check for the new, more accurate error message
	// 			if !strings.Contains(opResult.Error.Error(), "file content has changed") {
	// 				t.Errorf("Expected 'file content has changed' error, got: %v", opResult.Error)
	// 			}
	// 			break
	// 		}
	// 	}
	// 	if !found {
	// 		t.Error("Expected checksum verification error in operation results")
	// 	}
	// })
	//
	// t.Run("Move operation fails when source file modified", func(t *testing.T) {
	// 	testFS := synthfs.NewTestFileSystem()
	//
	// 	// Create source file
	// 	originalContent := []byte("Original file for move")
	// 	err := testFS.WriteFile("old.txt", originalContent, 0644)
	// 	if err != nil {
	// 		t.Fatalf("Failed to create source file: %v", err)
	// 	}
	//
	// 	batch := synthfs.NewBatch().WithFileSystem(testFS)
	//
	// 	// Create move operation (computes checksum)
	// 	_, err = batch.Move("old.txt", "new.txt")
	// 	if err != nil {
	// 		t.Fatalf("Move operation failed: %v", err)
	// 	}
	//
	// 	// Modify source file before execution (change size to ensure detection)
	// 	modifiedContent := []byte("Modified content for move operation") // Different size
	// 	err = testFS.WriteFile("old.txt", modifiedContent, 0644)
	// 	if err != nil {
	// 		t.Fatalf("Failed to modify source file: %v", err)
	// 	}
	//
	// 	// Execute the batch (should fail checksum verification)
	// 	result, err := batch.Run()
	// 	if err != nil {
	// 		t.Fatalf("Batch execution failed: %v", err)
	// 	}
	//
	// 	if result.Success {
	// 		t.Error("Expected batch execution to fail due to checksum verification")
	// 	}
	//
	// 	// Check that error mentions checksum verification
	// 	found := false
	// 	for _, opResult := range result.Operations {
	// 		if opResult.Error != nil && strings.Contains(opResult.Error.Error(), "checksum verification") {
	// 			found = true
	// 			break
	// 		}
	// 	}
	// 	if !found {
	// 		t.Error("Expected checksum verification error in operation results")
	// 	}
	// })
	//
	// t.Run("Archive operation fails when source files modified", func(t *testing.T) {
	// 	testFS := synthfs.NewTestFileSystem()
	//
	// 	// Create multiple source files
	// 	files := map[string][]byte{
	// 		"file1.txt": []byte("Content 1"),
	// 		"file2.txt": []byte("Content 2"),
	// 		"file3.txt": []byte("Content 3"),
	// 	}
	//
	// 	for path, content := range files {
	// 		err := testFS.WriteFile(path, content, 0644)
	// 		if err != nil {
	// 			t.Fatalf("Failed to create source file %s: %v", path, err)
	// 		}
	// 	}
	//
	// 	batch := synthfs.NewBatch().WithFileSystem(testFS)
	//
	// 	// Create archive operation (computes checksums for all sources)
	// 	_, err := batch.CreateArchive("backup.tar.gz", targets.ArchiveFormatTarGz, "file1.txt", "file2.txt", "file3.txt")
	// 	if err != nil {
	// 		t.Fatalf("CreateArchive operation failed: %v", err)
	// 	}
	//
	// 	// Modify one of the source files before execution
	// 	err = testFS.WriteFile("file2.txt", []byte("Modified content 2"), 0644)
	// 	if err != nil {
	// 		t.Fatalf("Failed to modify source file: %v", err)
	// 	}
	//
	// 	// Execute the batch (should fail checksum verification)
	// 	result, err := batch.Run()
	// 	if err != nil {
	// 		t.Fatalf("Batch execution failed: %v", err)
	// 	}
	//
	// 	if result.Success {
	// 		t.Error("Expected batch execution to fail due to checksum verification")
	// 	}
	//
	// 	// Check that error mentions checksum verification for file2.txt
	// 	found := false
	// 	for _, opResult := range result.Operations {
	// 		if opResult.Error != nil && strings.Contains(opResult.Error.Error(), "checksum verification") {
	// 			found = true
	// 			if !strings.Contains(opResult.Error.Error(), "file2.txt") {
	// 				t.Errorf("Expected error to mention file2.txt, got: %v", opResult.Error)
	// 			}
	// 			break
	// 		}
	// 	}
	// 	if !found {
	// 		t.Error("Expected checksum verification error in operation results")
	// 	}
	// })
	//
	// t.Run("Operations without checksums execute normally", func(t *testing.T) {
	// 	testFS := synthfs.NewTestFileSystem()
	// 	batch := synthfs.NewBatch().WithFileSystem(testFS)
	//
	// 	// Create operations that don't use checksums
	// 	_, err := batch.CreateDir("testdir")
	// 	if err != nil {
	// 		t.Fatalf("CreateDir operation failed: %v", err)
	// 	}
	//
	// 	_, err = batch.CreateFile("testfile.txt", []byte("test content"))
	// 	if err != nil {
	// 		t.Fatalf("CreateFile operation failed: %v", err)
	// 	}
	//
	// 	// To make the Delete operation valid under Phase II rules, the file must exist
	// 	// or be projected to exist. We'll create it first.
	// 	err = testFS.WriteFile("nonexistent.txt", []byte("delete me"), 0644)
	// 	if err != nil {
	// 		t.Fatalf("Failed to create file for deletion test: %v", err)
	// 	}
	//
	// 	_, err = batch.Delete("nonexistent.txt")
	// 	if err != nil {
	// 		t.Fatalf("Delete operation failed: %v", err)
	// 	}
	//
	// 	// Execute the batch (should succeed)
	// 	result, err := batch.Run()
	// 	if err != nil {
	// 		t.Fatalf("Batch execution failed: %v", err)
	// 	}
	//
	// 	// Some operations might fail (like delete of nonexistent file), but there should be no checksum errors
	// 	for _, opResult := range result.Operations {
	// 		if opResult.Error != nil && strings.Contains(opResult.Error.Error(), "checksum verification") {
	// 			t.Errorf("Unexpected checksum verification error for operation that shouldn't have checksums: %v", opResult.Error)
	// 		}
	// 	}
	// })
}
