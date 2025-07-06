package synthfs

import (
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
	"github.com/arthur-debert/synthfs/pkg/synthfs/validation"
)

// Filesystem interfaces are now in the filesystem package
// Type aliases are provided in types.go for backward compatibility

// OSFileSystem is now a type alias for the filesystem package version
type OSFileSystem = filesystem.OSFileSystem

// NewOSFileSystem creates a new OS-based filesystem rooted at the given path
func NewOSFileSystem(root string) *OSFileSystem {
	return filesystem.NewOSFileSystem(root)
}

// ComputeFileChecksum is now a wrapper for the validation package function
func ComputeFileChecksum(fsys FullFileSystem, filePath string) (*ChecksumRecord, error) {
	return validation.ComputeFileChecksum(fsys, filePath)
}

// ReadOnlyWrapper is now a type alias for the filesystem package version
type ReadOnlyWrapper = filesystem.ReadOnlyWrapper

// NewReadOnlyWrapper creates a new wrapper for an fs.FS
func NewReadOnlyWrapper(fsys ReadFS) *ReadOnlyWrapper {
	return filesystem.NewReadOnlyWrapper(fsys)
}