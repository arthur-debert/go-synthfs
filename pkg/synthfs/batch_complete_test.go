package synthfs_test

import (
	"io/fs"
	"runtime"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

func TestBatchCompleteAPI(t *testing.T) {
	testFS := testutil.NewTestFileSystem()
	fileSystem := testutil.NewTestFileSystem()
	batch := synthfs.NewBatch(fileSystem).WithFileSystem(testFS)

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
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Fatalf("Batch execution failed: %v", result.GetError())
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

		testFS := testutil.NewTestFileSystem()
		fileSystem := testutil.NewTestFileSystem()
		setupBatch := synthfs.NewBatch(fileSystem).WithFileSystem(testFS)

		// Create some files to archive
		_, err := setupBatch.CreateDir("archive-test")
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}

		_, err = setupBatch.CreateFile("archive-test/file1.txt", []byte("File 1 content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		_, err = setupBatch.CreateFile("archive-test/file2.txt", []byte("File 2 content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		// Run setup batch to ensure files exist before creating archive operation
		setupResult, err := setupBatch.Run()
		if err != nil || !setupResult.IsSuccess() {
			t.Fatalf("Setup batch failed: %v, errors: %v", err, setupResult.GetError())
		}

		// Now, create the batch for the actual test
		archiveBatch := synthfs.NewBatch(fileSystem).WithFileSystem(testFS)

		// Create tar.gz archive
		_, err = archiveBatch.CreateArchive("backup.tar.gz", synthfs.ArchiveFormatTarGz, "archive-test/file1.txt", "archive-test/file2.txt")
		if err != nil {
			t.Fatalf("CreateArchive tar.gz failed: %v", err)
		}

		// Create zip archive
		_, err = archiveBatch.CreateArchive("backup.zip", synthfs.ArchiveFormatZip, "archive-test/file1.txt", "archive-test/file2.txt")
		if err != nil {
			t.Fatalf("CreateArchive zip failed: %v", err)
		}

		// Execute the archive batch
		result, err := archiveBatch.Run()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Fatalf("Archive creation failed: %v", result.GetError())
		}

		// For now, just verify the operations were created and executed
		// Real archive verification would require extracting and checking contents
		t.Logf("Archive operations executed successfully: %d operations", len(result.GetOperations()))
		for i, opResult := range result.GetOperations() {
			// Check if it's already a value type
			var operationResult core.OperationResult
			switch v := opResult.(type) {
			case core.OperationResult:
				operationResult = v
			case *core.OperationResult:
				operationResult = *v
			default:
				t.Fatalf("Unexpected operation result type: %T", opResult)
			}
			
			// Get operation description
			var desc core.OperationDesc
			if op, ok := operationResult.Operation.(interface{ Describe() core.OperationDesc }); ok {
				desc = op.Describe()
			}
			t.Logf("Operation %d: %s %s -> %s", i+1, desc.Type, desc.Path, operationResult.Status)
		}
	})

	t.Run("Complete workflow with all operations", func(t *testing.T) {
		// Test dependency resolution between operations
		testFS := testutil.NewTestFileSystem()
		fileSystem := testutil.NewTestFileSystem()

		// --- Setup Phase ---
		// Create initial files and directories first.
		setupBatch := synthfs.NewBatch(fileSystem).WithFileSystem(testFS)
		_, err := setupBatch.CreateDir("project")
		if err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}
		_, err = setupBatch.CreateFile("project/main.go", []byte("package main"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		_, err = setupBatch.CreateFile("README.md", []byte("# Project"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		setupResult, err := setupBatch.Run()
		if err != nil || !setupResult.IsSuccess() {
			t.Fatalf("Setup batch for workflow failed: %v, errors: %v", err, setupResult.GetError())
		}

		// --- Test Phase ---
		// Now run the workflow on the pre-existing files.
		fullBatch := synthfs.NewBatch(fileSystem).WithFileSystem(testFS)

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
		result, err := fullBatch.Run()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Fatalf("Complete workflow failed: %v", result.GetError())
		}

		t.Logf("Complete workflow executed: %d operations in %v", len(result.GetOperations()), result.GetDuration())

		// Verify some key results
		operations := fullBatch.Operations()
		if len(operations) < 5 { // Should have 5 main operations
			t.Errorf("Expected at least 5 operations, got %d", len(operations))
		}

		// Check operation types
		operationTypes := make(map[string]int)
		for _, op := range operations {
			operationTypes[op.(synthfs.Operation).Describe().Type]++
		}

		expectedTypes := []string{"create_directory", "copy", "create_symlink", "move", "create_archive"}
		for _, expectedType := range expectedTypes {
			if operationTypes[expectedType] == 0 {
				t.Errorf("Expected at least one %s operation", expectedType)
			}
		}

		t.Logf("Operation types: %+v", operationTypes)
	})

	t.Run("Operation objects detailed inspection", func(t *testing.T) {
		testFS := testutil.NewTestFileSystem()
		fileSystem := testutil.NewTestFileSystem()

		// --- Setup Phase ---
		setupBatch := synthfs.NewBatch(fileSystem).WithFileSystem(testFS)
		_, err := setupBatch.CreateFile("inspect.txt", []byte("inspection content"))
		if err != nil {
			t.Fatalf("CreateFile for setup failed: %v", err)
		}
		setupResult, err := setupBatch.Run()
		if err != nil || !setupResult.IsSuccess() {
			t.Fatalf("Setup batch for inspection failed: %v, errors: %v", err, setupResult.GetError())
		}

		// --- Test Phase ---
		inspectBatch := synthfs.NewBatch(fileSystem).WithFileSystem(testFS)

		// This operation is now invalid because the file already exists.
		// We'll test symlink and archive on the existing file.
		fileOp, err := inspectBatch.CreateFile("newfile.txt", []byte("new content"))
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
		t.Logf("File Operation: %s (ID: %s)", fileOp.(synthfs.Operation).Describe().Type, fileOp.(synthfs.Operation).ID())
		t.Logf("  Path: %s", fileOp.(synthfs.Operation).Describe().Path)
		t.Logf("  Details: %+v", fileOp.(synthfs.Operation).Describe().Details)
		t.Logf("  Dependencies: %v", fileOp.(synthfs.Operation).Dependencies())

		t.Logf("Symlink Operation: %s (ID: %s)", symlinkOp.(synthfs.Operation).Describe().Type, symlinkOp.(synthfs.Operation).ID())
		t.Logf("  Path: %s", symlinkOp.(synthfs.Operation).Describe().Path)
		t.Logf("  Details: %+v", symlinkOp.(synthfs.Operation).Describe().Details)
		t.Logf("  Dependencies: %v", symlinkOp.(synthfs.Operation).Dependencies())

		t.Logf("Archive Operation: %s (ID: %s)", archiveOp.(synthfs.Operation).Describe().Type, archiveOp.(synthfs.Operation).ID())
		t.Logf("  Path: %s", archiveOp.(synthfs.Operation).Describe().Path)
		t.Logf("  Details: %+v", archiveOp.(synthfs.Operation).Describe().Details)
		t.Logf("  Dependencies: %v", archiveOp.(synthfs.Operation).Dependencies())

		// Execute and inspect results
		result, err := inspectBatch.Run()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Fatalf("Execution failed: %v", result.GetError())
		}

		// Inspect execution results (like Python example)
		t.Logf("Execution Result: Success=%v, Duration=%v", result.IsSuccess(), result.GetDuration())
		for i, opResult := range result.GetOperations() {
			// Check if it's already a value type
			var operationResult core.OperationResult
			switch v := opResult.(type) {
			case core.OperationResult:
				operationResult = v
			case *core.OperationResult:
				operationResult = *v
			default:
				t.Fatalf("Unexpected operation result type: %T", opResult)
			}
			
			// Get operation ID
			var opID string
			if op, ok := operationResult.Operation.(interface{ ID() core.OperationID }); ok {
				opID = string(op.ID())
			}
			
			t.Logf("  Operation %d: %s -> %s (Duration: %v)",
				i+1,
				opID,
				operationResult.Status,
				operationResult.Duration)
		}
	})
}
