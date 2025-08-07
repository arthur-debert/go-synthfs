package filesystem

import (
	"io/fs"
)

// ReadFS is an alias for fs.FS, representing a read-only file system.
type ReadFS = fs.FS

// WriteFS defines the interface for write operations on a file system.
type WriteFS interface {
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(name string) error
	RemoveAll(name string) error
	Symlink(oldname, newname string) error
	Readlink(name string) (string, error)
	Rename(oldpath, newname string) error
}

// FileSystem is the unified filesystem interface that provides all operations.
// This consolidates the previously separate interfaces into a single, comprehensive interface.
type FileSystem interface {
	fs.FS                                    // Read operations (Open, etc.)
	Stat(name string) (fs.FileInfo, error)  // Stat operations
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(name string) error
	RemoveAll(name string) error
	Symlink(oldname, newname string) error
	Readlink(name string) (string, error)
	Rename(oldpath, newpath string) error
}

// Legacy type aliases for backward compatibility - these will be removed in Phase 2
type StatFS = FileSystem
type FullFileSystem = FileSystem
