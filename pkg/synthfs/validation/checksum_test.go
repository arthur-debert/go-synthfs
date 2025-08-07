package validation_test

import (
	"context"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)

func TestBatchChecksumming(t *testing.T) {
	// Checksum functionality restored - checksums are computed during operation execution
	
	// Phase I, Milestone 3: Test basic checksumming for copy/move operations

	t.Run("Copy operation computes source checksum", func(t *testing.T) {
		testFS := testutil.NewTestFileSystem()

		// Create source file
		sourceContent := []byte("This is test content for checksumming")
		err := testFS.WriteFile("source.txt", sourceContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		sfs := synthfs.New()
		ctx := context.Background()

		// Copy operation should compute checksum during execution
		op := sfs.Copy("source.txt", "destination.txt")
		if op == nil {
			t.Fatal("Copy operation should not be nil")
		}

		// Execute the operation so checksum gets computed
		result, err := synthfs.Run(ctx, testFS, op)
		if err != nil {
			t.Fatalf("Failed to execute copy operation: %v", err)
		}
		if !result.Success {
			t.Fatalf("Copy operation should succeed")
		}

		// Check that checksum was computed and stored
		checksum := op.GetChecksum("source.txt")
		if checksum == nil {
			t.Error("Expected checksum to be computed for source file")
		} else {
			cr, ok := checksum.(*validation.ChecksumRecord)
			if !ok {
				t.Fatalf("Expected *validation.ChecksumRecord, got %T", checksum)
			}
			if cr.Path != "source.txt" {
				t.Errorf("Expected checksum path 'source.txt', got '%s'", cr.Path)
			}
			if cr.MD5 == "" {
				t.Error("Expected non-empty MD5 checksum")
			}
			if cr.Size != int64(len(sourceContent)) {
				t.Errorf("Expected checksum size %d, got %d", len(sourceContent), cr.Size)
			}
			
			// Check that checksum is in operation description
			desc := op.Describe()
			if sourceChecksum, exists := desc.Details["source_checksum"]; !exists {
				t.Error("Expected source_checksum in operation details")
			} else {
				if sourceChecksum != cr.MD5 {
					t.Errorf("Expected source_checksum %s, got %v", cr.MD5, sourceChecksum)
				}
			}
		}
	})

	t.Run("Move operation computes source checksum", func(t *testing.T) {
		testFS := testutil.NewTestFileSystem()

		// Create source file
		sourceContent := []byte("This content will be moved with checksum validation")
		err := testFS.WriteFile("old.txt", sourceContent, 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		sfs := synthfs.New()
		ctx := context.Background()

		// Move operation should compute checksum during execution
		op := sfs.Move("old.txt", "new.txt")
		if op == nil {
			t.Fatal("Move operation should not be nil")
		}

		// Execute the operation so checksum gets computed
		result, err := synthfs.Run(ctx, testFS, op)
		if err != nil {
			t.Fatalf("Failed to execute move operation: %v", err)
		}
		if !result.Success {
			t.Fatalf("Move operation should succeed")
		}

		// Check that checksum was computed and stored
		checksum := op.GetChecksum("old.txt")
		if checksum == nil {
			t.Error("Expected checksum to be computed for source file")
		} else {
			cr, ok := checksum.(*validation.ChecksumRecord)
			if !ok {
				t.Fatalf("Expected *validation.ChecksumRecord, got %T", checksum)
			}
			if cr.Path != "old.txt" {
				t.Errorf("Expected checksum path 'old.txt', got '%s'", cr.Path)
			}
			if cr.MD5 == "" {
				t.Error("Expected non-empty MD5 checksum")
			}
			if cr.Size != int64(len(sourceContent)) {
				t.Errorf("Expected checksum size %d, got %d", len(sourceContent), cr.Size)
			}
		}

		// Check that checksum is in operation description
		desc := op.Describe()
		if sourceChecksum, exists := desc.Details["source_checksum"]; !exists {
			t.Error("Expected source_checksum in operation details")
		} else {
			cr, _ := checksum.(*validation.ChecksumRecord)
			if sourceChecksum != cr.MD5 {
				t.Errorf("Expected source_checksum %s, got %v", cr.MD5, sourceChecksum)
			}
		}
	})

	t.Run("Archive operation computes checksums for all sources", func(t *testing.T) {
		testFS := testutil.NewTestFileSystem()

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

		sfs := synthfs.New()
		ctx := context.Background()

		// Archive operation should compute checksums for all sources during execution
		op := sfs.CreateArchive("backup.tar.gz", "file1.txt", "file2.txt", "file3.txt")
		if op == nil {
			t.Fatal("CreateArchive operation should not be nil")
		}

		// Execute the operation so checksums get computed
		result, err := synthfs.Run(ctx, testFS, op)
		if err != nil {
			t.Fatalf("Failed to execute archive operation: %v", err)
		}
		if !result.Success {
			t.Fatalf("Archive operation should succeed")
		}

		// Check that checksums were computed for all source files
		allChecksums := op.GetAllChecksums()
		if len(allChecksums) != 3 {
			t.Errorf("Expected 3 checksums, got %d", len(allChecksums))
		}

		for path, expectedContent := range files {
			checksum := op.GetChecksum(path)
			if checksum == nil {
				t.Errorf("Expected checksum for source file %s", path)
				continue
			}

			cr, ok := checksum.(*validation.ChecksumRecord)
			if !ok {
				t.Fatalf("Expected *validation.ChecksumRecord, got %T", checksum)
			}
			if cr.Path != path {
				t.Errorf("Expected checksum path '%s', got '%s'", path, cr.Path)
			}
			if cr.MD5 == "" {
				t.Errorf("Expected non-empty MD5 checksum for %s", path)
			}
			if cr.Size != int64(len(expectedContent)) {
				t.Errorf("Expected checksum size %d for %s, got %d", len(expectedContent), path, cr.Size)
			}
		}

		// Check that sources_checksummed is in operation description
		desc := op.Describe()
		if sourcesChecksummed, exists := desc.Details["sources_checksummed"]; !exists {
			t.Error("Expected sources_checksummed in operation details")
		} else if sourcesChecksummed != 3 {
			t.Errorf("Expected sources_checksummed 3, got %v", sourcesChecksummed)
		}
	})

	t.Run("Checksum computation handles directories gracefully", func(t *testing.T) {
		testFS := testutil.NewTestFileSystem()

		// Create directory
		err := testFS.MkdirAll("testdir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		sfs := synthfs.New()
		ctx := context.Background()

		// Try to copy a directory (should fail because directory copy is not implemented)
		op := sfs.Copy("testdir", "copydir")
		if op == nil {
			t.Fatal("Copy operation should not be nil")
		}

		// Execute the operation - this will fail because directory copy is not implemented
		_, err = synthfs.Run(ctx, testFS, op)
		if err == nil {
			t.Skip("Directory copy is not implemented yet, skipping this test")
		}

		// Even with failure, checksum should be nil for directories
		checksum := op.GetChecksum("testdir")
		if checksum != nil {
			t.Error("Expected no checksum for directory, but got one")
		}
	})

	t.Run("Different files produce different checksums", func(t *testing.T) {
		testFS := testutil.NewTestFileSystem()

		// Create two files with different content
		err := testFS.WriteFile("file1.txt", []byte("Content A"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file1: %v", err)
		}
		err = testFS.WriteFile("file2.txt", []byte("Content B"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file2: %v", err)
		}

		sfs := synthfs.New()
		ctx := context.Background()

		// Create two copy operations
		op1 := sfs.Copy("file1.txt", "copy1.txt")
		if op1 == nil {
			t.Fatal("First copy operation should not be nil")
		}

		op2 := sfs.Copy("file2.txt", "copy2.txt")
		if op2 == nil {
			t.Fatal("Second copy operation should not be nil")
		}

		// Execute both operations
		result1, err1 := synthfs.Run(ctx, testFS, op1)
		if err1 != nil {
			t.Fatalf("Failed to execute first copy operation: %v", err1)
		}
		if !result1.Success {
			t.Fatalf("First copy operation should succeed")
		}

		result2, err2 := synthfs.Run(ctx, testFS, op2)
		if err2 != nil {
			t.Fatalf("Failed to execute second copy operation: %v", err2)
		}
		if !result2.Success {
			t.Fatalf("Second copy operation should succeed")
		}

		// Checksums should be different
		checksum1 := op1.GetChecksum("file1.txt")
		checksum2 := op2.GetChecksum("file2.txt")

		if checksum1 == nil || checksum2 == nil {
			t.Fatal("Both checksums should be computed")
		}

		cr1, ok1 := checksum1.(*validation.ChecksumRecord)
		cr2, ok2 := checksum2.(*validation.ChecksumRecord)
		if !ok1 || !ok2 {
			t.Fatal("Expected *validation.ChecksumRecord for both checksums")
		}

		if cr1.MD5 == cr2.MD5 {
			t.Error("Different files should have different checksums")
		}
	})

	t.Run("Same file content produces same checksum", func(t *testing.T) {
		testFS := testutil.NewTestFileSystem()

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

		sfs := synthfs.New()
		ctx := context.Background()

		// Create two copy operations
		op1 := sfs.Copy("identical1.txt", "copy1.txt")
		if op1 == nil {
			t.Fatal("First copy operation should not be nil")
		}

		op2 := sfs.Copy("identical2.txt", "copy2.txt")
		if op2 == nil {
			t.Fatal("Second copy operation should not be nil")
		}

		// Execute both operations
		result1, err1 := synthfs.Run(ctx, testFS, op1)
		if err1 != nil {
			t.Fatalf("Failed to execute first copy operation: %v", err1)
		}
		if !result1.Success {
			t.Fatalf("First copy operation should succeed")
		}

		result2, err2 := synthfs.Run(ctx, testFS, op2)
		if err2 != nil {
			t.Fatalf("Failed to execute second copy operation: %v", err2)
		}
		if !result2.Success {
			t.Fatalf("Second copy operation should succeed")
		}

		// Checksums should be identical
		checksum1 := op1.GetChecksum("identical1.txt")
		checksum2 := op2.GetChecksum("identical2.txt")

		if checksum1 == nil || checksum2 == nil {
			t.Fatal("Both checksums should be computed")
		}

		cr1, ok1 := checksum1.(*validation.ChecksumRecord)
		cr2, ok2 := checksum2.(*validation.ChecksumRecord)
		if !ok1 || !ok2 {
			t.Fatal("Expected *validation.ChecksumRecord for both checksums")
		}

		if cr1.MD5 != cr2.MD5 {
			t.Errorf("Identical content should have same checksum, got %s vs %s",
				cr1.MD5, cr2.MD5)
		}
	})
}
