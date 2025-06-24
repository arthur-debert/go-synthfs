package synthfs_test

import (
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestBatchDuplicatePathDetection(t *testing.T) {
	// Phase I, Milestone 2: Test duplicate path detection in batch operations

	t.Run("Detect duplicate file creation", func(t *testing.T) {
		batch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// Create first file - should succeed
		_, err := batch.CreateFile("test.txt", []byte("content1"))
		if err != nil {
			t.Fatalf("First CreateFile should succeed: %v", err)
		}

		// Create second file with same path - should fail
		_, err = batch.CreateFile("test.txt", []byte("content2"))
		if err == nil {
			t.Error("Expected conflict error for duplicate file creation")
		}

		if !strings.Contains(err.Error(), "already scheduled for creation") {
			t.Errorf("Expected creation conflict error, got: %v", err)
		}
	})

	t.Run("Detect duplicate directory creation", func(t *testing.T) {
		batch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// Create first directory - should succeed
		_, err := batch.CreateDir("testdir")
		if err != nil {
			t.Fatalf("First CreateDir should succeed: %v", err)
		}

		// Create second directory with same path - should fail
		_, err = batch.CreateDir("testdir")
		if err == nil {
			t.Error("Expected conflict error for duplicate directory creation")
		}

		if !strings.Contains(err.Error(), "already scheduled for creation") {
			t.Errorf("Expected creation conflict error, got: %v", err)
		}
	})

	t.Run("Detect duplicate delete operations", func(t *testing.T) {
		batch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// First delete - should succeed validation
		_, err := batch.Delete("somefile.txt")
		if err != nil {
			t.Fatalf("First Delete should succeed validation: %v", err)
		}

		// Second delete of same file - should fail
		_, err = batch.Delete("somefile.txt")
		if err == nil {
			t.Error("Expected conflict error for duplicate delete operations")
		}

		if !strings.Contains(err.Error(), "already scheduled for deletion") {
			t.Errorf("Expected deletion conflict error, got: %v", err)
		}
	})

	t.Run("Detect copy destination conflicts", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		
		// Create source files first
		err := testFS.WriteFile("source1.txt", []byte("source1"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}
		err = testFS.WriteFile("source2.txt", []byte("source2"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		batch := synthfs.NewBatch().WithFileSystem(testFS)

		// First copy - should succeed
		_, err = batch.Copy("source1.txt", "destination.txt")
		if err != nil {
			t.Fatalf("First Copy should succeed: %v", err)
		}

		// Second copy to same destination - should fail
		_, err = batch.Copy("source2.txt", "destination.txt")
		if err == nil {
			t.Error("Expected conflict error for copy to same destination")
		}

		if !strings.Contains(err.Error(), "already scheduled for creation") {
			t.Errorf("Expected destination conflict error, got: %v", err)
		}
	})

	t.Run("Detect move destination conflicts", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		
		// Create source files first
		err := testFS.WriteFile("old1.txt", []byte("old1"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}
		err = testFS.WriteFile("old2.txt", []byte("old2"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		batch := synthfs.NewBatch().WithFileSystem(testFS)

		// First move - should succeed
		_, err = batch.Move("old1.txt", "new.txt")
		if err != nil {
			t.Fatalf("First Move should succeed: %v", err)
		}

		// Second move to same destination - should fail
		_, err = batch.Move("old2.txt", "new.txt")
		if err == nil {
			t.Error("Expected conflict error for move to same destination")
		}

		if !strings.Contains(err.Error(), "already scheduled for creation") {
			t.Errorf("Expected destination conflict error, got: %v", err)
		}
	})

	t.Run("Detect symlink creation conflicts", func(t *testing.T) {
		batch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// First symlink - should succeed
		_, err := batch.CreateSymlink("target1.txt", "link.txt")
		if err != nil {
			t.Fatalf("First CreateSymlink should succeed: %v", err)
		}

		// Second symlink with same path - should fail
		_, err = batch.CreateSymlink("target2.txt", "link.txt")
		if err == nil {
			t.Error("Expected conflict error for duplicate symlink creation")
		}

		if !strings.Contains(err.Error(), "already scheduled for creation") {
			t.Errorf("Expected creation conflict error, got: %v", err)
		}
	})

	t.Run("Detect archive creation conflicts", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		
		// Create source file for archives
		err := testFS.WriteFile("source.txt", []byte("source"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		batch := synthfs.NewBatch().WithFileSystem(testFS)

		// First archive - should succeed
		_, err = batch.CreateArchive("backup.tar.gz", synthfs.ArchiveFormatTarGz, "source.txt")
		if err != nil {
			t.Fatalf("First CreateArchive should succeed: %v", err)
		}

		// Second archive with same path - should fail
		_, err = batch.CreateArchive("backup.tar.gz", synthfs.ArchiveFormatZip, "source.txt")
		if err == nil {
			t.Error("Expected conflict error for duplicate archive creation")
		}

		if !strings.Contains(err.Error(), "already scheduled for creation") {
			t.Errorf("Expected creation conflict error, got: %v", err)
		}
	})

	t.Run("Allow valid non-conflicting operations", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		
		// Create source files
		err := testFS.WriteFile("source.txt", []byte("source"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		batch := synthfs.NewBatch().WithFileSystem(testFS)

		// All these operations should succeed because they don't conflict
		_, err = batch.CreateDir("dir1")
		if err != nil {
			t.Fatalf("CreateDir should succeed: %v", err)
		}

		_, err = batch.CreateDir("dir2")
		if err != nil {
			t.Fatalf("CreateDir should succeed: %v", err)
		}

		_, err = batch.CreateFile("file1.txt", []byte("content1"))
		if err != nil {
			t.Fatalf("CreateFile should succeed: %v", err)
		}

		_, err = batch.CreateFile("file2.txt", []byte("content2"))
		if err != nil {
			t.Fatalf("CreateFile should succeed: %v", err)
		}

		_, err = batch.Copy("source.txt", "copy1.txt")
		if err != nil {
			t.Fatalf("Copy should succeed: %v", err)
		}

		_, err = batch.Copy("source.txt", "copy2.txt")
		if err != nil {
			t.Fatalf("Copy should succeed: %v", err)
		}

		_, err = batch.Delete("to-delete1.txt")
		if err != nil {
			t.Fatalf("Delete should succeed: %v", err)
		}

		_, err = batch.Delete("to-delete2.txt")
		if err != nil {
			t.Fatalf("Delete should succeed: %v", err)
		}

		// Verify we have all the expected operations
		ops := batch.Operations()
		if len(ops) < 8 {
			t.Errorf("Expected at least 8 operations, got %d", len(ops))
		}
	})

	t.Run("Mixed conflict scenarios", func(t *testing.T) {
		batch := synthfs.NewBatch().WithFileSystem(synthfs.NewTestFileSystem())

		// Create file
		_, err := batch.CreateFile("conflict.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile should succeed: %v", err)
		}

		// Try to create directory with same name - should fail
		_, err = batch.CreateDir("conflict.txt")
		if err == nil {
			t.Error("Expected conflict error for file/directory name collision")
		}

		// Try to delete the file we're creating - should fail
		_, err = batch.Delete("conflict.txt")
		if err == nil {
			t.Error("Expected conflict error for create/delete on same path")
		}

		if !strings.Contains(err.Error(), "already scheduled for creation") {
			t.Errorf("Expected creation conflict error, got: %v", err)
		}
	})
}