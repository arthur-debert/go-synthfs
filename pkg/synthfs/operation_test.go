package synthfs

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
)

func TestSimpleOperation_GetItem(t *testing.T) {
	t.Run("GetItem returns nil when no item set", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_file", "test/path")

		item := op.GetItem()
		if item != nil {
			t.Errorf("Expected nil item, got %v", item)
		}
	})

	t.Run("GetItem returns set item", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_file", "test/file.txt")
		fileItem := NewFile("test/file.txt").WithContent([]byte("test content"))
		op.SetItem(fileItem)

		item := op.GetItem()
		if item != fileItem {
			t.Errorf("Expected fileItem, got %v", item)
		}
	})
}

func TestValidationError_Error(t *testing.T) {
	op := NewSimpleOperation("test-op", "create_file", "test/path")

	t.Run("Error without cause", func(t *testing.T) {
		err := &ValidationError{
			Operation: op,
			Reason:    "test reason",
			Cause:     nil,
		}

		expected := "validation error for operation test-op (test/path): test reason"
		if err.Error() != expected {
			t.Errorf("Expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("Error with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := &ValidationError{
			Operation: op,
			Reason:    "test reason",
			Cause:     cause,
		}

		expected := "validation error for operation test-op (test/path): test reason: underlying error"
		if err.Error() != expected {
			t.Errorf("Expected %q, got %q", expected, err.Error())
		}
	})
}

func TestValidationError_Unwrap(t *testing.T) {
	op := NewSimpleOperation("test-op", "create_file", "test/path")

	t.Run("Unwrap returns nil when no cause", func(t *testing.T) {
		err := &ValidationError{
			Operation: op,
			Reason:    "test reason",
			Cause:     nil,
		}

		unwrapped := err.Unwrap()
		if unwrapped != nil {
			t.Errorf("Expected nil, got %v", unwrapped)
		}
	})

	t.Run("Unwrap returns cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := &ValidationError{
			Operation: op,
			Reason:    "test reason",
			Cause:     cause,
		}

		unwrapped := err.Unwrap()
		if unwrapped != cause {
			t.Errorf("Expected %v, got %v", cause, unwrapped)
		}
	})
}

func TestDependencyError_Error(t *testing.T) {
	op := NewSimpleOperation("test-op", "create_file", "test/path")

	err := &DependencyError{
		Operation:    op,
		Dependencies: []OperationID{"dep1", "dep2"},
		Missing:      []OperationID{"dep1"},
	}

	expected := "dependency error for operation test-op: missing dependencies [dep1] (required: [dep1 dep2])"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestConflictError_Error(t *testing.T) {
	op := NewSimpleOperation("test-op", "create_file", "test/path")

	err := &ConflictError{
		Operation: op,
		Conflicts: []OperationID{"conflict1", "conflict2"},
	}

	expected := "conflict error for operation test-op: conflicts with operations [conflict1 conflict2]"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestSimpleOperation_Rollback(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	t.Run("Rollback create operation", func(t *testing.T) {
		// Setup: create a file first
		op := NewSimpleOperation("test-op", "create_file", "test/file.txt")
		fileItem := NewFile("test/file.txt").WithContent([]byte("test"))
		op.SetItem(fileItem)

		// Execute to create the file
		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file exists
		_, err = fs.Stat("test/file.txt")
		if err != nil {
			t.Fatalf("File should exist: %v", err)
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify file is removed
		_, err = fs.Stat("test/file.txt")
		if err == nil {
			t.Error("File should be removed after rollback")
		}
	})

	t.Run("Rollback copy operation", func(t *testing.T) {
		// Setup: create source file and copy operation
		err := fs.WriteFile("test/source.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := NewSimpleOperation("test-op", "copy", "test/source.txt")
		op.SetPaths("test/source.txt", "test/destination.txt")

		// Execute copy
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify destination exists
		_, err = fs.Stat("test/destination.txt")
		if err != nil {
			t.Fatalf("Destination should exist: %v", err)
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify destination is removed
		_, err = fs.Stat("test/destination.txt")
		if err == nil {
			t.Error("Destination should be removed after rollback")
		}

		// Verify source still exists
		_, err = fs.Stat("test/source.txt")
		if err != nil {
			t.Error("Source should still exist after copy rollback")
		}
	})

	t.Run("Rollback move operation", func(t *testing.T) {
		// Setup: create source file and move operation
		err := fs.WriteFile("test/movesource.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		op := NewSimpleOperation("test-op", "move", "test/movesource.txt")
		op.SetPaths("test/movesource.txt", "test/movedest.txt")

		// Execute move
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify file moved
		_, err = fs.Stat("test/movedest.txt")
		if err != nil {
			t.Fatalf("Destination should exist: %v", err)
		}
		_, err = fs.Stat("test/movesource.txt")
		if err == nil {
			t.Error("Source should not exist after move")
		}

		// Rollback
		err = op.Rollback(ctx, fs)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		// Verify file moved back
		_, err = fs.Stat("test/movesource.txt")
		if err != nil {
			t.Error("Source should exist after move rollback")
		}
		_, err = fs.Stat("test/movedest.txt")
		if err == nil {
			t.Error("Destination should not exist after move rollback")
		}
	})

	t.Run("Rollback delete operation returns error", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "delete", "test/file.txt")

		err := op.Rollback(ctx, fs)
		if err == nil {
			t.Error("Expected error for delete rollback")
		}

		expectedMsg := "rollback of delete operations not yet implemented"
		if err.Error() != expectedMsg {
			t.Errorf("Expected %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("Rollback unknown operation type", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "unknown", "test/path")

		err := op.Rollback(ctx, fs)
		if err == nil {
			t.Error("Expected error for unknown operation type")
		}

		expectedMsg := "unknown operation type for rollback: unknown"
		if err.Error() != expectedMsg {
			t.Errorf("Expected %q, got %q", expectedMsg, err.Error())
		}
	})
}

func TestSimpleOperation_ExecuteDelete_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("ExecuteDelete with filesystem that doesn't support Stat", func(t *testing.T) {
		// Use a basic filesystem that doesn't implement FullFileSystem
		fs := &basicFS{}
		op := NewSimpleOperation("test-op", "delete", "test/file.txt")

		// This should use the fallback path
		err := op.Execute(ctx, fs)
		// We expect an error since the file doesn't exist, but it should try Remove first
		if err == nil {
			t.Error("Expected error when deleting non-existent file")
		}
	})

	t.Run("ExecuteDelete removes directory", func(t *testing.T) {
		fs := NewTestFileSystem()

		// Create a directory
		err := fs.MkdirAll("test/dir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		op := NewSimpleOperation("test-op", "delete", "test/dir")
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify directory is removed
		_, err = fs.Stat("test/dir")
		if err == nil {
			t.Error("Directory should be removed")
		}
	})

	t.Run("ExecuteDelete removes file", func(t *testing.T) {
		fs := NewTestFileSystem()

		// Create a file
		err := fs.WriteFile("test/file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		op := NewSimpleOperation("test-op", "delete", "test/file.txt")
		err = op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify file is removed
		_, err = fs.Stat("test/file.txt")
		if err == nil {
			t.Error("File should be removed")
		}
	})
}

func TestSimpleOperation_Validation_EdgeCases(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	t.Run("ValidateCreateFile with wrong item type", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_file", "test/file.txt")
		// Set wrong item type
		dirItem := NewDirectory("test/dir")
		op.SetItem(dirItem)

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for wrong item type")
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if !strings.Contains(validationErr.Reason, "expected FileItem") {
			t.Errorf("Expected wrong type error, got: %s", validationErr.Reason)
		}
	})

	t.Run("ValidateCreateDirectory with wrong item type", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_directory", "test/dir")
		fileItem := NewFile("test/file.txt")
		op.SetItem(fileItem)

		err := op.Validate(ctx, fs)
		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if !strings.Contains(validationErr.Reason, "expected DirectoryItem") {
			t.Errorf("Expected wrong type error, got: %s", validationErr.Reason)
		}
	})

	t.Run("ValidateCreateSymlink with empty target", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_symlink", "test/link")
		symlinkItem := NewSymlink("test/link", "") // Empty target
		op.SetItem(symlinkItem)

		err := op.Validate(ctx, fs)
		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if !strings.Contains(validationErr.Reason, "target cannot be empty") {
			t.Errorf("Expected empty target error, got: %s", validationErr.Reason)
		}
	})

	t.Run("ValidateCreateArchive with no sources", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.tar.gz")
		archiveItem := NewArchive("test/archive.tar.gz", ArchiveFormatTarGz, []string{})
		// Empty sources slice to test validation error
		op.SetItem(archiveItem)

		err := op.Validate(ctx, fs)
		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		if !strings.Contains(validationErr.Reason, "at least one source") {
			t.Errorf("Expected no sources error, got: %s", validationErr.Reason)
		}
	})
}

// Helper type - basicFS is a minimal filesystem implementation that doesn't support Stat
type basicFS struct{}

func (fs *basicFS) Open(name string) (fs.File, error) {
	return nil, errors.New("not implemented")
}

func (fs *basicFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return errors.New("not implemented")
}

func (fs *basicFS) MkdirAll(path string, perm fs.FileMode) error {
	return errors.New("not implemented")
}

func (fs *basicFS) Remove(name string) error {
	return errors.New("file not found")
}

func (fs *basicFS) RemoveAll(path string) error {
	return errors.New("file not found")
}

func (fs *basicFS) Symlink(oldname, newname string) error {
	return errors.New("not implemented")
}

func (fs *basicFS) Readlink(name string) (string, error) {
	return "", errors.New("not implemented")
}

func (fs *basicFS) Rename(oldpath, newpath string) error {
	return errors.New("not implemented")
}

// Test addToZipArchive function coverage improvement
func TestSimpleOperation_AddToZipArchive_EdgeCases(t *testing.T) {
	t.Run("AddToZipArchive with filesystem that doesn't support Stat", func(t *testing.T) {
		// Use a basic filesystem that doesn't implement FullFileSystem
		fs := &basicFS{}
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		// Create a zip writer
		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)
		defer zipWriter.Close()

		err := op.addToZipArchive(zipWriter, "test/file.txt", fs)
		if err == nil {
			t.Error("Expected error when filesystem doesn't support Stat")
		}

		expectedMsg := "filesystem does not support Stat operation needed for archiving"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got: %s", expectedMsg, err.Error())
		}
	})

	t.Run("AddToZipArchive with Stat error", func(t *testing.T) {
		fs := NewTestFileSystem()
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)
		defer zipWriter.Close()

		// Try to add non-existent file
		err := op.addToZipArchive(zipWriter, "nonexistent/file.txt", fs)
		if err == nil {
			t.Error("Expected error when file doesn't exist")
		}

		expectedMsg := "failed to stat nonexistent/file.txt"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got: %s", expectedMsg, err.Error())
		}
	})

	t.Run("AddToZipArchive successfully adds directory", func(t *testing.T) {
		fs := NewTestFileSystem()
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		// Create a directory
		err := fs.MkdirAll("test/mydir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		// Add directory to zip
		err = op.addToZipArchive(zipWriter, "test/mydir", fs)
		if err != nil {
			t.Fatalf("Failed to add directory to zip: %v", err)
		}

		err = zipWriter.Close()
		if err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}

		// Verify directory entry was created
		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("Failed to read zip: %v", err)
		}

		found := false
		for _, file := range reader.File {
			if file.Name == "test/mydir/" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Directory entry not found in zip")
		}
	})

	t.Run("AddToZipArchive successfully adds file", func(t *testing.T) {
		fs := NewTestFileSystem()
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		// Create a file
		fileContent := []byte("test file content")
		err := fs.WriteFile("test/myfile.txt", fileContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)

		// Add file to zip
		err = op.addToZipArchive(zipWriter, "test/myfile.txt", fs)
		if err != nil {
			t.Fatalf("Failed to add file to zip: %v", err)
		}

		err = zipWriter.Close()
		if err != nil {
			t.Fatalf("Failed to close zip writer: %v", err)
		}

		// Verify file entry was created with correct content
		reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatalf("Failed to read zip: %v", err)
		}

		found := false
		for _, file := range reader.File {
			if file.Name == "test/myfile.txt" {
				found = true

				// Read file content from zip
				rc, err := file.Open()
				if err != nil {
					t.Fatalf("Failed to open file in zip: %v", err)
				}
				defer rc.Close()

				content, err := io.ReadAll(rc)
				if err != nil {
					t.Fatalf("Failed to read file content: %v", err)
				}

				if !bytes.Equal(content, fileContent) {
					t.Errorf("File content mismatch. Expected %q, got %q", fileContent, content)
				}
				break
			}
		}
		if !found {
			t.Error("File entry not found in zip")
		}
	})

	t.Run("AddToZipArchive with file open error", func(t *testing.T) {
		fs := &errorFS{
			testFS:    NewTestFileSystem(),
			openError: true,
		}
		op := NewSimpleOperation("test-op", "create_archive", "test/archive.zip")

		// Create a file in the underlying test filesystem first
		err := fs.testFS.WriteFile("test/myfile.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		var buf bytes.Buffer
		zipWriter := zip.NewWriter(&buf)
		defer zipWriter.Close()

		// Try to add file - should fail on Open
		err = op.addToZipArchive(zipWriter, "test/myfile.txt", fs)
		if err == nil {
			t.Error("Expected error when file open fails")
		}

		expectedMsg := "failed to open file test/myfile.txt"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain %q, got: %s", expectedMsg, err.Error())
		}
	})
}

// Test validateCopy function coverage improvement
func TestSimpleOperation_ValidateCopy_EdgeCases(t *testing.T) {
	ctx := context.Background()
	fs := NewTestFileSystem()

	t.Run("ValidateCopy with empty source path", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "copy", "test/source.txt") // Non-empty description path
		op.SetPaths("", "test/destination.txt")                        // Empty source path

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty source path")
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		expectedMsg := "copy source path cannot be empty"
		if validationErr.Reason != expectedMsg {
			t.Errorf("Expected reason %q, got %q", expectedMsg, validationErr.Reason)
		}
	})

	t.Run("ValidateCopy with empty destination path", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "copy", "test/source.txt")
		op.SetPaths("test/source.txt", "") // Empty destination path

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty destination path")
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		expectedMsg := "copy destination path cannot be empty"
		if validationErr.Reason != expectedMsg {
			t.Errorf("Expected reason %q, got %q", expectedMsg, validationErr.Reason)
		}
	})

	t.Run("ValidateCopy with valid paths", func(t *testing.T) {
		op := NewSimpleOperation("test-op", "copy", "test/source.txt")
		op.SetPaths("test/source.txt", "test/destination.txt")

		err := op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no validation error for valid paths, got: %v", err)
		}
	})

	t.Run("ValidateCopy doesn't check source existence at validation time", func(t *testing.T) {
		// This tests the comment in validateCopy about not checking source existence
		op := NewSimpleOperation("test-op", "copy", "nonexistent/source.txt")
		op.SetPaths("nonexistent/source.txt", "test/destination.txt")

		err := op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Expected no validation error even for non-existent source, got: %v", err)
		}
	})

	t.Run("ValidateCopy with empty description path (general validation)", func(t *testing.T) {
		// This tests that general validation happens before validateCopy
		op := NewSimpleOperation("test-op", "copy", "") // Empty description path
		op.SetPaths("test/source.txt", "test/destination.txt")

		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty description path")
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		expectedMsg := "path cannot be empty"
		if validationErr.Reason != expectedMsg {
			t.Errorf("Expected reason %q, got %q", expectedMsg, validationErr.Reason)
		}
	})
}

// Helper filesystem that can simulate errors
type errorFS struct {
	testFS    *TestFileSystem
	openError bool
	statError bool
}

func (fs *errorFS) Open(name string) (fs.File, error) {
	if fs.openError {
		return nil, errors.New("simulated open error")
	}
	return fs.testFS.Open(name)
}

func (fs *errorFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return fs.testFS.WriteFile(name, data, perm)
}

func (fs *errorFS) MkdirAll(path string, perm fs.FileMode) error {
	return fs.testFS.MkdirAll(path, perm)
}

func (fs *errorFS) Remove(name string) error {
	return fs.testFS.Remove(name)
}

func (fs *errorFS) RemoveAll(path string) error {
	return fs.testFS.RemoveAll(path)
}

func (fs *errorFS) Symlink(oldname, newname string) error {
	return fs.testFS.Symlink(oldname, newname)
}

func (fs *errorFS) Readlink(name string) (string, error) {
	return fs.testFS.Readlink(name)
}

func (fs *errorFS) Rename(oldpath, newpath string) error {
	return fs.testFS.Rename(oldpath, newpath)
}

func (fs *errorFS) Stat(name string) (fs.FileInfo, error) {
	if fs.statError {
		return nil, errors.New("simulated stat error")
	}
	return fs.testFS.Stat(name)
}
