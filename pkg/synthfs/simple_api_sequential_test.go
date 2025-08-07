package synthfs_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// TestSimpleAPISequentialOperations verifies that the simple API can handle
// sequential operations where later operations depend on earlier ones.
// This tests the fix for issue #56.
func TestSimpleAPISequentialOperations(t *testing.T) {
	tests := []struct {
		name string
		ops  func(sfs *synthfs.SynthFS) []synthfs.Operation
		verify func(t *testing.T, fs filesystem.FileSystem)
	}{
		{
			name: "create directory then file",
			ops: func(sfs *synthfs.SynthFS) []synthfs.Operation {
				return []synthfs.Operation{
					sfs.CreateDir("mydir", 0755),
					sfs.CreateFile("mydir/file.txt", []byte("content"), 0644),
				}
			},
			verify: func(t *testing.T, fs filesystem.FileSystem) {
				// Check directory exists
				info, err := fs.Stat("mydir")
				if err != nil {
					t.Errorf("Directory should exist: %v", err)
				}
				if info != nil && !info.IsDir() {
					t.Error("mydir should be a directory")
				}
				
				// Check file exists
				if _, err := fs.Stat("mydir/file.txt"); err != nil {
					t.Errorf("File should exist: %v", err)
				}
			},
		},
		{
			name: "create file then copy it",
			ops: func(sfs *synthfs.SynthFS) []synthfs.Operation {
				return []synthfs.Operation{
					sfs.CreateFile("original.txt", []byte("original content"), 0644),
					sfs.Copy("original.txt", "copy.txt"),
				}
			},
			verify: func(t *testing.T, fs filesystem.FileSystem) {
				// Check both files exist
				if _, err := fs.Stat("original.txt"); err != nil {
					t.Errorf("Original file should exist: %v", err)
				}
				
				if _, err := fs.Stat("copy.txt"); err != nil {
					t.Errorf("Copy should exist: %v", err)
				}
			},
		},
		{
			name: "complex sequential operations",
			ops: func(sfs *synthfs.SynthFS) []synthfs.Operation {
				return []synthfs.Operation{
					sfs.CreateDir("project", 0755),
					sfs.CreateDir("project/src", 0755),
					sfs.CreateFile("project/README.md", []byte("# Project"), 0644),
					sfs.CreateFile("project/src/main.go", []byte("package main"), 0644),
					sfs.Copy("project/README.md", "project/README.backup.md"),
					sfs.CreateSymlink("project/src/main.go", "project/main"),
				}
			},
			verify: func(t *testing.T, fs filesystem.FileSystem) {
				// Verify directory structure
				paths := []string{
					"project",
					"project/src",
					"project/README.md",
					"project/src/main.go",
					"project/README.backup.md",
					"project/main",
				}
				
				for _, path := range paths {
					if _, err := fs.Stat(path); err != nil {
						t.Errorf("Path %s should exist: %v", path, err)
					}
				}
				
				// Verify symlink
				target, err := fs.Readlink("project/main")
				if err != nil {
					t.Errorf("Should be able to read symlink: %v", err)
				}
				if target != "project/src/main.go" {
					t.Errorf("Symlink target mismatch: got %q, want %q", target, "project/src/main.go")
				}
			},
		},
		{
			name: "move operation depending on copy",
			ops: func(sfs *synthfs.SynthFS) []synthfs.Operation {
				return []synthfs.Operation{
					sfs.CreateFile("source.txt", []byte("content"), 0644),
					sfs.Copy("source.txt", "temp.txt"),
					sfs.Move("temp.txt", "final.txt"),
				}
			},
			verify: func(t *testing.T, fs filesystem.FileSystem) {
				// Source should exist
				if _, err := fs.Stat("source.txt"); err != nil {
					t.Error("Source file should still exist")
				}
				
				// Temp should not exist (moved)
				if _, err := fs.Stat("temp.txt"); err == nil {
					t.Error("Temp file should not exist after move")
				}
				
				// Final should exist
				if _, err := fs.Stat("final.txt"); err != nil {
					t.Errorf("Final file should exist: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use TestFileSystem for isolation
			fs := filesystem.NewTestFileSystem()
			sfs := synthfs.New()
			
			// Get operations
			ops := tt.ops(sfs)
			
			// Execute in a single Run call
			result, err := synthfs.Run(context.Background(), fs, ops...)
			if err != nil {
				t.Fatalf("Run failed: %v", err)
			}
			
			if !result.IsSuccess() {
				t.Fatal("Run should have succeeded")
			}
			
			// Verify results
			tt.verify(t, fs)
		})
	}
}

// TestSimpleAPIDryRun verifies that dry run works with sequential operations
func TestSimpleAPIDryRun(t *testing.T) {
	fs := filesystem.NewTestFileSystem()
	sfs := synthfs.New()
	
	ops := []synthfs.Operation{
		sfs.CreateDir("testdir", 0755),
		sfs.CreateFile("testdir/file.txt", []byte("content"), 0644),
	}
	
	opts := synthfs.DefaultPipelineOptions()
	opts.DryRun = true
	
	result, err := synthfs.RunWithOptions(context.Background(), fs, opts, ops...)
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}
	
	if !result.IsSuccess() {
		t.Fatal("Dry run should succeed")
	}
	
	// Verify nothing was actually created
	if _, err := fs.Stat("testdir"); err == nil {
		t.Error("Directory should not exist after dry run")
	}
}