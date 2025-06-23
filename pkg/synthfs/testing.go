package synthfs

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"
)

// TestFileSystem extends fstest.MapFS to implement our WriteFS interface
// This allows using testing/fstest with synthfs operations
type TestFileSystem struct {
	fstest.MapFS
}

// NewTestFileSystem creates a new test filesystem based on fstest.MapFS
func NewTestFileSystem() *TestFileSystem {
	return &TestFileSystem{
		MapFS: make(fstest.MapFS),
	}
}

// NewTestFileSystemFromMap creates a test filesystem from an existing map
func NewTestFileSystemFromMap(files map[string]*fstest.MapFile) *TestFileSystem {
	return &TestFileSystem{
		MapFS: files,
	}
}

// WriteFile implements WriteFS for testing
func (tfs *TestFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrInvalid}
	}
	tfs.MapFS[name] = &fstest.MapFile{
		Data: data,
		Mode: perm,
	}
	return nil
}

// MkdirAll implements WriteFS for testing
func (tfs *TestFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: "mkdirall", Path: path, Err: fs.ErrInvalid}
	}
	tfs.MapFS[path] = &fstest.MapFile{
		Mode: perm | fs.ModeDir,
	}
	return nil
}

// Remove implements WriteFS for testing
func (tfs *TestFileSystem) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}
	if _, exists := tfs.MapFS[name]; !exists {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	delete(tfs.MapFS, name)
	return nil
}

// RemoveAll implements WriteFS for testing
func (tfs *TestFileSystem) RemoveAll(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "removeall", Path: name, Err: fs.ErrInvalid}
	}
	// Remove the path and all its children
	for path := range tfs.MapFS {
		if path == name || isSubPath(name, path) {
			delete(tfs.MapFS, path)
		}
	}
	return nil
}

// Stat implements StatFS for testing
func (tfs *TestFileSystem) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	// Use Open to get the file and then get its info
	file, err := tfs.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file.Stat()
}

// isSubPath returns true if child is a subpath of parent
func isSubPath(parent, child string) bool {
	if parent == "" || parent == "." {
		return true
	}
	if len(child) <= len(parent) {
		return false
	}
	return child[:len(parent)+1] == parent+"/"
}

// TestHelper provides utilities for testing synthfs operations
type TestHelper struct {
	t   *testing.T
	fs  *TestFileSystem
	ctx context.Context
}

// NewTestHelper creates a new test helper with a fresh filesystem
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{
		t:   t,
		fs:  NewTestFileSystem(),
		ctx: context.Background(),
	}
}

// NewTestHelperWithFiles creates a test helper with predefined files
func NewTestHelperWithFiles(t *testing.T, files map[string]*fstest.MapFile) *TestHelper {
	return &TestHelper{
		t:   t,
		fs:  NewTestFileSystemFromMap(files),
		ctx: context.Background(),
	}
}

// FS returns the test filesystem
func (th *TestHelper) FS() *TestFileSystem {
	return th.fs
}

// Context returns the test context
func (th *TestHelper) Context() context.Context {
	return th.ctx
}

// AssertFileExists verifies that a file exists with optional content check
func (th *TestHelper) AssertFileExists(path string, expectedContent ...[]byte) {
	th.t.Helper()

	file, err := th.fs.Open(path)
	if err != nil {
		th.t.Fatalf("Expected file %s to exist, but got error: %v", path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		th.t.Fatalf("Failed to stat file %s: %v", path, err)
	}

	if info.IsDir() {
		th.t.Fatalf("Expected %s to be a file, but it's a directory", path)
	}

	// Check content if provided
	if len(expectedContent) > 0 {
		mapFile := th.fs.MapFS[path]
		if mapFile == nil {
			th.t.Fatalf("File %s exists but has no content data", path)
		}

		expected := expectedContent[0]
		if string(mapFile.Data) != string(expected) {
			th.t.Errorf("File %s content mismatch.\nExpected: %q\nGot: %q",
				path, string(expected), string(mapFile.Data))
		}
	}
}

// AssertDirExists verifies that a directory exists
func (th *TestHelper) AssertDirExists(path string) {
	th.t.Helper()

	info, err := th.fs.Stat(path)
	if err != nil {
		th.t.Fatalf("Expected directory %s to exist, but got error: %v", path, err)
	}

	if !info.IsDir() {
		th.t.Fatalf("Expected %s to be a directory, but it's a file", path)
	}
}

// AssertNotExists verifies that a path does not exist
func (th *TestHelper) AssertNotExists(path string) {
	th.t.Helper()

	_, err := th.fs.Stat(path)
	if err == nil {
		th.t.Fatalf("Expected %s to not exist, but it does", path)
	}

	if !isNotExistError(err) {
		th.t.Fatalf("Expected %s to not exist, but got unexpected error: %v", path, err)
	}
}

// ExecuteAndAssert executes a queue and asserts it succeeds
func (th *TestHelper) ExecuteAndAssert(queue Queue, opts ...ExecuteOption) *Result {
	th.t.Helper()

	executor := NewExecutor()
	result := executor.Execute(th.ctx, queue, th.fs, opts...)

	if !result.Success {
		th.t.Fatalf("Expected execution to succeed, but it failed with errors: %v", result.Errors)
	}

	return result
}

// ExecuteAndExpectError executes a queue and expects it to fail
func (th *TestHelper) ExecuteAndExpectError(queue Queue, opts ...ExecuteOption) *Result {
	th.t.Helper()

	executor := NewExecutor()
	result := executor.Execute(th.ctx, queue, th.fs, opts...)

	if result.Success {
		th.t.Fatalf("Expected execution to fail, but it succeeded")
	}

	return result
}

// ValidateTestFS validates a TestFileSystem using testing/fstest
func ValidateTestFS(t *testing.T, testFS *TestFileSystem) {
	t.Helper()

	// Use testing/fstest to validate the filesystem
	if err := fstest.TestFS(testFS.MapFS, ""); err != nil {
		t.Fatalf("TestFileSystem validation failed: %v", err)
	}
}

// isNotExistError checks if an error indicates that a file/directory doesn't exist
func isNotExistError(err error) bool {
	if pathErr, ok := err.(*fs.PathError); ok {
		return pathErr.Err == fs.ErrNotExist
	}
	return false
}
