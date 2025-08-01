package synthfs

import (
	"context"
	"io/fs"
	"testing"
	"time"
)

func TestSyncPatterns(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("Basic directory sync", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Create source directory structure
		if err := filesys.MkdirAll("src/subdir", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/file1.txt", []byte("content1"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/file2.txt", []byte("content2"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/subdir/file3.txt", []byte("content3"), 0644); err != nil {
			t.Fatal(err)
		}

		// Sync to destination
		result, err := SyncDirectories(ctx, filesys, "src", "dst")
		if err != nil {
			t.Fatalf("Sync failed: %v", err)
		}

		// Verify result
		if len(result.FilesCreated) != 3 {
			t.Errorf("Expected 3 files created, got %d", len(result.FilesCreated))
		}
		if len(result.DirsCreated) != 1 {
			t.Errorf("Expected 1 directory created, got %d", len(result.DirsCreated))
		}

		// Verify files exist
		files := []string{"dst/file1.txt", "dst/file2.txt", "dst/subdir/file3.txt"}
		for _, file := range files {
			content, err := filesys.ReadFile(file)
			if err != nil {
				t.Errorf("File %s should exist: %v", file, err)
				continue
			}
			// Verify content was copied
			if file == "dst/file1.txt" && string(content) != "content1" {
				t.Errorf("File %s has wrong content", file)
			}
		}
	})

	t.Run("Sync with updates", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Create initial structure
		if err := filesys.MkdirAll("src", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.MkdirAll("dst", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/file1.txt", []byte("original"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("dst/file1.txt", []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}

		// Update source
		if err := filesys.WriteFile("src/file1.txt", []byte("updated"), 0644); err != nil {
			t.Fatal(err)
		}

		// Sync without UpdateNewer (should always update)
		result, err := SyncDirectories(ctx, filesys, "src", "dst")
		if err != nil {
			t.Fatal(err)
		}

		if len(result.FilesUpdated) != 1 {
			t.Errorf("Expected 1 file updated, got %d", len(result.FilesUpdated))
		}

		// Verify content
		content, _ := filesys.ReadFile("dst/file1.txt")
		if string(content) != "updated" {
			t.Error("File not updated correctly")
		}
	})

	t.Run("Sync with DeleteExtra", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Create structures
		if err := filesys.MkdirAll("src", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.MkdirAll("dst/extra", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/keep.txt", []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("dst/keep.txt", []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("dst/remove.txt", []byte("remove"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("dst/extra/file.txt", []byte("extra"), 0644); err != nil {
			t.Fatal(err)
		}

		// Sync with DeleteExtra
		opts := SyncOptions{DeleteExtra: true}
		result, err := SyncDirectories(ctx, filesys, "src", "dst", opts)
		if err != nil {
			t.Fatal(err)
		}

		// Check deletions
		if len(result.FilesDeleted) != 2 {
			t.Errorf("Expected 2 files deleted, got %d", len(result.FilesDeleted))
		}
		if len(result.DirsDeleted) != 1 {
			t.Errorf("Expected 1 directory deleted, got %d", len(result.DirsDeleted))
		}

		// Verify deletions
		if _, err := filesys.Stat("dst/remove.txt"); err == nil {
			t.Error("Extra file should be deleted")
		}
		if _, err := filesys.Stat("dst/extra"); err == nil {
			t.Error("Extra directory should be deleted")
		}
		// Keep file should still exist
		if _, err := filesys.Stat("dst/keep.txt"); err != nil {
			t.Error("Matching file should not be deleted")
		}
	})

	t.Run("Sync with filter", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Create source files
		if err := filesys.MkdirAll("src", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/include.txt", []byte("include"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/exclude.log", []byte("exclude"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/readme.md", []byte("readme"), 0644); err != nil {
			t.Fatal(err)
		}

		// Filter to exclude .log files
		opts := SyncOptions{
			Filter: func(path string, info fs.FileInfo) bool {
				return len(path) < 4 || path[len(path)-4:] != ".log"
			},
		}

		result, err := SyncDirectories(ctx, filesys, "src", "dst", opts)
		if err != nil {
			t.Fatal(err)
		}

		// Should only sync 2 files
		if len(result.FilesCreated) != 2 {
			t.Errorf("Expected 2 files created, got %d", len(result.FilesCreated))
		}

		// Verify filtered file not synced
		if _, err := filesys.Stat("dst/exclude.log"); err == nil {
			t.Error("Filtered file should not be synced")
		}
		if _, err := filesys.Stat("dst/include.txt"); err != nil {
			t.Error("Non-filtered file should be synced")
		}
	})

	t.Run("Sync with UpdateNewer", func(t *testing.T) {
		// Use a real filesystem to test modification time behavior
		tmpDir := t.TempDir()
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewOSFileSystemWithPaths(tmpDir)

		// Create files
		if err := filesys.MkdirAll("src", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.MkdirAll("dst", 0755); err != nil {
			t.Fatal(err)
		}
		
		// Create older source file
		if err := filesys.WriteFile("src/old.txt", []byte("old content"), 0644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		if err := filesys.WriteFile("dst/old.txt", []byte("newer content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create newer source file
		if err := filesys.WriteFile("dst/new.txt", []byte("old content"), 0644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
		if err := filesys.WriteFile("src/new.txt", []byte("newer content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Sync with UpdateNewer
		opts := SyncOptions{UpdateNewer: true}
		result, err := SyncDirectories(ctx, filesys, "src", "dst", opts)
		if err != nil {
			t.Fatal(err)
		}

		// Should only update the newer file
		if len(result.FilesUpdated) != 1 {
			t.Errorf("Expected 1 file updated, got %d", len(result.FilesUpdated))
		}

		// Verify correct file was updated
		content, _ := filesys.ReadFile("dst/new.txt")
		if string(content) != "newer content" {
			t.Error("Newer file should be updated")
		}
		content, _ = filesys.ReadFile("dst/old.txt")
		if string(content) != "newer content" {
			t.Error("Older file should not be updated")
		}
	})

	t.Run("Sync with DryRun", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Create source
		if err := filesys.MkdirAll("src", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/file.txt", []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		// Dry run sync
		opts := SyncOptions{DryRun: true}
		result, err := SyncDirectories(ctx, filesys, "src", "dst", opts)
		if err != nil {
			t.Fatal(err)
		}

		// Should report changes but not make them
		if len(result.FilesCreated) != 1 {
			t.Error("Dry run should report file creation")
		}

		// Verify no actual changes
		if _, err := filesys.Stat("dst/file.txt"); err == nil {
			t.Error("Dry run should not create files")
		}
	})

	t.Run("SyncBuilder fluent API", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Setup
		if err := filesys.MkdirAll("src", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.MkdirAll("dst", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/keep.txt", []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("dst/remove.txt", []byte("remove"), 0644); err != nil {
			t.Fatal(err)
		}

		// Use builder
		result, err := NewSyncBuilder("src", "dst").
			DeleteExtra().
			WithFilter(func(path string, info fs.FileInfo) bool {
				return path != ".git" // Exclude .git
			}).
			Execute(ctx, filesys)

		if err != nil {
			t.Fatal(err)
		}

		// Verify
		if len(result.FilesCreated) != 1 {
			t.Error("Should create file")
		}
		if len(result.FilesDeleted) != 1 {
			t.Error("Should delete extra file")
		}
	})

	t.Run("Sync operation validation", func(t *testing.T) {
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Non-existent source
		op := sfs.Sync("nonexistent", "dst")
		err := op.Validate(ctx, filesys)
		if err == nil {
			t.Error("Should fail validation with non-existent source")
		}

		// Create source as file, not directory
		if err := filesys.WriteFile("notdir", []byte("file"), 0644); err != nil {
			t.Fatal(err)
		}
		op = sfs.Sync("notdir", "dst")
		err = op.Validate(ctx, filesys)
		if err == nil {
			t.Error("Should fail validation when source is not a directory")
		}
	})

	t.Run("Sync with symlinks", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Create source with symlink
		if err := filesys.MkdirAll("src", 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile("src/target.txt", []byte("target"), 0644); err != nil {
			t.Fatal(err)
		}
		if fullFS, ok := FileSystem(filesys).(FullFileSystem); ok {
			_ = fullFS.Symlink("target.txt", "src/link.txt")
		}

		// Sync with PreserveSymlinks
		opts := SyncOptions{PreserveSymlinks: true}
		result, err := SyncDirectories(ctx, filesys, "src", "dst", opts)
		if err != nil {
			t.Fatal(err)
		}

		// Check if symlink was created
		if len(result.SymlinksCreated) > 0 {
			// Verify symlink
			if fullFS, ok := FileSystem(filesys).(FullFileSystem); ok {
				target, err := fullFS.Readlink("dst/link.txt")
				if err == nil && target != "target.txt" {
					t.Errorf("Symlink has wrong target: %s", target)
				}
			}
		} else {
			t.Log("Warning: Symlinks not supported by test filesystem")
		}
	})

	t.Run("Sync empty directories", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Create empty source directory
		if err := filesys.MkdirAll("src/empty", 0755); err != nil {
			t.Fatal(err)
		}

		// Sync
		result, err := SyncDirectories(ctx, filesys, "src", "dst")
		if err != nil {
			t.Fatal(err)
		}

		// Should create the empty directory
		if len(result.DirsCreated) != 1 {
			t.Error("Should create empty directory")
		}

		// Verify
		info, err := filesys.Stat("dst/empty")
		if err != nil {
			t.Error("Empty directory should be created")
		} else if !info.IsDir() {
			t.Error("Should be a directory")
		}
	})

	t.Run("Sync deeply nested structure", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		filesys := NewTestFileSystemWithPaths("/workspace")

		// Create deep structure
		deepPath := "src/a/b/c/d/e"
		if err := filesys.MkdirAll(deepPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := filesys.WriteFile(deepPath+"/deep.txt", []byte("deep"), 0644); err != nil {
			t.Fatal(err)
		}

		// Sync
		result, err := SyncDirectories(ctx, filesys, "src", "dst")
		if err != nil {
			t.Fatal(err)
		}

		// Verify result has files
		if len(result.FilesCreated) == 0 {
			t.Error("Should have created files")
		}

		// Verify deep file
		content, err := filesys.ReadFile("dst/a/b/c/d/e/deep.txt")
		if err != nil {
			t.Error("Deep file should be synced")
		} else if string(content) != "deep" {
			t.Error("Deep file has wrong content")
		}
	})
}
