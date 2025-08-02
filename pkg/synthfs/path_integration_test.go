package synthfs

import (
	"context"
	"runtime"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestPathHandlingIntegration(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("Convenience API with path-aware filesystem", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()

		if runtime.GOOS == "windows" {
			t.Skip("SynthFS does not officially support Windows")
		}
		// Create a path-aware filesystem
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)

		// Test direct execution with various path styles

		// Create data directory first
		err := MkdirAll(ctx, fs, "data", 0755)
		if err != nil {
			t.Fatalf("Failed to create data directory: %v", err)
		}

		// Relative path
		err = WriteFile(ctx, fs, "data/file1.txt", []byte("content1"), 0644)
		if err != nil {
			t.Fatalf("Failed to write with relative path: %v", err)
		}

		// Another relative path (avoiding absolute path issues)
		err = WriteFile(ctx, fs, "data/file2.txt", []byte("content2"), 0644)
		if err != nil {
			t.Fatalf("Failed to write file2: %v", err)
		}

		// Path with ./
		err = MkdirAll(ctx, fs, "./config", 0755)
		if err != nil {
			t.Fatalf("Failed to create dir with ./ path: %v", err)
		}

		// Verify files exist using different path styles
		if _, err := fs.Stat("data/file1.txt"); err != nil {
			t.Error("file1 should exist")
		}
		if _, err := fs.Stat("data/file2.txt"); err != nil {
			t.Error("file2 should exist")
		}
		if _, err := fs.Stat("./config"); err != nil {
			t.Error("config dir should exist")
		}
	})

	t.Run("SimpleBatch with path-aware filesystem", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SynthFS does not officially support Windows")
		}
		ResetSequenceCounter()
		ctx := context.Background()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)

		batch := NewSimpleBatch(fs).WithContext(ctx)

		// Mix of path styles in batch
		batch.
			CreateDir("src", 0755).                                    // relative
			WriteFile("src/main.go", []byte("package main"), 0644).   // relative
			WriteFile("./src/utils.go", []byte("package main"), 0644). // ./ prefix
			Copy("src/main.go", "src/main.go.bak")                     // relative

		err := batch.Execute()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}

		// Verify all operations succeeded
		files := []string{
			"src/main.go",
			"src/utils.go",
			"./src/main.go.bak",
		}

		for _, file := range files {
			if _, err := fs.Stat(file); err != nil {
				t.Errorf("File %q should exist: %v", file, err)
			}
		}
	})

	t.Run("Pipeline with path normalization", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SynthFS does not officially support Windows")
		}
		ResetSequenceCounter()
		ctx := context.Background()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir).WithAutoDetectPaths()

		// Create operations with unnormalized paths
		op1 := sfs.CreateDir("path//to///dir", 0755)
		op2 := sfs.CreateFile("./path/to/../to/dir/file.txt", []byte("content"), 0644)
		op3 := sfs.Copy("path/to/dir/file.txt", "path/to/dir/backup.txt")

		_, err := Run(ctx, fs, op1, op2, op3)
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		// All these should resolve to the same file
		paths := []string{
			"path/to/dir/file.txt",
			"./path/to/dir/file.txt",
		}

		for _, path := range paths {
			if _, err := fs.Stat(path); err != nil {
				t.Errorf("Path %q should resolve to the same file: %v", path, err)
			}
		}
		
		// Also check the backup was created
		if _, err := fs.Stat("path/to/dir/backup.txt"); err != nil {
			t.Error("Backup file should exist")
		}
	})

	t.Run("Security test with convenience API", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SynthFS does not officially support Windows")
		}
		ctx := context.Background()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)

		// These should all fail with proper error messages
		maliciousPaths := []string{
			"../../../etc/passwd",
			"/etc/passwd",
			"files/../../../../../../../etc/shadow",
		}

		for _, path := range maliciousPaths {
			err := WriteFile(ctx, fs, path, []byte("evil"), 0644)
			if err == nil {
				t.Errorf("Expected security error for path %q", path)
			}

			// Check that we get a proper error message
			if opErr, ok := err.(*OperationError); ok {
				t.Logf("Got proper error for %q: %v", path, opErr)
			}
		}
	})

	t.Run("Path mode switching", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SynthFS does not officially support Windows")
		}
		ctx := context.Background()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)

		// Start with auto mode
		fs := NewPathAwareFileSystem(osFS, tempDir)

		// This works - relative path
		err := WriteFile(ctx, fs, "file1.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Auto mode failed: %v", err)
		}

		// Switch to absolute mode
		fs = fs.WithAbsolutePaths()

		// Now relative paths should fail or be interpreted differently
		err = WriteFile(ctx, fs, "file2.txt", []byte("content"), 0644)
		if err == nil {
			// In absolute mode, "file2.txt" becomes "/file2.txt" which is outside base
			t.Error("Expected error in absolute mode with relative path")
		}

		// But absolute paths within base should work
		err = WriteFile(ctx, fs, tempDir+"/file2.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Absolute mode with proper path failed: %v", err)
		}

		// Switch to relative mode
		fs = fs.WithRelativePaths()

		// Create subdir first
		err = MkdirAll(ctx, fs, "subdir", 0755)
		if err != nil {
			t.Fatalf("Failed to create subdir: %v", err)
		}

		// Now absolute paths are treated as relative
		err = WriteFile(ctx, fs, "/subdir/file3.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Relative mode failed: %v", err)
		}

		// The file should be at /data/subdir/file3.txt
		if _, err := fs.Stat("subdir/file3.txt"); err != nil {
			t.Error("File should exist at relative path")
		}
	})
}
