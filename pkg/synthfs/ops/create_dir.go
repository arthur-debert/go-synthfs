package ops

import (
	"io/fs"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
)

// NewCreateDirectory creates a new directory creation operation.
// This is a convenience function that creates a DirectoryItem and uses the generic Create() function.
//
// It returns a synthfs.Operation, which can be customized by modifying the returned operation directly.
func NewCreateDirectory(path string, mode fs.FileMode) synthfs.Operation {
	// 1. Create the FsItem representing the directory.
	dirItem := synthfs.NewDirectory(path).
		WithMode(mode)

	// 2. Use the generic Create operation.
	return Create(dirItem)
}
