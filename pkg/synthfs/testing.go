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

	// Check if a file (not directory) already exists at this path
	if existing, exists := tfs.MapFS[path]; exists {
		if !existing.Mode.IsDir() {
			// Can't create directory where file exists
			return &fs.PathError{Op: "mkdirall", Path: path, Err: fs.ErrExist}
		}
		// Directory already exists, that's fine for MkdirAll
		return nil
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

// Symlink implements WriteFS for testing
func (tfs *TestFileSystem) Symlink(oldname, newname string) error {
	if !fs.ValidPath(oldname) || !fs.ValidPath(newname) {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrInvalid}
	}

	// Check if target exists
	if _, exists := tfs.MapFS[oldname]; !exists {
		// Unlike real symlinks, for testing we'll require the target to exist
		return &fs.PathError{Op: "symlink", Path: oldname, Err: fs.ErrNotExist}
	}

	// Check if newname already exists
	if _, exists := tfs.MapFS[newname]; exists {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrExist}
	}

	// Create symlink as a special file with ModeSymlink
	tfs.MapFS[newname] = &fstest.MapFile{
		Data: []byte(oldname), // Store target path as data
		Mode: fs.ModeSymlink | 0777,
	}
	return nil
}

// Readlink implements WriteFS for testing
func (tfs *TestFileSystem) Readlink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	file, exists := tfs.MapFS[name]
	if !exists {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}

	if file.Mode&fs.ModeSymlink == 0 {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	return string(file.Data), nil
}

// Rename implements WriteFS for testing
func (tfs *TestFileSystem) Rename(oldpath, newpath string) error {
	if !fs.ValidPath(oldpath) || !fs.ValidPath(newpath) {
		return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrInvalid}
	}

	// Check if source exists
	file, exists := tfs.MapFS[oldpath]
	if !exists {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}

	// Check if destination already exists and is not a directory
	if destFile, destExists := tfs.MapFS[newpath]; destExists {
		if destFile.Mode.IsDir() {
			return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrExist}
		}
		// File exists, will be overwritten (matches os.Rename behavior)
	}

	// Move the file
	tfs.MapFS[newpath] = file
	delete(tfs.MapFS, oldpath)

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
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't fail the operation
			Logger().Warn().Err(closeErr).Msg("failed to close file in Stat")
		}
	}()

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
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			th.t.Logf("Warning: failed to close file %s: %v", path, closeErr)
		}
	}()

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

// RunAndAssert runs a pipeline and asserts it succeeds
func (th *TestHelper) RunAndAssert(pipeline Pipeline) *Result {
	th.t.Helper()

	executor := NewExecutor()
	result := executor.Run(th.ctx, pipeline, th.fs)

	if !result.Success {
		th.t.Fatalf("Expected run to succeed, but it failed with errors: %v", result.Errors)
	}

	return result
}

// RunAndExpectError runs a pipeline and expects it to fail
func (th *TestHelper) RunAndExpectError(pipeline Pipeline) *Result {
	th.t.Helper()

	executor := NewExecutor()
	result := executor.Run(th.ctx, pipeline, th.fs)

	if result.Success {
		th.t.Fatalf("Expected run to fail, but it succeeded")
	}

	return result
}

// ValidateTestFS validates a TestFileSystem using testing/fstest
func ValidateTestFS(t *testing.T, testFS *TestFileSystem) {
	t.Helper()

	// Collect expected files from the filesystem
	expectedFiles := make([]string, 0, len(testFS.MapFS))
	for path := range testFS.MapFS {
		expectedFiles = append(expectedFiles, path)
	}

	// Use testing/fstest to validate the filesystem
	if err := fstest.TestFS(testFS.MapFS, expectedFiles...); err != nil {
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
