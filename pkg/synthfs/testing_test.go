package synthfs_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

func TestTestFileSystem(t *testing.T) {
	tfs := synthfs.NewTestFileSystem()

	t.Run("WriteFile and Open", func(t *testing.T) {
		content := []byte("test content")
		path := "test.txt"

		// Write file
		err := tfs.WriteFile(path, content, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Open and read
		file, err := tfs.Open(path)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				t.Logf("Warning: failed to close file: %v", closeErr)
			}
		}()

		info, err := file.Stat()
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.IsDir() {
			t.Errorf("Expected file, got directory")
		}

		if info.Size() != int64(len(content)) {
			t.Errorf("Expected size %d, got %d", len(content), info.Size())
		}
	})

	t.Run("MkdirAll", func(t *testing.T) {
		path := "test/dir"
		err := tfs.MkdirAll(path, 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		info, err := tfs.Stat(path)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if !info.IsDir() {
			t.Errorf("Expected directory, got file")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		// Create a file
		path := "to-remove.txt"
		err := tfs.WriteFile(path, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Remove it
		err = tfs.Remove(path)
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		// Verify it's gone
		_, err = tfs.Stat(path)
		if err == nil {
			t.Errorf("Expected file to be removed")
		}
	})

	t.Run("RemoveAll", func(t *testing.T) {
		// Create directory structure
		err := tfs.MkdirAll("parent/child", 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		err = tfs.WriteFile("parent/file.txt", []byte("test"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		err = tfs.WriteFile("parent/child/nested.txt", []byte("test"), 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Remove parent and all children
		err = tfs.RemoveAll("parent")
		if err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}

		// Verify everything is gone
		_, err = tfs.Stat("parent")
		if err == nil {
			t.Errorf("Expected parent to be removed")
		}

		_, err = tfs.Stat("parent/file.txt")
		if err == nil {
			t.Errorf("Expected parent/file.txt to be removed")
		}

		_, err = tfs.Stat("parent/child/nested.txt")
		if err == nil {
			t.Errorf("Expected parent/child/nested.txt to be removed")
		}
	})

	t.Run("Invalid paths", func(t *testing.T) {
		invalidPath := "../../../etc/passwd"

		err := tfs.WriteFile(invalidPath, []byte("test"), 0644)
		if err == nil {
			t.Errorf("Expected WriteFile to fail with invalid path")
		}

		err = tfs.MkdirAll(invalidPath, 0755)
		if err == nil {
			t.Errorf("Expected MkdirAll to fail with invalid path")
		}
	})
}

func TestNewTestFileSystemFromMap(t *testing.T) {
	files := map[string]*fstest.MapFile{
		"existing.txt": {
			Data: []byte("existing content"),
			Mode: 0644,
		},
		"dir": {
			Mode: fs.ModeDir | 0755,
		},
	}

	tfs := synthfs.NewTestFileSystemFromMap(files)

	t.Run("Existing file", func(t *testing.T) {
		info, err := tfs.Stat("existing.txt")
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if info.IsDir() {
			t.Errorf("Expected file, got directory")
		}

		if info.Size() != 16 { // len("existing content")
			t.Errorf("Expected size 16, got %d", info.Size())
		}
	})

	t.Run("Existing directory", func(t *testing.T) {
		info, err := tfs.Stat("dir")
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}

		if !info.IsDir() {
			t.Errorf("Expected directory, got file")
		}
	})
}

func TestTestHelper(t *testing.T) {
	t.Run("NewTestHelper", func(t *testing.T) {
		helper := synthfs.NewTestHelper(t)

		if helper.FS() == nil {
			t.Errorf("Expected non-nil filesystem")
		}

		if helper.Context() == nil {
			t.Errorf("Expected non-nil context")
		}
	})

	t.Run("NewTestHelperWithFiles", func(t *testing.T) {
		files := map[string]*fstest.MapFile{
			"test.txt": {
				Data: []byte("test content"),
				Mode: 0644,
			},
		}

		helper := synthfs.NewTestHelperWithFiles(t, files)

		// The file should exist
		helper.AssertFileExists("test.txt", []byte("test content"))
	})

	t.Run("AssertFileExists", func(t *testing.T) {
		helper := synthfs.NewTestHelper(t)

		// Create a file
		content := []byte("test content")
		err := helper.FS().WriteFile("test.txt", content, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Should not panic
		helper.AssertFileExists("test.txt", content)
	})

	t.Run("AssertDirExists", func(t *testing.T) {
		helper := synthfs.NewTestHelper(t)

		// Create a directory
		err := helper.FS().MkdirAll("testdir", 0755)
		if err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		// Should not panic
		helper.AssertDirExists("testdir")
	})

	t.Run("AssertNotExists", func(t *testing.T) {
		helper := synthfs.NewTestHelper(t)

		// Should not panic for non-existent file
		helper.AssertNotExists("nonexistent.txt")
	})

	t.Run("ExecuteAndAssert", func(t *testing.T) {
		helper := synthfs.NewTestHelper(t)

		// Use the new Batch API instead of old ops
		batch := synthfs.NewBatch().WithFileSystem(helper.FS())
		_, err := batch.CreateFile("success.txt", []byte("content"))
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		result, err := batch.Execute()
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected successful execution")
		}
	})

	t.Run("ExecuteAndExpectError", func(t *testing.T) {
		helper := synthfs.NewTestHelper(t)

		// Use the new Batch API with invalid operation to cause error
		batch := synthfs.NewBatch().WithFileSystem(helper.FS())
		_, err := batch.CreateFile("", []byte("content")) // Empty path should fail validation
		if err == nil {
			t.Fatal("Expected validation error for empty path")
		}

		// The error should have occurred during validation, which is correct behavior
		t.Log("Validation correctly caught empty path error")
	})
}

func TestValidateTestFS(t *testing.T) {
	t.Run("Valid filesystem", func(t *testing.T) {
		tfs := synthfs.NewTestFileSystem()

		// Add some test files
		if err := tfs.WriteFile("test.txt", []byte("content"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if err := tfs.MkdirAll("testdir", 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		// Should not panic - provide the expected files list
		synthfs.ValidateTestFS(t, tfs)
	})
}

func TestIsSubPath(t *testing.T) {
	// This tests the internal isSubPath function via RemoveAll behavior
	tfs := synthfs.NewTestFileSystem()

	// Create nested structure
	if err := tfs.WriteFile("parent/child/file.txt", []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := tfs.WriteFile("parent/sibling.txt", []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := tfs.WriteFile("other/file.txt", []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Remove parent should remove its children but not others
	err := tfs.RemoveAll("parent")
	if err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	// Parent and its children should be gone
	_, err = tfs.Stat("parent/child/file.txt")
	if err == nil {
		t.Errorf("Expected parent/child/file.txt to be removed")
	}

	_, err = tfs.Stat("parent/sibling.txt")
	if err == nil {
		t.Errorf("Expected parent/sibling.txt to be removed")
	}

	// Other should still exist
	_, err = tfs.Stat("other/file.txt")
	if err != nil {
		t.Errorf("Expected other/file.txt to still exist")
	}
}

// Test the fs.FS compliance of TestFileSystem
func TestTestFileSystem_fstest_Compliance(t *testing.T) {
	tfs := synthfs.NewTestFileSystem()

	// Add some test files
	if err := tfs.WriteFile("file.txt", []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := tfs.MkdirAll("dir", 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := tfs.WriteFile("dir/nested.txt", []byte("nested"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Use Go's standard filesystem test
	err := fstest.TestFS(tfs.MapFS, "file.txt", "dir", "dir/nested.txt")
	if err != nil {
		t.Fatalf("TestFS validation failed: %v", err)
	}
}
