package operations_test

import (
	"context"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

// TestArchiveValidationEdgeCases targets the low coverage areas in archive validation
func TestArchiveValidationEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateArchive validation with no sources from item or details", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.zip")

		// Don't set any sources - should fail validation
		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for no sources")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "must specify at least one source") {
			t.Errorf("Expected 'must specify at least one source' error, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateArchive validation with non-existent source files", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.zip")

		// Set sources that don't exist
		op.SetDescriptionDetail("sources", []string{"nonexistent1.txt", "nonexistent2.txt"})

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for non-existent sources")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "source does not exist") {
			t.Errorf("Expected 'source does not exist' error, got: %s", valErr.Reason)
		}
	})

	t.Run("CreateArchive validation with mixed existing and non-existent sources", func(t *testing.T) {
		fs := NewMockFilesystem()

		// Create one existing file
		if err := fs.WriteFile("existing.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test.zip")

		// Set mix of existing and non-existent sources
		op.SetDescriptionDetail("sources", []string{"existing.txt", "nonexistent.txt"})

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for non-existent source")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "nonexistent.txt") {
			t.Errorf("Expected error to mention 'nonexistent.txt', got: %s", valErr.Reason)
		}
	})

	t.Run("Unarchive validation with missing item", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.zip")

		// Don't set item - should fail validation
		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for missing item")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "unarchive operation requires an UnarchiveItem") {
			t.Errorf("Expected 'requires an UnarchiveItem' error, got: %s", valErr.Reason)
		}
	})

	t.Run("Unarchive validation with item missing required interfaces", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.zip")

		// Set item that doesn't implement required interfaces
		op.SetItem(&TestFileItem{path: "test.zip", content: []byte("data"), mode: 0644})

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for wrong item type")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "expected UnarchiveItem but got different type") {
			t.Errorf("Expected 'expected UnarchiveItem' error, got: %s", valErr.Reason)
		}
	})

	t.Run("Unarchive validation with empty archive path", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.zip")

		// Set item with empty archive path
		unarchiveItem := &TestUnarchiveItem{
			archivePath: "", // Empty path
			extractPath: "extract",
			patterns:    []string{},
		}
		op.SetItem(unarchiveItem)

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for empty archive path")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "archive path cannot be empty") {
			t.Errorf("Expected 'archive path cannot be empty' error, got: %s", valErr.Reason)
		}
	})

	t.Run("Unarchive validation with empty extract path", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "test.zip")

		// Set item with empty extract path
		unarchiveItem := &TestUnarchiveItem{
			archivePath: "test.zip",
			extractPath: "", // Empty path
			patterns:    []string{},
		}
		op.SetItem(unarchiveItem)

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for empty extract path")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "extract path cannot be empty") {
			t.Errorf("Expected 'extract path cannot be empty' error, got: %s", valErr.Reason)
		}
	})

	t.Run("Unarchive validation with unsupported archive format", func(t *testing.T) {
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

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for unsupported format")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "unsupported archive format") {
			t.Errorf("Expected 'unsupported archive format' error, got: %s", valErr.Reason)
		}
	})

	t.Run("Unarchive validation with non-existent archive file", func(t *testing.T) {
		fs := NewMockFilesystem()
		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "nonexistent.zip")

		unarchiveItem := &TestUnarchiveItem{
			archivePath: "nonexistent.zip",
			extractPath: "extract",
			patterns:    []string{},
		}
		op.SetItem(unarchiveItem)

		err := op.Validate(ctx, nil, fs)

		if err == nil {
			t.Error("Expected validation error for non-existent archive")
		}

		valErr, ok := err.(*core.ValidationError)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		} else if !strings.Contains(valErr.Reason, "archive does not exist") {
			t.Errorf("Expected 'archive does not exist' error, got: %s", valErr.Reason)
		}
	})

	t.Run("Unarchive validation with supported archive formats", func(t *testing.T) {
		fs := NewMockFilesystem()

		supportedFormats := []string{".zip", ".tar", ".gz", ".tar.gz", ".tgz"}

		for _, ext := range supportedFormats {
			filename := "archive" + ext

			// Create archive file
			if err := fs.WriteFile(filename, []byte("data"), 0644); err != nil {
				t.Fatalf("Failed to create archive file %s: %v", filename, err)
			}

			op := operations.NewUnarchiveOperation(core.OperationID("test-op"), filename)

			unarchiveItem := &TestUnarchiveItem{
				archivePath: filename,
				extractPath: "extract",
				patterns:    []string{},
			}
			op.SetItem(unarchiveItem)

			err := op.Validate(ctx, nil, fs)

			if err != nil {
				t.Errorf("Unexpected validation error for supported format %s: %v", ext, err)
			}
		}
	})
}

// TestUnarchiveItem helper for testing unarchive operations
type TestUnarchiveItem struct {
	archivePath string
	extractPath string
	patterns    []string
	overwrite   bool
}

func (t *TestUnarchiveItem) Path() string        { return t.archivePath }
func (t *TestUnarchiveItem) Type() string        { return "unarchive" }
func (t *TestUnarchiveItem) ArchivePath() string { return t.archivePath }
func (t *TestUnarchiveItem) ExtractPath() string { return t.extractPath }
func (t *TestUnarchiveItem) Patterns() []string  { return t.patterns }
func (t *TestUnarchiveItem) Overwrite() bool     { return t.overwrite }
func (t *TestUnarchiveItem) WithPatterns(patterns []string) *TestUnarchiveItem {
	t.patterns = patterns
	return t
}
func (t *TestUnarchiveItem) WithOverwrite(overwrite bool) *TestUnarchiveItem {
	t.overwrite = overwrite
	return t
}
