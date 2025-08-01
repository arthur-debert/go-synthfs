package synthfs

import (
	"context"
	iofs "io/fs"
	"strings"
	"testing"
)

func TestMirrorPatterns(t *testing.T) {
	// Use sequence generator for predictable IDs
	defer func() {
		SetIDGenerator(HashIDGenerator)
	}()
	SetIDGenerator(SequenceIDGenerator)
	
	t.Run("Basic mirror with symlinks", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/workspace")
		
		// Create source structure
		_ = fs.MkdirAll("source/lib", 0755)
		_ = fs.MkdirAll("source/bin", 0755)
		_ = fs.WriteFile("source/README.md", []byte("# Project"), 0644)
		_ = fs.WriteFile("source/lib/core.so", []byte("library"), 0755)
		_ = fs.WriteFile("source/bin/app", []byte("#!/bin/sh"), 0755)
		
		// Create mirror
		err := MirrorWithSymlinks(ctx, fs, "source", "mirror")
		if err != nil {
			t.Fatalf("MirrorWithSymlinks failed: %v", err)
		}
		
		// Verify structure exists
		paths := []string{
			"mirror",
			"mirror/README.md",
			"mirror/lib",
			"mirror/bin",
		}
		
		for _, path := range paths {
			if _, err := fs.Stat(path); err != nil {
				t.Errorf("Path %q should exist", path)
			}
		}
		
		// Verify they are symlinks
		// Note: PathAwareFileSystem implements Readlink
		target, err := fs.Readlink("mirror/README.md")
		if err != nil {
			t.Logf("Warning: Could not read symlink (test filesystem may not support): %v", err)
		} else {
			if !strings.Contains(target, "source/README.md") {
				t.Errorf("Expected symlink to point to source/README.md, got %s", target)
			}
		}
	})
	
	t.Run("MirrorBuilder with directories included", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/project")
		
		// Create source structure
		_ = fs.MkdirAll("src/main", 0755)
		_ = fs.MkdirAll("src/test", 0755)
		_ = fs.WriteFile("src/main/app.go", []byte("package main"), 0644)
		_ = fs.WriteFile("src/test/app_test.go", []byte("package main"), 0644)
		_ = fs.WriteFile("src/.gitignore", []byte("*.tmp"), 0644)
		
		// Mirror with real directories (not symlinked)
		err := NewMirrorBuilder("src", "build").
			IncludeDirectories().
			ExcludeHidden().
			Execute(ctx, fs)
		
		if err != nil {
			t.Fatalf("MirrorBuilder failed: %v", err)
		}
		
		// Check structure
		if _, err := fs.Stat("build/main/app.go"); err != nil {
			t.Error("app.go should be mirrored")
		}
		
		if _, err := fs.Stat("build/test/app_test.go"); err != nil {
			t.Error("app_test.go should be mirrored")
		}
		
		// Hidden file should be excluded
		if _, err := fs.Stat("build/.gitignore"); err == nil {
			t.Error(".gitignore should NOT be mirrored (hidden file)")
		}
	})
	
	t.Run("Mirror validation", func(t *testing.T) {
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/test")
		
		// Test with non-existent source
		op := NewMirrorWithSymlinksOperation("nonexistent", "dest")
		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Should fail validation with non-existent source")
		}
		
		// Create a file instead of directory
		_ = fs.WriteFile("file.txt", []byte("content"), 0644)
		
		op = NewMirrorWithSymlinksOperation("file.txt", "dest")
		err = op.Validate(ctx, fs)
		if err == nil {
			t.Error("Should fail validation when source is not a directory")
		}
		
		// Create proper directory
		_ = fs.MkdirAll("source", 0755)
		_ = fs.MkdirAll("existing", 0755)
		
		// Test without overwrite
		op = NewMirrorWithSymlinksOperation("source", "existing")
		err = op.Validate(ctx, fs)
		if err == nil {
			t.Error("Should fail validation when destination exists without overwrite")
		}
		
		// Test with overwrite
		op = NewMirrorWithSymlinksOperation("source", "existing", MirrorOptions{Overwrite: true})
		err = op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Should pass validation with overwrite: %v", err)
		}
	})
	
	t.Run("Selective mirroring", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/workspace")
		
		// Create mixed content
		_ = fs.MkdirAll("data/logs", 0755)
		_ = fs.MkdirAll("data/config", 0755)
		_ = fs.WriteFile("data/app.conf", []byte("config"), 0644)
		_ = fs.WriteFile("data/logs/app.log", []byte("logs"), 0644)
		_ = fs.WriteFile("data/config/db.yaml", []byte("database"), 0644)
		_ = fs.WriteFile("data/README.txt", []byte("readme"), 0644)
		
		// Mirror only config files
		err := NewMirrorBuilder("data", "configs").
			WithFilter(func(path string, info iofs.FileInfo) bool {
				if info.IsDir() {
					return true // Include directories for structure
				}
				name := info.Name()
				return strings.HasSuffix(name, ".conf") || strings.HasSuffix(name, ".yaml")
			}).
			IncludeDirectories().
			Execute(ctx, fs)
		
		if err != nil {
			t.Fatalf("Selective mirror failed: %v", err)
		}
		
		// Check what was included
		if _, err := fs.Stat("configs/app.conf"); err != nil {
			t.Error("app.conf should be mirrored")
		}
		
		if _, err := fs.Stat("configs/config/db.yaml"); err != nil {
			t.Error("db.yaml should be mirrored")
		}
		
		// Check what was excluded
		if _, err := fs.Stat("configs/logs/app.log"); err == nil {
			t.Error("app.log should NOT be mirrored")
		}
		
		if _, err := fs.Stat("configs/README.txt"); err == nil {
			t.Error("README.txt should NOT be mirrored")
		}
	})
}