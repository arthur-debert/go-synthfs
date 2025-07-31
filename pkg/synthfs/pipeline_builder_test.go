package synthfs

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestPipelineBuilder(t *testing.T) {
	// Use sequence generator for predictable IDs
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	SetIDGenerator(SequenceIDGenerator)
	
	t.Run("Pipeline function", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()
		
		// One-liner pipeline creation and execution
		result, err := BuildPipeline(
			CreateDir("dir1", 0755),
			CreateFile("dir1/file.txt", []byte("content"), 0644),
			Copy("dir1/file.txt", "dir1/copy.txt"),
		).Execute(ctx, fs)
		
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}
		
		if len(result.GetOperations()) != 3 {
			t.Errorf("Expected 3 operations, got %d", len(result.GetOperations()))
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
		
		op1 := CreateDir("base", 0755)
		op2 := CreateFile("base/file1.txt", []byte("content1"), 0644)
		op3 := CreateFile("base/file2.txt", []byte("content2"), 0644)
		
		_, err := BuildPipeline(op1, op2, op3).
			WithDependency(op2, op1).
			WithDependency(op3, op1).
			Execute(ctx, fs)
		
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}
		
		// Check dependencies were set
		if len(op2.Dependencies()) != 1 || op2.Dependencies()[0] != op1.ID() {
			t.Error("op2 should depend on op1")
		}
		if len(op3.Dependencies()) != 1 || op3.Dependencies()[0] != op1.ID() {
			t.Error("op3 should depend on op1")
		}
	})
	
	t.Run("PipelineBuilder", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()
		
		// Build pipeline step by step
		builder := NewPipelineBuilder()
		
		op1 := CreateDir("step1", 0755)
		op2 := CreateFile("step1/data.txt", []byte("data"), 0644)
		op3 := Move("step1/data.txt", "step1/renamed.txt")
		
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
			CreateDir("optdir", 0755),
			CreateFile("optdir/file.txt", []byte("content"), 0644),
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
			CreateDir("custom", 0755),
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
			CreateDir("errtest", 0755),
			CreateFile("errtest/file.txt", []byte("content"), 0644),
			CreateDir("conflict.txt", 0755), // This will fail
		).Execute(ctx, fs)
		
		if err == nil {
			t.Fatal("Expected error from conflicting operation")
		}
		
		// Should have PipelineError
		if pipelineErr, ok := err.(*PipelineError); ok {
			if pipelineErr.FailedIndex != 3 {
				t.Errorf("Expected failure at operation 3, got %d", pipelineErr.FailedIndex)
			}
			if len(pipelineErr.SuccessfulOps) != 2 {
				t.Errorf("Expected 2 successful operations, got %d", len(pipelineErr.SuccessfulOps))
			}
		} else {
			t.Errorf("Expected PipelineError, got %T", err)
		}
		
		// Result should still be available
		if result == nil {
			t.Error("Result should be returned even on error")
		}
	})
}

func TestExecutablePipeline(t *testing.T) {
	// Use sequence generator for predictable IDs
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	SetIDGenerator(SequenceIDGenerator)
	
	t.Run("Default executor", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()
		
		pipeline := NewExecutablePipeline()
		err := pipeline.Add(
			CreateDir("exec", 0755),
			CreateFile("exec/file.txt", []byte("content"), 0644),
		)
		if err != nil {
			t.Fatalf("Failed to add operations: %v", err)
		}
		
		// Execute with default executor
		result, err := pipeline.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}
		
		if len(result.GetOperations()) != 2 {
			t.Error("Should have executed 2 operations")
		}
		
		// Verify results
		if _, err := fs.Stat("exec/file.txt"); err != nil {
			t.Error("File should exist")
		}
	})
	
	t.Run("Custom executor", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := filesystem.NewTestFileSystem()
		
		pipeline := NewExecutablePipeline()
		err := pipeline.Add(CreateDir("custom", 0755))
		if err != nil {
			t.Fatalf("Failed to add operation: %v", err)
		}
		
		customExecutor := NewExecutor()
		_, err = pipeline.ExecuteWith(ctx, fs, customExecutor)
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}
		
		if _, err := fs.Stat("custom"); err != nil {
			t.Error("Directory should exist")
		}
	})
}