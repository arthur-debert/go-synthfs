package operations_test

import (
	"context"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// TestArchiveExecutionErrorPaths targets the low coverage in archive Execute methods
func TestArchiveExecutionErrorPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateArchive execution with no sources", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.zip")

		// Don't set sources - should fail during execution
		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected execution error for no sources")
		}

		if !strings.Contains(err.Error(), "create_archive operation requires sources") {
			t.Errorf("Expected 'requires sources' error, got: %s", err.Error())
		}
	})

	t.Run("CreateArchive execution with no format", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file
		if err := fs.WriteFile("source.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.unknown")
		op.SetDescriptionDetail("sources", []string{"source.txt"})
		// Don't set format - should fail during execution

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected execution error for no format")
		}

		if !strings.Contains(err.Error(), "create_archive operation requires format") {
			t.Errorf("Expected 'requires format' error, got: %s", err.Error())
		}
	})

	t.Run("CreateArchive execution with unsupported format string", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file
		if err := fs.WriteFile("source.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.unsupported")
		op.SetDescriptionDetail("sources", []string{"source.txt"})
		op.SetDescriptionDetail("format", "unsupported_format")

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected execution error for unsupported format")
		}

		if !strings.Contains(err.Error(), "unsupported archive format") {
			t.Errorf("Expected 'unsupported archive format' error, got: %s", err.Error())
		}
	})

	t.Run("CreateArchive execution with unsupported file extension", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file
		if err := fs.WriteFile("source.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.unsupported")
		op.SetDescriptionDetail("sources", []string{"source.txt"})
		op.SetDescriptionDetail("format", "other_format") // Will fallback to extension

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected execution error for unsupported extension")
		}

		if !strings.Contains(err.Error(), "unsupported archive format") {
			t.Errorf("Expected 'unsupported archive format' error, got: %s", err.Error())
		}
	})

	t.Run("CreateArchive execution with format from item", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file
		if err := fs.WriteFile("source.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.zip")

		// Set archive item with sources and format
		archiveItem := &TestArchiveItem{
			path:    "test.zip",
			sources: []string{"source.txt"},
			format:  "zip",
		}
		op.SetItem(archiveItem)

		err := op.Execute(ctx, fs)

		if err != nil {
			t.Errorf("Expected no execution error for valid zip creation, got: %v", err)
		}

		// Verify archive was created
		if _, ok := fs.Files()["test.zip"]; !ok {
			t.Error("Archive file was not created")
		}
	})

	t.Run("CreateArchive tar format execution", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file
		if err := fs.WriteFile("source.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.tar")
		op.SetDescriptionDetail("sources", []string{"source.txt"})
		op.SetDescriptionDetail("format", "tar")

		err := op.Execute(ctx, fs)

		if err != nil {
			t.Errorf("Expected no execution error for tar creation, got: %v", err)
		}

		// Verify archive was created
		if _, ok := fs.Files()["test.tar"]; !ok {
			t.Error("TAR archive file was not created")
		}
	})

	t.Run("CreateArchive tar.gz format execution", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create source file
		if err := fs.WriteFile("source.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.tar.gz")
		op.SetDescriptionDetail("sources", []string{"source.txt"})
		op.SetDescriptionDetail("format", "tar.gz")

		err := op.Execute(ctx, fs)

		if err != nil {
			t.Errorf("Expected no execution error for tar.gz creation, got: %v", err)
		}

		// Verify archive was created
		if _, ok := fs.Files()["test.tar.gz"]; !ok {
			t.Error("TAR.GZ archive file was not created")
		}
	})

	t.Run("Unarchive execution with unsupported extension", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create archive file with unsupported extension
		if err := fs.WriteFile("test.unsupported", []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create archive file: %v", err)
		}

		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.unsupported")

		unarchiveItem := &TestUnarchiveItem{
			archivePath: "test.unsupported",
			extractPath: "extract",
			patterns:    []string{},
		}
		op.SetItem(unarchiveItem)

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected execution error for unsupported format")
		}

		if !strings.Contains(err.Error(), "unsupported archive format") {
			t.Errorf("Expected 'unsupported archive format' error, got: %s", err.Error())
		}
	})

	t.Run("Unarchive execution with .gz file that's not tar.gz", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create .gz file that's not tar.gz
		if err := fs.WriteFile("test.gz", []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create .gz file: %v", err)
		}

		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.gz")

		unarchiveItem := &TestUnarchiveItem{
			archivePath: "test.gz",
			extractPath: "extract",
			patterns:    []string{},
		}
		op.SetItem(unarchiveItem)

		err := op.Execute(ctx, fs)

		if err == nil {
			t.Error("Expected execution error for non-tar .gz file")
		}

		if !strings.Contains(err.Error(), "unsupported archive format") {
			t.Errorf("Expected 'unsupported archive format' error, got: %s", err.Error())
		}
	})

	t.Run("Unarchive execution with extract path from details", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create minimal zip archive
		if err := fs.WriteFile("test.zip", []byte("test zip data"), 0644); err != nil {
			t.Fatalf("Failed to create zip file: %v", err)
		}

		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.zip")

		// Set extract path in details instead of item
		op.SetDescriptionDetail("extract_path", "extract_dir")

		// Don't set proper UnarchiveItem to test fallback
		op.SetItem(&TestFileItem{path: "test.zip", content: []byte("data"), mode: 0644})

		err := op.Execute(ctx, fs)

		// Should not fail due to extract path source (though may fail due to invalid zip data)
		if err != nil && strings.Contains(err.Error(), "extract path") {
			t.Errorf("Should not fail due to extract path resolution, got: %v", err)
		}
	})

	t.Run("Unarchive execution with default extract path", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create minimal zip archive
		if err := fs.WriteFile("test.zip", []byte("test zip data"), 0644); err != nil {
			t.Fatalf("Failed to create zip file: %v", err)
		}

		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.zip")

		// Don't set extract path anywhere - should default to "."
		op.SetItem(&TestFileItem{path: "test.zip", content: []byte("data"), mode: 0644})

		err := op.Execute(ctx, fs)

		// Should not fail due to extract path (defaults to current directory)
		if err != nil && strings.Contains(err.Error(), "extract path") {
			t.Errorf("Should not fail due to extract path (should default to current dir), got: %v", err)
		}
	})

	t.Run("Unarchive execution with patterns from details", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create minimal zip archive
		if err := fs.WriteFile("test.zip", []byte("test zip data"), 0644); err != nil {
			t.Fatalf("Failed to create zip file: %v", err)
		}

		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.zip")

		// Set patterns in details instead of item
		op.SetDescriptionDetail("patterns", []string{"*.txt"})
		op.SetDescriptionDetail("extract_path", "extract")

		// Don't set proper UnarchiveItem to test fallback
		op.SetItem(&TestFileItem{path: "test.zip", content: []byte("data"), mode: 0644})

		err := op.Execute(ctx, fs)

		// Should not fail due to pattern source (though may fail due to invalid zip data)
		if err != nil && strings.Contains(err.Error(), "pattern") {
			t.Errorf("Should not fail due to pattern resolution, got: %v", err)
		}
	})
}

// TestArchiveItem helper for testing archive operations
type TestArchiveItem struct {
	path    string
	sources []string
	format  interface{}
}

func (t *TestArchiveItem) Path() string        { return t.path }
func (t *TestArchiveItem) Type() string        { return "archive" }
func (t *TestArchiveItem) Sources() []string   { return t.sources }
func (t *TestArchiveItem) Format() interface{} { return t.format }
