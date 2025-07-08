package synthfs_test

import (
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestBatchDuplicatePathDetection(t *testing.T) {
	// Phase I, Milestone 2: Test duplicate path detection in batch operations

	t.Run("Detect duplicate file creation", func(t *testing.T) {
		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(synthfs.NewTestFileSystem())
		_, err := batch.CreateFile("test.txt", []byte("content1"))
		if err != nil {
			t.Fatalf("First CreateFile failed: %v", err)
		}

		// Second creation on the same path should fail
		_, err = batch.CreateFile("test.txt", []byte("content2"))
		if err == nil {
			t.Fatal("Expected creation conflict error, but got nil")
		}

		// The error message has changed in Phase II, so we just check for a conflict.
		if !strings.Contains(err.Error(), "conflicts with existing state") {
			t.Errorf("Expected a conflict error, but got: %v", err)
		}
	})

	t.Run("Detect duplicate directory creation", func(t *testing.T) {
		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(synthfs.NewTestFileSystem())
		_, err := batch.CreateDir("testdir")
		if err != nil {
			t.Fatalf("First CreateDir failed: %v", err)
		}

		// Second creation on the same path should fail
		_, err = batch.CreateDir("testdir")
		if err == nil {
			t.Fatal("Expected creation conflict error, but got nil")
		}
		if !strings.Contains(err.Error(), "conflicts with existing state") {
			t.Errorf("Expected a conflict error, but got: %v", err)
		}
	})

	t.Run("Detect duplicate delete operations", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		// Pre-create the file to be deleted to satisfy Phase II validation
		err := testFS.WriteFile("somefile.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)
		_, err = batch.Delete("somefile.txt")
		if err != nil {
			t.Fatalf("First Delete should succeed validation: %v", err)
		}

		// Second delete on the same path should fail because the path is no longer projected to exist.
		_, err = batch.Delete("somefile.txt")
		if err == nil {
			t.Fatal("Expected deletion conflict error, but got nil")
		}
		if !strings.Contains(err.Error(), "is not projected to exist") {
			t.Errorf("Expected a 'not projected to exist' error, but got: %v", err)
		}
	})

	t.Run("Detect copy destination conflicts", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		// Pre-create source files
		err := testFS.WriteFile("source1.txt", []byte("s1"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source1: %v", err)
		}
		err = testFS.WriteFile("source2.txt", []byte("s2"), 0644)
		if err != nil {
			t.Fatalf("Failed to create source2: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)
		_, err = batch.Copy("source1.txt", "destination.txt")
		if err != nil {
			t.Fatalf("First Copy failed: %v", err)
		}

		// Second copy to the same destination should fail
		_, err = batch.Copy("source2.txt", "destination.txt")
		if err == nil {
			t.Fatal("Expected destination conflict error, but got nil")
		}
		if !strings.Contains(err.Error(), "conflicts with existing state") {
			t.Errorf("Expected a destination conflict error, but got: %v", err)
		}
	})

	t.Run("Detect move destination conflicts", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		// Pre-create source files
		err := testFS.WriteFile("old1.txt", []byte("o1"), 0644)
		if err != nil {
			t.Fatalf("Failed to create old1: %v", err)
		}
		err = testFS.WriteFile("old2.txt", []byte("o2"), 0644)
		if err != nil {
			t.Fatalf("Failed to create old2: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)
		_, err = batch.Move("old1.txt", "new.txt")
		if err != nil {
			t.Fatalf("First Move failed: %v", err)
		}

		// Second move to the same destination should fail
		_, err = batch.Move("old2.txt", "new.txt")
		if err == nil {
			t.Fatal("Expected destination conflict error, but got nil")
		}
		if !strings.Contains(err.Error(), "conflicts with existing state") {
			t.Errorf("Expected a destination conflict error, but got: %v", err)
		}
	})

	t.Run("Detect symlink creation conflicts", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		// Pre-create targets
		err := testFS.WriteFile("target1.txt", []byte("t1"), 0644)
		if err != nil {
			t.Fatalf("Failed to create target1: %v", err)
		}
		err = testFS.WriteFile("target2.txt", []byte("t2"), 0644)
		if err != nil {
			t.Fatalf("Failed to create target2: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)
		_, err = batch.CreateSymlink("target1.txt", "link.txt")
		if err != nil {
			t.Fatalf("First CreateSymlink failed: %v", err)
		}

		// Second symlink with the same name should fail
		_, err = batch.CreateSymlink("target2.txt", "link.txt")
		if err == nil {
			t.Fatal("Expected creation conflict error, but got nil")
		}
		if !strings.Contains(err.Error(), "conflicts with existing state") {
			t.Errorf("Expected a conflict error, but got: %v", err)
		}
	})

	t.Run("Detect archive creation conflicts", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		// Pre-create source files
		err := testFS.WriteFile("src1.txt", []byte("s1"), 0644)
		if err != nil {
			t.Fatalf("Failed to create src1: %v", err)
		}
		err = testFS.WriteFile("src2.txt", []byte("s2"), 0644)
		if err != nil {
			t.Fatalf("Failed to create src2: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)
		_, err = batch.CreateArchive("backup.tar.gz", synthfs.ArchiveFormatTarGz, "src1.txt")
		if err != nil {
			t.Fatalf("First CreateArchive failed: %v", err)
		}

		// Second archive with the same name should fail
		_, err = batch.CreateArchive("backup.tar.gz", synthfs.ArchiveFormatTarGz, "src2.txt")
		if err == nil {
			t.Fatal("Expected creation conflict error, but got nil")
		}
		if !strings.Contains(err.Error(), "conflicts with existing state") {
			t.Errorf("Expected a conflict error, but got: %v", err)
		}
	})

	t.Run("Allow creating a file after its deletion", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		err := testFS.WriteFile("file.txt", []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)
		// This sequence should be allowed by a smart tracker.
		// Delete the original file.
		_, err = batch.Delete("file.txt")
		if err != nil {
			t.Fatalf("Delete operation failed: %v", err)
		}
		// Re-create it.
		_, err = batch.CreateFile("file.txt", []byte("new content"))
		if err == nil {
			t.Fatal("Expected conflict error for create-after-delete in this phase, but got nil")
		}
		// In a more advanced phase, this might be allowed. For now, we expect a conflict.
		if !strings.Contains(err.Error(), "path was scheduled for deletion") {
			t.Errorf("Expected a specific conflict error for create-after-delete, got: %v", err)
		}
	})

	t.Run("Allow valid, non-conflicting operations", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		// Pre-create files to avoid non-existence errors
		err := testFS.WriteFile("to-copy.txt", []byte("c"), 0644)
		if err != nil {
			t.Fatal(err)
		}
		err = testFS.WriteFile("to-move.txt", []byte("m"), 0644)
		if err != nil {
			t.Fatal(err)
		}
		err = testFS.WriteFile("to-delete1.txt", []byte("d1"), 0644)
		if err != nil {
			t.Fatal(err)
		}
		err = testFS.WriteFile("to-delete2.txt", []byte("d2"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		registry := synthfs.GetDefaultRegistry()
	fs := synthfs.NewTestFileSystem()
	batch := synthfs.NewBatch(fs, registry).WithFileSystem(testFS)

		// A mix of valid operations that should all be added successfully.
		// The `add` function handles adding to the operations slice.
		if _, err := batch.CreateFile("file1.txt", []byte("content1")); err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}
		if _, err := batch.CreateDir("dir1"); err != nil {
			t.Fatalf("CreateDir failed: %v", err)
		}
		if _, err := batch.Copy("to-copy.txt", "copy.txt"); err != nil {
			t.Fatalf("Copy failed: %v", err)
		}
		if _, err := batch.Move("to-move.txt", "move.txt"); err != nil {
			t.Fatalf("Move failed: %v", err)
		}
		if _, err := batch.Delete("to-delete1.txt"); err != nil {
			t.Fatalf("Delete should succeed: %v", err)
		}
		if _, err := batch.Delete("to-delete2.txt"); err != nil {
			t.Fatalf("Delete should succeed: %v", err)
		}

		// All operations should be added without error
		if len(batch.Operations()) != 6 {
			t.Errorf("Expected 6 operations, got %d", len(batch.Operations()))
		}
	})

	t.Run("Mixed conflict scenarios", func(t *testing.T) {
		testFS := synthfs.NewTestFileSystem()
		if err := testFS.WriteFile("file.txt", []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Scenario 1: Create then delete -> Should conflict
		batch1 := synthfs.NewBatch().WithFileSystem(testFS)
		if _, err := batch1.CreateFile("new-file.txt", []byte("new")); err != nil {
			t.Fatalf("CreateFile should not fail here: %v", err)
		}
		if _, err := batch1.Delete("new-file.txt"); err == nil {
			t.Fatal("Expected conflict error for create/delete on same path")
		}

		// Scenario 2: Delete then create -> Should conflict under current rules
		batch2 := synthfs.NewBatch().WithFileSystem(testFS)
		if _, err := batch2.Delete("file.txt"); err != nil {
			t.Fatalf("Delete should not fail here: %v", err)
		}
		if _, err := batch2.CreateFile("file.txt", []byte("recreated")); err == nil {
			t.Fatal("Expected conflict error for delete/create on same path")
		}
	})
}
