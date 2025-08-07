package synthfs

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestPipelineBuilder(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("Pipeline function", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// One-liner pipeline creation and execution
		_, err := BuildPipeline(
			sfs.CreateDir("dir1", 0755),
			sfs.CreateFile("dir1/file.txt", []byte("content"), 0644),
		).Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Pipeline execution failed for creation: %v", err)
		}

		result, err := BuildPipeline(
			sfs.Copy("dir1/file.txt", "dir1/copy.txt"),
		).Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Pipeline execution failed for copy: %v", err)
		}

		if len(result.GetOperations()) != 1 {
			t.Errorf("Expected 1 operations, got %d", len(result.GetOperations()))
		}

		// Verify results
		if _, err := fs.Stat("dir1"); err != nil {
			t.Error("Directory should exist")
		}
		if _, err := fs.Stat("dir1/file.txt"); err != nil {
			t.Error("File should exist")
		}
		if _, err := fs.Stat("dir1/copy.txt"); err != nil {
			t.Error("Copy should exist")
		}
	})

	t.Run("Pipeline with dependencies", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		op1 := sfs.CreateDir("base", 0755)
		op2 := sfs.CreateFile("base/file1.txt", []byte("content1"), 0644)
		op3 := sfs.CreateFile("base/file2.txt", []byte("content2"), 0644)

		_, err := BuildPipeline(op1, op2, op3).
			WithDependency(op2, op1).
			WithDependency(op3, op1).
			Execute(ctx, fs)

		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		// Dependencies are no longer tracked, but operations should still execute successfully
		// Verify the files were created
		if _, err := fs.Stat("base"); err != nil {
			t.Error("base directory should exist")
		}
		if _, err := fs.Stat("base/file1.txt"); err != nil {
			t.Error("file1.txt should exist")
		}
		if _, err := fs.Stat("base/file2.txt"); err != nil {
			t.Error("file2.txt should exist")
		}
	})

	t.Run("PipelineBuilder", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// Build complete pipeline and execute once
		builder := NewPipelineBuilder()

		op1 := sfs.CreateDir("step1", 0755)
		op2 := sfs.CreateFile("step1/data.txt", []byte("data"), 0644)
		op3 := sfs.Move("step1/data.txt", "step1/renamed.txt")

		_, err := builder.
			Add(op1).
			Add(op2).After(op1).
			Add(op3).After(op2).
			Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		// Verify final state
		if _, err := fs.Stat("step1/renamed.txt"); err != nil {
			t.Error("Renamed file should exist")
		}
		if _, err := fs.Stat("step1/data.txt"); err == nil {
			t.Error("Original file should not exist after move")
		}
	})

	t.Run("Pipeline with options", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		options := DefaultPipelineOptions()

		result, err := BuildPipeline(
			sfs.CreateDir("optdir", 0755),
			sfs.CreateFile("optdir/file.txt", []byte("content"), 0644),
		).WithOptions(options).Execute(ctx, fs)

		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		if len(result.GetOperations()) != 2 {
			t.Error("Should have executed 2 operations")
		}
	})

	t.Run("Pipeline with custom executor", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		customExecutor := NewExecutor()

		_, err := BuildPipeline(
			sfs.CreateDir("custom", 0755),
		).ExecuteWith(ctx, fs, customExecutor)

		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		if _, err := fs.Stat("custom"); err != nil {
			t.Error("Directory should exist")
		}
	})

	t.Run("Pipeline error handling", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()

		// Create conflict
		err := fs.WriteFile("conflict.txt", []byte("existing"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		result, err := BuildPipeline(
			sfs.CreateDir("errtest", 0755),
			sfs.CreateFile("errtest/file.txt", []byte("content"), 0644),
			sfs.CreateDir("conflict.txt", 0755), // This will fail
		).Execute(ctx, fs)

		if err == nil {
			t.Fatal("Expected error from conflicting operation")
		}

		// With upfront validation, no operations should have run.
		if _, err := fs.Stat("errtest"); err == nil {
			t.Error("Directory should not exist")
		}

		// Result should still be available
		if result == nil {
			t.Error("Result should be returned even on error")
		}
	})
}
