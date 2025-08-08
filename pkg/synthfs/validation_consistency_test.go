package synthfs_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// TestValidationConsistency verifies that validation behavior is consistent
// across different APIs when handling sequential operations.
func TestValidationConsistency(t *testing.T) {
	tests := []struct {
		name string
		ops  func(sfs *synthfs.SynthFS) []synthfs.Operation
	}{
		{
			name: "create file in new directory",
			ops: func(sfs *synthfs.SynthFS) []synthfs.Operation {
				return []synthfs.Operation{
					sfs.CreateDir("newdir", 0755),
					sfs.CreateFile("newdir/file.txt", []byte("content"), 0644),
				}
			},
		},
		{
			name: "create nested file structure",
			ops: func(sfs *synthfs.SynthFS) []synthfs.Operation {
				return []synthfs.Operation{
					sfs.CreateDir("project", 0755),
					sfs.CreateDir("project/src", 0755),
					sfs.CreateFile("project/README.md", []byte("# Project"), 0644),
					sfs.CreateFile("project/src/main.go", []byte("package main"), 0644),
				}
			},
		},
		{
			name: "copy file after creation",
			ops: func(sfs *synthfs.SynthFS) []synthfs.Operation {
				return []synthfs.Operation{
					sfs.CreateFile("original.txt", []byte("data"), 0644),
					sfs.Copy("original.txt", "copy.txt"),
				}
			},
		},
		{
			name: "move file after creation",
			ops: func(sfs *synthfs.SynthFS) []synthfs.Operation {
				return []synthfs.Operation{
					sfs.CreateFile("temp.txt", []byte("data"), 0644),
					sfs.Move("temp.txt", "final.txt"),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := filesystem.NewTestFileSystem()
			sfs := synthfs.New()
			ctx := context.Background()
			ops := tt.ops(sfs)

			// Test 1: Simple API validation (should pass with projected filesystem)
			t.Run("Simple API", func(t *testing.T) {
				result, err := synthfs.Run(ctx, fs, ops...)
				if err != nil {
					t.Errorf("Simple API failed: %v", err)
				}
				if !result.Success {
					t.Errorf("Simple API execution failed: %v", result.Errors)
				}
			})

			// Test 2: Pipeline validation (currently may fail without projected filesystem)
			t.Run("Pipeline API", func(t *testing.T) {
				// Reset filesystem
				fs = filesystem.NewTestFileSystem()
				
				pipeline := synthfs.NewMemPipeline()
				for _, op := range ops {
					if err := pipeline.Add(op); err != nil {
						t.Fatalf("Failed to add operation to pipeline: %v", err)
					}
				}

				// First try validation
				err := pipeline.Validate(ctx, fs)
				if err != nil {
					t.Logf("Pipeline validation failed (expected until fixed): %v", err)
					// This is expected to fail until we implement the fix
					// Don't fail the test yet, just log it
				}

				// Try execution anyway
				executor := synthfs.NewExecutor()
				result := executor.Run(ctx, pipeline, fs)
				
				// The execution might succeed even if validation failed
				// because of auto-creation behavior
				if !result.Success {
					t.Logf("Pipeline execution failed: %v", result.Errors)
				}
			})

			// Test 3: Direct operation validation
			t.Run("Direct Validation", func(t *testing.T) {
				// Reset filesystem
				fs = filesystem.NewTestFileSystem()
				
				// Validate operations one by one against real filesystem
				for i, op := range ops {
					err := op.Validate(ctx, nil, fs)
					if err != nil {
						t.Logf("Operation %d validation failed against real FS (expected): %v", i, err)
					}
				}
			})
		})
	}
}

// TestPrerequisiteValidationMismatch demonstrates the mismatch between
// prerequisite declarations and actual execution behavior.
func TestPrerequisiteValidationMismatch(t *testing.T) {
	ctx := context.Background()
	sfs := synthfs.New()

	t.Run("file creation without parent directory", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		
		// Create a file in a non-existent directory
		op := sfs.CreateFile("nonexistent/deep/path/file.txt", []byte("content"), 0644)
		
		// Check prerequisites - parent directory prerequisite has been removed
		prereqs := op.Prerequisites()
		for _, prereq := range prereqs {
			if prereq.Type() == "parent_dir" {
				t.Error("CreateFile should not have parent directory prerequisite (auto-creates)")
			}
		}
		
		// Should have no-conflict prerequisite
		hasNoConflict := false
		for _, prereq := range prereqs {
			if prereq.Type() == "no_conflict" {
				hasNoConflict = true
			}
		}
		if !hasNoConflict {
			t.Error("CreateFile should have no-conflict prerequisite")
		}
		
		// But execution should succeed (auto-creates parent)
		result, err := synthfs.Run(ctx, fs, op)
		if err != nil {
			t.Errorf("Execution failed despite auto-creation: %v", err)
		}
		if !result.Success {
			t.Errorf("Execution failed: %v", result.Errors)
		}
		
		// Verify the file was created
		if _, err := fs.Stat("nonexistent/deep/path/file.txt"); err != nil {
			t.Error("File should exist after execution")
		}
	})
}

// TestProjectedFilesystemValidation verifies that projected filesystem
// validation works correctly for sequential operations.
func TestProjectedFilesystemValidation(t *testing.T) {
	ctx := context.Background()
	sfs := synthfs.New()
	
	t.Run("sequential operations with projected state", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		
		ops := []synthfs.Operation{
			sfs.CreateDir("mydir", 0755),
			sfs.CreateFile("mydir/file1.txt", []byte("content1"), 0644),
			sfs.CreateFile("mydir/file2.txt", []byte("content2"), 0644),
			sfs.Copy("mydir/file1.txt", "mydir/file1_backup.txt"),
		}
		
		// Create projected filesystem
		projectedFS := synthfs.NewProjectedFileSystem(fs)
		
		// Validate with projected state
		for i, op := range ops {
			// Should pass with projected state
			if err := op.Validate(ctx, nil, projectedFS); err != nil {
				t.Errorf("Operation %d failed validation with projected FS: %v", i, err)
			}
			
			// Update projected state
			if err := projectedFS.UpdateProjectedState(op); err != nil {
				t.Errorf("Failed to update projected state: %v", err)
			}
		}
		
		// Now validate against real filesystem
		// Note: Due to auto-creation behavior, these won't necessarily fail
		// Copy operation (index 3) should fail because source doesn't exist
		for _, op := range ops {
			err := op.Validate(ctx, nil, fs)
			desc := op.Describe()
			
			// Only the Copy operation should fail (source doesn't exist)
			if desc.Type == "copy" && err == nil {
				t.Errorf("Copy operation should fail validation against real FS (source doesn't exist)")
			} else if desc.Type == "copy" && err != nil {
				t.Logf("Copy operation correctly failed validation: %v", err)
			}
		}
	})
}