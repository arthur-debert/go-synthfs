package synthfs

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Filesystem interfaces are defined in types.go

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
	if filepath.IsAbs(target) {
		rel, err := filepath.Rel(osfs.root, target)
		if err == nil && !strings.HasPrefix(rel, "..") {
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

// ComputeFileChecksum calculates the MD5 checksum and gathers file metadata.
// It requires a FullFileSystem to access both file content and metadata.
func ComputeFileChecksum(fsys FullFileSystem, filePath string) (*ChecksumRecord, error) {
	// Get file info first
	info, err := fsys.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	// Skip checksumming for directories
	if info.IsDir() {
		return nil, nil
	}

	// Open file for reading
	file, err := fsys.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s for checksumming: %w", filePath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			Logger().Warn().Err(closeErr).Str("path", filePath).Msg("failed to close file during checksumming")
		}
	}()

	// Calculate MD5 hash
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("failed to calculate checksum for %s: %w", filePath, err)
	}

	// Create checksum record
	checksum := &ChecksumRecord{
		Path:         filePath,
		MD5:          fmt.Sprintf("%x", hash.Sum(nil)),
		Size:         info.Size(),
		ModTime:      info.ModTime(),
		ChecksumTime: time.Now(),
	}

	return checksum, nil
}

// ReadOnlyWrapper wraps an fs.FS to add StatFS capabilities if not already present
type ReadOnlyWrapper struct {
	fs.FS
}

// NewReadOnlyWrapper creates a new wrapper for an fs.FS
func NewReadOnlyWrapper(fsys fs.FS) *ReadOnlyWrapper {
	return &ReadOnlyWrapper{FS: fsys}
}

// Stat implements the StatFS interface
func (w *ReadOnlyWrapper) Stat(name string) (fs.FileInfo, error) {
	// If the underlying FS already implements StatFS, use that
	if statFS, ok := w.FS.(StatFS); ok {
		return statFS.Stat(name)
	}

	// Otherwise, open the file and get its info
	file, err := w.FS.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file.Stat()
}
