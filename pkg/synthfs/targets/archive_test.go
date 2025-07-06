package targets_test

import (
	"reflect"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/targets"
)

func TestArchiveItem(t *testing.T) {
	archivePath := "/tmp/testarchive.tar.gz"
	sources := []string{"/tmp/file1", "/tmp/dir1"}
	format := targets.ArchiveFormatTarGz

	archive := targets.NewArchive(archivePath, format, sources)

	if archive.Path() != archivePath {
		t.Errorf("Expected path %s, got %s", archivePath, archive.Path())
	}
	if archive.Type() != "archive" {
		t.Errorf("Expected type 'archive', got %s", archive.Type())
	}
	if archive.Format() != format {
		t.Errorf("Expected format %v, got %v", format, archive.Format())
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
