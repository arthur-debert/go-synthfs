// Package testutil provides test utilities for synthfs
package testutil

import (
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// OperationsMockFS is a lightweight mock filesystem for testing operations.
// It implements the filesystem.FileSystem interface with proper fs.FileMode parameters.
// This version now includes basic Symlink/Readlink support for full interface compliance.
type OperationsMockFS struct {
	files    map[string][]byte
	dirs     map[string]bool
	symlinks map[string]string
}

// Files returns the internal files map for testing assertions
func (m *OperationsMockFS) Files() map[string][]byte {
	return m.files
}

// Dirs returns the internal dirs map for testing assertions
func (m *OperationsMockFS) Dirs() map[string]bool {
	return m.dirs
}

// Symlinks returns the internal symlinks map for testing assertions
func (m *OperationsMockFS) Symlinks() map[string]string {
	return m.symlinks
}

// NewOperationsMockFS creates a new lightweight mock filesystem
func NewOperationsMockFS() *OperationsMockFS {
	return &OperationsMockFS{
		files:    make(map[string][]byte),
		dirs:     make(map[string]bool),
		symlinks: make(map[string]string),
	}
}

// WriteFile implements the filesystem.FileSystem WriteFile with fs.FileMode perm
func (m *OperationsMockFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	m.files[name] = data
	return nil
}

// MkdirAll implements the filesystem.FileSystem MkdirAll with fs.FileMode perm
func (m *OperationsMockFS) MkdirAll(path string, perm fs.FileMode) error {
	// Create all parent directories too
	parts := strings.Split(path, "/")
	for i := 1; i <= len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		if dir != "" {
			m.dirs[dir] = true
		}
	}
	return nil
}

// Remove removes a file or empty directory
func (m *OperationsMockFS) Remove(name string) error {
	delete(m.files, name)
	delete(m.dirs, name)
	delete(m.symlinks, name)
	return nil
}

// RemoveAll removes a path and all its children
func (m *OperationsMockFS) RemoveAll(path string) error {
	// Remove the path and all its children
	delete(m.dirs, path)
	delete(m.files, path)
	delete(m.symlinks, path)

	// Remove all children
	prefix := path + "/"
	for p := range m.files {
		if strings.HasPrefix(p, prefix) {
			delete(m.files, p)
		}
	}
	for p := range m.dirs {
		if strings.HasPrefix(p, prefix) {
			delete(m.dirs, p)
		}
	}
	for p := range m.symlinks {
		if strings.HasPrefix(p, prefix) {
			delete(m.symlinks, p)
		}
	}
	return nil
}

// Stat returns file info implementing filesystem.FileSystem interface
func (m *OperationsMockFS) Stat(name string) (fs.FileInfo, error) {
	if _, ok := m.files[name]; ok {
		return &opsMockFileInfo{name: name, size: int64(len(m.files[name]))}, nil
	}
	if _, ok := m.dirs[name]; ok {
		return &opsMockFileInfo{name: name, isDir: true}, nil
	}
	if _, ok := m.symlinks[name]; ok {
		return &opsMockFileInfo{name: name, mode: fs.ModeSymlink}, nil
	}
	return nil, fs.ErrNotExist
}

// Open returns a file handle implementing filesystem.FileSystem interface
func (m *OperationsMockFS) Open(name string) (fs.File, error) {
	if content, ok := m.files[name]; ok {
		return &opsMockFile{Reader: bytes.NewReader(content), name: name}, nil
	}
	return nil, fs.ErrNotExist
}

// Rename moves a file or directory
func (m *OperationsMockFS) Rename(oldpath, newpath string) error {
	// Check if source exists
	if content, ok := m.files[oldpath]; ok {
		// It's a file
		m.files[newpath] = content
		delete(m.files, oldpath)
		return nil
	}
	if target, ok := m.symlinks[oldpath]; ok {
		// It's a symlink
		m.symlinks[newpath] = target
		delete(m.symlinks, oldpath)
		return nil
	}
	if _, ok := m.dirs[oldpath]; ok {
		// It's a directory
		m.dirs[newpath] = true
		delete(m.dirs, oldpath)

		// Move all children
		oldPrefix := oldpath + "/"
		newPrefix := newpath + "/"

		// Collect paths to rename (can't modify map while iterating)
		var filesToRename []struct{ old, new string }
		var dirsToRename []struct{ old, new string }
		var symlinksToRename []struct{ old, new string }

		for path := range m.files {
			if strings.HasPrefix(path, oldPrefix) {
				newPath := newPrefix + strings.TrimPrefix(path, oldPrefix)
				filesToRename = append(filesToRename, struct{ old, new string }{path, newPath})
			}
		}

		for path := range m.dirs {
			if strings.HasPrefix(path, oldPrefix) {
				newPath := newPrefix + strings.TrimPrefix(path, oldPrefix)
				dirsToRename = append(dirsToRename, struct{ old, new string }{path, newPath})
			}
		}

		for path := range m.symlinks {
			if strings.HasPrefix(path, oldPrefix) {
				newPath := newPrefix + strings.TrimPrefix(path, oldPrefix)
				symlinksToRename = append(symlinksToRename, struct{ old, new string }{path, newPath})
			}
		}

		// Apply renames
		for _, r := range filesToRename {
			m.files[r.new] = m.files[r.old]
			delete(m.files, r.old)
		}
		for _, r := range dirsToRename {
			m.dirs[r.new] = true
			delete(m.dirs, r.old)
		}
		for _, r := range symlinksToRename {
			m.symlinks[r.new] = m.symlinks[r.old]
			delete(m.symlinks, r.old)
		}

		return nil
	}
	return fs.ErrNotExist
}

// Symlink creates a symbolic link
func (m *OperationsMockFS) Symlink(oldname, newname string) error {
	m.symlinks[newname] = oldname
	return nil
}

// Readlink reads a symbolic link
func (m *OperationsMockFS) Readlink(name string) (string, error) {
	if target, ok := m.symlinks[name]; ok {
		return target, nil
	}
	return "", errors.New("not a symlink")
}

// OperationsMockFSWithSymlink extends OperationsMockFS with Symlink/Readlink support
type OperationsMockFSWithSymlink struct {
	*OperationsMockFS
}

// NewOperationsMockFSWithSymlink creates a mock filesystem with symlink support
func NewOperationsMockFSWithSymlink() *OperationsMockFSWithSymlink {
	return &OperationsMockFSWithSymlink{
		OperationsMockFS: NewOperationsMockFS(),
	}
}

// Symlink creates a symbolic link
func (m *OperationsMockFSWithSymlink) Symlink(oldname, newname string) error {
	// For simplicity, store symlinks as files with special marker
	m.files[newname] = []byte("SYMLINK:" + oldname)
	return nil
}

// Readlink reads a symbolic link
func (m *OperationsMockFSWithSymlink) Readlink(name string) (string, error) {
	if content, ok := m.files[name]; ok {
		if strings.HasPrefix(string(content), "SYMLINK:") {
			return strings.TrimPrefix(string(content), "SYMLINK:"), nil
		}
	}
	return "", errors.New("not a symlink")
}

// OperationsMockFSWithReadDir extends OperationsMockFS with ReadDir support
type OperationsMockFSWithReadDir struct {
	*OperationsMockFS
}

// NewOperationsMockFSWithReadDir creates a mock filesystem with ReadDir support
func NewOperationsMockFSWithReadDir() *OperationsMockFSWithReadDir {
	return &OperationsMockFSWithReadDir{
		OperationsMockFS: NewOperationsMockFS(),
	}
}

// ReadDir reads a directory's contents
func (m *OperationsMockFSWithReadDir) ReadDir(name string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry

	// Check if the directory exists
	if _, ok := m.dirs[name]; !ok {
		// Check if it's the parent of any files
		hasChildren := false
		for path := range m.files {
			if dir := filepath.Dir(path); dir == name {
				hasChildren = true
				break
			}
		}
		for path := range m.dirs {
			if dir := filepath.Dir(path); dir == name {
				hasChildren = true
				break
			}
		}
		if !hasChildren {
			return nil, fs.ErrNotExist
		}
	}

	// Collect direct children
	children := make(map[string]bool)

	// Add files
	for path := range m.files {
		if dir := filepath.Dir(path); dir == name {
			base := filepath.Base(path)
			if !children[base] {
				children[base] = true
				entries = append(entries, &opsMockDirEntry{
					name:  base,
					isDir: false,
					info:  &opsMockFileInfo{name: base, size: int64(len(m.files[path]))},
				})
			}
		}
	}

	// Add subdirectories
	for path := range m.dirs {
		if dir := filepath.Dir(path); dir == name {
			base := filepath.Base(path)
			if !children[base] {
				children[base] = true
				entries = append(entries, &opsMockDirEntry{
					name:  base,
					isDir: true,
					info:  &opsMockFileInfo{name: base, isDir: true},
				})
			}
		}
	}

	return entries, nil
}

// opsMockFileInfo implements fs.FileInfo for operations mocks
type opsMockFileInfo struct {
	name  string
	size  int64
	isDir bool
	mode  fs.FileMode
}

func (m *opsMockFileInfo) Name() string { return m.name }
func (m *opsMockFileInfo) Size() int64  { return m.size }
func (m *opsMockFileInfo) Mode() fs.FileMode {
	if m.mode != 0 {
		return m.mode
	}
	if m.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (m *opsMockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *opsMockFileInfo) IsDir() bool        { return m.isDir }
func (m *opsMockFileInfo) Sys() interface{}   { return nil }

// opsMockFile implements fs.File for basic file reading
type opsMockFile struct {
	*bytes.Reader
	name string
}

func (m *opsMockFile) Close() error { return nil }
func (m *opsMockFile) Stat() (fs.FileInfo, error) {
	return &opsMockFileInfo{name: m.name, size: int64(m.Len())}, nil
}

// opsMockDirEntry implements fs.DirEntry
type opsMockDirEntry struct {
	name  string
	isDir bool
	info  fs.FileInfo
}

func (m *opsMockDirEntry) Name() string               { return m.name }
func (m *opsMockDirEntry) IsDir() bool                { return m.isDir }
func (m *opsMockDirEntry) Type() fs.FileMode          { return m.info.Mode().Type() }
func (m *opsMockDirEntry) Info() (fs.FileInfo, error) { return m.info, nil }
