package operations_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestArchiveOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("create archive validation with no sources", func(t *testing.T) {
		fs := NewMockFilesystem()

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "test/archive.tar.gz")
		// Don't set sources in description - should fail validation

		err := op.Validate(ctx, nil, fs)
		if err == nil {
			t.Error("Expected validation error for missing sources")
		}
	})

	t.Run("create zip archive with files", func(t *testing.T) {

		fs := NewExtendedMockFilesystem()

		// Create test files
		if err := fs.WriteFile("file1.txt", []byte("content1"), 0644); err != nil {
			t.Fatalf("Failed to write file1.txt: %v", err)
		}
		if err := fs.WriteFile("file2.txt", []byte("content2"), 0644); err != nil {
			t.Fatalf("Failed to write file2.txt: %v", err)
		}

		op := operations.NewCreateArchiveOperation(core.OperationID("test-op"), "archive.zip")
		// Set archive format and sources in description
		op.SetDescriptionDetail("format", "zip")
		op.SetDescriptionDetail("sources", []string{"file1.txt", "file2.txt"})

		err := op.Execute(ctx, nil, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify archive was created
		archiveData, ok := fs.Files()["archive.zip"]
		if !ok {
			t.Fatal("Archive file was not created")
		}

		// Verify archive contents
		zipReader, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
		if err != nil {
			t.Fatalf("Failed to read zip archive: %v", err)
		}

		if len(zipReader.File) != 2 {
			t.Errorf("Expected 2 files in archive, got %d", len(zipReader.File))
		}

		// Check file contents
		expectedFiles := map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		}

		for _, file := range zipReader.File {
			expectedContent, ok := expectedFiles[file.Name]
			if !ok {
				t.Errorf("Unexpected file in archive: %s", file.Name)
				continue
			}

			reader, err := file.Open()
			if err != nil {
				t.Errorf("Failed to open %s in archive: %v", file.Name, err)
				continue
			}

			content, err := io.ReadAll(reader)
			if closeErr := reader.Close(); closeErr != nil {
				t.Errorf("Failed to close reader for %s: %v", file.Name, closeErr)
			}
			if err != nil {
				t.Errorf("Failed to read %s in archive: %v", file.Name, err)
				continue
			}

			if string(content) != expectedContent {
				t.Errorf("Wrong content for %s: expected %q, got %q", file.Name, expectedContent, string(content))
			}
		}
	})

	t.Run("unarchive zip file", func(t *testing.T) {

		fs := NewExtendedMockFilesystem()

		// Create a zip archive
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)

		f1, err := w.Create("extracted/file1.txt")
		if err != nil {
			t.Fatalf("Failed to create file1 in zip: %v", err)
		}
		if _, err := f1.Write([]byte("content1")); err != nil {
			t.Fatalf("Failed to write content1: %v", err)
		}

		f2, err := w.Create("extracted/file2.txt")
		if err != nil {
			t.Fatalf("Failed to create file2 in zip: %v", err)
		}
		if _, err := f2.Write([]byte("content2")); err != nil {
			t.Fatalf("Failed to write content2: %v", err)
		}

		if err := w.Close(); err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}

		// Write archive to filesystem
		if err := fs.WriteFile("archive.zip", buf.Bytes(), 0644); err != nil {
			t.Fatalf("Failed to write archive.zip: %v", err)
		}

		op := operations.NewUnarchiveOperation(core.OperationID("test-op"), "archive.zip")
		// Set extract_path in description
		op.SetDescriptionDetail("extract_path", "output")

		if err := op.Execute(ctx, nil, fs); err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify files were extracted
		content1, ok := fs.Files()["output/extracted/file1.txt"]
		if !ok {
			t.Error("Expected output/extracted/file1.txt to be extracted")
		} else if string(content1) != "content1" {
			t.Errorf("Wrong content for file1.txt: %q", string(content1))
		}

		content2, ok := fs.Files()["output/extracted/file2.txt"]
		if !ok {
			t.Error("Expected output/extracted/file2.txt to be extracted")
		} else if string(content2) != "content2" {
			t.Errorf("Wrong content for file2.txt: %q", string(content2))
		}
	})
}
