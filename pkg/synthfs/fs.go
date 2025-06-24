package synthfs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// ReadFS is an alias for fs.FS, representing a read-only file system.
type ReadFS = fs.FS

// WriteFS defines the interface for write operations on a file system.
type WriteFS interface {
	// WriteFile writes data to a file named name.
	// If the file does not exist, WriteFile creates it with permissions perm;
	// otherwise WriteFile truncates it before writing.
	WriteFile(name string, data []byte, perm fs.FileMode) error

	// MkdirAll creates a directory named path,
	// along with any necessary parents, and returns nil,
	// or else returns an error.
	// The permission bits perm (before umask) are used for all
	// directories that MkdirAll creates.
	MkdirAll(path string, perm fs.FileMode) error

	// Remove removes the named file or (empty) directory.
	Remove(name string) error

	// RemoveAll removes path and any children it contains.
	// It removes everything it can but returns the first error
	// it encounters. If the path does not exist, RemoveAll
	// returns nil (no error).
	RemoveAll(name string) error

	// Symlink creates newname as a symbolic link to oldname.
	// On Windows, a symlink to a non-existent oldname creates a file symlink;
	// if oldname is later created as a directory, the symlink will not work.
	// If there is an error, it will be of type *LinkError.
	Symlink(oldname, newname string) error

	// Readlink returns the destination of the named symbolic link.
	// If there is an error, it will be of type *PathError.
	Readlink(name string) (string, error)

	// Rename renames (moves) oldpath to newpath.
	// If newpath already exists and is not a directory, Rename replaces it.
	// OS-specific restrictions may apply when oldpath and newpath are in different directories.
	// If there is an error, it will be of type *LinkError.
	Rename(oldpath, newpath string) error
}

// FileSystem combines read and write operations.
type FileSystem interface {
	ReadFS
	WriteFS
}

// StatFS extends ReadFS with Stat capabilities for better io/fs compatibility
type StatFS interface {
	ReadFS
	Stat(name string) (fs.FileInfo, error)
}

// FullFileSystem provides the complete filesystem interface including Stat
type FullFileSystem interface {
	FileSystem
	Stat(name string) (fs.FileInfo, error)
}

// OSFileSystem implements FullFileSystem using the OS filesystem
type OSFileSystem struct {
	root string
}

// NewOSFileSystem creates a new OS-based filesystem rooted at the given path
func NewOSFileSystem(root string) *OSFileSystem {
	return &OSFileSystem{root: root}
}

// Open implements fs.FS
func (osfs *OSFileSystem) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	fullPath := filepath.Join(osfs.root, name)
	return os.Open(fullPath)
}

// Stat implements StatFS
func (osfs *OSFileSystem) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	fullPath := filepath.Join(osfs.root, name)
	return os.Stat(fullPath)
}

// WriteFile implements WriteFS
func (osfs *OSFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "writefile", Path: name, Err: fs.ErrInvalid}
	}
	fullPath := filepath.Join(osfs.root, name)
	return os.WriteFile(fullPath, data, perm)
}

// MkdirAll implements WriteFS
func (osfs *OSFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: "mkdirall", Path: path, Err: fs.ErrInvalid}
	}
	fullPath := filepath.Join(osfs.root, path)
	return os.MkdirAll(fullPath, perm)
}

// Remove implements WriteFS
func (osfs *OSFileSystem) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}
	fullPath := filepath.Join(osfs.root, name)
	return os.Remove(fullPath)
}

// RemoveAll implements WriteFS
func (osfs *OSFileSystem) RemoveAll(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "removeall", Path: name, Err: fs.ErrInvalid}
	}
	fullPath := filepath.Join(osfs.root, name)
	return os.RemoveAll(fullPath)
}

// Symlink implements WriteFS
func (osfs *OSFileSystem) Symlink(oldname, newname string) error {
	if !fs.ValidPath(oldname) || !fs.ValidPath(newname) {
		return &fs.PathError{Op: "symlink", Path: newname, Err: fs.ErrInvalid}
	}
	oldPath := filepath.Join(osfs.root, oldname)
	newPath := filepath.Join(osfs.root, newname)
	return os.Symlink(oldPath, newPath)
}

// Readlink implements WriteFS
func (osfs *OSFileSystem) Readlink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}
	fullPath := filepath.Join(osfs.root, name)
	target, err := os.Readlink(fullPath)
	if err != nil {
		return "", err
	}
	// Convert absolute path back to relative if it's within our root
	if filepath.IsAbs(target) && filepath.HasPrefix(target, osfs.root) {
		rel, err := filepath.Rel(osfs.root, target)
		if err == nil {
			return rel, nil
		}
	}
	return target, nil
}

// Rename implements WriteFS
func (osfs *OSFileSystem) Rename(oldpath, newpath string) error {
	if !fs.ValidPath(oldpath) || !fs.ValidPath(newpath) {
		return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrInvalid}
	}
	oldFullPath := filepath.Join(osfs.root, oldpath)
	newFullPath := filepath.Join(osfs.root, newpath)
	return os.Rename(oldFullPath, newFullPath)
}

// ReadOnlyWrapper wraps any fs.FS to provide StatFS functionality if possible
type ReadOnlyWrapper struct {
	fs.FS
}

// NewReadOnlyWrapper creates a wrapper around any fs.FS
func NewReadOnlyWrapper(fsys fs.FS) *ReadOnlyWrapper {
	return &ReadOnlyWrapper{FS: fsys}
}

// Stat implements StatFS if the underlying filesystem supports fs.StatFS
func (w *ReadOnlyWrapper) Stat(name string) (fs.FileInfo, error) {
	if statFS, ok := w.FS.(fs.StatFS); ok {
		return statFS.Stat(name)
	}
	// Fallback: try to open and get file info
	file, err := w.FS.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}
