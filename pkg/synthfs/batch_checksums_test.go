package synthfs_test

import (
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestBatchChecksumming(t *testing.T) {
	// Phase I, Milestone 3: Test basic checksumming for copy/move operations

	t.Run("Copy operation computes source checksum", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()

		// Create source file
		sourceContent := []byte("This is test content for checksumming")
		err := testFS.WriteFile("source.txt", sourceContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

		// Copy operation should compute checksum
		op, err := batch.Copy("source.txt", "destination.txt")
		if err != nil {
			t.Fatalf("Copy operation failed: %v", err)
		}

		// Check that checksum was computed and stored
		checksum := op.GetChecksum("source.txt")
		if checksum == nil {
			t.Error("Expected checksum to be computed for source file")
		} else {
			if checksum.Path != "source.txt" {
				t.Errorf("Expected checksum path 'source.txt', got '%s'", checksum.Path)
			}
			if checksum.MD5 == "" {
				t.Error("Expected non-empty MD5 checksum")
			}
			if checksum.Size != int64(len(sourceContent)) {
				t.Errorf("Expected checksum size %d, got %d", len(sourceContent), checksum.Size)
			}
		}

		// Check that checksum is in operation description
		desc := op.(synthfs.Operation).Describe()
		if sourceChecksum, exists := desc.Details["source_checksum"]; !exists {
			t.Error("Expected source_checksum in operation details")
		} else if sourceChecksum != checksum.MD5 {
			t.Errorf("Expected source_checksum %s, got %v", checksum.MD5, sourceChecksum)
		}
	})

	t.Run("Move operation computes source checksum", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()

		// Create source file
		sourceContent := []byte("This content will be moved with checksum validation")
		err := testFS.WriteFile("old.txt", sourceContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

		// Move operation should compute checksum
		op, err := batch.Move("old.txt", "new.txt")
		if err != nil {
			t.Fatalf("Move operation failed: %v", err)
		}

		// Check that checksum was computed and stored
		checksum := op.GetChecksum("old.txt")
		if checksum == nil {
			t.Error("Expected checksum to be computed for source file")
		} else {
			if checksum.Path != "old.txt" {
				t.Errorf("Expected checksum path 'old.txt', got '%s'", checksum.Path)
			}
			if checksum.MD5 == "" {
				t.Error("Expected non-empty MD5 checksum")
			}
			if checksum.Size != int64(len(sourceContent)) {
				t.Errorf("Expected checksum size %d, got %d", len(sourceContent), checksum.Size)
			}
		}

		// Check that checksum is in operation description
		desc := op.(synthfs.Operation).Describe()
		if sourceChecksum, exists := desc.Details["source_checksum"]; !exists {
			t.Error("Expected source_checksum in operation details")
		} else if sourceChecksum != checksum.MD5 {
			t.Errorf("Expected source_checksum %s, got %v", checksum.MD5, sourceChecksum)
		}
	})

	t.Run("Archive operation computes checksums for all sources", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()

		// Create multiple source files
		files := map[string][]byte{
			"file1.txt": []byte("Content of file 1"),
			"file2.txt": []byte("Content of file 2"),
			"file3.txt": []byte("Content of file 3"),
		}

		for path, content := range files {
			err := testFS.WriteFile(path, content, 0644)
			if err != nil {
				t.Fatalf("Failed to create source file %s: %v", path, err)
			}
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

		// Archive operation should compute checksums for all sources
		op, err := batch.CreateArchive("backup.tar.gz", synthfs.ArchiveFormatTarGz, "file1.txt", "file2.txt", "file3.txt")
		if err != nil {
			t.Fatalf("CreateArchive operation failed: %v", err)
		}

		// Check that checksums were computed for all source files
		allChecksums := op.(synthfs.Operation).GetAllChecksums()
		if len(allChecksums) != 3 {
			t.Errorf("Expected 3 checksums, got %d", len(allChecksums))
		}

		for path, expectedContent := range files {
			checksum := op.GetChecksum(path)
			if checksum == nil {
				t.Errorf("Expected checksum for source file %s", path)
				continue
			}

			if checksum.Path != path {
				t.Errorf("Expected checksum path '%s', got '%s'", path, checksum.Path)
			}
			if checksum.MD5 == "" {
				t.Errorf("Expected non-empty MD5 checksum for %s", path)
			}
			if checksum.Size != int64(len(expectedContent)) {
				t.Errorf("Expected checksum size %d for %s, got %d", len(expectedContent), path, checksum.Size)
			}
		}

		// Check that sources_checksummed is in operation description
		desc := op.(synthfs.Operation).Describe()
		if sourcesChecksummed, exists := desc.Details["sources_checksummed"]; !exists {
			t.Error("Expected sources_checksummed in operation details")
		} else if sourcesChecksummed != 3 {
			t.Errorf("Expected sources_checksummed 3, got %v", sourcesChecksummed)
		}
	})

	t.Run("Checksum computation handles directories gracefully", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()

		// Create directory
		err := testFS.MkdirAll("testdir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

		// Try to copy a directory (should not fail checksum computation)
		op, err := batch.Copy("testdir", "copydir")
		if err != nil {
			t.Fatalf("Copy directory operation should succeed: %v", err)
		}

		// Checksum should be nil for directories
		checksum := op.GetChecksum("testdir")
		if checksum != nil {
			t.Error("Expected no checksum for directory, but got one")
		}
	})

	t.Run("Different files produce different checksums", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()

		// Create two files with different content
		err := testFS.WriteFile("file1.txt", []byte("Content A"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file1: %v", err)
		}
		err = testFS.WriteFile("file2.txt", []byte("Content B"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file2: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

		// Create two copy operations
		op1, err := batch.Copy("file1.txt", "copy1.txt")
		if err != nil {
			t.Fatalf("First copy operation failed: %v", err)
		}

		op2, err := batch.Copy("file2.txt", "copy2.txt")
		if err != nil {
			t.Fatalf("Second copy operation failed: %v", err)
		}

		// Checksums should be different
		checksum1 := op1.GetChecksum("file1.txt")
		checksum2 := op2.GetChecksum("file2.txt")

		if checksum1 == nil || checksum2 == nil {
			t.Fatal("Both checksums should be computed")
		}

		if checksum1.MD5 == checksum2.MD5 {
			t.Error("Different files should have different checksums")
		}
	})

	t.Run("Same file content produces same checksum", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()

		// Create two files with identical content
		content := []byte("Identical content")
		err := testFS.WriteFile("identical1.txt", content, 0644)
		if err != nil {
			t.Fatalf("Failed to create identical1: %v", err)
		}
		err = testFS.WriteFile("identical2.txt", content, 0644)
		if err != nil {
			t.Fatalf("Failed to create identical2: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

		// Create two copy operations
		op1, err := batch.Copy("identical1.txt", "copy1.txt")
		if err != nil {
			t.Fatalf("First copy operation failed: %v", err)
		}

		op2, err := batch.Copy("identical2.txt", "copy2.txt")
		if err != nil {
			t.Fatalf("Second copy operation failed: %v", err)
		}

		// Checksums should be identical
		checksum1 := op1.GetChecksum("identical1.txt")
		checksum2 := op2.GetChecksum("identical2.txt")

		if checksum1 == nil || checksum2 == nil {
			t.Fatal("Both checksums should be computed")
		}

		if checksum1.MD5 != checksum2.MD5 {
			t.Errorf("Identical content should have same checksum, got %s vs %s",
				checksum1.MD5, checksum2.MD5)
		}
	})

	t.Run("Checksum computation failure is handled gracefully", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

		// Try to copy non-existent file (should fail during source existence validation first)
		_, err := batch.Copy("nonexistent.txt", "dest.txt")
		if err == nil {
			t.Fatal("Expected error for non-existent source file")
		}

		// Should be a source existence error, not a checksum error
		if !strings.Contains(err.Error(), "copy source") || !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Expected source existence error, got: %v", err)
		}
	})
}
