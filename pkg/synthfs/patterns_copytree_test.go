package synthfs

import (
	"context"
	iofs "io/fs"
	"strings"
	"testing"
)

func TestCopyTreePatterns(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("Basic copy tree", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/workspace")

		// Create source structure
		_ = fs.MkdirAll("src/lib", 0755)
		_ = fs.MkdirAll("src/tests", 0755)
		_ = fs.WriteFile("src/main.go", []byte("package main"), 0644)
		_ = fs.WriteFile("src/lib/utils.go", []byte("package lib"), 0644)
		_ = fs.WriteFile("src/tests/main_test.go", []byte("package main"), 0644)
		_ = fs.WriteFile("src/.gitignore", []byte("*.tmp"), 0644)

		// Copy tree
		err := CopyTreeFunc(ctx, fs, "src", "backup")
		if err != nil {
			t.Fatalf("CopyTree failed: %v", err)
		}

		// Verify all files were copied
		files := []string{
			"backup/main.go",
			"backup/lib/utils.go",
			"backup/tests/main_test.go",
			"backup/.gitignore",
		}

		for _, file := range files {
			if _, err := fs.Stat(file); err != nil {
				t.Errorf("File %q should exist in backup", file)
			}
		}
	})

	t.Run("CopyTreeBuilder with filters", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/project")

		// Create source structure
		_ = fs.MkdirAll("code/src", 0755)
		_ = fs.MkdirAll("code/build", 0755)
		_ = fs.WriteFile("code/src/app.js", []byte("console.log()"), 0644)
		_ = fs.WriteFile("code/src/.env", []byte("SECRET=123"), 0644)
		_ = fs.WriteFile("code/build/app.min.js", []byte("minified"), 0644)
		_ = fs.WriteFile("code/README.md", []byte("# Code"), 0644)

		// Copy with filters
		err := NewCopyTreeBuilder("code", "dist").
			ExcludeHidden().
			ExcludePattern("build").
			PreservePermissions().
			Execute(ctx, fs)

		if err != nil {
			t.Fatalf("CopyTreeBuilder failed: %v", err)
		}

		// Check what was copied
		if _, err := fs.Stat("dist/src/app.js"); err != nil {
			t.Error("app.js should be copied")
		}

		if _, err := fs.Stat("dist/README.md"); err != nil {
			t.Error("README.md should be copied")
		}

		// Check what was excluded
		if _, err := fs.Stat("dist/src/.env"); err == nil {
			t.Error(".env should NOT be copied (hidden file)")
		}

		if _, err := fs.Stat("dist/build"); err == nil {
			t.Error("build directory should NOT be copied")
		}
	})

	t.Run("CopyTreeOperation validation", func(t *testing.T) {
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/test")

		// Test with non-existent source
		op := sfs.NewCopyTreeOperation("nonexistent", "dest")
		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Should fail validation with non-existent source")
		}

		// Create a file instead of directory
		_ = fs.WriteFile("file.txt", []byte("content"), 0644)

		op = sfs.NewCopyTreeOperation("file.txt", "dest")
		err = op.Validate(ctx, fs)
		if err == nil {
			t.Error("Should fail validation when source is not a directory")
		}

		// Create proper directory
		_ = fs.MkdirAll("source", 0755)
		_ = fs.MkdirAll("existing", 0755)

		// Test without overwrite
		op = sfs.NewCopyTreeOperation("source", "existing")
		err = op.Validate(ctx, fs)
		if err == nil {
			t.Error("Should fail validation when destination exists without overwrite")
		}

		// Test with overwrite
		op = sfs.NewCopyTreeOperation("source", "existing", CopyTreeOptions{Overwrite: true})
		err = op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Should pass validation with overwrite: %v", err)
		}
	})

	t.Run("Custom filter function", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/workspace")

		// Create source files
		_ = fs.MkdirAll("project", 0755)
		_ = fs.WriteFile("project/main.go", []byte("package main"), 0644)
		_ = fs.WriteFile("project/main_test.go", []byte("package main"), 0644)
		_ = fs.WriteFile("project/README.md", []byte("# Project"), 0644)
		_ = fs.WriteFile("project/go.mod", []byte("module test"), 0644)

		// Copy only Go source files (not tests)
		filter := func(path string, info iofs.FileInfo) bool {
			if info.IsDir() {
				return true
			}
			name := info.Name()
			return len(name) > 3 && name[len(name)-3:] == ".go" && !strings.Contains(name, "_test")
		}

		err := NewCopyTreeBuilder("project", "filtered").
			WithFilter(filter).
			Execute(ctx, fs)

		if err != nil {
			t.Fatalf("CopyTree with filter failed: %v", err)
		}

		// Check results
		if _, err := fs.Stat("filtered/main.go"); err != nil {
			t.Error("main.go should be copied")
		}

		if _, err := fs.Stat("filtered/main_test.go"); err == nil {
			t.Error("main_test.go should NOT be copied")
		}

		if _, err := fs.Stat("filtered/README.md"); err == nil {
			t.Error("README.md should NOT be copied")
		}
	})
}
