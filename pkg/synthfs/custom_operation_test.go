package synthfs_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestCustomOperation_Basic(t *testing.T) {
	t.Run("execute custom operation", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()

		executed := false
		op := synthfs.NewCustomOperation("test-op", func(ctx context.Context, fs filesystem.FileSystem) error {
			executed = true
			return nil
		})

		adapter := synthfs.NewCustomOperationAdapter(op)
		err := adapter.Execute(context.Background(), fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !executed {
			t.Error("Custom operation was not executed")
		}
	})

	t.Run("custom operation with error", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()

		expectedErr := errors.New("custom error")
		op := synthfs.NewCustomOperation("error-op", func(ctx context.Context, fs filesystem.FileSystem) error {
			return expectedErr
		})

		adapter := synthfs.NewCustomOperationAdapter(op)
		err := adapter.Execute(context.Background(), fs)
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("custom operation with validation", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()

		validated := false
		op := synthfs.NewCustomOperation("validated-op", func(ctx context.Context, fs filesystem.FileSystem) error {
			return nil
		}).WithValidation(func(ctx context.Context, fs filesystem.FileSystem) error {
			validated = true
			return nil
		})

		adapter := synthfs.NewCustomOperationAdapter(op)
		err := adapter.Validate(context.Background(), fs)
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}

		if !validated {
			t.Error("Validation function was not called")
		}
	})

	t.Run("custom operation with rollback", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()

		rolledBack := false
		op := synthfs.NewCustomOperation("rollback-op", func(ctx context.Context, fs filesystem.FileSystem) error {
			return nil
		}).WithRollback(func(ctx context.Context, fs filesystem.FileSystem) error {
			rolledBack = true
			return nil
		})

		adapter := synthfs.NewCustomOperationAdapter(op)
		err := adapter.Rollback(context.Background(), fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		if !rolledBack {
			t.Error("Rollback function was not called")
		}
	})
}

func TestCustomOperation_WithFileSystem(t *testing.T) {
	t.Run("custom operation creates file", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()

		testFile := "custom-created.txt"
		content := []byte("created by custom operation")

		op := synthfs.NewCustomOperation("create-file-op", func(ctx context.Context, fs filesystem.FileSystem) error {
			return fs.WriteFile(testFile, content, 0644)
		})

		adapter := synthfs.NewCustomOperationAdapter(op)
		err := adapter.Execute(context.Background(), fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file was created
		if fullFS, ok := fs.(filesystem.FullFileSystem); ok {
			if _, err := fullFS.Stat(testFile); err != nil {
				t.Fatalf("Failed to stat created file: %v", err)
			}
			// Read the file to verify content
			file, err := fullFS.Open(testFile)
			if err != nil {
				t.Fatalf("Failed to open created file: %v", err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Logf("Failed to close file: %v", err)
				}
			}()
			
			var buf [1024]byte
			n, err := file.Read(buf[:])
			if err != nil {
				t.Fatalf("Failed to read file content: %v", err)
			}
			if string(buf[:n]) != string(content) {
				t.Errorf("File content mismatch: expected %q, got %q", string(content), string(buf[:n]))
			}
		} else {
			t.Fatal("Filesystem does not support FullFileSystem interface")
		}
	})

	t.Run("custom operation with filesystem validation", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()

		// Create a test file
		testFile := "validate-me.txt"
		err := fs.WriteFile(testFile, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		op := synthfs.NewCustomOperation("validate-file-op", func(ctx context.Context, fs filesystem.FileSystem) error {
			// Operation would modify the file
			return fs.WriteFile(testFile, []byte("modified"), 0644)
		}).WithValidation(func(ctx context.Context, fs filesystem.FileSystem) error {
			// Validate file exists
			if statFS, ok := fs.(filesystem.StatFS); ok {
				info, err := statFS.Stat(testFile)
				if err != nil {
					return fmt.Errorf("validation failed: file does not exist")
				}
				if info.IsDir() {
					return fmt.Errorf("validation failed: expected file, got directory")
				}
			} else {
				return fmt.Errorf("filesystem does not support Stat")
			}
			return nil
		})

		adapter := synthfs.NewCustomOperationAdapter(op)
		
		// Validation should pass
		err = adapter.Validate(context.Background(), fs)
		if err != nil {
			t.Fatalf("Validation failed unexpectedly: %v", err)
		}

		// Delete file and validation should fail
		err = fs.Remove(testFile)
		if err != nil {
			t.Fatalf("Failed to remove test file: %v", err)
		}

		err = adapter.Validate(context.Background(), fs)
		if err == nil {
			t.Error("Expected validation to fail for missing file")
		}
	})
}

func TestCustomOperation_InPipeline(t *testing.T) {
	t.Run("custom operation in pipeline", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create a marker file to track execution
		markerFile := "pipeline-marker.txt"
		
		customOp := sfs.CustomOperation("write-marker", func(ctx context.Context, fs filesystem.FileSystem) error {
			return fs.WriteFile(markerFile, []byte("custom operation executed"), 0644)
		})

		// Run pipeline with mixed operations
		result, err := synthfs.Run(context.Background(), fs,
			sfs.CreateDir("testdir", 0755),
			customOp,
			sfs.CreateFile("testdir/result.txt", []byte("pipeline complete"), 0644),
		)

		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Pipeline did not succeed: %v", result.GetError())
		}

		// Verify custom operation executed
		if _, err := fs.Stat(markerFile); err != nil {
			t.Error("Custom operation did not create marker file")
		}

		// Verify other operations also executed
		if _, err := fs.Stat("testdir/result.txt"); err != nil {
			t.Error("File operation did not execute")
		}
	})

	t.Run("custom operation with dependencies", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		inputFile := "input.txt"
		outputFile := "output.txt"

		// Create input file first
		createOp := sfs.CreateFile(inputFile, []byte("input data"), 0644)
		
		// Custom operation that depends on the input file
		processOp := sfs.CustomOperationWithID("process-file", func(ctx context.Context, fs filesystem.FileSystem) error {
			// Read input file
			file, err := fs.Open(inputFile)
			if err != nil {
				return fmt.Errorf("failed to open input: %w", err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Logf("Failed to close file: %v", err)
				}
			}()
			
			var data []byte
			buf := make([]byte, 1024)
			n, err := file.Read(buf)
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			data = buf[:n]
			
			processed := fmt.Sprintf("Processed: %s", string(data))
			return fs.WriteFile(outputFile, []byte(processed), 0644)
		})

		// Build pipeline with dependencies
		builder := synthfs.NewPipelineBuilder()
		builder.Add(createOp)
		processOp.AddDependency(createOp.ID())
		builder.Add(processOp)
		
		pipeline := builder.Build()

		executor := synthfs.NewExecutor()
		result := executor.Run(context.Background(), pipeline, fs)

		if !result.IsSuccess() {
			t.Errorf("Pipeline execution failed: %v", result.GetError())
		}

		// Verify output file contains processed data
		file, err := fs.Open(outputFile)
		if err != nil {
			t.Fatalf("Failed to open output file: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Logf("Failed to close file: %v", err)
			}
		}()
		
		var buf [1024]byte
		n, err := file.Read(buf[:])
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}
		
		expected := "Processed: input data"
		if string(buf[:n]) != expected {
			t.Errorf("Output mismatch: expected %q, got %q", expected, string(buf[:n]))
		}
	})
}

func TestCustomOperation_ErrorHandling(t *testing.T) {
	t.Run("rollback on error with custom operation", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		cleanupFile := "cleanup-me.txt"
		rollbackExecuted := false

		// Custom operation that creates a file and can clean it up on rollback
		op := synthfs.NewCustomOperation("create-with-cleanup", func(ctx context.Context, fs filesystem.FileSystem) error {
			return fs.WriteFile(cleanupFile, []byte("temporary data"), 0644)
		}).WithRollback(func(ctx context.Context, fs filesystem.FileSystem) error {
			rollbackExecuted = true
			return fs.Remove(cleanupFile)
		})

		// Create adapter for the custom operation
		customOp := synthfs.NewCustomOperationAdapter(op)

		// Operation that will fail
		failOp := sfs.CustomOperation("fail-op", func(ctx context.Context, fs filesystem.FileSystem) error {
			return errors.New("intentional failure")
		})

		// Run with rollback enabled
		opts := synthfs.DefaultPipelineOptions()
		opts.RollbackOnError = true

		result, err := synthfs.RunWithOptions(context.Background(), fs, opts,
			customOp,
			failOp,
		)

		if err == nil {
			t.Error("Expected error from failed operation")
		}

		if result.IsSuccess() {
			t.Error("Expected pipeline to fail")
		}

		// Verify rollback was executed
		if !rollbackExecuted {
			t.Error("Rollback was not executed")
		}

		// Verify cleanup file was removed
		if _, err := fs.Stat(cleanupFile); !os.IsNotExist(err) {
			t.Error("Cleanup file should have been removed by rollback")
		}
	})

	t.Run("validation prevents execution", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)

		executed := false
		op := synthfs.NewCustomOperation("validated-op", func(ctx context.Context, fs filesystem.FileSystem) error {
			executed = true
			return nil
		}).WithValidation(func(ctx context.Context, fs filesystem.FileSystem) error {
			return errors.New("validation failed")
		})

		customOp := synthfs.NewCustomOperationAdapter(op)

		// Build and validate pipeline
		builder := synthfs.NewPipelineBuilder()
		builder.Add(customOp)
		
		pipeline := builder.Build()

		// Validation should fail
		err := pipeline.Validate(context.Background(), fs)
		if err == nil {
			t.Error("Expected validation to fail")
		}

		// Execute should not have been called
		if executed {
			t.Error("Operation should not have been executed after validation failure")
		}
	})
}

func TestCustomOperation_RealWorldExample(t *testing.T) {
	t.Run("npm build workflow", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem().(filesystem.FullFileSystem)
		sfs := synthfs.New()

		// Create a mock package.json
		packageJSON := `{
  "name": "test-app",
  "version": "1.0.0",
  "scripts": {
    "build": "echo 'Building...' > dist/output.txt"
  }
}`

		// Create npm build operation with validation
		npmBuildOp := synthfs.NewCustomOperation("npm-build", func(ctx context.Context, fs filesystem.FileSystem) error {
			// Simulate build output
			return fs.WriteFile("dist/output.txt", []byte("Build complete!"), 0644)
		}).WithValidation(func(ctx context.Context, fs filesystem.FileSystem) error {
			// Validate package.json exists
			if fullFS, ok := fs.(filesystem.FullFileSystem); ok {
				if _, err := fullFS.Stat("package.json"); err != nil {
					return fmt.Errorf("package.json not found")
				}
			} else {
				return fmt.Errorf("filesystem does not support Stat")
			}
			return nil
		})

		// Simulate npm build workflow
		ops := []synthfs.Operation{
			sfs.CreateFile("package.json", []byte(packageJSON), 0644),
			sfs.CreateDir("dist", 0755),
			synthfs.NewCustomOperationAdapter(npmBuildOp),
		}

		result, err := synthfs.Run(context.Background(), fs, ops...)
		if err != nil {
			t.Fatalf("Workflow failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Errorf("Workflow did not succeed: %v", result.GetError())
		}

		// Verify build output
		file, err := fs.Open("dist/output.txt")
		if err != nil {
			t.Fatalf("Failed to open build output: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Logf("Failed to close file: %v", err)
			}
		}()
		
		var buf [1024]byte
		n, err := file.Read(buf[:])
		if err != nil {
			t.Fatalf("Failed to read build output: %v", err)
		}
		if string(buf[:n]) != "Build complete!" {
			t.Errorf("Unexpected build output: %s", string(buf[:n]))
		}
	})

	t.Run("database migration workflow", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		tempDir := helper.TempDir()
		fs := helper.FileSystem().(filesystem.FullFileSystem)

		// Track migration state
		migrationLog := filepath.Join(tempDir, "migrations.log")
		
		// Helper to log migrations
		logMigration := func(version string) error {
			f, err := os.OpenFile(migrationLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					t.Logf("Failed to close file: %v", err)
				}
			}()
			
			_, err = fmt.Fprintf(f, "Applied migration: %s\n", version)
			return err
		}

		// Create migration operations
		migration1Op := synthfs.NewCustomOperation("migration-v1", func(ctx context.Context, fs filesystem.FileSystem) error {
			// Simulate schema change
			if err := fs.MkdirAll("db/v1", 0755); err != nil {
				return err
			}
			return logMigration("v1")
		}).WithRollback(func(ctx context.Context, fs filesystem.FileSystem) error {
			// Rollback v1
			if err := fs.RemoveAll("db/v1"); err != nil {
				return fmt.Errorf("failed to remove db/v1: %w", err)
			}
			return logMigration("rollback-v1")
		})
		migration1 := synthfs.NewCustomOperationAdapter(migration1Op)

		migration2Op := synthfs.NewCustomOperation("migration-v2", func(ctx context.Context, fs filesystem.FileSystem) error {
			// v2 requires v1 to be applied - check at execution time
			if fullFS, ok := fs.(filesystem.FullFileSystem); ok {
				if _, err := fullFS.Stat("db/v1"); err != nil {
					return fmt.Errorf("migration v1 must be applied first")
				}
			}
			
			// Simulate another schema change
			if err := fs.MkdirAll("db/v2", 0755); err != nil {
				return err
			}
			return logMigration("v2")
		}).WithRollback(func(ctx context.Context, fs filesystem.FileSystem) error {
			// Rollback v2
			if err := fs.RemoveAll("db/v2"); err != nil {
				return fmt.Errorf("failed to remove db/v2: %w", err)
			}
			return logMigration("rollback-v2")
		})
		migration2 := synthfs.NewCustomOperationAdapter(migration2Op)

		// Build pipeline with dependencies
		builder := synthfs.NewPipelineBuilder()
		builder.Add(migration1)
		migration2.AddDependency(migration1.ID())
		builder.Add(migration2)
		
		pipeline := builder.Build()

		executor := synthfs.NewExecutor()
		result := executor.Run(context.Background(), pipeline, fs)

		if !result.IsSuccess() {
			t.Errorf("Migration pipeline failed: %v", result.GetError())
		}

		// Verify both migrations were applied
		if _, err := fs.Stat("db/v1"); err != nil {
			t.Error("Migration v1 was not applied")
		}
		if _, err := fs.Stat("db/v2"); err != nil {
			t.Error("Migration v2 was not applied")
		}

		// Verify migration log
		logContent, err := os.ReadFile(migrationLog)
		if err != nil {
			t.Fatalf("Failed to read migration log: %v", err)
		}
		
		expectedLog := "Applied migration: v1\nApplied migration: v2\n"
		if string(logContent) != expectedLog {
			t.Errorf("Migration log mismatch:\nexpected: %q\ngot: %q", expectedLog, string(logContent))
		}
	})
}