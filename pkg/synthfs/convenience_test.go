package synthfs

import (
	"context"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestConvenienceConstructors(t *testing.T) {
	// Use sequence generator for predictable IDs in tests
	// Save and restore original generator at the end
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	SetIDGenerator(SequenceIDGenerator)
	ResetSequenceCounter()
	
	t.Run("CreateFile", func(t *testing.T) {
		op := CreateFile("path/to/file.txt", []byte("content"), 0644)
		
		if op.ID() != "create_file-1" {
			t.Errorf("Expected auto-generated ID 'create_file-1', got: %s", op.ID())
		}
		
		desc := op.Describe()
		if desc.Type != "create_file" {
			t.Errorf("Expected operation type 'create_file', got: %s", desc.Type)
		}
		if desc.Path != "path/to/file.txt" {
			t.Errorf("Expected path 'path/to/file.txt', got: %s", desc.Path)
		}
	})
	
	t.Run("CreateFileWithID", func(t *testing.T) {
		op := CreateFileWithID("custom-id", "path/to/file.txt", []byte("content"), 0644)
		
		if op.ID() != "custom-id" {
			t.Errorf("Expected ID 'custom-id', got: %s", op.ID())
		}
	})
	
	t.Run("CreateDir", func(t *testing.T) {
		op := CreateDir("path/to/dir", 0755)
		
		desc := op.Describe()
		if desc.Type != "create_directory" {
			t.Errorf("Expected operation type 'create_directory', got: %s", desc.Type)
		}
		if desc.Path != "path/to/dir" {
			t.Errorf("Expected path 'path/to/dir', got: %s", desc.Path)
		}
	})
	
	t.Run("Delete", func(t *testing.T) {
		op := Delete("path/to/delete")
		
		desc := op.Describe()
		if desc.Type != "delete" {
			t.Errorf("Expected operation type 'delete', got: %s", desc.Type)
		}
		if desc.Path != "path/to/delete" {
			t.Errorf("Expected path 'path/to/delete', got: %s", desc.Path)
		}
	})
	
	t.Run("Copy", func(t *testing.T) {
		op := Copy("src/path", "dst/path")
		
		desc := op.Describe()
		if desc.Type != "copy" {
			t.Errorf("Expected operation type 'copy', got: %s", desc.Type)
		}
	})
	
	t.Run("Move", func(t *testing.T) {
		op := Move("old/path", "new/path")
		
		desc := op.Describe()
		if desc.Type != "move" {
			t.Errorf("Expected operation type 'move', got: %s", desc.Type)
		}
	})
	
	t.Run("CreateSymlink", func(t *testing.T) {
		op := CreateSymlink("target/path", "link/path")
		
		desc := op.Describe()
		if desc.Type != "create_symlink" {
			t.Errorf("Expected operation type 'create_symlink', got: %s", desc.Type)
		}
		if desc.Path != "link/path" {
			t.Errorf("Expected path 'link/path', got: %s", desc.Path)
		}
	})
}

func TestDirectExecutionMethods(t *testing.T) {
	ctx := context.Background()
	
	t.Run("WriteFile", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		
		err := WriteFile(ctx, fs, "test/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		
		// Verify file was created
		info, err := fs.Stat("test/file.txt")
		if err != nil {
			t.Fatalf("File not created: %v", err)
		}
		if info.Size() != 7 {
			t.Errorf("Expected file size 7, got: %d", info.Size())
		}
	})
	
	t.Run("MkdirAll", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		
		err := MkdirAll(ctx, fs, "test/nested/dir", 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		
		// Verify directory was created
		info, err := fs.Stat("test/nested/dir")
		if err != nil {
			t.Fatalf("Directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("Expected directory, got file")
		}
	})
	
	t.Run("Remove", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		
		// Create a file first
		err := WriteFile(ctx, fs, "test/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		
		// Remove it
		err = Remove(ctx, fs, "test/file.txt")
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}
		
		// Verify file was removed
		_, err = fs.Stat("test/file.txt")
		if err == nil {
			t.Error("File should have been removed")
		}
	})
	
	t.Run("WriteFile with validation error", func(t *testing.T) {
		fs := filesystem.NewTestFileSystem()
		
		// Try to write with empty path
		err := WriteFile(ctx, fs, "", []byte("content"), 0644)
		if err == nil {
			t.Error("Expected validation error for empty path")
		}
		if !strings.Contains(err.Error(), "path cannot be empty") {
			t.Errorf("Expected 'path cannot be empty' error, got: %v", err)
		}
	})
}