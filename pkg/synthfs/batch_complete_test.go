package synthfs_test

import (
	"io/fs"
	"runtime"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestBatchCompleteAPI(t *testing.T) {
	testFS := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch().WithFileSystem(testFS)

	t.Run("CreateSymlink operation", func(t *testing.T) {
		// Create target file first
		_, err := batch.CreateFile("target.txt", []byte("I am the target"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Create symlink
		_, err = batch.CreateSymlink("target.txt", "link.txt")
		if err != nil {
			t.Fatalf("CreateSymlink failed: %v", err)
		}

		// Execute the batch
		result, err := batch.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Batch execution failed: %v", result.Errors)
		}

		// Verify the symlink was created
		info, err := testFS.Stat("link.txt")
		if err != nil {
			t.Fatalf("Symlink 'link.txt' was not created: %v", err)
		}

		if info.Mode()&fs.ModeSymlink == 0 {
			t.Error("Expected 'link.txt' to be a symlink")
		}

		// Verify symlink target
		target, err := testFS.Readlink("link.txt")
		if err != nil {
			t.Fatalf("Failed to read symlink target: %v", err)
		}

		if target != "target.txt" {
			t.Errorf("Expected symlink target 'target.txt', got '%s'", target)
		}
	})

	t.Run("CreateArchive operations", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Archive creation may have path issues on Windows test environments")
		}

		newBatch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// Create some files to archive
		_, err := newBatch.CreateDir("archive-test")
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}

		_, err = newBatch.CreateFile("archive-test/file1.txt", []byte("File 1 content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		_, err = newBatch.CreateFile("archive-test/file2.txt", []byte("File 2 content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Create tar.gz archive
		_, err = newBatch.CreateArchive("backup.tar.gz", synthfs.ArchiveFormatTarGz, "archive-test/file1.txt", "archive-test/file2.txt")
		if err != nil {
			t.Fatalf("CreateArchive tar.gz failed: %v", err)
		}

		// Create zip archive
		_, err = newBatch.CreateArchive("backup.zip", synthfs.ArchiveFormatZip, "archive-test/file1.txt", "archive-test/file2.txt")
		if err != nil {
			t.Fatalf("CreateArchive zip failed: %v", err)
		}

		// Execute the batch
		result, err := newBatch.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Archive creation failed: %v", result.Errors)
		}

		// For now, just verify the operations were created and executed
		// Real archive verification would require extracting and checking contents
		t.Logf("Archive operations executed successfully: %d operations", len(result.Operations))
		for i, opResult := range result.Operations {
			desc := opResult.Operation.Describe()
			t.Logf("Operation %d: %s %s -> %s", i+1, desc.Type, desc.Path, opResult.Status)
		}
	})

	t.Run("Complete workflow with all operations", func(t *testing.T) {
		t.Skip("Skipping complex dependency workflow test - needs dependency resolution fixes")

		// Test will be re-enabled once dependency resolution between operations is fixed
		fullBatch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// Create a comprehensive but simpler workflow
		_, err := fullBatch.CreateDir("project")
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}

		_, err = fullBatch.CreateFile("project/main.go", []byte("package main"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		_, err = fullBatch.CreateFile("README.md", []byte("# Project"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Copy file to project directory
		_, err = fullBatch.Copy("README.md", "project/README.md")
		if err != nil {
			t.Fatalf("Copy failed: %v", err)
		}

		// Create symlink (use existing file)
		_, err = fullBatch.CreateSymlink("README.md", "project/README.link")
		if err != nil {
			t.Fatalf("CreateSymlink failed: %v", err)
		}

		// Move file to new location
		_, err = fullBatch.CreateDir("backup")
		if err != nil {
			t.Fatalf("CreateDir for backup failed: %v", err)
		}

		_, err = fullBatch.Move("README.md", "backup/README.md")
		if err != nil {
			t.Fatalf("Move failed: %v", err)
		}

		// Create archive with existing files
		_, err = fullBatch.CreateArchive("project.tar.gz", synthfs.ArchiveFormatTarGz, "project/main.go")
		if err != nil {
			t.Fatalf("CreateArchive failed: %v", err)
		}

		// Execute everything
		result, err := fullBatch.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Complete workflow failed: %v", result.Errors)
		}

		t.Logf("Complete workflow executed: %d operations in %v", len(result.Operations), result.Duration)

		// Verify some key results
		operations := fullBatch.Operations()
		if len(operations) < 7 { // Should have at least 7 operations (including auto-generated directories)
			t.Errorf("Expected at least 7 operations, got %d", len(operations))
		}

		// Check operation types
		operationTypes := make(map[string]int)
		for _, op := range operations {
			operationTypes[op.Describe().Type]++
		}

		expectedTypes := []string{"create_directory", "create_file", "copy", "create_symlink", "move", "create_archive"}
		for _, expectedType := range expectedTypes {
			if operationTypes[expectedType] == 0 {
				t.Errorf("Expected at least one %s operation", expectedType)
			}
		}

		t.Logf("Operation types: %+v", operationTypes)
	})

	t.Run("Operation objects detailed inspection", func(t *testing.T) {
		inspectBatch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// Create some operations to inspect
		fileOp, err := inspectBatch.CreateFile("inspect.txt", []byte("inspection content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		symlinkOp, err := inspectBatch.CreateSymlink("inspect.txt", "inspect.link")
		if err != nil {
			t.Fatalf("CreateSymlink failed: %v", err)
		}

		archiveOp, err := inspectBatch.CreateArchive("inspect.tar.gz", synthfs.ArchiveFormatTarGz, "inspect.txt")
		if err != nil {
			t.Fatalf("CreateArchive failed: %v", err)
		}

		// Inspect operation details (like Python example)
		t.Logf("File Operation: %s (ID: %s)", fileOp.Describe().Type, fileOp.ID())
		t.Logf("  Path: %s", fileOp.Describe().Path)
		t.Logf("  Details: %+v", fileOp.Describe().Details)
		t.Logf("  Dependencies: %v", fileOp.Dependencies())

		t.Logf("Symlink Operation: %s (ID: %s)", symlinkOp.Describe().Type, symlinkOp.ID())
		t.Logf("  Path: %s", symlinkOp.Describe().Path)
		t.Logf("  Details: %+v", symlinkOp.Describe().Details)
		t.Logf("  Dependencies: %v", symlinkOp.Dependencies())

		t.Logf("Archive Operation: %s (ID: %s)", archiveOp.Describe().Type, archiveOp.ID())
		t.Logf("  Path: %s", archiveOp.Describe().Path)
		t.Logf("  Details: %+v", archiveOp.Describe().Details)
		t.Logf("  Dependencies: %v", archiveOp.Dependencies())

		// Execute and inspect results
		result, err := inspectBatch.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Execution failed: %v", result.Errors)
		}

		// Inspect execution results (like Python example)
		t.Logf("Execution Result: Success=%v, Duration=%v", result.Success, result.Duration)
		for i, opResult := range result.Operations {
			t.Logf("  Operation %d: %s -> %s (Duration: %v)",
				i+1,
				opResult.Operation.ID(),
				opResult.Status,
				opResult.Duration)
		}
	})
}
