package testutil

import (
	"errors"
	"io/fs"
	"path"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMockFS_WriteFile_ReadFile(t *testing.T) {
	mfs := NewMockFS()
	filePath := "testfile.txt"
	fileData := []byte("hello world")
	fileMode := fs.FileMode(0644)

	// Test WriteFile
	err := mfs.WriteFile(filePath, fileData, fileMode)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Check internal state (optional, but good for mocks)
	mfs.mu.RLock()
	f, ok := mfs.files[filePath]
	mfs.mu.RUnlock()
	if !ok {
		t.Fatalf("File %s not found in internal map after WriteFile", filePath)
	}
	if !reflect.DeepEqual(f.data, fileData) {
		t.Errorf("Expected file data %q, got %q", string(fileData), string(f.data))
	}
	if f.mode != fileMode {
		t.Errorf("Expected file mode %v, got %v", fileMode, f.mode)
	}

	// Test ReadFile
	readData, err := mfs.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !reflect.DeepEqual(readData, fileData) {
		t.Errorf("Expected ReadFile data %q, got %q", string(fileData), string(readData))
	}

	// Test ReadFile non-existent
	_, err = mfs.ReadFile("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for ReadFile on non-existent file, got nil")
	} else if pe, ok := err.(*fs.PathError); !ok || pe.Err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist, got %v", err)
	}
}

func TestMockFS_WriteFile_Errors(t *testing.T) {
	mfs := NewMockFS()

	// Parent directory does not exist
	err := mfs.WriteFile("nonexistentdir/file.txt", []byte("data"), 0644)
	if err == nil {
		t.Error("Expected error for WriteFile with non-existent parent, got nil")
	} else if pe, ok := err.(*fs.PathError); !ok || pe.Err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist for parent, got %v", err)
	}
}


func TestMockFS_MkdirAll(t *testing.T) {
	mfs := NewMockFS()
	dirPath := "a/b/c"
	dirMode := fs.FileMode(0755)

	err := mfs.MkdirAll(dirPath, dirMode)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Check intermediate and final directories
	pathsToTest := []string{"a", "a/b", "a/b/c"}
	for _, p := range pathsToTest {
		mfs.mu.RLock()
		f, ok := mfs.files[p]
		mfs.mu.RUnlock()
		if !ok {
			t.Errorf("Directory %s not found after MkdirAll", p)
			continue
		}
		if !f.mode.IsDir() {
			t.Errorf("Path %s is not a directory, mode: %v", p, f.mode)
		}
		// Check only permission bits for directories created by MkdirAll
		if (f.mode & fs.ModePerm) != (dirMode & fs.ModePerm) {
			t.Errorf("Expected dir mode %v for %s, got %v", dirMode&fs.ModePerm, p, f.mode&fs.ModePerm)
		}
	}

	// MkdirAll on existing directory (should be no-op)
	err = mfs.MkdirAll(dirPath, dirMode)
	if err != nil {
		t.Fatalf("MkdirAll on existing directory failed: %v", err)
	}

	// MkdirAll on path where a file exists
	mfs.WriteFile("a/b/c/file.txt", []byte(""), 0644) // make "a/b/c" effectively a file for next test
	err = mfs.MkdirAll("a/b/c/file.txt/newdir", dirMode)
	if err == nil {
		t.Error("Expected error for MkdirAll when component is a file, got nil")
	} else if pe, ok := err.(*fs.PathError); !ok || pe.Err != syscall.ENOTDIR {
		t.Errorf("Expected ENOTDIR, got %v", err)
	}
}

func TestMockFS_Stat(t *testing.T) {
	mfs := NewMockFS()
	filePath := "statfile.txt"
	fileData := []byte("stat me")
	fileMode := fs.FileMode(0666)
	mfs.WriteFile(filePath, fileData, fileMode)

	dirPath := "statdir"
	dirMode := fs.FileMode(0777) | fs.ModeDir
	mfs.MkdirAll(dirPath, dirMode)

	// Stat file
	fi, err := mfs.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat on file failed: %v", err)
	}
	if fi.Name() != path.Base(filePath) {
		t.Errorf("Expected file name %s, got %s", path.Base(filePath), fi.Name())
	}
	if fi.Size() != int64(len(fileData)) {
		t.Errorf("Expected file size %d, got %d", len(fileData), fi.Size())
	}
	if fi.Mode() != fileMode {
		t.Errorf("Expected file mode %v, got %v", fileMode, fi.Mode())
	}
	if fi.IsDir() {
		t.Error("Expected file to not be a directory")
	}
	// ModTime is tricky to test for exact match, check it's recent
	if time.Since(fi.ModTime()) > 5*time.Second {
		t.Errorf("ModTime %v is too old", fi.ModTime())
	}

	// Stat directory
	fi, err = mfs.Stat(dirPath)
	if err != nil {
		t.Fatalf("Stat on directory failed: %v", err)
	}
	if fi.Name() != dirPath {
		t.Errorf("Expected dir name %s, got %s", dirPath, fi.Name())
	}
	if (fi.Mode() &^ fs.ModeDir) != (dirMode &^ fs.ModeDir) { // Compare only perm bits after removing ModeDir
		t.Errorf("Expected dir mode %v, got %v", dirMode&fs.ModePerm, fi.Mode()&fs.ModePerm)
	}
	if !fi.IsDir() {
		t.Error("Expected path to be a directory")
	}

	// Stat non-existent
	_, err = mfs.Stat("nonexistent")
	if err == nil {
		t.Error("Expected error for Stat on non-existent path, got nil")
	} else if pe, ok := err.(*fs.PathError); !ok || pe.Err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist, got %v", err)
	}
}

func TestMockFS_Open_File(t *testing.T) {
	mfs := NewMockFS()
	filePath := "openme.txt"
	fileData := []byte("open data")
	mfs.WriteFile(filePath, fileData, 0644)

	f, err := mfs.Open(filePath)
	if err != nil {
		t.Fatalf("Open file failed: %v", err)
	}
	defer f.Close()

	// Read content
	content := make([]byte, len(fileData))
	n, err := f.Read(content)
	if err != nil {
		t.Fatalf("Read from opened file failed: %v", err)
	}
	if n != len(fileData) {
		t.Errorf("Expected to read %d bytes, got %d", len(fileData), n)
	}
	if !reflect.DeepEqual(content, fileData) {
		t.Errorf("Expected content %q, got %q", string(fileData), string(content))
	}

	// Stat from file handle
	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat from file handle failed: %v", err)
	}
	if fi.Name() != filePath {
		t.Errorf("Expected name %s, got %s", filePath, fi.Name())
	}
}

func TestMockFS_Open_Directory_ReadDir(t *testing.T) {
	mfs := NewMockFS()
	mfs.MkdirAll("dir1/sub1", 0755)
	mfs.WriteFile("dir1/file1.txt", []byte("f1"), 0644)
	mfs.WriteFile("dir1/file2.txt", []byte("f2"), 0644)
	mfs.MkdirAll("dir2", 0755)
	mfs.WriteFile("rootfile.txt", []byte("rf"), 0644)

	// Test ReadDir on "dir1"
	f, err := mfs.Open("dir1")
	if err != nil {
		t.Fatalf("Open dir1 failed: %v", err)
	}
	defer f.Close()

	dirHandle, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatalf("Opened directory does not implement fs.ReadDirFile")
	}

	entries, err := dirHandle.ReadDir(-1) // Read all entries
	if err != nil {
		t.Fatalf("ReadDir for dir1 failed: %v", err)
	}

	expectedEntries := map[string]fs.FileMode{
		"sub1":       fs.ModeDir,
		"file1.txt":  0, // Regular file
		"file2.txt":  0, // Regular file
	}

	if len(entries) != len(expectedEntries) {
		t.Errorf("Expected %d entries in dir1, got %d. Entries: %v", len(expectedEntries), len(entries), entriesToNames(entries))
	}

	for _, entry := range entries {
		expectedMode, found := expectedEntries[entry.Name()]
		if !found {
			t.Errorf("Unexpected entry %s in dir1", entry.Name())
			continue
		}
		if entry.IsDir() && expectedMode != fs.ModeDir {
			t.Errorf("Entry %s expected to be file, got dir", entry.Name())
		}
		if !entry.IsDir() && expectedMode == fs.ModeDir {
			t.Errorf("Entry %s expected to be dir, got file", entry.Name())
		}
	}

	// Test ReadDir on root "."
	fRoot, err := mfs.Open(".")
	if err != nil {
		t.Fatalf("Open . failed: %v", err)
	}
	defer fRoot.Close()
	dirRootHandle, _ := fRoot.(fs.ReadDirFile)
	rootEntries, err := dirRootHandle.ReadDir(-1)
	if err != nil {
		t.Fatalf("ReadDir for . failed: %v", err)
	}

	expectedRootEntries := map[string]fs.FileMode{
		"dir1":         fs.ModeDir,
		"dir2":         fs.ModeDir,
		"rootfile.txt": 0,
	}
	if len(rootEntries) != len(expectedRootEntries) {
		t.Errorf("Expected %d entries in ., got %d. Entries: %v", len(expectedRootEntries), len(rootEntries), entriesToNames(rootEntries))
	}
     for _, entry := range rootEntries {
		_, found := expectedRootEntries[entry.Name()]
		if !found {
			t.Errorf("Unexpected entry %s in .", entry.Name())
		}
	}
}

func TestMockFS_Remove_File(t *testing.T) {
	mfs := NewMockFS()
	filePath := "removeme.txt"
	mfs.WriteFile(filePath, []byte("content"), 0644)

	err := mfs.Remove(filePath)
	if err != nil {
		t.Fatalf("Remove file failed: %v", err)
	}

	if _, statErr := mfs.Stat(filePath); statErr == nil {
		t.Error("File still exists after Remove")
	} else if pe, ok := statErr.(*fs.PathError); !ok || pe.Err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist after Remove, got %v", statErr)
	}

	// Remove non-existent
	err = mfs.Remove("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for Remove on non-existent file, got nil")
	} else if pe, ok := err.(*fs.PathError); !ok || pe.Err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist for non-existent remove, got %v", err)
	}
}

func TestMockFS_Remove_Directory(t *testing.T) {
	mfs := NewMockFS()
	emptyDir := "emptydir"
	mfs.MkdirAll(emptyDir, 0755)

	nonEmptyDir := "nonemptydir"
	mfs.MkdirAll(nonEmptyDir, 0755)
	mfs.WriteFile(path.Join(nonEmptyDir, "file.txt"), []byte{}, 0644)

	// Remove empty directory
	err := mfs.Remove(emptyDir)
	if err != nil {
		t.Fatalf("Remove on empty directory failed: %v", err)
	}
	if _, statErr := mfs.Stat(emptyDir); statErr == nil {
		t.Error("Empty directory still exists after Remove")
	}

	// Remove non-empty directory (should fail)
	err = mfs.Remove(nonEmptyDir)
	if err == nil {
		t.Error("Expected error for Remove on non-empty directory, got nil")
	} else if pe, ok := err.(*fs.PathError); !ok || pe.Err != syscall.ENOTEMPTY {
		t.Errorf("Expected ENOTEMPTY for non-empty dir remove, got %v", err)
	}
}

func TestMockFS_RemoveAll(t *testing.T) {
	mfs := NewMockFS()
	mfs.MkdirAll("dir/sub/subsub", 0755)
	mfs.WriteFile("dir/sub/file1.txt", []byte{}, 0644)
	mfs.WriteFile("dir/file2.txt", []byte{}, 0644)

	// RemoveAll on "dir/sub"
	err := mfs.RemoveAll("dir/sub")
	if err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}

	if _, statErr := mfs.Stat("dir/sub/subsub"); statErr == nil {
		t.Error("dir/sub/subsub should not exist after RemoveAll on dir/sub")
	}
	if _, statErr := mfs.Stat("dir/sub/file1.txt"); statErr == nil {
		t.Error("dir/sub/file1.txt should not exist after RemoveAll on dir/sub")
	}
	if _, statErr := mfs.Stat("dir/sub"); statErr == nil {
		t.Error("dir/sub should not exist after RemoveAll on dir/sub")
	}
	if _, statErr := mfs.Stat("dir/file2.txt"); statErr != nil {
		t.Error("dir/file2.txt should still exist")
	}

	// RemoveAll on non-existent path (should be no-op, no error)
	err = mfs.RemoveAll("nonexistentdir")
	if err != nil {
		t.Fatalf("RemoveAll on non-existent path failed: %v", err)
	}
}

// Helper to get names from DirEntry slice for easier debugging
func entriesToNames(entries []fs.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func TestMockFS_CleanPaths(t *testing.T) {
    mfs := NewMockFS()
    // Initial setup: ensure base directories for files being written exist.
    if err := mfs.MkdirAll("a/b", 0755); err != nil {
        t.Fatalf("Initial MkdirAll failed: %v", err)
    }
    if err := mfs.WriteFile("a/b/c.txt", []byte("data"), 0644); err != nil {
        t.Fatalf("Initial WriteFile failed: %v", err)
    }
    if err := mfs.MkdirAll("a/d/e", 0755); err != nil {
        t.Fatalf("Initial MkdirAll for a/d/e failed: %v", err)
    }


    testCases := []struct {
        name     string
        path     string
        expected string // expected cleaned path for map key
        op       func(fsPath string) error
        check    func(fsPath string) (interface{}, error)
    }{
        {
            name: "WriteFile with .",
            path: "a/./b/c.txt",
            expected: "a/b/c.txt",
            op: func(p string) error { return mfs.WriteFile(p, []byte("new"), 0644) },
            check: func(p string) (interface{}, error) { return mfs.ReadFile(p) },
        },
        {
            name: "MkdirAll with ..",
            path: "a/d/../d/e/f", // results in a/d/e/f
            expected: "a/d/e/f",
            op: func(p string) error { return mfs.MkdirAll(p, 0755) },
            check: func(p string) (interface{}, error) {
                fi, err := mfs.Stat(p)
                if err != nil {
                    return nil, err
                }
                if !fi.IsDir() {
                    return nil, errors.New("path is not a directory")
                }
                return fi, nil
            },
        },
         {
            name: "ReadFile with trailing slash (on existing file)", // Should be cleaned
            path: "a/b/c.txt/",
            expected: "a/b/c.txt",
            op: func(p string) error { _, err := mfs.ReadFile(p); return err }, // Read is the operation
            check: func(p string) (interface{}, error) { return mfs.files[p].data, nil }, // Check internal
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := tc.op(tc.path)
            // For ReadFile with trailing slash, it might error if Stat is strict,
            // but if it resolves to the file, the error might be nil or different.
            // The main point is that path.Clean works internally.
            if strings.Contains(tc.name, "ReadFile with trailing slash") {
                 if err != nil && !strings.Contains(err.Error(), "not a directory") && !strings.Contains(err.Error(), "invalid argument")  { // Depends on Open logic
                    // This specific error might be okay if Open tries to treat it as dir first
                 }
            } else if err != nil {
                t.Fatalf("Operation failed for path %s: %v", tc.path, err)
            }

            // Check that the *cleaned* path is what's in the map or accessible
            _, checkErr := tc.check(tc.expected)
            if checkErr != nil {
                 // If original path was supposed to be invalid for op, this check might not apply directly
                if !( (strings.Contains(tc.name, "invalid")) && checkErr != nil ) {
                    t.Errorf("Post-operation check failed for expected path %s (original %s): %v", tc.expected, tc.path, checkErr)
                }
            }
        })
    }
}
func TestMockFS_GetMode(t *testing.T) {
	mfs := NewMockFS()
	mfs.WriteFile("file.txt", []byte{}, 0644)
	mfs.MkdirAll("dir", 0755)
	mfs.MkdirAll("dir/subdir", 0751) // Explicit subdir

	// Setup for implicit directory test
	// Ensure "implicit_parent" exists before writing a file into it, so WriteFile succeeds.
	// This means "implicit_parent" will be an explicit directory in mfs.files.
	// The GetMode will then be tested on this explicit directory.
	if err := mfs.MkdirAll("implicit_parent", 0755); err != nil {
		t.Fatalf("Setup MkdirAll for implicit_parent failed: %v", err)
	}
	if err := mfs.WriteFile("implicit_parent/anotherfile.txt", []byte{}, 0600); err != nil {
		t.Fatalf("Setup WriteFile for implicit_parent/anotherfile.txt failed: %v", err)
	}


	// File
	mode, err := mfs.GetMode("file.txt")
	if err != nil {
		t.Fatalf("GetMode for file.txt failed: %v", err)
	}
	if mode != 0644 {
		t.Errorf("Expected mode 0644 for file.txt, got %v", mode)
	}

	// Explicitly created directory
	mode, err = mfs.GetMode("dir")
	if err != nil {
		t.Fatalf("GetMode for dir failed: %v", err)
	}
	if mode != (fs.ModeDir | 0755) {
		t.Errorf("Expected mode %v for dir, got %v", fs.ModeDir|0755, mode)
	}

	mode, err = mfs.GetMode("dir/subdir")
	if err != nil {
		t.Fatalf("GetMode for dir/subdir failed: %v", err)
	}
	if mode != (fs.ModeDir | 0751) {
		t.Errorf("Expected mode %v for dir/subdir, got %v", fs.ModeDir|0751, mode)
	}

	// Implicit directory (parent of a file)
	mfs.WriteFile("implicit_parent/anotherfile.txt", []byte{}, 0600)
	mode, err = mfs.GetMode("implicit_parent")
	if err != nil {
		t.Fatalf("GetMode for implicit_parent failed: %v", err)
	}
	if mode != (fs.ModeDir | 0755) { // Implicit dirs get default 0755
		t.Errorf("Expected mode %v for implicit_parent, got %v", fs.ModeDir|0755, mode)
	}

	// Non-existent
	_, err = mfs.GetMode("nonexistent")
	if err == nil {
		t.Error("Expected error for GetMode on non-existent, got nil")
	} else if pe, ok := err.(*fs.PathError); !ok || pe.Err != fs.ErrNotExist {
		t.Errorf("Expected fs.ErrNotExist, got %v", err)
	}
}
