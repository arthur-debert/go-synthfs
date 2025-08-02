package synthfs

import (
	"context"
	"runtime"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestArchivePatterns(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("CreateArchive convenience function", func(t *testing.T) {
		ResetSequenceCounter()

		op := sfs.CreateArchive("/tmp/backup.zip", "file1.txt", "file2.txt", "data/")

		if op.ID() != "create_archive-1" {
			t.Errorf("Expected ID 'create_archive-1', got %s", op.ID())
		}

		desc := op.Describe()
		if desc.Type != "create_archive" {
			t.Errorf("Expected type 'create_archive', got %s", desc.Type)
		}

		if desc.Path != "/tmp/backup.zip" {
			t.Errorf("Expected path '/tmp/backup.zip', got %s", desc.Path)
		}
	})

	t.Run("Archive format detection", func(t *testing.T) {
		tests := []struct {
			path       string
			wantFormat string
		}{
			{"archive.zip", "zip"},
			{"archive.tar", "tar.gz"}, // We map tar to tar.gz
			{"archive.tar.gz", "tar.gz"},
			{"archive.tgz", "tar.gz"},
			{"archive.txt", "zip"}, // Default
		}

		for _, tt := range tests {
			format := detectArchiveFormat(tt.path)
			if format.String() != tt.wantFormat {
				t.Errorf("detectArchiveFormat(%q) = %q, want %q", tt.path, format.String(), tt.wantFormat)
			}
		}
	})

	t.Run("CreateZipArchive", func(t *testing.T) {
		ResetSequenceCounter()
		op := sfs.CreateZipArchive("/backup.zip", "src/", "README.md")

		desc := op.Describe()
		if desc.Type != "create_archive" {
			t.Errorf("Expected type 'create_archive', got %s", desc.Type)
		}
	})

	t.Run("ExtractArchive", func(t *testing.T) {
		ResetSequenceCounter()
		op := sfs.ExtractArchive("/backup.zip", "/tmp/extracted")

		if op.ID() != "unarchive-1" {
			t.Errorf("Expected ID 'unarchive-1', got %s", op.ID())
		}

		desc := op.Describe()
		if desc.Type != "unarchive" {
			t.Errorf("Expected type 'unarchive', got %s", desc.Type)
		}
	})

	t.Run("Direct archive execution", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SynthFS does not officially support Windows")
		}
		ctx := context.Background()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)

		// Create some test files
		_ = fs.WriteFile("file1.txt", []byte("content1"), 0644)
		_ = fs.WriteFile("file2.txt", []byte("content2"), 0644)

		// Create archive
		err := Archive(ctx, fs, "backup.zip", "file1.txt", "file2.txt")
		if err != nil {
			t.Fatalf("Archive failed: %v", err)
		}

		// Verify archive was created
		if _, err := fs.Stat("backup.zip"); err != nil {
			t.Error("Archive should exist")
		}
	})

	t.Run("ArchiveBuilder", func(t *testing.T) {
		ResetSequenceCounter()

		op := NewArchiveBuilder("/backup.tar.gz").
			AddSource("src/").
			AddSources("README.md", "LICENSE").
			AsTarGz().
			Build()

		desc := op.Describe()
		if desc.Type != "create_archive" {
			t.Errorf("Expected type 'create_archive', got %s", desc.Type)
		}

		if desc.Path != "/backup.tar.gz" {
			t.Errorf("Expected path '/backup.tar.gz', got %s", desc.Path)
		}
	})

	t.Run("ArchiveBuilder execution", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("SynthFS does not officially support Windows")
		}
		ctx := context.Background()
		tempDir := t.TempDir()
		osFS := filesystem.NewOSFileSystem(tempDir)
		fs := NewPathAwareFileSystem(osFS, tempDir)

		// Create test files
		_ = fs.MkdirAll("src", 0755)
		_ = fs.WriteFile("src/main.go", []byte("package main"), 0644)
		_ = fs.WriteFile("README.md", []byte("# Project"), 0644)

		// Build and execute archive
		err := NewArchiveBuilder("project.zip").
			AddSource("src/main.go").
			AddSource("README.md").
			AsZip().
			Execute(ctx, fs)

		if err != nil {
			t.Fatalf("ArchiveBuilder execution failed: %v", err)
		}

		// Verify archive exists
		if _, err := fs.Stat("project.zip"); err != nil {
			t.Error("Archive should exist")
		}
	})

	t.Run("ExtractBuilder", func(t *testing.T) {
		ResetSequenceCounter()

		op := NewExtractBuilder("/archive.zip").
			To("/extracted").
			WithPattern("*.txt").
			WithPatterns("*.md", "*.go").
			Build()

		desc := op.Describe()
		if desc.Type != "unarchive" {
			t.Errorf("Expected type 'unarchive', got %s", desc.Type)
		}
	})

	t.Run("ExtractBuilder with OnlyFiles", func(t *testing.T) {
		ResetSequenceCounter()

		op := NewExtractBuilder("/data.tar").
			To("/output").
			OnlyFiles("*.json", "*.yaml").
			Build()

		desc := op.Describe()
		if desc.Type != "unarchive" {
			t.Errorf("Expected type 'unarchive', got %s", desc.Type)
		}
	})
}
