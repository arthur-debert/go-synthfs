package synthfs

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestSimpleRunAPI(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("Run with multiple operations", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// Create operations
		op1 := sfs.CreateDir("testdir", 0755)
		op2 := sfs.CreateFile("testdir/file.txt", []byte("content"), 0644)

		// Run creation operations
		_, err := Run(ctx, fs, op1, op2)
		if err != nil {
			t.Fatalf("Run failed for creation: %v", err)
		}

		// Now run the copy operation
		op3 := sfs.Copy("testdir/file.txt", "testdir/file-copy.txt")
		result, err := Run(ctx, fs, op3)
		if err != nil {
			t.Fatalf("Run failed for copy: %v", err)
		}

		// Check result
		if len(result.GetOperations()) != 1 {
			t.Errorf("Expected 1 operation result for the second run, got %d", len(result.GetOperations()))
		}

		// Verify filesystem state
		if _, err := fs.Stat("testdir"); err != nil {
			t.Error("Directory should exist")
		}
		if _, err := fs.Stat("testdir/file.txt"); err != nil {
			t.Error("File should exist")
		}
		if _, err := fs.Stat("testdir/file-copy.txt"); err != nil {
			t.Error("Copy should exist")
		}
	})

	t.Run("Run with no operations", func(t *testing.T) {
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		result, err := Run(ctx, fs)
		if err != nil {
			t.Fatalf("Run with no operations should succeed: %v", err)
		}

		if len(result.GetOperations()) != 0 {
			t.Error("Should have no operation results")
		}
	})

	t.Run("Run with failure", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// Create a conflict
		err := fs.WriteFile("conflict.txt", []byte("existing"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Create operations where the third will fail validation
		op1 := sfs.CreateDir("dir1", 0755)
		op2 := sfs.CreateFile("dir1/file.txt", []byte("content"), 0644)
		op3 := sfs.CreateDir("conflict.txt", 0755) // This will fail validation

		result, err := Run(ctx, fs, op1, op2, op3)
		if err == nil {
			t.Fatal("Expected error from conflicting operation")
		}

		// With upfront validation, no operations should succeed.
		if _, err := fs.Stat("dir1"); err == nil {
			t.Error("First operation should not have succeeded")
		}
		if _, err := fs.Stat("dir1/file.txt"); err == nil {
			t.Error("Second operation should not have succeeded")
		}

		// Check result
		if result == nil {
			t.Error("Result should be returned even on error")
		}
	})

	t.Run("RunWithOptions with custom options", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// Create custom options
		options := DefaultPipelineOptions()
		options.DryRun = true

		// Create operations
		op1 := sfs.CreateDir("testdir", 0755)
		op2 := sfs.CreateFile("testdir/file.txt", []byte("content"), 0644)

		result, err := RunWithOptions(ctx, fs, options, op1, op2)
		if err != nil {
			t.Fatalf("RunWithOptions failed: %v", err)
		}

		if len(result.GetOperations()) != 2 {
			t.Error("Should have 2 operation results")
		}

		// Since DryRun is implemented, this test should be updated to check that the files are not created.
		if _, err := fs.Stat("testdir"); err == nil {
			t.Error("Directory should not exist")
		}
		if _, err := fs.Stat("testdir/file.txt"); err == nil {
			t.Error("File should not exist")
		}
	})
}

func TestSimpleRunAPIValidationFailure(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	ctx := context.Background()
	fs := filesystem.NewTestFileSystem()

	// Create operations where the second will fail validation
	op1 := sfs.CreateDir("dir1", 0755)
	op2 := sfs.CreateFile("", []byte("content"), 0644) // Invalid path

	result, err := Run(ctx, fs, op1, op2)
	if err == nil {
		t.Fatal("Expected validation error")
	}

	// Result should still be returned
	if result == nil {
		t.Error("Result should be returned even on error")
	}

	// With upfront validation, the first operation should not have run
	if _, err := fs.Stat("dir1"); err == nil {
		t.Error("First operation should not have succeeded on validation failure")
	}
}

func TestSimpleRunWithRollback(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	ctx := context.Background()
	fs := filesystem.NewTestFileSystem()

	// Create a conflict
	err := fs.WriteFile("conflict.txt", []byte("existing"), 0644)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create operations where the third will fail
	op1 := sfs.CreateDir("dir1", 0755)
	op2 := sfs.CreateFile("dir1/file.txt", []byte("content"), 0644)
	op3 := sfs.CreateDir("conflict.txt", 0755) // This will fail

	options := DefaultPipelineOptions()
	options.RollbackOnError = true

	_, err = RunWithOptions(ctx, fs, options, op1, op2, op3)
	if err == nil {
		t.Fatal("Expected error from conflicting operation")
	}

	// With upfront validation, no operations are run, so nothing is rolled back.
	if _, err := fs.Stat("dir1"); err == nil {
		t.Error("Directory 'dir1' should not have been created")
	}
	if _, err := fs.Stat("dir1/file.txt"); err == nil {
		t.Error("File 'dir1/file.txt' should not have been created")
	}
}

func TestSimpleRunWithDryRun(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	ctx := context.Background()
	fs := filesystem.NewTestFileSystem()

	// Create operations
	op1 := sfs.CreateDir("testdir", 0755)
	op2 := sfs.CreateFile("testdir/file.txt", []byte("content"), 0644)

	options := DefaultPipelineOptions()
	options.DryRun = true

	result, err := RunWithOptions(ctx, fs, options, op1, op2)
	if err != nil {
		t.Fatalf("RunWithOptions with DryRun failed: %v", err)
	}

	if len(result.GetOperations()) != 2 {
		t.Error("Should have 2 operation results")
	}

	// Verify that no changes were made to the real filesystem
	if _, err := fs.Stat("testdir"); err == nil {
		t.Error("Directory 'testdir' should not have been created")
	}
	if _, err := fs.Stat("testdir/file.txt"); err == nil {
		t.Error("File 'testdir/file.txt' should not have been created")
	}
}

func TestSimpleRunAPI_FirstOperationFailure(t *testing.T) {
	t.Run("failure on first operation", func(t *testing.T) {
		ResetSequenceCounter()
		fs := filesystem.NewTestFileSystem()
		sfs := New()

		// Create operations where the first one will fail
		ops := []Operation{
			sfs.CreateFile("/nonexistent/file.txt", []byte("fail"), 0644), // Will fail - parent doesn't exist
			sfs.CreateDir("/should-not-run", 0755),
			sfs.CreateFile("/also-should-not-run.txt", []byte("nope"), 0644),
		}

		result, err := Run(context.Background(), fs, ops...)
		
		// Should have error
		if err == nil {
			t.Fatal("Expected error when first operation fails")
		}

		// Check result
		if result.IsSuccess() {
			t.Error("Result should not be successful")
		}

		// Verify no operations were executed
		entries, _ := fs.ReadDir("/")
		if len(entries) != 0 {
			t.Errorf("Expected filesystem to be empty, found %d entries", len(entries))
		}
	})

	t.Run("validation failure on first operation", func(t *testing.T) {
		ResetSequenceCounter()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)
		sfs := New()
		
		// Create a file that will cause validation to fail for copy
		if err := osFS.WriteFile("conflict.txt", []byte("existing"), 0644); err != nil {
			t.Fatalf("Failed to create conflict file: %v", err)
		}

		// First operation tries to copy non-existent file
		ops := []Operation{
			sfs.Copy("/nonexistent/source.txt", "conflict.txt"), // Will fail validation - source doesn't exist
			sfs.CreateFile("should-not-run.txt", []byte("nope"), 0644),
		}

		_, err := Run(context.Background(), fs, ops...)
		
		// Should have error
		if err == nil {
			t.Fatal("Expected error when first operation validation fails")
		}

		// Verify second operation was not created
		shouldNotExist := "should-not-run.txt"
		if _, err := fs.Stat(shouldNotExist); err == nil {
			t.Error("Second operation should not have been executed")
		}
	})
}

func TestSimpleRunAPI_ComplexOperationSequences(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("long chain of operations", func(t *testing.T) {
		ResetSequenceCounter()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)

		// Create a long chain of 15+ operations
		ops := []Operation{
			// Create directory structure
			sfs.CreateDir("project", 0755),
			sfs.CreateDir("project/src", 0755),
			sfs.CreateDir("project/tests", 0755),
			sfs.CreateDir("project/docs", 0755),
			sfs.CreateDir("project/configs", 0755),
			
			// Create files
			sfs.CreateFile("project/README.md", []byte("# Project"), 0644),
			sfs.CreateFile("project/src/main.go", []byte("package main"), 0644),
			sfs.CreateFile("project/src/utils.go", []byte("package main"), 0644),
			sfs.CreateFile("project/tests/main_test.go", []byte("package main"), 0644),
			sfs.CreateFile("project/configs/dev.json", []byte("{}"), 0644),
		}

		_, err := Run(context.Background(), fs, ops...)
		if err != nil {
			t.Fatalf("Expected creation to succeed, got error: %v", err)
		}

		ops2 := []Operation{
			// Copy operations
			sfs.Copy("project/configs/dev.json", "project/configs/prod.json"),
			sfs.Copy("project/configs/dev.json", "project/configs/test.json"),
			
			// Create more files
			sfs.CreateFile("project/.gitignore", []byte("*.tmp"), 0644),
			sfs.CreateFile("project/Makefile", []byte("build:"), 0644),
			sfs.CreateFile("project/go.mod", []byte("module test"), 0644),
		}

		_, err = Run(context.Background(), fs, ops2...)
		if err != nil {
			t.Fatalf("Expected copy and create to succeed, got error: %v", err)
		}

		ops3 := []Operation{
			// Move operations
			sfs.Move("project/configs/test.json", "project/configs/staging.json"),
			
			// Create symlinks
			sfs.CreateSymlink("project/README.md", "project/README.link"),
			sfs.CreateSymlink("project/configs/dev.json", "project/config.json"),
		}

		result, err := Run(context.Background(), fs, ops3...)
		if err != nil {
			t.Fatalf("Expected move and symlink to succeed, got error: %v", err)
		}

		if !result.IsSuccess() {
			t.Error("Result should be successful")
		}

		// Verify filesystem state
		verifyPaths := []struct {
			path   string
			isDir  bool
		}{
			{"project", true},
			{"project/src", true},
			{"project/tests", true},
			{"project/docs", true},
			{"project/configs", true},
			{"project/README.md", false},
			{"project/src/main.go", false},
			{"project/src/utils.go", false},
			{"project/tests/main_test.go", false},
			{"project/configs/dev.json", false},
			{"project/configs/prod.json", false},
			{"project/configs/staging.json", false},
			{"project/.gitignore", false},
			{"project/Makefile", false},
			{"project/go.mod", false},
		}

		for _, vp := range verifyPaths {
			info, err := fs.Stat(vp.path)
			if err != nil {
				t.Errorf("Path %s should exist: %v", vp.path, err)
				continue
			}
			if info.IsDir() != vp.isDir {
				t.Errorf("Path %s isDir=%v, expected %v", vp.path, info.IsDir(), vp.isDir)
			}
		}

		// Verify moved file doesn't exist at old location
		if _, err := fs.Stat("project/configs/test.json"); err == nil {
			t.Error("Moved file should not exist at old location")
		}
	})

	t.Run("operations interacting with same files", func(t *testing.T) {
		ResetSequenceCounter()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)
		sfs := New()

		// Operations that interact with the same files/directories multiple times
		// Create initial structure
		_, err := Run(context.Background(), fs, sfs.CreateDir("workspace", 0755), sfs.CreateFile("workspace/data.txt", []byte("initial"), 0644))
		if err != nil {
			t.Fatal(err)
		}
		
		// Copy the file
		_, err = Run(context.Background(), fs, sfs.Copy("workspace/data.txt", "workspace/data.backup.txt"))
		if err != nil {
			t.Fatal(err)
		}
		
		// Modify the original by deleting and recreating
		_, err = Run(context.Background(), fs, sfs.Delete("workspace/data.txt"), sfs.CreateFile("workspace/data.txt", []byte("modified"), 0644))
		if err != nil {
			t.Fatal(err)
		}
		
		// Copy the backup to a new location
		_, err = Run(context.Background(), fs, sfs.Copy("workspace/data.backup.txt", "workspace/data.old.txt"))
		if err != nil {
			t.Fatal(err)
		}
		
		// Create a directory with the same base name
		_, err = Run(context.Background(), fs, sfs.CreateDir("workspace/data", 0755), sfs.CreateFile("workspace/data/info.txt", []byte("info"), 0644))
		if err != nil {
			t.Fatal(err)
		}
		
		// Move files around
		_, err = Run(context.Background(), fs, sfs.Move("workspace/data.old.txt", "workspace/data/original.txt"))
		if err != nil {
			t.Fatal(err)
		}
		
		// Delete and recreate with different content
		_, err = Run(context.Background(), fs, sfs.Delete("workspace/data.backup.txt"), sfs.CreateFile("workspace/data.backup.txt", []byte("new backup"), 0644))
		if err != nil {
			t.Fatal(err)
		}
		
		// Create nested structure
		_, err = Run(context.Background(), fs, sfs.CreateDir("workspace/archive", 0755), sfs.Move("workspace/data.backup.txt", "workspace/archive/data.backup.txt"))
		if err != nil {
			t.Fatal(err)
		}

		// Verify final state
		content, err := fs.ReadFile("workspace/data.txt")
		if err != nil || string(content) != "modified" {
			t.Errorf("Expected modified content in data.txt, got %v, %s", err, content)
		}

		content, err = fs.ReadFile("workspace/data/original.txt")
		if err != nil || string(content) != "initial" {
			t.Errorf("Expected initial content in original.txt, got %v, %s", err, content)
		}

		content, err = fs.ReadFile("workspace/archive/data.backup.txt")
		if err != nil || string(content) != "new backup" {
			t.Errorf("Expected new backup content, got %v, %s", err, content)
		}
	})

	t.Run("varied operation types in single run", func(t *testing.T) {
		ResetSequenceCounter()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)
		sfs := New()

		// Mix of all operation types in a realistic workflow
		// Setup project structure
		_, err := Run(context.Background(), fs,
			sfs.CreateDir("myapp", 0755),
			sfs.CreateDir("myapp/src", 0755),
			sfs.CreateDir("myapp/build", 0755),
			sfs.CreateDir("myapp/dist", 0755),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Create source files
		_, err = Run(context.Background(), fs,
			sfs.CreateFile("myapp/src/app.js", []byte("console.log('app')"), 0644),
			sfs.CreateFile("myapp/src/utils.js", []byte("exports.util = {}"), 0644),
			sfs.CreateFile("myapp/package.json", []byte(`{"name":"myapp"}`), 0644),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Build process simulation
		_, err = Run(context.Background(), fs,
			sfs.Copy("myapp/src/app.js", "myapp/build/app.js"),
			sfs.Copy("myapp/src/utils.js", "myapp/build/utils.js"),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Create config files
		_, err = Run(context.Background(), fs,
			sfs.CreateFile("myapp/config.dev.json", []byte(`{"env":"dev"}`), 0644),
		)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Run(context.Background(), fs,
			sfs.Copy("myapp/config.dev.json", "myapp/config.prod.json"),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Create distribution
		_, err = Run(context.Background(), fs,
			sfs.Copy("myapp/build/app.js", "myapp/dist/app.min.js"),
			sfs.Copy("myapp/build/utils.js", "myapp/dist/utils.min.js"),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Create symlinks for convenience
		_, err = Run(context.Background(), fs,
			sfs.CreateSymlink("myapp/config.dev.json", "myapp/config.json"),
			sfs.CreateSymlink("myapp/dist/app.min.js", "myapp/app.js"),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Cleanup build artifacts
		_, err = Run(context.Background(), fs,
			sfs.Delete("myapp/build/app.js"),
			sfs.Delete("myapp/build/utils.js"),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Reorganize
		_, err = Run(context.Background(), fs,
			sfs.CreateDir("myapp/legacy", 0755),
		)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Run(context.Background(), fs,
			sfs.Move("myapp/src/utils.js", "myapp/legacy/utils.js"),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Final setup
		_, err = Run(context.Background(), fs,
			sfs.CreateFile("myapp/README.md", []byte("# MyApp"), 0644),
			sfs.CreateFile("myapp/.gitignore", []byte("node_modules/"), 0644),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Verify key outcomes
		if _, err := fs.Stat("myapp/dist/app.min.js"); err != nil {
			t.Error("Distribution file should exist")
		}

		if _, err := fs.Stat("myapp/build/app.js"); err == nil {
			t.Error("Build artifact should have been deleted")
		}

		if _, err := fs.Stat("myapp/legacy/utils.js"); err != nil {
			t.Error("Moved file should exist in new location")
		}

		if _, err := fs.Stat("myapp/src/utils.js"); err == nil {
			t.Error("Moved file should not exist in old location")
		}
	})
}
