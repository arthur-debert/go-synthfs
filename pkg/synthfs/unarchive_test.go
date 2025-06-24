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
)

func TestUnarchiveItem(t *testing.T) {
	t.Run("NewUnarchive creates correct item", func(t *testing.T) {
		item := NewUnarchive("test.tar.gz", "extract/")
		
		if item.ArchivePath() != "test.tar.gz" {
			t.Errorf("Expected archive path 'test.tar.gz', got '%s'", item.ArchivePath())
		}
		
		if item.ExtractPath() != "extract/" {
			t.Errorf("Expected extract path 'extract/', got '%s'", item.ExtractPath())
		}
		
		if item.Type() != "unarchive" {
			t.Errorf("Expected type 'unarchive', got '%s'", item.Type())
		}
		
		if item.Path() != "test.tar.gz" {
			t.Errorf("Expected path 'test.tar.gz', got '%s'", item.Path())
		}
		
		if len(item.Patterns()) != 0 {
			t.Errorf("Expected empty patterns, got %v", item.Patterns())
		}
		
		if item.Overwrite() != false {
			t.Errorf("Expected overwrite false, got %v", item.Overwrite())
		}
	})
	
	t.Run("WithPatterns sets patterns correctly", func(t *testing.T) {
		item := NewUnarchive("test.zip", "extract/").
			WithPatterns("*.txt", "docs/**")
		
		patterns := item.Patterns()
		if len(patterns) != 2 {
			t.Errorf("Expected 2 patterns, got %d", len(patterns))
		}
		
		if patterns[0] != "*.txt" || patterns[1] != "docs/**" {
			t.Errorf("Expected patterns ['*.txt', 'docs/**'], got %v", patterns)
		}
	})
	
	t.Run("WithOverwrite sets overwrite correctly", func(t *testing.T) {
		item := NewUnarchive("test.zip", "extract/").
			WithOverwrite(true)
		
		if !item.Overwrite() {
			t.Errorf("Expected overwrite true, got %v", item.Overwrite())
		}
	})
}

func TestUnarchiveValidation(t *testing.T) {
	ctx := context.Background()
	fs := NewOSFileSystem(".")
	
	t.Run("validates archive path", func(t *testing.T) {
		op := NewSimpleOperation("test", "unarchive", "")
		item := NewUnarchive("", "extract/")
		op.SetItem(item)
		
		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty archive path")
		}
		
		// The error might be generic path validation or specific archive path validation
		if !strings.Contains(err.Error(), "path cannot be empty") && !strings.Contains(err.Error(), "archive path cannot be empty") {
			t.Errorf("Expected path validation error, got: %v", err)
		}
	})
	
	t.Run("validates extract path", func(t *testing.T) {
		op := NewSimpleOperation("test", "unarchive", "test.tar.gz")
		item := NewUnarchive("test.tar.gz", "")
		op.SetItem(item)
		
		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for empty extract path")
		}
		
		if !strings.Contains(err.Error(), "extract path cannot be empty") {
			t.Errorf("Expected extract path error, got: %v", err)
		}
	})
	
	t.Run("validates archive format", func(t *testing.T) {
		op := NewSimpleOperation("test", "unarchive", "test.unknown")
		item := NewUnarchive("test.unknown", "extract/")
		op.SetItem(item)
		
		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for unsupported format")
		}
		
		if !strings.Contains(err.Error(), "unsupported archive format") {
			t.Errorf("Expected format error, got: %v", err)
		}
	})
	
	t.Run("accepts supported formats", func(t *testing.T) {
		supportedFormats := []string{"test.tar.gz", "test.tgz", "test.zip"}
		
		for _, format := range supportedFormats {
			op := NewSimpleOperation("test", "unarchive", format)
			item := NewUnarchive(format, "extract/")
			op.SetItem(item)
			
			err := op.Validate(ctx, fs)
			if err != nil {
				t.Errorf("Expected no validation error for %s, got: %v", format, err)
			}
		}
	})
	
	t.Run("validates item type", func(t *testing.T) {
		op := NewSimpleOperation("test", "unarchive", "test.tar.gz")
		item := NewFile("test.tar.gz") // Wrong item type
		op.SetItem(item)
		
		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Expected validation error for wrong item type")
		}
		
		if !strings.Contains(err.Error(), "expected UnarchiveItem") {
			t.Errorf("Expected item type error, got: %v", err)
		}
	})
}

func TestUnarchivePatternMatching(t *testing.T) {
	op := &SimpleOperation{}
	
	tests := []struct {
		name     string
		filePath string
		patterns []string
		expected bool
	}{
		{
			name:     "no patterns matches all",
			filePath: "any/file.txt",
			patterns: []string{},
			expected: true,
		},
		{
			name:     "simple wildcard match",
			filePath: "file.txt",
			patterns: []string{"*.txt"},
			expected: true,
		},
		{
			name:     "simple wildcard no match",
			filePath: "file.jpg",
			patterns: []string{"*.txt"},
			expected: false,
		},
		{
			name:     "directory pattern match",
			filePath: "docs/readme.txt",
			patterns: []string{"docs/**"},
			expected: true,
		},
		{
			name:     "directory pattern no match",
			filePath: "src/main.go",
			patterns: []string{"docs/**"},
			expected: false,
		},
		{
			name:     "complex pattern match",
			filePath: "docs/guide.html",
			patterns: []string{"docs/*.html"},
			expected: true,
		},
		{
			name:     "multiple patterns first match",
			filePath: "test.txt",
			patterns: []string{"*.txt", "*.html"},
			expected: true,
		},
		{
			name:     "multiple patterns second match",
			filePath: "index.html",
			patterns: []string{"*.txt", "*.html"},
			expected: true,
		},
		{
			name:     "multiple patterns no match",
			filePath: "image.jpg",
			patterns: []string{"*.txt", "*.html"},
			expected: false,
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := op.matchesPatterns(test.filePath, test.patterns)
			if result != test.expected {
				t.Errorf("Expected %v for file '%s' with patterns %v, got %v",
					test.expected, test.filePath, test.patterns, result)
			}
		})
	}
}

func TestUnarchiveIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	fs := NewOSFileSystem(tempDir)
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
		op := NewSimpleOperation("test", "unarchive", archivePath)
		item := NewUnarchive(archivePath, extractPath)
		op.SetItem(item)
		
		// Execute unarchive
		err = op.Execute(ctx, fs)
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
		op := NewSimpleOperation("test", "unarchive", archivePath)
		item := NewUnarchive(archivePath, extractPath)
		op.SetItem(item)
		
		// Execute unarchive
		err = op.Execute(ctx, fs)
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
			"file1.txt":         []byte("content1"),
			"file2.html":        []byte("content2"),
			"docs/readme.txt":   []byte("readme"),
			"docs/guide.html":   []byte("guide"),
			"src/main.go":       []byte("go code"),
			"images/photo.jpg":  []byte("photo"),
		}
		
		archiveData := createTestTarGz(t, testFiles)
		
		// Write archive to filesystem
		err := fs.WriteFile(archivePath, archiveData, 0644)
		if err != nil {
			t.Fatalf("Failed to write archive: %v", err)
		}
		
		// Create unarchive operation with patterns
		op := NewSimpleOperation("test", "unarchive", archivePath)
		item := NewUnarchive(archivePath, extractPath).WithPatterns("*.txt", "docs/**")
		op.SetItem(item)
		
		// Execute unarchive
		err = op.Execute(ctx, fs)
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
	tempDir := t.TempDir()
	batch := NewBatch().WithFileSystem(NewOSFileSystem(tempDir))
	
	t.Run("batch unarchive operation", func(t *testing.T) {
		// Create test archive first
		testFiles := map[string][]byte{
			"batch_file.txt": []byte("batch content"),
		}
		archiveData := createTestTarGz(t, testFiles)
		
		// Create archive file
		_, err := batch.CreateFile("batch_test.tar.gz", archiveData)
		if err != nil {
			t.Fatalf("Failed to create archive file: %v", err)
		}
		
		// Add unarchive operation
		op, err := batch.Unarchive("batch_test.tar.gz", "batch_extracted/")
		if err != nil {
			t.Fatalf("Failed to add unarchive operation: %v", err)
		}
		
		if op == nil {
			t.Fatal("Unarchive returned nil operation")
		}
		
		desc := op.Describe()
		if desc.Type != "unarchive" {
			t.Errorf("Expected operation type 'unarchive', got '%s'", desc.Type)
		}
		
		// Execute batch
		result, err := batch.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Batch execution was not successful: %v", result.Errors)
		}
		
		// Verify extraction
		fs := NewOSFileSystem(tempDir)
		if _, err := fs.Stat("batch_extracted/batch_file.txt"); err != nil {
			t.Errorf("Extracted file not found: %v", err)
		}
	})
	
	t.Run("batch unarchive with patterns", func(t *testing.T) {
		// Create test archive with multiple files
		testFiles := map[string][]byte{
			"include.txt":    []byte("include me"),
			"exclude.html":   []byte("exclude me"),
			"docs/help.txt":  []byte("help content"),
		}
		archiveData := createTestTarGz(t, testFiles)
		
		// Create new batch for this test
		batch2 := NewBatch().WithFileSystem(NewOSFileSystem(tempDir))
		
		// Create archive file
		_, err := batch2.CreateFile("pattern_test.tar.gz", archiveData)
		if err != nil {
			t.Fatalf("Failed to create archive file: %v", err)
		}
		
		// Add unarchive operation with patterns
		_, err = batch2.UnarchiveWithPatterns("pattern_test.tar.gz", "pattern_extracted/", "*.txt", "docs/**")
		if err != nil {
			t.Fatalf("Failed to add unarchive operation with patterns: %v", err)
		}
		
		// Execute batch
		result, err := batch2.Run()
		if err != nil {
			t.Fatalf("Batch execution failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Batch execution was not successful: %v", result.Errors)
		}
		
		// Verify only matching files were extracted
		fs := NewOSFileSystem(tempDir)
		
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

func (d testDirInfo) Name() string       { return d.name }
func (d testDirInfo) Size() int64        { return 0 }
func (d testDirInfo) Mode() fs.FileMode  { 
	if d.isDir {
		return 0755 | fs.ModeDir
	}
	return 0644
}
func (d testDirInfo) ModTime() time.Time { return time.Time{} }
func (d testDirInfo) IsDir() bool        { return d.isDir }
func (d testDirInfo) Sys() interface{}   { return nil }