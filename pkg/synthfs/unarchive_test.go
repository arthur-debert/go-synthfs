package synthfs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

func TestUnarchiveValidation(t *testing.T) {
	ctx := context.Background()
	fs := filesystem.NewOSFileSystem(".")

	t.Run("validates archive path", func(t *testing.T) {
		registry := NewOperationRegistry()
		op, err := registry.CreateOperation(core.OperationID("test"), "unarchive", "")
		if err != nil {
			t.Fatalf("Failed to create operation: %v", err)
		}
		item := NewUnarchive("", "extract/")
		if err := registry.SetItemForOperation(op.(Operation), item); err != nil {
			t.Fatalf("Failed to set item: %v", err)
		}

		err = op.(Operation).Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty archive path")
		}

		// The error might be generic path validation or specific archive path validation
		if !strings.Contains(err.Error(), "path cannot be empty") && !strings.Contains(err.Error(), "archive path cannot be empty") {
			t.Errorf("Expected path validation error, got: %v", err)
		}
	})

	t.Run("validates extract path", func(t *testing.T) {
		registry := NewOperationRegistry()
		op, err := registry.CreateOperation(core.OperationID("test"), "unarchive", "test.tar.gz")
		if err != nil {
			t.Fatalf("Failed to create operation: %v", err)
		}
		item := NewUnarchive("test.tar.gz", "")
		if err := registry.SetItemForOperation(op.(Operation), item); err != nil {
			t.Fatalf("Failed to set item: %v", err)
		}

		err = op.(Operation).Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty extract path")
		}

		if !strings.Contains(err.Error(), "extract path cannot be empty") {
			t.Errorf("Expected extract path error, got: %v", err)
		}
	})

	t.Run("validates archive format", func(t *testing.T) {
		registry := NewOperationRegistry()
		op, err := registry.CreateOperation(core.OperationID("test"), "unarchive", "test.unknown")
		if err != nil {
			t.Fatalf("Failed to create operation: %v", err)
		}
		item := NewUnarchive("test.unknown", "extract/")
		if err := registry.SetItemForOperation(op.(Operation), item); err != nil {
			t.Fatalf("Failed to set item: %v", err)
		}

		err = op.(Operation).Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for unsupported format")
		}

		if !strings.Contains(err.Error(), "unsupported archive format") {
			t.Errorf("Expected format error, got: %v", err)
		}
	})

	t.Run("accepts supported formats", func(t *testing.T) {
		supportedFormats := []string{"test.tar.gz", "test.tgz", "test.zip"}

		// Phase I, Milestone 1: Create dummy archive files since we now validate existence
		testFS := filesystem.NewOSFileSystem(".")
		for _, format := range supportedFormats {
			// Create a minimal dummy archive file for testing
			err := testFS.WriteFile(format, []byte("dummy archive content"), 0644)
			if err != nil {
				t.Fatalf("Failed to create test archive %s: %v", format, err)
			}
			defer func() { _ = testFS.Remove(format) }() // Clean up
		}

		for _, format := range supportedFormats {
			registry := NewOperationRegistry()
			op, err := registry.CreateOperation(core.OperationID("test"), "unarchive", format)
			if err != nil {
				t.Fatalf("Failed to create operation: %v", err)
			}
			item := NewUnarchive(format, "extract/")
			if err := registry.SetItemForOperation(op.(Operation), item); err != nil {
				t.Fatalf("Failed to set item: %v", err)
			}

			err = op.(Operation).Validate(ctx, testFS)
			if err != nil {
				t.Errorf("Expected no validation error for %s, got: %v", format, err)
			}
		}
	})

	t.Run("validates item type", func(t *testing.T) {
		registry := NewOperationRegistry()
		op, err := registry.CreateOperation(core.OperationID("test"), "unarchive", "test.tar.gz")
		if err != nil {
			t.Fatalf("Failed to create operation: %v", err)
		}
		item := NewFile("test.tar.gz") // Wrong item type
		if err := registry.SetItemForOperation(op.(Operation), item); err != nil {
			t.Fatalf("Failed to set item: %v", err)
		}

		err = op.(Operation).Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for wrong item type")
		}

		if !strings.Contains(err.Error(), "expected UnarchiveItem") {
			t.Errorf("Expected item type error, got: %v", err)
		}
	})
}

func TestUnarchiveIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	fs := filesystem.NewOSFileSystem(tempDir)
	ctx := context.Background()

	t.Run("extract tar.gz archive", func(t *testing.T) {
		// Create a test tar.gz archive
		archivePath := "test.tar.gz"
		extractPath := "extracted/"

		// Create archive content
		testFiles := map[string][]byte{
			"file1.txt":      []byte("content1"),
			"dir1/file2.txt": []byte("content2"),
			"dir1/file3.txt": []byte("content3"),
		}

		archiveData := createTestTarGz(t, testFiles)

		// Write archive to filesystem
		err := fs.WriteFile(archivePath, archiveData, 0644)
		if err != nil {
			t.Fatalf("Failed to write archive: %v", err)
		}

		// Create unarchive operation
		registry := NewOperationRegistry()
		op, err := registry.CreateOperation(core.OperationID("test"), "unarchive", archivePath)
		if err != nil {
			t.Fatalf("Failed to create operation: %v", err)
		}
		item := NewUnarchive(archivePath, extractPath)
		if err := registry.SetItemForOperation(op.(Operation), item); err != nil {
			t.Fatalf("Failed to set item: %v", err)
		}

		// Execute unarchive
		err = op.(Operation).Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Unarchive failed: %v", err)
		}

		// Verify extracted files
		for filePath, expectedContent := range testFiles {
			extractedPath := filepath.Join(extractPath, filePath)

			// Check file exists
			info, err := fs.Stat(extractedPath)
			if err != nil {
				t.Errorf("Extracted file %s not found: %v", extractedPath, err)
				continue
			}

			if info.IsDir() {
				t.Errorf("Expected file %s but found directory", extractedPath)
				continue
			}

			// Check file content
			file, err := fs.Open(extractedPath)
			if err != nil {
				t.Errorf("Failed to open extracted file %s: %v", extractedPath, err)
				continue
			}

			content, err := io.ReadAll(file)
			if closeErr := file.Close(); closeErr != nil {
				t.Logf("Warning: failed to close file %s: %v", extractedPath, closeErr)
			}
			if err != nil {
				t.Errorf("Failed to read extracted file %s: %v", extractedPath, err)
				continue
			}

			if !bytes.Equal(content, expectedContent) {
				t.Errorf("Content mismatch for %s: expected %s, got %s",
					extractedPath, string(expectedContent), string(content))
			}
		}
	})

	t.Run("extract zip archive", func(t *testing.T) {
		// Create a test zip archive
		archivePath := "test.zip"
		extractPath := "extracted_zip/"

		// Create archive content
		testFiles := map[string][]byte{
			"file1.txt":      []byte("zip content1"),
			"dir1/file2.txt": []byte("zip content2"),
		}

		archiveData := createTestZip(t, testFiles)

		// Write archive to filesystem
		err := fs.WriteFile(archivePath, archiveData, 0644)
		if err != nil {
			t.Fatalf("Failed to write archive: %v", err)
		}

		// Create unarchive operation
		registry := NewOperationRegistry()
		op, err := registry.CreateOperation(core.OperationID("test"), "unarchive", archivePath)
		if err != nil {
			t.Fatalf("Failed to create operation: %v", err)
		}
		item := NewUnarchive(archivePath, extractPath)
		if err := registry.SetItemForOperation(op.(Operation), item); err != nil {
			t.Fatalf("Failed to set item: %v", err)
		}

		// Execute unarchive
		err = op.(Operation).Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Unarchive failed: %v", err)
		}

		// Verify extracted files
		for filePath, expectedContent := range testFiles {
			extractedPath := filepath.Join(extractPath, filePath)

			// Check file content
			file, err := fs.Open(extractedPath)
			if err != nil {
				t.Errorf("Failed to open extracted file %s: %v", extractedPath, err)
				continue
			}

			content, err := io.ReadAll(file)
			if closeErr := file.Close(); closeErr != nil {
				t.Logf("Warning: failed to close file %s: %v", extractedPath, closeErr)
			}
			if err != nil {
				t.Errorf("Failed to read extracted file %s: %v", extractedPath, err)
				continue
			}

			if !bytes.Equal(content, expectedContent) {
				t.Errorf("Content mismatch for %s: expected %s, got %s",
					extractedPath, string(expectedContent), string(content))
			}
		}
	})

	t.Run("extract with patterns", func(t *testing.T) {
		// Create a test archive with various files
		archivePath := "pattern_test.tar.gz"
		extractPath := "pattern_extracted/"

		testFiles := map[string][]byte{
			"file1.txt":        []byte("content1"),
			"file2.html":       []byte("content2"),
			"docs/readme.txt":  []byte("readme"),
			"docs/guide.html":  []byte("guide"),
			"src/main.go":      []byte("go code"),
			"images/photo.jpg": []byte("photo"),
		}

		archiveData := createTestTarGz(t, testFiles)

		// Write archive to filesystem
		err := fs.WriteFile(archivePath, archiveData, 0644)
		if err != nil {
			t.Fatalf("Failed to write archive: %v", err)
		}

		// Create unarchive operation with patterns
		registry := NewOperationRegistry()
		op, err := registry.CreateOperation(core.OperationID("test"), "unarchive", archivePath)
		if err != nil {
			t.Fatalf("Failed to create operation: %v", err)
		}
		item := NewUnarchive(archivePath, extractPath).WithPatterns("*.txt", "docs/**")
		if err := registry.SetItemForOperation(op.(Operation), item); err != nil {
			t.Fatalf("Failed to set item: %v", err)
		}

		// Execute unarchive
		err = op.(Operation).Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Unarchive with patterns failed: %v", err)
		}

		// Verify only matching files were extracted
		expectedFiles := []string{"file1.txt", "docs/readme.txt", "docs/guide.html"}
		unexpectedFiles := []string{"file2.html", "src/main.go", "images/photo.jpg"}

		for _, filePath := range expectedFiles {
			extractedPath := filepath.Join(extractPath, filePath)
			if _, err := fs.Stat(extractedPath); err != nil {
				t.Errorf("Expected file %s was not extracted: %v", extractedPath, err)
			}
		}

		for _, filePath := range unexpectedFiles {
			extractedPath := filepath.Join(extractPath, filePath)
			if _, err := fs.Stat(extractedPath); err == nil {
				t.Errorf("Unexpected file %s was extracted", extractedPath)
			}
		}
	})
}

func TestBatchUnarchive(t *testing.T) {
	t.Run("batch unarchive operation", func(t *testing.T) {
		tempDir := t.TempDir()
		fs := filesystem.NewOSFileSystem(tempDir)

		// Create test archive first
		testFiles := map[string][]byte{
			"batch_file.txt": []byte("batch content"),
		}
		archiveData := createTestTarGz(t, testFiles)

		// Write the archive file to the filesystem directly before creating the operation
		archivePath := "batch_test.tar.gz"
		if err := fs.WriteFile(archivePath, archiveData, 0644); err != nil {
			t.Fatalf("Failed to write test archive: %v", err)
		}

		// Create a new batch with the file already in the filesystem
		batch := NewBatch(fs).WithFileSystem(fs)

		// Add unarchive operation
		op, err := batch.Unarchive(archivePath, "batch_extracted/")
		if err != nil {
			t.Fatalf("Failed to add unarchive operation: %v", err)
		}

		if op == nil {
			t.Fatal("Unarchive returned nil operation")
		}

		if opTyped, ok := op.(Operation); ok {
			desc := opTyped.Describe()
			if desc.Type != "unarchive" {
				t.Errorf("Expected operation type 'unarchive', got '%s'", desc.Type)
			}
		} else {
			t.Error("Expected operation to be of type Operation")
		}

		// Execute batch
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", result.GetError())
		}

		// Verify extraction
		if _, err := fs.Stat("batch_extracted/batch_file.txt"); err != nil {
			t.Errorf("Extracted file not found: %v", err)
		}
	})

	t.Run("batch unarchive with patterns", func(t *testing.T) {
		tempDir := t.TempDir()
		fs := filesystem.NewOSFileSystem(tempDir)

		// Create test archive with multiple files
		testFiles := map[string][]byte{
			"include.txt":   []byte("include me"),
			"exclude.html":  []byte("exclude me"),
			"docs/help.txt": []byte("help content"),
		}
		archiveData := createTestTarGz(t, testFiles)

		// Write the archive file to the filesystem directly
		archivePath := "pattern_test.tar.gz"
		if err := fs.WriteFile(archivePath, archiveData, 0644); err != nil {
			t.Fatalf("Failed to write test archive: %v", err)
		}

		// Create new batch for this test
		batch := NewBatch(fs).WithFileSystem(fs)

		// Add unarchive operation with patterns
		patterns := []string{"*.txt", "docs/**"}
		_, err := batch.UnarchiveWithPatterns(archivePath, "pattern_extracted/", patterns)
		if err != nil {
			t.Fatalf("Failed to add unarchive operation with patterns: %v", err)
		}

		// Execute batch
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}

		if !result.IsSuccess() {
			t.Fatalf("Batch execution was not successful: %v", result.GetError())
		}

		// Verify only matching files were extracted
		// Should exist
		if _, err := fs.Stat("pattern_extracted/include.txt"); err != nil {
			t.Errorf("Expected file include.txt not found: %v", err)
		}
		if _, err := fs.Stat("pattern_extracted/docs/help.txt"); err != nil {
			t.Errorf("Expected file docs/help.txt not found: %v", err)
		}

		// Should not exist
		if _, err := fs.Stat("pattern_extracted/exclude.html"); err == nil {
			t.Error("Unexpected file exclude.html was extracted")
		}
	})
}

// Helper function to create a test tar.gz archive
func createTestTarGz(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for path, content := range files {
		// Add directory entries if needed
		dir := filepath.Dir(path)
		if dir != "." && dir != "/" {
			header := &tar.Header{
				Name:     dir + "/",
				Mode:     0755,
				Typeflag: tar.TypeDir,
			}
			if err := tw.WriteHeader(header); err != nil {
				t.Fatalf("Failed to write dir header: %v", err)
			}
		}

		// Add file
		header := &tar.Header{
			Name: path,
			Mode: 0644,
			Size: int64(len(content)),
		}

		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write header: %v", err)
		}

		if _, err := tw.Write(content); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

// Helper function to create a test zip archive
func createTestZip(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Track created directories to avoid duplicates
	createdDirs := make(map[string]bool)

	for path, content := range files {
		// Create directory structure
		dir := filepath.Dir(path)
		dirs := []string{}
		currentDir := dir

		// Collect all parent directories
		for currentDir != "." && currentDir != "/" && currentDir != "" {
			dirs = append([]string{currentDir}, dirs...)
			currentDir = filepath.Dir(currentDir)
		}

		// Create directory entries
		for _, d := range dirs {
			dirPath := d + "/"
			if !createdDirs[dirPath] {
				dirHeader, err := zip.FileInfoHeader(testDirInfo{name: filepath.Base(d), isDir: true})
				if err != nil {
					t.Fatalf("Failed to create directory header: %v", err)
				}
				dirHeader.Name = dirPath
				dirHeader.Method = zip.Store

				_, err = w.CreateHeader(dirHeader)
				if err != nil {
					t.Fatalf("Failed to create directory entry: %v", err)
				}
				createdDirs[dirPath] = true
			}
		}

		// Add file
		f, err := w.Create(path)
		if err != nil {
			t.Fatalf("Failed to create file entry: %v", err)
		}

		if _, err := f.Write(content); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

// testDirInfo implements fs.FileInfo for directory entries
type testDirInfo struct {
	name  string
	isDir bool
}

func (d testDirInfo) Name() string { return d.name }
func (d testDirInfo) Size() int64  { return 0 }
func (d testDirInfo) Mode() fs.FileMode {
	if d.isDir {
		return 0755 | fs.ModeDir
	}
	return 0644
}
func (d testDirInfo) ModTime() time.Time { return time.Time{} }
func (d testDirInfo) IsDir() bool        { return d.isDir }
func (d testDirInfo) Sys() interface{}   { return nil }
