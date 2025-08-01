//go:build !coverage

package testutil

import (
	"fmt"
	"io" // Import standard io for io.EOF
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

// MockFile represents a file in the mock filesystem.
type mockFile struct {
	data    []byte
	mode    fs.FileMode
	modTime time.Time
}

// MockFS is an in-memory implementation of synthfs.FileSystem for testing.
// It is intended for use in tests and is not performance-optimized.
type MockFS struct {
	mu    sync.RWMutex
	files map[string]*mockFile // path -> file
}

// NewMockFS creates a new mock filesystem.
func NewMockFS() *MockFS {
	return &MockFS{
		files: make(map[string]*mockFile),
	}
}

// --- fs.FS (ReadFS) Implementation ---

func (mfs *MockFS) Open(name string) (fs.File, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	f, ok := mfs.files[name]
	if !ok {
		isDir := false
		prefix := name + "/"
		if name == "." {
			isDir = true
		} else {
			for p := range mfs.files {
				if strings.HasPrefix(p, prefix) {
					isDir = true
					break
				}
			}
		}
		if isDir {
			return &mockDirEntry{name: path.Base(name), path: name, mfs: mfs, mode: fs.ModeDir | 0755, modTime: time.Now()}, nil
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	if f.mode.IsDir() {
		return &mockDirEntry{name: path.Base(name), path: name, mfs: mfs, mode: f.mode, modTime: f.modTime}, nil
	}
	return newMockFileHandle(name, f.data, f.mode, f.modTime), nil
}

func (mfs *MockFS) ReadFile(name string) ([]byte, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	name = path.Clean(name)
	f, ok := mfs.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrNotExist}
	}
	if f.mode.IsDir() {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}
	return append([]byte(nil), f.data...), nil
}

func (mfs *MockFS) Stat(name string) (fs.FileInfo, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	name = path.Clean(name)
	f, ok := mfs.files[name]
	if !ok {
		isDir := false
		prefix := name + "/"
		if name == "." {
			isDir = true
		} else {
			for p := range mfs.files {
				if strings.HasPrefix(p, prefix) {
					isDir = true
					break
				}
			}
		}
		if isDir {
			return &mockFileInfo{name: path.Base(name), mode: fs.ModeDir | 0755, modTime: time.Now(), isDir: true}, nil
		}
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return &mockFileInfo{name: path.Base(name), size: int64(len(f.data)), mode: f.mode, modTime: f.modTime, isDir: f.mode.IsDir()}, nil
}

// --- synthfs.WriteFS Implementation ---

func (mfs *MockFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	originalName := name // Keep original name for certain checks if needed, though not used in this revision.
	name = path.Clean(name)

	// fs.ValidPath checks for invalid characters or empty names.
	// path.Clean handles ".." , ".", and trailing slashes.
	// So, after cleaning, "dir/" becomes "dir". "invalid/../path.txt" becomes "path.txt".
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "writefile", Path: originalName, Err: fs.ErrInvalid}
	}
	// If the original path explicitly ended with a slash, it might indicate an attempt to write to a directory.
	// However, standard os.WriteFile("path/to/dir/", data) on Unix might create a file named "dir" if "path/to" exists.
	// For simplicity, we'll rely on the parent dir check. If 'name' refers to an existing directory, it should fail there.

	parent := path.Dir(name)
	if parent != "." && parent != "/" {
		// Ensure parent directory exists for WriteFile to succeed, like a real FS.
		// This mock previously didn't strictly enforce parent existence.
		// For more realistic behavior, we should check.
		if pFile, ok := mfs.files[parent]; !ok || !pFile.mode.IsDir() {
			// More sophisticated check: is parent an implicit dir?
			isImplicitDir := false
			for p := range mfs.files {
				if strings.HasPrefix(p, parent+"/") {
					isImplicitDir = true
					break
				}
			}
			if !isImplicitDir {
				return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrNotExist} // Parent does not exist
			}
		}
	}

	mfs.files[name] = &mockFile{
		data:    append([]byte(nil), data...),
		mode:    perm &^ fs.ModeDir,
		modTime: time.Now(),
	}
	return nil
}

func (mfs *MockFS) MkdirAll(name string, perm fs.FileMode) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "mkdirall", Path: name, Err: fs.ErrInvalid}
	}

	currentPath := ""
	parts := strings.Split(name, "/")
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") { // Handle "." or ""
		if name == "." { // MkdirAll on "." is a no-op if it exists
			if _, ok := mfs.files["."]; !ok {
				mfs.files["."] = &mockFile{mode: fs.ModeDir | (perm & fs.ModePerm), modTime: time.Now()}
			}
			return nil
		}
		// Other empty/invalid path cases might be caught by ValidPath or need specific handling
	}

	for i, part := range parts {
		if part == "" {
			switch currentPath {
			case "": // Leading "/"
				if i == 0 {
					currentPath = "/"
					continue
				}
				// // in path, skip
				continue
			default:
				// // in path, skip
				continue
			}
		}
		switch currentPath {
		case "/":
			currentPath += part
		case "":
			currentPath = part
		default:
			currentPath += "/" + part
		}

		currentPathClean := path.Clean(currentPath)

		if f, ok := mfs.files[currentPathClean]; ok {
			if !f.mode.IsDir() {
				return &fs.PathError{Op: "mkdirall", Path: currentPathClean, Err: syscall.ENOTDIR} // File exists at this path and is not a dir
			}
		} else {
			mfs.files[currentPathClean] = &mockFile{
				mode:    fs.ModeDir | (perm & fs.ModePerm),
				modTime: time.Now(),
			}
		}
	}
	return nil
}

func (mfs *MockFS) Remove(name string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}

	f, ok := mfs.files[name]
	if !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if f.mode.IsDir() {
		if name == "." {
			return fmt.Errorf("cannot remove current directory \".\"")
		}
		prefix := name + "/"
		if name == "/" { // Cannot remove root if not empty
			prefix = "/"
		}

		for p := range mfs.files {
			if p != name && strings.HasPrefix(p, prefix) {
				// For root, check if any file exists other than root itself
				if name == "/" && p == "/" {
					continue
				}
				return &fs.PathError{Op: "remove", Path: name, Err: syscall.ENOTEMPTY}
			}
		}
	}
	delete(mfs.files, name)
	return nil
}

func (mfs *MockFS) RemoveAll(name string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "removeall", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." || name == "/" { // Remove all entries if root is specified
		for k := range mfs.files {
			delete(mfs.files, k)
		}
		// Add back the root if it was "/"
		if name == "/" {
			mfs.files["/"] = &mockFile{mode: fs.ModeDir | 0755, modTime: time.Now()}
		}
		return nil
	}

	// Check if the path itself exists. If not, it's a no-op.
	if _, ok := mfs.files[name]; !ok {
		// It might be an implicit directory. If no files start with its prefix, it's truly non-existent.
		isImplicitOrNonExistent := true
		prefixCheck := name + "/"
		for p := range mfs.files {
			if strings.HasPrefix(p, prefixCheck) {
				isImplicitOrNonExistent = false
				break
			}
		}
		if isImplicitOrNonExistent {
			return nil
		}
	}

	prefix := name + "/"
	delete(mfs.files, name)
	for p := range mfs.files {
		if strings.HasPrefix(p, prefix) {
			delete(mfs.files, p)
		}
	}
	return nil
}

// Symlink implements WriteFS for MockFS
func (mfs *MockFS) Symlink(oldname, newname string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	oldname = path.Clean(oldname)
	newname = path.Clean(newname)

	if !fs.ValidPath(oldname) || !fs.ValidPath(newname) {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrInvalid}
	}

	// Note: Real filesystems allow creating symlinks to non-existent targets (dangling symlinks)
	// We'll allow this in MockFS to match real filesystem behavior

	// Check if newname already exists
	if _, exists := mfs.files[newname]; exists {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrExist}
	}

	// Create symlink as a special file with ModeSymlink
	mfs.files[newname] = &mockFile{
		data:    []byte(oldname), // Store target path as data
		mode:    fs.ModeSymlink | 0777,
		modTime: time.Now(),
	}
	return nil
}

// Readlink implements WriteFS for MockFS
func (mfs *MockFS) Readlink(name string) (string, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	name = path.Clean(name)
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	file, exists := mfs.files[name]
	if !exists {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}

	if file.mode&fs.ModeSymlink == 0 {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	return string(file.data), nil
}

// Rename implements WriteFS for MockFS
func (mfs *MockFS) Rename(oldpath, newpath string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	oldpath = path.Clean(oldpath)
	newpath = path.Clean(newpath)

	if !fs.ValidPath(oldpath) || !fs.ValidPath(newpath) {
		return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrInvalid}
	}

	// Check if source exists
	file, exists := mfs.files[oldpath]
	if !exists {
		return &fs.PathError{Op: "rename", Path: oldpath, Err: fs.ErrNotExist}
	}

	// Check if destination already exists and is a directory
	if destFile, destExists := mfs.files[newpath]; destExists {
		if destFile.mode.IsDir() {
			return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrExist}
		}
		// File exists, will be overwritten (matches os.Rename behavior)
	}

	// Move the file
	mfs.files[newpath] = file
	delete(mfs.files, oldpath)

	return nil
}

// --- fs.File, fs.FileInfo, fs.DirEntry Implementations ---
// These types remain unexported as they are internal to MockFS's implementation.

type mockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *mockFileInfo) Name() string       { return fi.name }
func (fi *mockFileInfo) Size() int64        { return fi.size }
func (fi *mockFileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *mockFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *mockFileInfo) IsDir() bool        { return fi.isDir || fi.mode.IsDir() }
func (fi *mockFileInfo) Sys() interface{}   { return nil }

type mockFileHandle struct {
	info   mockFileInfo
	data   []byte
	offset int64
	// path    string // No longer needed directly here for readdir
	// mfs     *MockFS // No longer needed directly here for readdir
	// readDirNames []string // No longer needed directly here for readdir
	isDir bool
}

func newMockFileHandle(name string, data []byte, mode fs.FileMode, modTime time.Time) *mockFileHandle {
	return &mockFileHandle{
		info: mockFileInfo{
			name:    path.Base(name),
			size:    int64(len(data)),
			mode:    mode,
			modTime: modTime,
			isDir:   mode.IsDir(),
		},
		data: data,
		// path:  name, // Store full path if needed for other methods
		isDir: mode.IsDir(),
	}
}

func (mfh *mockFileHandle) Stat() (fs.FileInfo, error) { return &mfh.info, nil }
func (mfh *mockFileHandle) Read(b []byte) (int, error) {
	if mfh.isDir {
		return 0, &fs.PathError{Op: "read", Path: mfh.info.name, Err: syscall.EISDIR}
	}
	if mfh.offset >= int64(len(mfh.data)) {
		return 0, io.EOF // Use standard io.EOF
	}
	n := copy(b, mfh.data[mfh.offset:])
	mfh.offset += int64(n)
	return n, nil
}
func (mfh *mockFileHandle) Close() error { return nil }

type mockDirEntry struct {
	name          string
	path          string
	mfs           *MockFS
	mode          fs.FileMode
	modTime       time.Time
	readDirOffset int
	entries       []fs.DirEntry
}

func (mde *mockDirEntry) Name() string               { return mde.name }
func (mde *mockDirEntry) IsDir() bool                { return true }
func (mde *mockDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (mde *mockDirEntry) Info() (fs.FileInfo, error) { return mde.Stat() }

func (mde *mockDirEntry) Stat() (fs.FileInfo, error) {
	return &mockFileInfo{name: mde.name, mode: mde.mode, modTime: mde.modTime, isDir: true}, nil
}
func (mde *mockDirEntry) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: mde.path, Err: syscall.EISDIR}
}
func (mde *mockDirEntry) Close() error { return nil }

func (mde *mockDirEntry) ReadDir(n int) ([]fs.DirEntry, error) {
	mde.mfs.mu.RLock()
	defer mde.mfs.mu.RUnlock()

	if mde.entries == nil {
		mde.entries = []fs.DirEntry{}
		directChildren := make(map[string]bool)

		// Normalize base path for comparison
		basePath := path.Clean(mde.path)
		if basePath == "." {
			basePath = ""
		} // Treat "." as root for prefix matching ""
		if basePath != "" && !strings.HasSuffix(basePath, "/") {
			basePath += "/"
		}
		if basePath == "/" {
			basePath = ""
		} // Special case for root, prefix is effectively empty string for direct children

		for p, f := range mde.mfs.files {
			if p == path.Clean(mde.path) {
				continue
			} // Skip the directory itself

			// Check if p is a direct child of basePath
			var entryName string
			if basePath == "" { // Root directory
				if !strings.Contains(p, "/") { // Direct child of root
					entryName = p
				} else {
					continue
				}
			} else {
				if strings.HasPrefix(p, basePath) {
					entryName = strings.TrimPrefix(p, basePath)
					if strings.Contains(entryName, "/") { // Not a direct child
						continue
					}
				} else {
					continue
				}
			}
			if entryName == "" {
				continue
			}

			if _, exists := directChildren[entryName]; !exists {
				directChildren[entryName] = true
				mde.entries = append(mde.entries, &mockDirEntryChild{
					name: entryName,
					mode: f.mode.Type(),
					info: &mockFileInfo{name: entryName, size: int64(len(f.data)), mode: f.mode, modTime: f.modTime, isDir: f.mode.IsDir()},
				})
			}
		}

		// Add implicit directories
		implicitDirs := make(map[string]bool)
		for p := range mde.mfs.files {
			if p == path.Clean(mde.path) {
				continue
			}

			var relPath string
			if basePath == "" { // Root
				relPath = p
			} else if strings.HasPrefix(p, basePath) {
				relPath = strings.TrimPrefix(p, basePath)
			} else {
				continue
			}

			slashIdx := strings.Index(relPath, "/")
			if slashIdx > 0 {
				dirName := relPath[:slashIdx]
				if _, exists := directChildren[dirName]; !exists && !implicitDirs[dirName] {
					implicitDirs[dirName] = true
					mde.entries = append(mde.entries, &mockDirEntryChild{
						name: dirName,
						mode: fs.ModeDir,
						info: &mockFileInfo{name: dirName, mode: fs.ModeDir | 0755, modTime: time.Now(), isDir: true},
					})
				}
			}
		}

		sort.Slice(mde.entries, func(i, j int) bool {
			return mde.entries[i].Name() < mde.entries[j].Name()
		})
	}

	if mde.readDirOffset >= len(mde.entries) {
		if n <= 0 {
			return nil, nil
		}
		return nil, io.EOF // Use standard io.EOF
	}

	end := mde.readDirOffset + n
	if n <= 0 || end > len(mde.entries) {
		end = len(mde.entries)
	}

	batch := mde.entries[mde.readDirOffset:end]
	mde.readDirOffset = end
	return batch, nil
}

type mockDirEntryChild struct {
	name string
	mode fs.FileMode
	info fs.FileInfo
}

func (mdec *mockDirEntryChild) Name() string               { return mdec.name }
func (mdec *mockDirEntryChild) IsDir() bool                { return mdec.mode&fs.ModeDir != 0 }
func (mdec *mockDirEntryChild) Type() fs.FileMode          { return mdec.mode }
func (mdec *mockDirEntryChild) Info() (fs.FileInfo, error) { return mdec.info, nil }

// --- Interface Assurances ---
var _ synthfs.FileSystem = (*MockFS)(nil)
var _ fs.FS = (*MockFS)(nil)
var _ fs.File = (*mockFileHandle)(nil)
var _ fs.File = (*mockDirEntry)(nil)
var _ fs.FileInfo = (*mockFileInfo)(nil)
var _ fs.DirEntry = (*mockDirEntryChild)(nil)
var _ fs.DirEntry = (*mockDirEntry)(nil)
var _ fs.ReadFileFS = (*MockFS)(nil)
var _ fs.StatFS = (*MockFS)(nil)

// Exists checks if a path exists in the MockFS.
func (mfs *MockFS) Exists(name string) bool {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()
	_, ok := mfs.files[path.Clean(name)]
	return ok
}

// GetMode retrieves the mode of a path in the MockFS.
func (mfs *MockFS) GetMode(name string) (fs.FileMode, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()
	cleanName := path.Clean(name)
	f, ok := mfs.files[cleanName]
	if !ok {
		// Check for implicit directory
		prefix := cleanName + "/"
		if cleanName == "." {
			prefix = "./"
		} // Avoid just "/" for "."

		for p := range mfs.files {
			if strings.HasPrefix(p, prefix) {
				return fs.ModeDir | 0755, nil // Implicit directory
			}
		}
		return 0, &fs.PathError{Op: "getmode", Path: name, Err: fs.ErrNotExist}
	}
	return f.mode, nil
}

// This is a temporary placeholder to satisfy the compiler.
// The actual implementation of this function might need to be updated
// to correctly handle the new synthfs.FileSystem interface.
func NewMockFSFrom(fs synthfs.FileSystem) (*MockFS, error) {
	return nil, fmt.Errorf("not implemented")
}
