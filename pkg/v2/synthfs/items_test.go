package synthfs_test

import (
	"io/fs"
	"reflect"
	"testing"

	v2 "github.com/arthur-debert/synthfs/pkg/v2/synthfs"
)

func TestFileItem(t *testing.T) {
	filePath := "/tmp/testfile.txt"
	fileContent := []byte("hello world")
	fileMode := fs.FileMode(0600)

	file := v2.NewFile(filePath).
		WithContent(fileContent).
		WithMode(fileMode)

	if file.Path() != filePath {
		t.Errorf("Expected path %s, got %s", filePath, file.Path())
	}
	if file.Type() != "file" {
		t.Errorf("Expected type 'file', got %s", file.Type())
	}
	if !reflect.DeepEqual(file.Content(), fileContent) {
		t.Errorf("Expected content %v, got %v", fileContent, file.Content())
	}
	if file.Mode() != fileMode {
		t.Errorf("Expected mode %v, got %v", fileMode, file.Mode())
	}

	// Test default mode
	defaultModeFile := v2.NewFile("default.txt")
	if defaultModeFile.Mode() != 0644 {
		t.Errorf("Expected default mode 0644, got %v", defaultModeFile.Mode())
	}
}

func TestDirectoryItem(t *testing.T) {
	dirPath := "/tmp/testdir"
	dirMode := fs.FileMode(0700)

	dir := v2.NewDirectory(dirPath).
		WithMode(dirMode)

	if dir.Path() != dirPath {
		t.Errorf("Expected path %s, got %s", dirPath, dir.Path())
	}
	if dir.Type() != "directory" {
		t.Errorf("Expected type 'directory', got %s", dir.Type())
	}
	if dir.Mode() != dirMode {
		t.Errorf("Expected mode %v, got %v", dirMode, dir.Mode())
	}

	// Test default mode
	defaultModeDir := v2.NewDirectory("defaultdir")
	if defaultModeDir.Mode() != 0755 {
		t.Errorf("Expected default mode 0755, got %v", defaultModeDir.Mode())
	}
}

func TestSymlinkItem(t *testing.T) {
	linkPath := "/tmp/testlink"
	targetPath := "/tmp/originalfile"

	link := v2.NewSymlink(linkPath, targetPath)

	if link.Path() != linkPath {
		t.Errorf("Expected path %s, got %s", linkPath, link.Path())
	}
	if link.Type() != "symlink" {
		t.Errorf("Expected type 'symlink', got %s", link.Type())
	}
	if link.Target() != targetPath {
		t.Errorf("Expected target %s, got %s", targetPath, link.Target())
	}
}

func TestArchiveItem(t *testing.T) {
	archivePath := "/tmp/testarchive.tar.gz"
	sources := []string{"/tmp/file1", "/tmp/dir1"}
	format := v2.ArchiveFormatTarGz

	archive := v2.NewArchive(archivePath, format, sources)

	if archive.Path() != archivePath {
		t.Errorf("Expected path %s, got %s", archivePath, archive.Path())
	}
	if archive.Type() != "archive" {
		t.Errorf("Expected type 'archive', got %s", archive.Type())
	}
	if archive.Format() != format {
		t.Errorf("Expected format %s, got %s", format, archive.Format())
	}
	if !reflect.DeepEqual(archive.Sources(), sources) {
		t.Errorf("Expected sources %v, got %v", sources, archive.Sources())
	}

	// Test WithSources
	newSources := []string{"/tmp/file2"}
	archive.WithSources(newSources)
	if !reflect.DeepEqual(archive.Sources(), newSources) {
		t.Errorf("Expected sources %v after WithSources, got %v", newSources, archive.Sources())
	}
}
